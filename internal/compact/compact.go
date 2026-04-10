package compact

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/blueberrycongee/wuu/internal/providers"
)

// EstimateTokens provides a rough token count estimate.
// English: ~4 chars per token. CJK: ~2 chars per token.
// This is for display only; API returns precise counts.
func EstimateTokens(text string) int {
	if text == "" {
		return 0
	}

	cjkCount := 0
	totalChars := utf8.RuneCountInString(text)
	for _, r := range text {
		if isCJK(r) {
			cjkCount++
		}
	}

	nonCJK := totalChars - cjkCount
	return (nonCJK / 4) + (cjkCount / 2) + 1
}

// EstimateMessagesTokens estimates total tokens for a message list.
func EstimateMessagesTokens(messages []providers.ChatMessage) int {
	total := 0
	for _, msg := range messages {
		total += EstimateTokens(msg.Content)
		total += 4 // per-message overhead (role, separators)
		for _, tc := range msg.ToolCalls {
			total += EstimateTokens(tc.Name)
			total += EstimateTokens(tc.Arguments)
		}
	}
	return total
}

// ShouldCompact returns true if messages exceed the threshold.
func ShouldCompact(messages []providers.ChatMessage, maxContextTokens int) bool {
	if maxContextTokens <= 0 {
		return false
	}
	estimated := EstimateMessagesTokens(messages)
	threshold := int(float64(maxContextTokens) * 0.8)
	return estimated > threshold
}

// maxCompactRetries caps how many times Compact will defensively trim
// the oldest message and re-issue the summarization request after
// hitting a context-overflow on the compact request itself. Aligned
// with Codex CLI's safeguard.
const maxCompactRetries = 3

// Compact compresses older messages into a summary. It finds an
// appropriate boundary near the end of the conversation, summarizes
// everything before it, and returns the compacted message list.
//
// Defensive trimming: if the summarization request itself overflows
// the model's context window (because the conversation being
// compacted is itself enormous), Compact drops the oldest entry from
// the to-be-summarized slice and retries up to maxCompactRetries
// times. This prevents the "compact → overflow → compact again →
// overflow again" deadlock the simple form is vulnerable to.
func Compact(ctx context.Context, messages []providers.ChatMessage, client providers.Client, model string) ([]providers.ChatMessage, error) {
	if len(messages) <= 2 {
		return messages, nil // nothing to compact
	}

	// Find compaction boundary: keep the last 2 exchanges (4 messages)
	keepCount := 4
	if keepCount >= len(messages) {
		return messages, nil
	}

	toSummarize := messages[:len(messages)-keepCount]
	toKeep := messages[len(messages)-keepCount:]

	for attempt := 0; ; attempt++ {
		summaryInput := buildSummaryPrompt(toSummarize)
		summaryReq := providers.ChatRequest{
			Model: model,
			Messages: []providers.ChatMessage{
				{Role: "user", Content: summaryInput},
			},
			Temperature: 0.3,
		}

		resp, err := client.Chat(ctx, summaryReq)
		if err != nil {
			// If the summary request itself overflowed the model's
			// context window, drop the oldest message from the slice
			// being summarized and try again. This is the "compact-
			// of-compact" backstop borrowed from Codex CLI.
			if providers.IsContextOverflow(err) && attempt < maxCompactRetries && len(toSummarize) > 1 {
				toSummarize = toSummarize[1:]
				continue
			}
			return messages, fmt.Errorf("compact summary failed: %w", err)
		}

		summary := strings.TrimSpace(resp.Content)
		if summary == "" {
			return messages, nil
		}

		compacted := []providers.ChatMessage{
			{Role: "system", Content: fmt.Sprintf("[Conversation summary]\n%s", summary)},
		}
		compacted = append(compacted, toKeep...)
		return compacted, nil
	}
}

// compactInstructionPrompt is the framing wuu wraps every
// summarization request in. Lifted in spirit from Codex CLI's
// compact/prompt.md: short, model-agnostic, focused on a clean
// hand-off to "another LLM that will resume the task". Deliberately
// avoids Claude-specific XML structures so it works equally well on
// GPT, DeepSeek, Gemini, and any third-party-routed model.
const compactInstructionPrompt = `You are performing a CONTEXT CHECKPOINT COMPACTION for an AI coding agent.
Create a handoff summary that another LLM will read to resume the task.

Include, in roughly this order:
- The user's goal and any explicit instructions or preferences
- Key decisions made and the reasoning behind them
- Files inspected or modified, with paths and the gist of the change
- Errors encountered and how they were resolved (or are still open)
- Outstanding work and clear next steps
- Any concrete data, examples, function signatures, or references the
  next agent will need to continue without re-reading the history

Be concrete and structured. Prefer bullet points over prose. Keep it
focused — the goal is to let the next agent pick up exactly where you
left off, not to retell the entire conversation.

--- Conversation to summarize ---

`

// buildSummaryPrompt is the inner formatting helper extracted so the
// retry loop above doesn't have to duplicate the string-builder code.
func buildSummaryPrompt(toSummarize []providers.ChatMessage) string {
	var b strings.Builder
	b.WriteString(compactInstructionPrompt)
	for _, msg := range toSummarize {
		fmt.Fprintf(&b, "[%s]: %s\n", msg.Role, truncate(msg.Content, 500))
		for _, tc := range msg.ToolCalls {
			fmt.Fprintf(&b, "  -> tool_call: %s(%s)\n", tc.Name, truncate(tc.Arguments, 200))
		}
		if msg.ToolCallID != "" {
			fmt.Fprintf(&b, "  (result for tool call %s)\n", msg.ToolCallID)
		}
		b.WriteString("\n")
	}
	return b.String()
}

func isCJK(r rune) bool {
	return (r >= 0x4E00 && r <= 0x9FFF) || // CJK Unified Ideographs
		(r >= 0x3400 && r <= 0x4DBF) || // CJK Extension A
		(r >= 0x3000 && r <= 0x303F) || // CJK Symbols
		(r >= 0x3040 && r <= 0x309F) || // Hiragana
		(r >= 0x30A0 && r <= 0x30FF) || // Katakana
		(r >= 0xAC00 && r <= 0xD7AF) // Hangul
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

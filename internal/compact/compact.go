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

// Compact compresses older messages into a summary.
// It finds the oldest assistant message as the compaction boundary,
// summarizes everything before it, and returns the compacted message list.
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

	// Build summary prompt
	var summaryInput strings.Builder
	summaryInput.WriteString("Summarize the following conversation concisely, preserving key decisions, code changes, and context:\n\n")
	for _, msg := range toSummarize {
		summaryInput.WriteString(fmt.Sprintf("[%s]: %s\n", msg.Role, truncate(msg.Content, 500)))
		for _, tc := range msg.ToolCalls {
			summaryInput.WriteString(fmt.Sprintf("  -> tool_call: %s(%s)\n", tc.Name, truncate(tc.Arguments, 200)))
		}
		if msg.ToolCallID != "" {
			summaryInput.WriteString(fmt.Sprintf("  (result for tool call %s)\n", msg.ToolCallID))
		}
		summaryInput.WriteString("\n")
	}

	summaryReq := providers.ChatRequest{
		Model: model,
		Messages: []providers.ChatMessage{
			{Role: "user", Content: summaryInput.String()},
		},
		Temperature: 0.3,
	}

	resp, err := client.Chat(ctx, summaryReq)
	if err != nil {
		return messages, fmt.Errorf("compact summary failed: %w", err)
	}

	summary := strings.TrimSpace(resp.Content)
	if summary == "" {
		return messages, nil
	}

	// Build compacted message list
	compacted := []providers.ChatMessage{
		{Role: "system", Content: fmt.Sprintf("[Conversation summary]\n%s", summary)},
	}
	compacted = append(compacted, toKeep...)

	return compacted, nil
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

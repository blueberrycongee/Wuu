package notecompaction

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	pluginapi "github.com/blueberrycongee/wuu/packages/plugin-go"
)

const (
	capabilityPreStep      = "agent.pre_step"
	capabilitySystemPrompt = "agent.system_prompt.section"
	capabilityCompaction   = "agent.compaction"
	toolWriteContextNote   = "write_context_note"

	checkpointReminderID     = "checkpoint-reminder"
	checkpointReminderMarker = "[experimental-context-note-request]"
	conversationSummaryMark  = "[Conversation summary]"

	defaultCheckpointIntervalTokens = 12_000
	minimumCheckpointIntervalTokens = 2_000
	maximumCheckpointIntervalTokens = 100_000
	maxContextNoteBytes             = 24_000
)

const systemPrompt = `Experimental context-note compaction is active.

This plugin may add a hidden message beginning with [experimental-context-note-request]. Only when that reminder appears, call this plugin's context-note checkpoint tool once before doing more task work. Write a self-contained note for a capable agent that will not see the earlier transcript. Preserve the user's objective and constraints, decisions and rationale, current implementation state, files and symbols involved, completed edits, commands and their results, external effects, blockers, and concrete next steps. Clearly distinguish verified facts from assumptions and never claim a check was run when it was not. After the tool succeeds, continue the task normally.

This mechanism is experimental. Do not create checkpoints on every turn and do not treat a checkpoint as durable user memory.`

type noteArguments struct {
	Note string `json:"note"`
}

type rawCompactionInput struct {
	Messages []json.RawMessage `json:"messages"`
}

type rawCompactionOutput struct {
	Messages    []json.RawMessage `json:"messages"`
	Unavailable bool              `json:"unavailable,omitempty"`
}

// compactionMessageView mirrors only the public fields the experiment needs.
// The output always reuses the original json.RawMessage so unknown provider
// fields survive the round trip unchanged.
type compactionMessageView struct {
	Role            string
	Content         string
	Hidden          bool
	OriginID        string
	ToolCallID      string
	ToolCalls       []compactionToolCall
	DiscoveredTools []json.RawMessage
}

type compactionToolCall struct {
	ID        string `json:"id,omitempty"`
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

type checkpoint struct {
	messageIndex int
	note         string
}

func Handler() pluginapi.Handler {
	return pluginapi.Handler{
		Definition: pluginapi.Definition{
			Tools: []pluginapi.Tool{{
				ID:          toolWriteContextNote,
				Description: "Record a self-contained continuation checkpoint when the experimental context-note reminder explicitly asks for one. Do not call on ordinary turns.",
				InputSchema: map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"properties": map[string]any{
						"note": map[string]any{
							"type":        "string",
							"minLength":   1,
							"maxLength":   maxContextNoteBytes,
							"description": "A self-contained continuation note with objective, constraints, decisions, current state, verification, blockers, and next steps.",
						},
					},
					"required": []string{"note"},
				},
				Activity: &pluginapi.ToolActivity{ConcurrencySafe: true, Risk: "low", Reason: "Records model-authored continuation context in the current transcript."},
			}},
			Capabilities: []pluginapi.Capability{
				{ID: capabilitySystemPrompt, Kind: "transform", Version: 1, Priority: 10},
				{ID: capabilityPreStep, Kind: "transform", Version: 1, Priority: 10},
				{ID: capabilityCompaction, Kind: "decision", Version: 1, Priority: 100},
			},
			RequiredHostServices: []pluginapi.HostService{{ID: pluginapi.HostServiceSettingsGet, Required: true}},
		},
		ExecuteTool: executeTool,
		InvokeCapability: func(ctx context.Context, host pluginapi.Host, call pluginapi.CapabilityCall) (json.RawMessage, error) {
			switch call.Capability {
			case capabilitySystemPrompt:
				return json.Marshal(map[string]string{"text": systemPrompt})
			case capabilityPreStep:
				return preStep(ctx, host, call.Input)
			case capabilityCompaction:
				return compactFromLatestNote(call.Input)
			default:
				return nil, fmt.Errorf("unknown note compaction capability %q", call.Capability)
			}
		},
	}
}

func executeTool(_ context.Context, _ pluginapi.Host, call pluginapi.ToolCall) (pluginapi.ToolResult, error) {
	if call.ToolID != toolWriteContextNote {
		return pluginapi.ToolResult{}, fmt.Errorf("unknown note compaction tool %q", call.ToolID)
	}
	var arguments noteArguments
	if err := json.Unmarshal(call.Arguments, &arguments); err != nil {
		return pluginapi.ToolResult{}, fmt.Errorf("decode context note: %w", err)
	}
	note := strings.TrimSpace(arguments.Note)
	if note == "" {
		return pluginapi.ToolResult{}, errors.New("context note must not be empty")
	}
	if len([]byte(note)) > maxContextNoteBytes {
		return pluginapi.ToolResult{}, fmt.Errorf("context note exceeds %d bytes", maxContextNoteBytes)
	}
	return pluginapi.TextResult(fmt.Sprintf("Experimental context checkpoint recorded (%d bytes).", len([]byte(note)))), nil
}

func preStep(ctx context.Context, host pluginapi.Host, raw json.RawMessage) (json.RawMessage, error) {
	var input pluginapi.AgentPreStepInput
	if err := json.Unmarshal(raw, &input); err != nil {
		return nil, fmt.Errorf("decode pre-step input: %w", err)
	}
	start := latestNoteViewIndex(input.Messages) + 1
	if hasPendingReminder(input.Messages[start:]) {
		return json.Marshal(pluginapi.AgentPreStepOutput{})
	}
	estimated := estimateViewTokens(input.Messages[start:])
	interval := checkpointInterval(ctx, host)
	if estimated < interval {
		return json.Marshal(pluginapi.AgentPreStepOutput{})
	}
	content := fmt.Sprintf(`%s

Approximately %d transcript tokens have accumulated since the latest context checkpoint. Before doing more task work, call the experimental context-note checkpoint tool once. Write a self-contained continuation note for an agent that will not receive the earlier transcript, then continue normally.`, checkpointReminderMarker, estimated)
	return json.Marshal(pluginapi.AgentPreStepOutput{AppendMessages: []pluginapi.AgentPreStepMessage{{
		ID: checkpointReminderID, Content: content,
	}}})
}

func checkpointInterval(ctx context.Context, host pluginapi.Host) int {
	interval := defaultCheckpointIntervalTokens
	if host == nil {
		return interval
	}
	var result struct {
		Value any `json:"value"`
	}
	if err := host.CallHost(ctx, pluginapi.HostServiceSettingsGet, map[string]string{"key": "checkpoint_interval_tokens"}, &result); err != nil {
		return interval
	}
	switch value := result.Value.(type) {
	case float64:
		interval = int(value)
	case string:
		if parsed, err := strconv.Atoi(strings.TrimSpace(value)); err == nil {
			interval = parsed
		}
	}
	if interval < minimumCheckpointIntervalTokens {
		return minimumCheckpointIntervalTokens
	}
	if interval > maximumCheckpointIntervalTokens {
		return maximumCheckpointIntervalTokens
	}
	return interval
}

func latestNoteViewIndex(messages []pluginapi.ModelMessageViewV1) int {
	latest := -1
	for index, message := range messages {
		if strings.HasPrefix(strings.TrimSpace(message.Content), conversationSummaryMark) {
			latest = index
		}
		for _, call := range message.ToolCalls {
			if isContextNoteTool(call.Name) && validNoteArguments(call.Arguments) {
				latest = index
			}
		}
	}
	return latest
}

func hasPendingReminder(messages []pluginapi.ModelMessageViewV1) bool {
	for _, message := range messages {
		if strings.Contains(message.Content, checkpointReminderMarker) || strings.HasSuffix(message.OriginID, ":"+checkpointReminderID) {
			return true
		}
	}
	return false
}

func estimateViewTokens(messages []pluginapi.ModelMessageViewV1) int {
	total := 0
	for _, message := range messages {
		total += estimateTextTokens(message.Content) + 8
		for _, call := range message.ToolCalls {
			total += estimateTextTokens(call.Name) + estimateTextTokens(call.Arguments) + 16
		}
		if message.HasToolResult {
			// The v1 pre-step projection intentionally hides raw tool results. Use a
			// conservative floor so tool-heavy runs still request checkpoints.
			total += 512
		}
		if message.HasImages || message.HasFiles {
			total += 1_000
		}
	}
	return total
}

func estimateTextTokens(text string) int {
	if text == "" {
		return 0
	}
	runes := 0
	cjk := 0
	for _, r := range text {
		runes++
		if (r >= 0x3400 && r <= 0x9fff) || (r >= 0xf900 && r <= 0xfaff) || (r >= 0x3040 && r <= 0x30ff) || (r >= 0xac00 && r <= 0xd7af) {
			cjk++
		}
	}
	return (runes-cjk+3)/4 + (cjk+1)/2
}

func compactFromLatestNote(raw json.RawMessage) (json.RawMessage, error) {
	var input rawCompactionInput
	if err := json.Unmarshal(raw, &input); err != nil {
		return nil, fmt.Errorf("decode compaction input: %w", err)
	}
	if len(input.Messages) == 0 {
		return json.Marshal(rawCompactionOutput{})
	}
	views := make([]compactionMessageView, len(input.Messages))
	for index, message := range input.Messages {
		if err := json.Unmarshal(message, &views[index]); err != nil {
			return nil, fmt.Errorf("decode compaction message %d: %w", index, err)
		}
	}

	systemPrefix := preservedSystemPrefix(input.Messages, views)
	latest, ok := latestCheckpoint(views)
	if !ok {
		// Without a model-authored checkpoint there is nothing trustworthy
		// to replace the older context with. Declare the strategy
		// unavailable so the host falls back to the default compactor
		// instead of collapsing the history into lossy transcript excerpts.
		return json.Marshal(rawCompactionOutput{Unavailable: true})
	}
	keepStart := checkpointBoundary(views, latest.messageIndex)
	note := latest.note

	discoveredTools := collectDiscoveredTools(views[:keepStart])
	summary, err := json.Marshal(struct {
		Role            string
		Content         string
		DiscoveredTools []json.RawMessage `json:",omitempty"`
	}{
		Role:            "system",
		Content:         buildSummaryContent(note),
		DiscoveredTools: discoveredTools,
	})
	if err != nil {
		return nil, fmt.Errorf("encode context note summary: %w", err)
	}

	output := make([]json.RawMessage, 0, len(systemPrefix)+1+len(input.Messages)-keepStart)
	output = append(output, systemPrefix...)
	output = append(output, summary)
	output = append(output, input.Messages[keepStart:]...)
	return json.Marshal(rawCompactionOutput{Messages: output})
}

func preservedSystemPrefix(raw []json.RawMessage, views []compactionMessageView) []json.RawMessage {
	prefix := make([]json.RawMessage, 0)
	for index := 0; index < len(views) && strings.EqualFold(strings.TrimSpace(views[index].Role), "system"); index++ {
		if !strings.HasPrefix(strings.TrimSpace(views[index].Content), conversationSummaryMark) {
			prefix = append(prefix, raw[index])
		}
	}
	return prefix
}

func latestCheckpoint(messages []compactionMessageView) (checkpoint, bool) {
	var latest checkpoint
	found := false
	for index, message := range messages {
		if strings.HasPrefix(strings.TrimSpace(message.Content), conversationSummaryMark) {
			if note := summaryBody(message.Content); note != "" {
				// A persisted summary (including one written by the default
				// compactor after a fallback pass) is already bounded by its
				// own output limits; keep it intact so re-compaction does not
				// silently cut off the tail of a long summary.
				latest = checkpoint{messageIndex: index, note: note}
				found = true
			}
		}
		for _, call := range message.ToolCalls {
			if !isContextNoteTool(call.Name) {
				continue
			}
			var arguments noteArguments
			if json.Unmarshal([]byte(call.Arguments), &arguments) != nil {
				continue
			}
			note := strings.TrimSpace(arguments.Note)
			if note == "" {
				continue
			}
			latest = checkpoint{messageIndex: index, note: truncateUTF8(note, maxContextNoteBytes)}
			found = true
		}
	}
	return latest, found
}

func checkpointBoundary(messages []compactionMessageView, noteMessageIndex int) int {
	index := noteMessageIndex + 1
	for index < len(messages) && strings.EqualFold(strings.TrimSpace(messages[index].Role), "tool") {
		index++
	}
	return index
}

func collectDiscoveredTools(messages []compactionMessageView) []json.RawMessage {
	var tools []json.RawMessage
	seen := make(map[string]int)
	for _, message := range messages {
		for _, tool := range message.DiscoveredTools {
			var identity struct {
				Name string `json:"name"`
			}
			if json.Unmarshal(tool, &identity) != nil || strings.TrimSpace(identity.Name) == "" {
				identity.Name = string(tool)
			}
			if index, exists := seen[identity.Name]; exists {
				tools[index] = tool
				continue
			}
			seen[identity.Name] = len(tools)
			tools = append(tools, tool)
		}
	}
	return tools
}

func buildSummaryContent(note string) string {
	return conversationSummaryMark + "\n" +
		"This session is being continued from an earlier context window. The experimental note below is the replacement context.\n\n" +
		"Summary:\n" + strings.TrimSpace(note)
}

func summaryBody(content string) string {
	content = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(content), conversationSummaryMark))
	if index := strings.Index(content, "\n\nSummary:\n"); index >= 0 {
		return strings.TrimSpace(content[index+len("\n\nSummary:\n"):])
	}
	if index := strings.Index(content, "Summary:"); index >= 0 {
		return strings.TrimSpace(content[index+len("Summary:"):])
	}
	return strings.TrimSpace(content)
}

func validNoteArguments(raw string) bool {
	var arguments noteArguments
	return json.Unmarshal([]byte(raw), &arguments) == nil && strings.TrimSpace(arguments.Note) != ""
}

func isContextNoteTool(name string) bool {
	return strings.Contains(strings.ToLower(strings.TrimSpace(name)), toolWriteContextNote)
}

func truncateUTF8(text string, maxBytes int) string {
	text = strings.TrimSpace(text)
	if maxBytes <= 0 || len([]byte(text)) <= maxBytes {
		return text
	}
	data := []byte(text)
	end := maxBytes
	for end > 0 && !utf8.Valid(data[:end]) {
		end--
	}
	return strings.TrimSpace(string(data[:end])) + "\n\n[truncated by experimental note compaction]"
}

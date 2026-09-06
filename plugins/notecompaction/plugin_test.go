package notecompaction

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	pluginapi "github.com/blueberrycongee/wuu/packages/plugin-go"
)

func rawMessage(t *testing.T, value any) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal message: %v", err)
	}
	return data
}

func invokeCompaction(t *testing.T, messages ...json.RawMessage) rawCompactionOutput {
	t.Helper()
	input, err := json.Marshal(rawCompactionInput{Messages: messages})
	if err != nil {
		t.Fatalf("marshal compaction input: %v", err)
	}
	output, err := Handler().InvokeCapability(context.Background(), nil, pluginapi.CapabilityCall{
		Capability: capabilityCompaction,
		Input:      input,
	})
	if err != nil {
		t.Fatalf("invoke compaction capability: %v", err)
	}
	var result rawCompactionOutput
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("decode compaction output: %v", err)
	}
	return result
}

func TestCompactionWithoutCheckpointDeclaresUnavailable(t *testing.T) {
	messages := []json.RawMessage{
		rawMessage(t, map[string]any{"Role": "system", "Content": "You are an assistant."}),
		rawMessage(t, map[string]any{"Role": "user", "Content": "implement feature X"}),
		rawMessage(t, map[string]any{"Role": "assistant", "Content": "working on it"}),
	}
	result := invokeCompaction(t, messages...)
	if !result.Unavailable {
		t.Fatal("expected unavailable=true when no checkpoint exists")
	}
	if len(result.Messages) != 0 {
		t.Errorf("expected no replacement messages, got %d", len(result.Messages))
	}
}

func TestCompactionEmptyInput(t *testing.T) {
	result := invokeCompaction(t)
	if result.Unavailable || len(result.Messages) != 0 {
		t.Errorf("expected empty passthrough output, got unavailable=%v messages=%d", result.Unavailable, len(result.Messages))
	}
}

func TestCompactionFromCheckpointReplacesOlderHistory(t *testing.T) {
	system := rawMessage(t, map[string]any{"Role": "system", "Content": "You are an assistant."})
	olderUser := rawMessage(t, map[string]any{"Role": "user", "Content": "implement feature X"})
	noteCall := rawMessage(t, map[string]any{
		"Role":    "assistant",
		"Content": "",
		"ToolCalls": []map[string]any{{
			"ID":        "call-note-1",
			"Name":      toolWriteContextNote,
			"Arguments": `{"note":"Objective: feature X. Done: scaffolding. Next: wire API."}`,
		}},
		"DiscoveredTools": []map[string]any{{"name": "custom_tool"}},
	})
	toolResult := rawMessage(t, map[string]any{"Role": "tool", "ToolCallID": "call-note-1", "Content": "recorded"})
	tailUser := rawMessage(t, map[string]any{"Role": "user", "Content": "now continue with the API"})

	result := invokeCompaction(t, system, olderUser, noteCall, toolResult, tailUser)
	if result.Unavailable {
		t.Fatal("expected a checkpoint-backed compaction")
	}
	if len(result.Messages) != 3 {
		t.Fatalf("expected system prefix + summary + tail = 3 messages, got %d", len(result.Messages))
	}
	if string(result.Messages[0]) != string(system) {
		t.Errorf("system prefix was not preserved verbatim")
	}
	var summary compactionMessageView
	if err := json.Unmarshal(result.Messages[1], &summary); err != nil {
		t.Fatalf("decode summary message: %v", err)
	}
	if !strings.EqualFold(summary.Role, "system") {
		t.Errorf("summary role = %q, want system", summary.Role)
	}
	if !strings.HasPrefix(summary.Content, conversationSummaryMark) {
		t.Errorf("summary does not carry the %q marker", conversationSummaryMark)
	}
	if !strings.Contains(summary.Content, "Objective: feature X") {
		t.Errorf("summary lost the checkpoint note body: %q", summary.Content)
	}
	if len(summary.DiscoveredTools) != 1 {
		t.Errorf("summary should carry 1 discovered tool from the dropped range, got %d", len(summary.DiscoveredTools))
	}
	if string(result.Messages[2]) != string(tailUser) {
		t.Errorf("tail message was not preserved verbatim")
	}
}

func TestCompactionFromPersistedSummaryKeepsBodyIntact(t *testing.T) {
	longBody := strings.Repeat("context detail ", 2_000) // > maxContextNoteBytes
	summary := rawMessage(t, map[string]any{
		"Role":    "system",
		"Content": buildSummaryContent(longBody),
	})
	tailUser := rawMessage(t, map[string]any{"Role": "user", "Content": "continue"})

	result := invokeCompaction(t, summary, tailUser)
	if result.Unavailable {
		t.Fatal("expected a summary-backed compaction")
	}
	if len(result.Messages) != 2 {
		t.Fatalf("expected summary + tail = 2 messages, got %d", len(result.Messages))
	}
	var replaced compactionMessageView
	if err := json.Unmarshal(result.Messages[0], &replaced); err != nil {
		t.Fatalf("decode replacement summary: %v", err)
	}
	if !strings.Contains(replaced.Content, strings.TrimSpace(longBody)) {
		t.Error("persisted summary body was truncated when reused as a checkpoint")
	}
	if string(result.Messages[1]) != string(tailUser) {
		t.Errorf("tail message was not preserved verbatim")
	}
}

func TestHandoffBriefPlanDoesNotSelectAModel(t *testing.T) {
	input, err := json.Marshal(rawCompactionInput{
		Operation: "handoff_brief_plan", Messages: []json.RawMessage{rawMessage(t, map[string]any{"Role": "user", "Content": "keep the performance fix"})},
		Intent: "redesign the visual hierarchy", SourceSessionID: "source-1", SourceThroughSeq: 4,
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	output, err := Handler().InvokeCapability(context.Background(), nil, pluginapi.CapabilityCall{Capability: capabilityCompaction, Input: input})
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}
	var result rawCompactionOutput
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !strings.Contains(result.NotePrompt, "Never select a provider or model") || !strings.Contains(result.NotePrompt, "source-1") {
		t.Fatalf("prompt = %q", result.NotePrompt)
	}
}

func TestRequestHandoffReturnsAwaitingUserConfiguration(t *testing.T) {
	result, err := Handler().ExecuteTool(context.Background(), nil, pluginapi.ToolCall{
		ToolID:    toolRequestHandoff,
		CallID:    "call-1",
		SessionID: "source-1",
		Arguments: []byte(`{"intent":"keep the verified performance fix"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Content) != 1 || !strings.Contains(result.Content[0].Text, `"awaiting_user_configuration":true`) || !strings.Contains(result.Content[0].Text, `"source_session_id":"source-1"`) {
		t.Fatalf("result = %+v", result)
	}
	if _, err := Handler().ExecuteTool(context.Background(), nil, pluginapi.ToolCall{
		ToolID:    toolRequestHandoff,
		Arguments: []byte(`{"intent":"keep going","provider":"openai"}`),
	}); err == nil {
		t.Fatal("provider selection was accepted")
	}
}

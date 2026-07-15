package compact

import (
	"strings"
	"testing"

	"github.com/blueberrycongee/wuu/internal/providers"
	"github.com/blueberrycongee/wuu/internal/toolresult"
)

// These tests cover the request-time tool-result prune through the real
// provider request projection. The compact placeholder must remain the single
// model-visible result even when the history message carries a rich ToolResult.

const bypassSentinel = "WIRE_BYPASS_SENTINEL_GREP_MATCH_LINE"

// oversizedGrepText returns text large enough to exceed the prune protect
// threshold (toolResultPruneProtectTokens = 40_000 tokens ≈ 160_000 ASCII
// chars) so a single old grep result is pruned rather than protected.
func oversizedGrepText() string {
	line := bypassSentinel + " 0123456789 abcdefghijklmnopqrstuvwxyz\n"
	// ~55 chars/line * 4000 lines ≈ 220_000 chars ≈ 55_000 estimated tokens.
	body := strings.Repeat(line, 4000)
	if got := EstimateTokens(body); got <= toolResultPruneProtectTokens {
		panic("test fixture too small to trigger prune")
	}
	return body
}

// findToolByCallID locates the tool message for a given tool-call id after any
// projection-time reordering (observation messages, separators).
func findToolByCallID(msgs []providers.ChatMessage, callID string) (providers.ChatMessage, bool) {
	for _, m := range msgs {
		if strings.EqualFold(m.Role, "tool") && m.ToolCallID == callID {
			return m, true
		}
	}
	return providers.ChatMessage{}, false
}

func TestPruneToolResults_RichProjectionKeepsPlaceholderOnWire(t *testing.T) {
	bigText := oversizedGrepText()
	bigResult := toolresult.FromText(bigText)

	messages := []providers.ChatMessage{
		{Role: "user", Content: "u1"},
		{Role: "assistant", ToolCalls: []providers.ToolCall{{ID: "call-old", Name: "grep"}}},
		{Role: "tool", ToolCallID: "call-old", Name: "grep", ToolResult: &bigResult},
		{Role: "user", Content: "u2"},
		{Role: "assistant", ToolCalls: []providers.ToolCall{{ID: "call-recent", Name: "grep"}}},
		{Role: "tool", ToolCallID: "call-recent", Name: "grep", Content: "small recent", ToolResult: ptrResult(toolresult.FromText("small recent"))},
		{Role: "user", Content: "u3"},
	}
	// The live-history Content of the old tool result mirrors the rich result,
	// as tool_runtime.toolResultMessage constructs it.
	messages[2].Content = bigText

	pruned := PruneToolResults(messages)

	// Step 1: the prune DOES rewrite Content in the returned slice.
	prunedOld, ok := findToolByCallID(pruned, "call-old")
	if !ok {
		t.Fatalf("old tool message missing after prune")
	}
	if !strings.HasPrefix(prunedOld.Content, "[Pruned grep result.") {
		t.Fatalf("expected pruned placeholder in Content, got prefix %q", head(prunedOld.Content, 60))
	}
	if strings.Contains(prunedOld.Content, bypassSentinel) {
		t.Fatalf("prune should have removed the sentinel from Content")
	}
	// The request-only rich result must agree with Content. Otherwise provider
	// projection would restore the original body below.
	if prunedOld.ToolResult == nil || prunedOld.ToolResult.TextProjection() != prunedOld.Content {
		t.Fatalf("pruned ToolResult must match Content, got rich prefix %q", head(prunedOld.ToolResult.TextProjection(), 80))
	}
	if !strings.Contains(messages[2].Content, bypassSentinel) || messages[2].ToolResult == nil ||
		!strings.Contains(messages[2].ToolResult.TextProjection(), bypassSentinel) {
		t.Fatal("request pruning must not modify durable history")
	}

	// Step 2: the real provider projection must preserve the placeholder on the
	// wire instead of restoring the full result.
	prepared, err := providers.PrepareMessagesForModelRequest("gpt-5", pruned)
	if err != nil {
		t.Fatalf("PrepareMessagesForModelRequest: %v", err)
	}
	wireOld, ok := findToolByCallID(prepared, "call-old")
	if !ok {
		t.Fatalf("old tool message missing after wire preparation")
	}
	if !strings.HasPrefix(wireOld.Content, "[Pruned grep result.") {
		t.Fatalf("expected placeholder to survive on the wire, got prefix %q", head(wireOld.Content, 80))
	}
	if strings.Contains(wireOld.Content, bypassSentinel) {
		t.Fatalf("provider projection restored the pruned result on the wire")
	}
}

// TestPruneToolResults_LegacyNilResult_PlaceholderReachesWire covers older
// persisted sessions that have no rich ToolResult. Their pruned Content must
// continue to reach the wire unchanged.
func TestPruneToolResults_LegacyNilResult_PlaceholderReachesWire(t *testing.T) {
	bigText := oversizedGrepText()

	messages := []providers.ChatMessage{
		{Role: "user", Content: "u1"},
		{Role: "assistant", ToolCalls: []providers.ToolCall{{ID: "call-old", Name: "grep"}}},
		{Role: "tool", ToolCallID: "call-old", Name: "grep", Content: bigText}, // ToolResult == nil
		{Role: "user", Content: "u2"},
		{Role: "assistant", ToolCalls: []providers.ToolCall{{ID: "call-recent", Name: "grep"}}},
		{Role: "tool", ToolCallID: "call-recent", Name: "grep", Content: "small recent"},
		{Role: "user", Content: "u3"},
	}

	pruned := PruneToolResults(messages)
	prepared, err := providers.PrepareMessagesForModelRequest("gpt-5", pruned)
	if err != nil {
		t.Fatalf("PrepareMessagesForModelRequest: %v", err)
	}
	wireOld, ok := findToolByCallID(prepared, "call-old")
	if !ok {
		t.Fatalf("old tool message missing after wire preparation")
	}
	if !strings.HasPrefix(wireOld.Content, "[Pruned grep result.") {
		t.Fatalf("expected pruned placeholder to survive on the wire for a nil ToolResult, got prefix %q", head(wireOld.Content, 80))
	}
	if strings.Contains(wireOld.Content, bypassSentinel) {
		t.Fatalf("nil-ToolResult message should not restore the full result")
	}
}

func ptrResult(r toolresult.Result) *toolresult.Result { return &r }

func head(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

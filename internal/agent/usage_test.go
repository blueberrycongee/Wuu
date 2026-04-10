package agent

import (
	"strings"
	"testing"

	"github.com/blueberrycongee/wuu/internal/providers"
)

func TestUsageTracker_EmptyIsZero(t *testing.T) {
	tr := NewUsageTracker()
	if got := tr.EstimateCurrent(); got != 0 {
		t.Fatalf("expected 0, got %d", got)
	}
}

func TestUsageTracker_NilSafe(t *testing.T) {
	var tr *UsageTracker // intentionally nil
	tr.RecordResponse(&providers.TokenUsage{InputTokens: 10})
	tr.RecordPendingMessages([]providers.ChatMessage{{Role: "user", Content: "hi"}})
	tr.Reset()
	if got := tr.EstimateCurrent(); got != 0 {
		t.Fatalf("nil tracker should return 0, got %d", got)
	}
}

func TestUsageTracker_GroundTruthFromResponse(t *testing.T) {
	tr := NewUsageTracker()
	tr.RecordResponse(&providers.TokenUsage{InputTokens: 1000, OutputTokens: 250})

	if got := tr.EstimateCurrent(); got != 1250 {
		t.Fatalf("expected 1250, got %d", got)
	}
	if got := tr.LastResponseTotal(); got != 1250 {
		t.Fatalf("expected last 1250, got %d", got)
	}
	if got := tr.PendingDelta(); got != 0 {
		t.Fatalf("expected pending 0, got %d", got)
	}
}

func TestUsageTracker_PendingDeltaAccumulates(t *testing.T) {
	tr := NewUsageTracker()
	tr.RecordResponse(&providers.TokenUsage{InputTokens: 1000, OutputTokens: 0})

	// 400 chars ≈ 100 tokens + 4 overhead = 104
	longMsg := providers.ChatMessage{
		Role:    "user",
		Content: strings.Repeat("x", 400),
	}
	tr.RecordPendingMessages([]providers.ChatMessage{longMsg})

	got := tr.EstimateCurrent()
	if got <= 1000 || got > 1200 {
		t.Fatalf("expected 1000 < estimate <= 1200, got %d", got)
	}
}

func TestUsageTracker_ResponseResetsPendingDelta(t *testing.T) {
	tr := NewUsageTracker()
	tr.RecordResponse(&providers.TokenUsage{InputTokens: 500})
	tr.RecordPendingMessages([]providers.ChatMessage{
		{Role: "user", Content: strings.Repeat("y", 200)},
	})
	if tr.PendingDelta() == 0 {
		t.Fatal("delta should be non-zero before second response")
	}

	// Second response collapses the delta into ground truth.
	tr.RecordResponse(&providers.TokenUsage{InputTokens: 1500, OutputTokens: 100})
	if got := tr.PendingDelta(); got != 0 {
		t.Fatalf("delta should be zero after RecordResponse, got %d", got)
	}
	if got := tr.EstimateCurrent(); got != 1600 {
		t.Fatalf("expected 1600, got %d", got)
	}
}

func TestUsageTracker_ToolCallEnvelopeCounted(t *testing.T) {
	tr := NewUsageTracker()
	withTool := providers.ChatMessage{
		Role:    "assistant",
		Content: "ok",
		ToolCalls: []providers.ToolCall{
			{Name: "run_shell", Arguments: `{"command":"ls"}`},
		},
	}
	tr.RecordPendingMessages([]providers.ChatMessage{withTool})
	if got := tr.PendingDelta(); got <= 0 {
		t.Fatalf("tool-call message should produce non-zero delta, got %d", got)
	}
}

func TestUsageTracker_Reset(t *testing.T) {
	tr := NewUsageTracker()
	tr.RecordResponse(&providers.TokenUsage{InputTokens: 1000})
	tr.RecordPendingMessages([]providers.ChatMessage{{Role: "user", Content: "hi"}})
	tr.Reset()
	if got := tr.EstimateCurrent(); got != 0 {
		t.Fatalf("expected 0 after Reset, got %d", got)
	}
	if got := tr.LastResponseTotal(); got != 0 {
		t.Fatalf("expected last 0 after Reset, got %d", got)
	}
}

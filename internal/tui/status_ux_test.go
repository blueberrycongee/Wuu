package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
)

func TestShouldRenderInlineStatusSuppressesTranscriptDuplicates(t *testing.T) {
	m := Model{streaming: true, statusLine: "thinking"}
	m.entries = []transcriptEntry{{Role: "ASSISTANT", ThinkingContent: "working", ThinkingDone: false}}

	if m.shouldRenderInlineStatus() {
		t.Fatal("expected inline thinking status to be hidden when transcript already shows a live thinking block")
	}
}

func TestShouldRenderInlineStatusKeepsRespondingWhileQueuedMessageAdded(t *testing.T) {
	m := Model{pendingRequest: true, streaming: true, statusLine: "streaming"}
	m.messageQueue = []queuedMessage{{Text: "queued follow-up"}}

	if !m.shouldRenderInlineStatus() {
		t.Fatal("expected inline responding status to stay visible while follow-up messages are queued")
	}
	if got := deriveWorkStatus(m.statusLine).Label; got != "Responding" {
		t.Fatalf("expected responding work status, got %q", got)
	}
}

func TestShouldRenderInlineStatusSuppressesToolDuplicate(t *testing.T) {
	m := Model{streaming: true, statusLine: "tool: grep"}
	m.entries = []transcriptEntry{{
		Role:      "ASSISTANT",
		ToolCalls: []ToolCallEntry{{Name: "grep", Status: ToolCallRunning}},
	}}

	if m.shouldRenderInlineStatus() {
		t.Fatal("expected inline tool status to be hidden when a live tool card is already visible")
	}
}

func TestShouldRenderInlineStatusKeepsSpawnAgentVisibleWithVisibleToolCard(t *testing.T) {
	m := Model{streaming: true, statusLine: "tool: spawn_agent"}
	m.entries = []transcriptEntry{{
		Role:      "ASSISTANT",
		ToolCalls: []ToolCallEntry{{Name: "spawn_agent", Status: ToolCallRunning}},
	}}

	if !m.shouldRenderInlineStatus() {
		t.Fatal("expected spawn_agent inline status to stay visible even when the tool card is visible")
	}
}

func TestRenderInlineStatus_UsesSpawnWorkerCopy(t *testing.T) {
	got := renderInlineWorkStatus(runningToolWorkStatus("spawn_agent"), 0, 120)
	if got == "" {
		t.Fatal("expected spawn_agent inline status to render")
	}
	plain := ansi.Strip(got)
	if strings.Contains(plain, "Running spawn_agent") || strings.Contains(plain, "Making progress with a tool") {
		t.Fatalf("expected spawn_agent inline status to use worker-oriented copy, got %q", plain)
	}
	if !strings.Contains(plain, "Spawning worker") || !strings.Contains(plain, "Dispatching the background task") {
		t.Fatalf("expected spawn_agent inline status to describe worker dispatch, got %q", plain)
	}
}

func TestStatusAnimationIntervalSlowerForCalmerShimmer(t *testing.T) {
	if statusAnimationInterval != 100*time.Millisecond {
		t.Fatalf("expected status animation interval to be 100ms, got %s", statusAnimationInterval)
	}
}

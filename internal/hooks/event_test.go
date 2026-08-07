package hooks

import "testing"

func TestEventString(t *testing.T) {
	if string(PreToolUse) != "PreToolUse" {
		t.Fatal("event string mismatch")
	}
}

func TestIsValidEvent(t *testing.T) {
	if !IsValid(PreToolUse) {
		t.Fatal("expected PreToolUse to be valid")
	}
	if IsValid(Event("FakeEvent")) {
		t.Fatal("expected FakeEvent to be invalid")
	}
}

func TestEveryEventHasDispatchPoint(t *testing.T) {
	dispatchFiles := map[Event]string{
		PreToolUse: "internal/hooks/executor.go", PostToolUse: "internal/hooks/executor.go",
		PostToolUseFailure: "internal/hooks/executor.go", FileChanged: "internal/runtime/session.go",
		UserPromptSubmit: "internal/appserver/turn_handlers.go", Stop: "internal/appserver/turn_handlers.go",
		SessionStart: "internal/runtime/session.go", SessionEnd: "internal/runtime/session.go",
		PermissionRequest: "internal/tools/toolkit.go", PreCompact: "internal/agent/loop.go",
		PostCompact: "internal/agent/loop.go", SubagentStart: "internal/subagent/manager.go",
		SubagentStop: "internal/subagent/manager.go",
	}
	for _, event := range AllEvents() {
		if !IsValid(event) {
			t.Errorf("AllEvents contains invalid event %q", event)
		}
		if dispatchFiles[event] == "" {
			t.Errorf("event %q has no declared production dispatch point", event)
		}
	}
	if len(dispatchFiles) != len(AllEvents()) {
		t.Fatalf("dispatch map has %d events, AllEvents has %d", len(dispatchFiles), len(AllEvents()))
	}
}

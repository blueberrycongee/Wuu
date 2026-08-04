package appserver

import (
	"strings"
	"testing"
	"time"

	"github.com/blueberrycongee/wuu/internal/providers"
)

func TestInterruptedTurnContinuationMessageIsHidden(t *testing.T) {
	message := interruptedTurnContinuationMessage()
	if message.Role != "user" || !message.Hidden || message.Name != turnContinuationMessageName {
		t.Fatalf("unexpected continuation message: %+v", message)
	}
	if !strings.Contains(message.Content, "[TURN_CONTINUATION]") {
		t.Fatalf("continuation marker missing from %q", message.Content)
	}
}

func TestStartTurnLockedHidesContinuationItem(t *testing.T) {
	now := time.Now()
	thread := newThreadState("thread-1", nil, "fake", "fake-model", t.TempDir(), false, now)

	turn := thread.startTurnLocked("turn-2", interruptedTurnContinuationMessage(), now)

	if turn.Kind != TurnKindContinuation {
		t.Fatalf("continuation kind = %q", turn.Kind)
	}
	if len(turn.Items) != 0 {
		t.Fatalf("continuation should not create a visible user item: %+v", turn.Items)
	}
	if thread.currentTurnKind != TurnKindContinuation {
		t.Fatalf("current turn kind = %q", thread.currentTurnKind)
	}
}

func TestValidateTurnContinuationLocked(t *testing.T) {
	base := func(status TurnStatus) *threadState {
		return &threadState{
			ID:    "thread-1",
			Turns: []Turn{{ID: "turn-1", Status: status}},
		}
	}

	t.Run("accepts interrupted tail turn", func(t *testing.T) {
		if err := validateTurnContinuationLocked(base(TurnStatusInterrupted)); err != nil {
			t.Fatalf("validate continuation: %v", err)
		}
	})

	for _, test := range []struct {
		name   string
		mutate func(*threadState)
	}{
		{name: "running", mutate: func(thread *threadState) { thread.running = true }},
		{name: "completed", mutate: func(thread *threadState) { thread.Turns[0].Status = TurnStatusCompleted }},
		{name: "not tail", mutate: func(thread *threadState) {
			thread.Turns = append(thread.Turns, Turn{ID: "turn-2", Status: TurnStatusCompleted})
		}},
		{name: "subagent", mutate: func(thread *threadState) { thread.AgentPath = "root/worker" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			thread := base(TurnStatusInterrupted)
			test.mutate(thread)
			if err := validateTurnContinuationLocked(thread); err == nil {
				t.Fatal("expected continuation validation error")
			}
		})
	}
}

func TestOrdinaryTurnStillCreatesVisibleUserItem(t *testing.T) {
	now := time.Now()
	thread := newThreadState("thread-1", nil, "fake", "fake-model", t.TempDir(), false, now)

	turn := thread.startTurnLocked("turn-1", providers.ChatMessage{Role: "user", Content: "hello"}, now)

	if len(turn.Items) != 1 || turn.Items[0].Type != "user_message" {
		t.Fatalf("ordinary turn items = %+v", turn.Items)
	}
}

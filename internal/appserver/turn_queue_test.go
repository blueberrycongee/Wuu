package appserver

import (
	"errors"
	"testing"

	"github.com/blueberrycongee/wuu/internal/config"
	"github.com/blueberrycongee/wuu/internal/providers"
)

func TestReplaceQueuedUserTurnPreservesOrder(t *testing.T) {
	const threadID = "thread-1"
	s := &Server{}
	s.enqueueQueuedUserTurn(threadID, queuedTurn{
		id:  "queue-1",
		msg: providers.ChatMessage{Role: "user", Content: "first"},
	})
	s.enqueueQueuedUserTurn(threadID, queuedTurn{
		id:  "queue-2",
		msg: providers.ChatMessage{Role: "user", Content: "second"},
	})
	s.enqueueQueuedUserTurn(threadID, queuedTurn{
		id:  "queue-3",
		msg: providers.ChatMessage{Role: "user", Content: "third"},
	})

	updated, ok := s.replaceQueuedUserTurn(threadID, "queue-2", providers.ChatMessage{
		Role:    "user",
		Content: "second edited",
	})
	if !ok {
		t.Fatal("replaceQueuedUserTurn returned false")
	}
	if updated.id != "queue-2" || updated.msg.ClientID != "queue-2" {
		t.Fatalf("replacement lost queue id: %+v", updated)
	}
	if updated.msg.Steered {
		t.Fatalf("queued replacement should not be marked steered: %+v", updated.msg)
	}

	var got []string
	for {
		entry, ok := s.takeNextQueuedUserTurn(threadID)
		if !ok {
			break
		}
		got = append(got, entry.id+":"+entry.msg.Content)
	}
	want := []string{
		"queue-1:first",
		"queue-2:second edited",
		"queue-3:third",
	}
	if len(got) != len(want) {
		t.Fatalf("drained %d queued turns, want %d: %v", len(got), len(want), got)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("drain order mismatch at %d: got %q want %q (all got %v)", index, got[index], want[index], got)
		}
	}
}

func TestReplaceQueuedUserTurnPreservesRuntimeSnapshot(t *testing.T) {
	readOnly := config.ResolvedPermissions{Mode: config.PermissionModeReadOnly}
	s := &Server{}
	s.enqueueQueuedUserTurn("thread-1", queuedTurn{
		id:       "queue-1",
		msg:      providers.ChatMessage{Role: "user", Content: "first"},
		snapshot: turnRuntimeSnapshot{}.withPermissions(readOnly),
	})

	updated, ok := s.replaceQueuedUserTurn("thread-1", "queue-1", providers.ChatMessage{
		Role:    "user",
		Content: "edited",
	})
	if !ok {
		t.Fatal("replaceQueuedUserTurn returned false")
	}
	if got := updated.snapshot.permissions(); got.Mode != config.PermissionModeReadOnly {
		t.Fatalf("replacement lost permission snapshot: %+v", got)
	}
}

func TestReplaceQueuedUserTurnReturnsFalseWhenMissing(t *testing.T) {
	s := &Server{}
	s.enqueueQueuedUserTurn("thread-1", queuedTurn{
		id:  "queue-1",
		msg: providers.ChatMessage{Role: "user", Content: "first"},
	})

	if _, ok := s.replaceQueuedUserTurn("thread-1", "missing", providers.ChatMessage{
		Role:    "user",
		Content: "edited",
	}); ok {
		t.Fatal("replaceQueuedUserTurn returned true for a missing queue id")
	}

	entry, ok := s.takeNextQueuedUserTurn("thread-1")
	if !ok {
		t.Fatal("queued turn was removed by failed replace")
	}
	if entry.id != "queue-1" || entry.msg.Content != "first" {
		t.Fatalf("failed replace mutated queue: %+v", entry)
	}
}

func TestQueuedTurnCanBeCancelledWhileClaimedForAdmission(t *testing.T) {
	const threadID = "thread-1"
	const queueID = "queue-1"
	s := &Server{}
	s.enqueueQueuedUserTurn(threadID, queuedTurn{
		id:  queueID,
		msg: providers.ChatMessage{Role: "user", Content: "cancel me"},
	})

	entry, ok := s.takeNextQueuedUserTurn(threadID)
	if !ok {
		t.Fatal("queued turn was not claimed")
	}
	removed, ok := s.removeQueuedUserTurn(threadID, queueID)
	if !ok || removed.id != queueID {
		t.Fatalf("cancel claimed turn = %+v, %v", removed, ok)
	}
	if err := s.commitQueuedTurnClaim(threadID, queueID); !errors.Is(err, errQueuedTurnCancelled) {
		t.Fatalf("commit after cancellation = %v, want errQueuedTurnCancelled", err)
	}
	if cancelled := s.settleQueuedTurnClaim(threadID, entry, true); !cancelled {
		t.Fatal("settlement did not observe cancellation")
	}
	if s.hasQueuedUserTurns(threadID) {
		t.Fatal("cancelled claimed turn was requeued")
	}
}

func TestQueuedTurnCannotBeCancelledAfterCommit(t *testing.T) {
	const threadID = "thread-1"
	const queueID = "queue-1"
	s := &Server{}
	s.enqueueQueuedUserTurn(threadID, queuedTurn{
		id:  queueID,
		msg: providers.ChatMessage{Role: "user", Content: "already committed"},
	})

	entry, ok := s.takeNextQueuedUserTurn(threadID)
	if !ok {
		t.Fatal("queued turn was not claimed")
	}
	if err := s.commitQueuedTurnClaim(threadID, queueID); err != nil {
		t.Fatalf("commit claim: %v", err)
	}
	if _, removed := s.removeQueuedUserTurn(threadID, queueID); removed {
		t.Fatal("committed queued turn was cancelled")
	}
	s.settleQueuedTurnClaim(threadID, entry, false)
}

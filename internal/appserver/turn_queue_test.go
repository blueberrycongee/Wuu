package appserver

import (
	"encoding/json"
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

func TestTurnRequeueAtomicallyMovesPendingSteerBackToQueue(t *testing.T) {
	const threadID = "thread-1"
	const steerID = "steer-1"
	out := &lockedBuffer{}
	th := &threadState{
		ID:          threadID,
		running:     true,
		currentTurn: "turn-1",
		pendingSteers: []providers.ChatMessage{{
			Role: "user", Content: "send this next", ClientID: steerID, Steered: true,
		}},
		steerDocumentOverrides: []activeDocumentOverride{{
			steerID: steerID, document: &ActiveDocument{Path: "docs/next.md"},
		}},
		activeSteerContextSet: true,
		activeSteerDocument:   &ActiveDocument{Path: "docs/next.md"},
	}
	s := &Server{
		out:                 out,
		threads:             map[string]*threadState{threadID: th},
		pendingQueuedTurns:  make(map[string][]queuedTurn),
		claimedQueuedTurns:  make(map[string]*queuedTurnClaim),
		drainingQueuedTurns: make(map[string]bool),
		activeRunByThread:   make(map[string]string),
	}
	params, err := json.Marshal(TurnRequeueParams{ThreadID: threadID, SteerID: steerID})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.handleTurnRequeue(Request{ID: json.RawMessage(`"requeue"`), Params: params}); err != nil {
		t.Fatalf("turn/requeue: %v", err)
	}

	th.mu.Lock()
	pendingSteers := len(th.pendingSteers)
	activeDocument := th.activeSteerDocument
	th.mu.Unlock()
	if pendingSteers != 0 || activeDocument != nil {
		t.Fatalf("steer state remained after requeue: pending=%d document=%+v", pendingSteers, activeDocument)
	}
	queued, ok := s.findQueuedUserTurn(threadID, steerID)
	if !ok || queued.msg.Content != "send this next" || queued.msg.Steered {
		t.Fatalf("requeued turn = %+v, %v", queued, ok)
	}
	if queued.snapshot.ActiveDocument == nil || queued.snapshot.ActiveDocument.Path != "docs/next.md" {
		t.Fatalf("requeued document snapshot = %+v", queued.snapshot.ActiveDocument)
	}
	result := remarshal[TurnRequeueResult](
		t,
		responseByID(t, parseOutput(t, out.String()), "requeue")["result"],
	)
	if !result.OK || result.State != "queued" || result.Queued.ID != steerID {
		t.Fatalf("turn/requeue result = %+v", result)
	}
}

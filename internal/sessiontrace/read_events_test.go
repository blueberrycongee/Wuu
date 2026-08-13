package sessiontrace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadEventsPreservesPayloadBytes(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "session-trace.jsonl")
	if err := AppendTurn(path,
		TurnRecord{ThreadID: "thread-1", TurnID: "turn-1", Status: "completed", InputTokens: 9007199254740993, OutputTokens: 2},
		FinalRecord{Status: "completed"},
		nil, nil, nil, nil, nil,
	); err != nil {
		t.Fatal(err)
	}

	events, err := ReadEvents(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) == 0 {
		t.Fatal("ReadEvents returned no events")
	}
	types := map[string]bool{}
	for _, event := range events {
		types[event.Type] = true
	}
	if !types["turn"] || !types["final"] {
		t.Fatalf("event types = %v, want turn and final", types)
	}
}

func TestReadEventsMissingFile(t *testing.T) {
	t.Parallel()

	_, err := ReadEvents(filepath.Join(t.TempDir(), "missing.jsonl"))
	if err == nil || !os.IsNotExist(err) {
		t.Fatalf("ReadEvents error = %v, want not-exist", err)
	}
}

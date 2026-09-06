package session

import (
	"context"
	"errors"
	"testing"
)

func TestCreateInitializedWithLaunchPersistsSeedWithoutSourceHistory(t *testing.T) {
	dir := t.TempDir()
	if _, err := CreateWithMetadata(dir, "source", t.TempDir()); err != nil {
		t.Fatalf("create source: %v", err)
	}
	if err := AppendHistoryRecords(dir, "source", []HistoryRecord{
		{Role: "user", Content: "SENTINEL-SOURCE-ONLY"},
		{Role: "assistant", Content: "investigation notes"},
	}); err != nil {
		t.Fatalf("append source: %v", err)
	}
	seed := ContextSeed{
		Version: ContextSeedVersionV1,
		Body:    "Continue from the verified performance fix. Do not treat layout advice as a hard constraint.",
		Source:  HistorySnapshot{SessionID: "source", ThroughSeq: 2},
		References: []ContextSeedReference{{
			ID: "r1", History: HistoryRef{Snapshot: HistorySnapshot{SessionID: "source", ThroughSeq: 2}, StartSeq: 1, EndSeq: 1},
		}},
		Provenance: ContextSeedProvenance{Producer: "test"},
	}
	created, err := CreateInitializedWithLaunch(dir, Session{
		ID: "target", CWD: t.TempDir(), Provider: "openai", Model: "gpt-test", ContextSource: "seed",
	}, nil, seed, SessionLaunchRecord{
		RequestID: "handoff-1", Revision: 1, Kind: SessionLaunchKindHandoff, SourceSession: "source", SourceCutoff: 2,
		Runtime: SessionRuntimeSelection{Provider: "openai", Model: "gpt-test"},
	})
	if err != nil {
		t.Fatalf("create target: %v", err)
	}
	if created.ContextSource != "seed" || created.SeedID == "" {
		t.Fatalf("created = %+v", created)
	}
	page, err := ReadHistoryPage(context.Background(), dir, "target", 1, 20)
	if err != nil {
		t.Fatalf("read target history: %v", err)
	}
	for _, record := range page.Records {
		if record.Content == "SENTINEL-SOURCE-ONLY" {
			t.Fatalf("target copied source history: %+v", page.Records)
		}
	}
	if len(page.Records) != 1 || page.Records[0].Name != "context_seed" || page.Records[0].Origin != "handoff" {
		t.Fatalf("target history = %+v", page.Records)
	}
	again, err := CreateInitializedWithLaunch(dir, Session{ID: "other", CWD: t.TempDir()}, nil, seed, SessionLaunchRecord{
		RequestID: "handoff-1", Revision: 1, Kind: SessionLaunchKindHandoff, SourceSession: "source", SourceCutoff: 2,
		Runtime: SessionRuntimeSelection{Provider: "openai", Model: "gpt-test"},
	})
	if err != nil {
		t.Fatalf("idempotent create: %v", err)
	}
	if again.ID != created.ID {
		t.Fatalf("idempotent target = %q, want %q", again.ID, created.ID)
	}
	_, err = CreateInitializedWithLaunch(dir, Session{ID: "conflict", CWD: t.TempDir()}, nil, seed, SessionLaunchRecord{
		RequestID: "handoff-1", Revision: 2, Kind: SessionLaunchKindHandoff, SourceSession: "source", SourceCutoff: 2,
		Runtime: SessionRuntimeSelection{Provider: "openai", Model: "other-model"},
	})
	if !errors.Is(err, ErrSessionLaunchConflict) {
		t.Fatalf("conflict error = %v", err)
	}
}

func TestValidateContextSeedRejectsOutOfRangeReferences(t *testing.T) {
	err := ValidateContextSeed(ContextSeed{
		Version: ContextSeedVersionV1,
		Body:    "brief",
		Source:  HistorySnapshot{SessionID: "source", ThroughSeq: 2},
		References: []ContextSeedReference{{
			ID: "r1", History: HistoryRef{Snapshot: HistorySnapshot{SessionID: "source", ThroughSeq: 2}, StartSeq: 1, EndSeq: 9},
		}},
		Provenance: ContextSeedProvenance{Producer: "test"},
	})
	if !errors.Is(err, ErrContextSeedInvalid) {
		t.Fatalf("error = %v", err)
	}
}

package agent

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/blueberrycongee/wuu/internal/providers"
)

type metricNoteStore struct {
	cancellationNoteStore
	metrics chan ContextNoteMetric
}

func (s *metricNoteStore) RecordContextNoteMetric(_ context.Context, metric ContextNoteMetric) error {
	s.metrics <- metric
	return nil
}

func TestBackgroundNoteStopsWhenExtensionIsRemoved(t *testing.T) {
	p := cancellationNoteProvider{}
	registry := NewCompactionRegistry()
	registry.RegisterWithOwner(p, "generation")
	r := &StreamRunner{CompactionRegistry: registry}
	s := &metricNoteStore{metrics: make(chan ContextNoteMetric, 1)}
	started := make(chan struct{})
	fork := func(ctx context.Context, _ []providers.ChatMessage, _ CompactionNotePlan) (CompactionNoteForkResult, error) {
		close(started)
		<-ctx.Done()
		return CompactionNoteForkResult{Markdown: "must not install after removal"}, nil
	}
	r.scheduleCompactionNote(context.Background(), []providers.ChatMessage{{Role: "user", Content: "task"}}, p, s, "model", fork)
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("fork did not start")
	}
	registry.RemoveByGeneration("generation")
	select {
	case <-s.metrics:
	case <-time.After(5 * time.Second):
		t.Fatal("extension removal did not stop its fork")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.storeCalls != 0 {
		t.Fatal("removed extension installed a late note")
	}
}

func TestBackgroundNoteFailureBackoffAndModelSwitch(t *testing.T) {
	r := &StreamRunner{}
	s := &metricNoteStore{metrics: make(chan ContextNoteMetric, 2)}
	p := cancellationNoteProvider{}
	history := []providers.ChatMessage{{Role: "user", Content: "complete this task"}}
	var calls atomic.Int32
	fork := func(context.Context, []providers.ChatMessage, CompactionNotePlan) (CompactionNoteForkResult, error) {
		calls.Add(1)
		return CompactionNoteForkResult{Usage: &providers.TokenUsage{InputTokens: 30, OutputTokens: 2, CacheReadTokens: 20}}, errors.New("provider unavailable")
	}
	await := func() ContextNoteMetric {
		t.Helper()
		select {
		case m := <-s.metrics:
			return m
		case <-time.After(5 * time.Second):
			t.Fatal("background completion not recorded")
			return ContextNoteMetric{}
		}
	}
	r.scheduleCompactionNote(context.Background(), history, p, s, "one", fork)
	m := await() // Metrics are recorded after scheduler state settles.
	if m.Outcome != "failed" || m.Usage.InputTokens != 30 || m.Usage.CacheReadTokens != 20 {
		t.Fatalf("failed request cost lost: %+v", m)
	}
	for range 20 {
		r.scheduleCompactionNote(context.Background(), history, p, s, "one", fork)
	}
	if calls.Load() != 1 {
		t.Fatal("safe boundaries bypassed failure backoff")
	}
	r.scheduleCompactionNote(context.Background(), history, p, s, "two", fork)
	if m := await(); m.Model != "two" || calls.Load() != 2 {
		t.Fatalf("model switch inherited old backoff: %+v", m)
	}
	// Advance the deadline directly: retries are boundary-driven, not timers.
	r.noteMu.Lock()
	r.noteRetryAfter = time.Now().Add(-time.Second)
	r.noteMu.Unlock()
	r.scheduleCompactionNote(context.Background(), history, p, s, "two", fork)
	await()
	if calls.Load() != 3 {
		t.Fatal("eligible boundary did not retry")
	}
}

package agent

import (
	"context"
	"errors"
	"testing"

	"github.com/blueberrycongee/wuu/internal/providers"
)

func TestManualContextWindowCommitsWithoutInference(t *testing.T) {
	for _, commitFailure := range []bool{false, true} {
		step := &contextSwitchStep{}
		commits := 0
		result, err := RunToolLoop(context.Background(), fallbackHistory(), LoopConfig{
			Model: "test", ForceInitialCompact: true, CompactOnly: true, FreshContextTokens: 4000,
			Compact: func(context.Context, []providers.ChatMessage) ([]providers.ChatMessage, error) {
				t.Fatal("manual window generated a summary")
				return nil, nil
			},
			ArchiveHistory: func(context.Context, []providers.ChatMessage) (HistoryArchive, error) {
				return HistoryArchive{HeadSeq: 100}, nil
			},
			FreshContext: func(_ context.Context, m []providers.ChatMessage, head, fixed, target int) ([]providers.ChatMessage, error) {
				return buildFreshContext(m, head, fixed, target)
			},
			AcceptFreshContext: func(_ context.Context, m []providers.ChatMessage, head int) ([]providers.ChatMessage, int, error) {
				commits++
				if commitFailure {
					return nil, head, errors.New("storage failed")
				}
				return m, head, nil
			},
		}, step)
		if (err != nil) != commitFailure || result.HistoryRewritten == commitFailure || commits != 1 || len(step.requests) != 0 {
			t.Fatalf("commitFailure=%v result=%+v err=%v commits=%d requests=%d", commitFailure, result, err, commits, len(step.requests))
		}
	}
}

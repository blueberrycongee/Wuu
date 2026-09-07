package agent

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/blueberrycongee/wuu/internal/providers"
)

func TestBuildFreshContextRejectsUncoveredHistoryLoss(t *testing.T) {
	messages := []providers.ChatMessage{
		{Role: "system", Content: "current system state"},
		{Role: "user", Content: "original task"},
		{Role: "assistant", Content: "initial decision"},
	}
	note := CompactionNote{
		Markdown:        "Goal: finish the task. Decision: preserve exact history.",
		CoveredMessages: len(messages),
		CoveredHash:     CompactionHistoryHash(messages),
	}
	for index := 0; index < 401; index++ {
		messages = append(messages, providers.ChatMessage{Role: "assistant", Content: strings.Repeat("x", 2400)})
	}
	messages = append(messages,
		providers.ChatMessage{Role: "user", Content: "finish the current implementation"},
		providers.ChatMessage{Role: "assistant", Content: "working set detail"},
	)

	replacement, reanchored, err := buildFreshContext(messages, note, true, 990, 7_000, FreshContextTargetTokens)
	if !errors.Is(err, ErrFreshContextCoverageGap) || replacement != nil || reanchored.Markdown != "" {
		t.Fatalf("uncovered history was silently dropped: replacement=%d note=%+v err=%v", len(replacement), reanchored, err)
	}
}

func TestBuildFreshContextWorksWithoutCompletedNote(t *testing.T) {
	messages := []providers.ChatMessage{{Role: "system", Content: "system"}}
	for index := 0; index < 80; index++ {
		messages = append(messages, providers.ChatMessage{Role: "user", Content: strings.Repeat("历史", 1000)})
		messages = append(messages, providers.ChatMessage{Role: "assistant", Content: "answer"})
	}
	replacement, reanchored, err := buildFreshContext(messages, CompactionNote{}, false, 321, 4_000, FreshContextTargetTokens)
	if err != nil {
		t.Fatalf("build without note: %v", err)
	}
	if strings.TrimSpace(reanchored.Markdown) != "" {
		t.Fatalf("unexpected reanchored note: %+v", reanchored)
	}
	if len(replacement) >= len(messages) || 4_000+estimateFreshContextMessages(replacement) > FreshContextTargetTokens {
		t.Fatalf("unbounded replacement: messages=%d/%d tokens=%d", len(replacement), len(messages), 4_000+estimateFreshContextMessages(replacement))
	}
}

func TestBuildFreshContextRejectsOversizedFixedContext(t *testing.T) {
	messages := []providers.ChatMessage{{Role: "system", Content: strings.Repeat("x", 100_000)}, {Role: "user", Content: "task"}}
	_, _, err := buildFreshContext(messages, CompactionNote{}, false, 1, 49_000, FreshContextTargetTokens)
	if !errors.Is(err, ErrFreshContextTooLarge) {
		t.Fatalf("error = %v, want ErrFreshContextTooLarge", err)
	}
}

type contextSwitchStep struct {
	requests []providers.ChatRequest
}

func (s *contextSwitchStep) Execute(_ context.Context, req providers.ChatRequest) (StepResult, error) {
	if _, err := providers.RepairAndValidateToolCallHistory(req.Messages); err != nil {
		return StepResult{}, err
	}
	s.requests = append(s.requests, req)
	if len(s.requests) == 1 {
		return StepResult{ToolCalls: []providers.ToolCall{{ID: "switch-1", Name: newContextToolName, Arguments: `{}`}}}, nil
	}
	return StepResult{Content: "continued in fresh context", StopReason: "stop"}, nil
}

type contextSwitchTools struct{}

func (contextSwitchTools) Definitions() []providers.ToolDefinition {
	return []providers.ToolDefinition{{Name: newContextToolName, InputSchema: map[string]any{"type": "object"}}}
}
func (contextSwitchTools) Execute(_ context.Context, _ providers.ToolCall) (string, error) {
	return `{"requested":true}`, nil
}

func TestRunToolLoopSwitchesOnlyAfterNewContextToolBatch(t *testing.T) {
	history := []providers.ChatMessage{{Role: "system", Content: "system"}, {Role: "user", Content: "long task"}}
	for index := 0; index < 120; index++ {
		history = append(history, providers.ChatMessage{Role: "assistant", Content: strings.Repeat("old context ", 1000)})
	}
	note := CompactionNote{Markdown: "Continue the long task.", CoveredMessages: len(history)}
	note.CoveredHash = CompactionHistoryHash(history[:note.CoveredMessages])
	var archived []providers.ChatMessage
	step := &contextSwitchStep{}
	res, err := RunToolLoop(context.Background(), history, LoopConfig{
		Model: "test", MaxSteps: 3, Tools: contextSwitchTools{},
		BeforeRequestContext: withContextWindowGuidance(nil),
		ArchiveHistory: func(_ context.Context, messages []providers.ChatMessage) (HistoryArchive, error) {
			archived = providers.CloneChatMessages(messages)
			return HistoryArchive{Seqs: []int{776, 777}, HeadSeq: 777}, nil
		},
		FreshContextTokens: FreshContextTargetTokens,
		FreshContext: func(_ context.Context, messages []providers.ChatMessage, headSeq, fixedTokens, targetTokens int) ([]providers.ChatMessage, error) {
			replacement, _, err := buildFreshContext(messages, note, true, headSeq, fixedTokens, targetTokens)
			return replacement, err
		},
	}, step)
	if err != nil {
		t.Fatalf("run loop: %v", err)
	}
	if !res.HistoryRewritten || res.HistoryArchiveHeadSeq != 777 {
		t.Fatalf("result = %+v", res)
	}
	if len(step.requests) != 2 || len(step.requests[1].Messages) >= len(step.requests[0].Messages) {
		t.Fatalf("request sizes = %d, %d", len(step.requests[0].Messages), len(step.requests[1].Messages))
	}
	if len(archived) != 2 || archived[0].Role != "assistant" || archived[1].Role != "tool" {
		t.Fatalf("archived tool batch = %+v", archived)
	}
	if len(res.DurableNewMessages) != 1 || res.DurableNewMessages[0].Content != "continued in fresh context" {
		t.Fatalf("pending durable messages = %+v", res.DurableNewMessages)
	}
}

func TestRunToolLoopFreshCommitIncludesConcurrentSteeringBeforeAction(t *testing.T) {
	history := []providers.ChatMessage{{Role: "system", Content: "instructions"}, {Role: "user", Content: "finish migration"}}
	for range 40 {
		history = append(history, providers.ChatMessage{Role: "assistant", Content: strings.Repeat("old progress ", 500)})
	}
	step := &contextSwitchStep{}
	committed := false
	res, err := RunToolLoop(context.Background(), history, LoopConfig{
		Model: "test", MaxSteps: 2, Tools: contextSwitchTools{}, FreshContextTokens: 6000,
		ArchiveHistory: func(context.Context, []providers.ChatMessage) (HistoryArchive, error) {
			return HistoryArchive{Seqs: []int{100, 101}, HeadSeq: 101}, nil
		},
		FreshContext: func(_ context.Context, messages []providers.ChatMessage, head, fixed, target int) ([]providers.ChatMessage, error) {
			replacement, _, err := buildFreshContext(messages, CompactionNote{}, false, head, fixed, target)
			return replacement, err
		},
		AcceptFreshContext: func(_ context.Context, messages []providers.ChatMessage, _ int) ([]providers.ChatMessage, int, error) {
			committed = true
			messages[1].Seq = 103
			return append(messages, providers.ChatMessage{Role: "user", Content: "stop before deployment", Seq: 102}), 103, nil
		},
		OnCompact: func(CompactInfo) {
			if !committed {
				t.Fatal("success emitted before durable commit")
			}
		},
	}, step)
	if err != nil {
		t.Fatal(err)
	}
	if len(step.requests) != 2 || res.HistoryArchiveHeadSeq != 103 {
		t.Fatalf("unexpected loop result: requests=%d head=%d", len(step.requests), res.HistoryArchiveHeadSeq)
	}
	seen := false
	for _, message := range step.requests[1].Messages {
		seen = seen || message.Seq == 102
	}
	if !seen {
		t.Fatal("provider acted before seeing committed concurrent steering")
	}
	if len(res.DurableNewMessages) != 1 {
		t.Fatalf("committed facts queued for duplicate persistence: %+v", res.DurableNewMessages)
	}
}

func TestRunToolLoopContextWindowRemindersPreserveValidRequests(t *testing.T) {
	for _, tt := range []struct {
		name          string
		guidance      bool
		inputTokens   int
		wantReminders int
		wantRollover  bool
	}{
		{name: "first request guidance", guidance: true, wantReminders: 1},
		{name: "ample budget", inputTokens: 10_000},
		{name: "low budget", inputTokens: 16_000, wantReminders: 1},
		{name: "failed rollover", inputTokens: 20_000, wantReminders: 2, wantRollover: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			history := []providers.ChatMessage{{Role: "system", Content: "system"}, {Role: "user", Content: "hello"}}
			usage := NewUsageTracker()
			usage.RecordResponse(&providers.TokenUsage{InputTokens: tt.inputTokens})
			rolloverCalled := false
			cfg := LoopConfig{
				Model: "test", MaxSteps: 1, UsageTracker: usage, CompactThresholdTokens: 20_000,
				ArchiveHistory: func(context.Context, []providers.ChatMessage) (HistoryArchive, error) {
					return HistoryArchive{}, nil
				},
				FreshContext: func(context.Context, []providers.ChatMessage, int, int, int) ([]providers.ChatMessage, error) {
					rolloverCalled = true
					return nil, ErrFreshContextTooLarge
				},
			}
			if tt.guidance {
				cfg.BeforeRequestContext = withContextWindowGuidance(nil)
			}
			step := &fakeStep{results: []StepResult{{Content: "done"}}}
			result, err := RunToolLoop(context.Background(), history, cfg, step)
			if err != nil {
				t.Fatal(err)
			}
			if rolloverCalled != tt.wantRollover {
				t.Fatalf("rollover called = %t, want %t", rolloverCalled, tt.wantRollover)
			}
			if len(step.calls) != 1 {
				t.Fatalf("provider requests = %d, want 1", len(step.calls))
			}
			request := step.calls[0]
			if _, err := providers.RepairAndValidateToolCallHistory(request.Messages); err != nil {
				t.Fatalf("provider rejected request: %v", err)
			}
			reminders := request.Messages[len(history):]
			if len(reminders) != tt.wantReminders {
				t.Fatalf("reminders = %d, want %d", len(reminders), tt.wantReminders)
			}
			for _, reminder := range reminders {
				if !reminder.Hidden {
					t.Fatal("context reminder leaked into the visible transcript")
				}
			}
			if len(result.DurableNewMessages) != 1 || result.DurableNewMessages[0].Content != "done" {
				t.Fatalf("durable messages = %+v, want only the assistant response", result.DurableNewMessages)
			}
		})
	}
}

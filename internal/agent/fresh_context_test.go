package agent

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/blueberrycongee/wuu/internal/compact"
	"github.com/blueberrycongee/wuu/internal/providers"
)

func TestBuildFreshContextBoundsStaleUncoveredSuffix(t *testing.T) {
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
	if err != nil {
		t.Fatalf("build fresh context: %v", err)
	}
	if got := 7_000 + estimateFreshContextMessages(replacement); got > FreshContextTargetTokens {
		t.Fatalf("replacement tokens = %d, target = %d", got, FreshContextTargetTokens)
	}
	if len(replacement) >= len(messages) {
		t.Fatalf("replacement did not shrink: %d >= %d", len(replacement), len(messages))
	}
	joined := compact.SummaryBodyFromContent(replacement[1].Content)
	if !strings.Contains(joined, "through Seq 990") || !strings.Contains(joined, "history_search") {
		t.Fatalf("missing recovery guidance: %q", joined)
	}
	if replacement[len(replacement)-2].Content != "finish the current implementation" {
		t.Fatalf("latest complete workset was not retained: %+v", replacement[len(replacement)-2:])
	}
	if reanchored.CoveredMessages != 2 || reanchored.CoveredHash == "" {
		t.Fatalf("reanchored note = %+v", reanchored)
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
	note := CompactionNote{Markdown: "Continue the long task.", CoveredMessages: 2}
	note.CoveredHash = CompactionHistoryHash(history[:note.CoveredMessages])
	var archived []providers.ChatMessage
	step := &contextSwitchStep{}
	res, err := RunToolLoop(context.Background(), history, LoopConfig{
		Model: "test", MaxSteps: 3, Tools: contextSwitchTools{},
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

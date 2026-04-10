package agent

import (
	"context"
	"errors"
	"strings"

	"github.com/blueberrycongee/wuu/internal/compact"
	"github.com/blueberrycongee/wuu/internal/providers"
)

// ToolExecutor executes model-requested tool calls.
type ToolExecutor interface {
	Definitions() []providers.ToolDefinition
	Execute(ctx context.Context, call providers.ToolCall) (string, error)
}

// Runner manages one multi-step coding turn against a non-streaming
// provider. It is a thin wrapper around RunToolLoop that supplies a
// chatStep adapter (Step → providers.Client.Chat).
type Runner struct {
	Client       providers.Client
	Tools        ToolExecutor
	Model        string
	SystemPrompt string
	MaxSteps     int
	Temperature  float64
}

// RunResult is the structured outcome of a Runner.RunWithUsage call.
type RunResult struct {
	Content      string
	InputTokens  int
	OutputTokens int
}

// Run executes one prompt with optional tool-use loop.
func (r *Runner) Run(ctx context.Context, prompt string) (string, error) {
	res, err := r.RunWithUsage(ctx, prompt, nil)
	if err != nil {
		return "", err
	}
	return res.Content, nil
}

// RunWithUsage is like Run but reports per-call token usage to the
// optional onUsage callback (called once per LLM round-trip) and
// returns cumulative totals in the result.
func (r *Runner) RunWithUsage(ctx context.Context, prompt string, onUsage func(input, output int)) (RunResult, error) {
	if r.Client == nil {
		return RunResult{}, errors.New("client is required")
	}
	if strings.TrimSpace(r.Model) == "" {
		return RunResult{}, errors.New("model is required")
	}
	if strings.TrimSpace(prompt) == "" {
		return RunResult{}, errors.New("prompt is required")
	}

	// Build the initial conversation: optional system prompt + the
	// user's request. The shared loop takes it from there.
	history := make([]providers.ChatMessage, 0, 2)
	if strings.TrimSpace(r.SystemPrompt) != "" {
		history = append(history, providers.ChatMessage{Role: "system", Content: r.SystemPrompt})
	}
	history = append(history, providers.ChatMessage{Role: "user", Content: prompt})

	cfg := LoopConfig{
		Tools:       r.Tools,
		Model:       r.Model,
		Temperature: r.Temperature,
		MaxSteps:    r.MaxSteps,
		OnUsage:     onUsage,
		Compact: func(ctx context.Context, messages []providers.ChatMessage) ([]providers.ChatMessage, error) {
			return compact.Compact(ctx, messages, r.Client, r.Model)
		},
	}

	res, err := RunToolLoop(ctx, history, cfg, &chatStep{client: r.Client})
	return RunResult{
		Content:      res.Content,
		InputTokens:  res.InputTokens,
		OutputTokens: res.OutputTokens,
	}, err
}

// chatStep adapts a non-streaming providers.Client to the Step
// interface. One Execute call = one Chat round-trip.
type chatStep struct {
	client providers.Client
}

func (s *chatStep) Execute(ctx context.Context, req providers.ChatRequest) (StepResult, error) {
	resp, err := s.client.Chat(ctx, req)
	if err != nil {
		return StepResult{}, err
	}
	return StepResult{
		Content:    resp.Content,
		ToolCalls:  resp.ToolCalls,
		Truncated:  resp.Truncated,
		StopReason: resp.StopReason,
		Usage:      resp.Usage,
	}, nil
}

package agent

import (
	"context"
	"errors"
	"testing"

	"github.com/blueberrycongee/wuu/internal/providers"
)

func TestRunner_RunSimple(t *testing.T) {
	client := &fakeClient{responses: []providers.ChatResponse{{Content: "done"}}}
	runner := Runner{Client: client, Model: "gpt-test", SystemPrompt: "sys"}

	answer, err := runner.Run(context.Background(), "hello")
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if answer != "done" {
		t.Fatalf("unexpected answer: %s", answer)
	}
}

func TestRunner_RunWithToolCall(t *testing.T) {
	client := &fakeClient{responses: []providers.ChatResponse{
		{
			ToolCalls: []providers.ToolCall{{ID: "call_1", Name: "run_shell", Arguments: `{"command":"echo hi"}`}},
		},
		{Content: "final answer"},
	}}
	tool := &fakeTools{}
	runner := Runner{Client: client, Tools: tool, Model: "gpt-test", SystemPrompt: "sys", MaxSteps: 4}

	answer, err := runner.Run(context.Background(), "task")
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if answer != "final answer" {
		t.Fatalf("unexpected answer: %s", answer)
	}
	if len(tool.calls) != 1 {
		t.Fatalf("expected one tool call, got %d", len(tool.calls))
	}

	lastReq := client.requests[len(client.requests)-1]
	foundToolMessage := false
	for _, msg := range lastReq.Messages {
		if msg.Role == "tool" && msg.ToolCallID == "call_1" {
			foundToolMessage = true
			break
		}
	}
	if !foundToolMessage {
		t.Fatal("expected tool message in follow-up request")
	}
}

func TestRunner_TruncationRecovery(t *testing.T) {
	client := &fakeClient{responses: []providers.ChatResponse{
		{Content: "part one ", Truncated: true, StopReason: "length"},
		{Content: "part two ", Truncated: true, StopReason: "max_tokens"},
		{Content: "part three."},
	}}
	runner := Runner{Client: client, Model: "gpt-test"}

	answer, err := runner.Run(context.Background(), "write a story")
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if answer != "part one part two part three." {
		t.Fatalf("expected concatenated answer, got %q", answer)
	}
	// Each recovery round should have appended a continuation prompt
	// to the conversation that the next request observes.
	if len(client.requests) != 3 {
		t.Fatalf("expected 3 chat calls, got %d", len(client.requests))
	}
	last := client.requests[2].Messages
	foundContinuePrompts := 0
	for _, msg := range last {
		if msg.Role == "user" && msg.Content == truncationContinuePrompt {
			foundContinuePrompts++
		}
	}
	if foundContinuePrompts != 2 {
		t.Fatalf("expected 2 continue prompts in final request, got %d", foundContinuePrompts)
	}
}

func TestRunner_TruncationRecoveryCappedAtLimit(t *testing.T) {
	// All responses stay truncated. Runner should bail with the
	// concatenated partial after maxTruncationRecoveries attempts.
	responses := make([]providers.ChatResponse, 0, maxTruncationRecoveries+1)
	for i := 0; i <= maxTruncationRecoveries; i++ {
		responses = append(responses, providers.ChatResponse{Content: "x", Truncated: true, StopReason: "length"})
	}
	client := &fakeClient{responses: responses}
	runner := Runner{Client: client, Model: "gpt-test"}

	answer, err := runner.Run(context.Background(), "loop")
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	// 3 buffered partials + 1 final partial that hit the cap
	expected := "xxxx"
	if answer != expected {
		t.Fatalf("expected %q, got %q", expected, answer)
	}
}

func TestRunner_ContextOverflowAutoCompact(t *testing.T) {
	overflow := &providers.HTTPError{StatusCode: 400, Body: "context_length_exceeded", ContextOverflow: true}
	client := &fakeClient{
		responses: []providers.ChatResponse{
			// 1) initial real call: overflow
			// 2) compact() will issue a summarization Chat call
			{Content: "summary of older turns"},
			// 3) re-issued real call after compaction: success
			{Content: "ok done"},
		},
		errors: []error{overflow, nil, nil},
	}
	// Need enough message history for compact() to actually trim some.
	runner := Runner{Client: client, Model: "gpt-test", SystemPrompt: "sys"}

	// Inject a long history by going through Run with repeated tool
	// calls? Simpler: just call directly. The compact package keeps
	// the last 4 messages, so we need >4 in the conversation. The
	// runner starts with system+user (2). To exercise the auto-compact
	// path we craft a tool-loop scenario.
	answer, err := runner.Run(context.Background(), "do work")
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if answer != "ok done" {
		t.Fatalf("expected 'ok done', got %q", answer)
	}
}

func TestRunner_MaxStepsExceeded(t *testing.T) {
	client := &fakeClient{responses: []providers.ChatResponse{{ToolCalls: []providers.ToolCall{{ID: "c", Name: "run_shell", Arguments: `{}`}}}}}
	runner := Runner{Client: client, Tools: &fakeTools{}, Model: "gpt-test", MaxSteps: 1}

	_, err := runner.Run(context.Background(), "task")
	if err == nil {
		t.Fatal("expected max steps error")
	}
}

type fakeClient struct {
	responses []providers.ChatResponse
	errors    []error // optional, indexed parallel to responses
	requests  []providers.ChatRequest
	idx       int
}

func (f *fakeClient) Chat(_ context.Context, req providers.ChatRequest) (providers.ChatResponse, error) {
	f.requests = append(f.requests, req)
	if f.idx >= len(f.responses) {
		return providers.ChatResponse{}, errors.New("unexpected chat call")
	}
	resp := f.responses[f.idx]
	var err error
	if f.idx < len(f.errors) {
		err = f.errors[f.idx]
	}
	f.idx++
	if err != nil {
		return providers.ChatResponse{}, err
	}
	return resp, nil
}

type fakeTools struct {
	calls []providers.ToolCall
}

func (f *fakeTools) Definitions() []providers.ToolDefinition {
	return []providers.ToolDefinition{{Name: "run_shell", InputSchema: map[string]any{"type": "object"}}}
}

func (f *fakeTools) Execute(_ context.Context, call providers.ToolCall) (string, error) {
	f.calls = append(f.calls, call)
	return `{"ok":true}`, nil
}

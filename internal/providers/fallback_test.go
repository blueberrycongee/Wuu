package providers

import (
	"context"
	"testing"
)

type stubStreamClient struct {
	chatErr error
}

func (s *stubStreamClient) Chat(_ context.Context, req ChatRequest) (ChatResponse, error) {
	return ChatResponse{}, s.chatErr
}
func (s *stubStreamClient) StreamChat(_ context.Context, req ChatRequest) (<-chan StreamEvent, error) {
	return nil, s.chatErr
}

func TestFallbackClient_TriggersAfterThreshold(t *testing.T) {
	overloadErr := &HTTPError{StatusCode: 529, Body: "overloaded"}
	stub := &stubStreamClient{chatErr: overloadErr}
	fc := NewFallbackClient(stub, "fallback-model", 3)

	for i := 0; i < 3; i++ {
		if fc.IsFallbackActive() {
			t.Fatalf("fallback should not be active after %d failures", i)
		}
		fc.Chat(context.Background(), ChatRequest{Model: "primary"})
	}

	if !fc.IsFallbackActive() {
		t.Error("fallback should be active after 3 consecutive overloads")
	}
}

func TestFallbackClient_ResetsOnSuccess(t *testing.T) {
	overloadErr := &HTTPError{StatusCode: 529, Body: "overloaded"}
	stub := &stubStreamClient{chatErr: overloadErr}
	fc := NewFallbackClient(stub, "fallback-model", 2)

	// Trigger fallback.
	fc.Chat(context.Background(), ChatRequest{Model: "primary"})
	fc.Chat(context.Background(), ChatRequest{Model: "primary"})
	if !fc.IsFallbackActive() {
		t.Fatal("expected fallback active")
	}

	// Simulate success.
	stub.chatErr = nil
	fc.Chat(context.Background(), ChatRequest{Model: "primary"})
	if fc.IsFallbackActive() {
		t.Error("fallback should be reset after successful request")
	}
}

func TestFallbackClient_NonOverloadDoesNotTrigger(t *testing.T) {
	authErr := &HTTPError{StatusCode: 401, Body: "unauthorized"}
	stub := &stubStreamClient{chatErr: authErr}
	fc := NewFallbackClient(stub, "fallback-model", 1)

	fc.Chat(context.Background(), ChatRequest{Model: "primary"})
	if fc.IsFallbackActive() {
		t.Error("non-overload errors should not trigger fallback")
	}
}

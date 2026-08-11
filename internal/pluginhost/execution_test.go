package pluginhost

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/blueberrycongee/wuu/internal/toolresult"
)

func TestExecutionTrackerOwnershipAndLiveness(t *testing.T) {
	tracker := NewExecutionTracker()
	first := tracker.Begin("alpha")
	second := tracker.Begin("alpha")
	if first == second || first == "" || second == "" {
		t.Fatalf("execution ids must be unique per dispatch: %q %q", first, second)
	}

	if err := tracker.RecordUpdate("alpha", ExecutionUpdateParams{ExecutionID: first, Message: "halfway", Detail: json.RawMessage(`{"pct":50}`)}); err != nil {
		t.Fatalf("record update: %v", err)
	}
	snapshot := tracker.Snapshot()
	if len(snapshot) != 2 {
		t.Fatalf("snapshot = %+v", snapshot)
	}
	if err := tracker.RecordUpdate("beta", ExecutionUpdateParams{ExecutionID: first, Message: "intruder"}); err == nil || err.Code != "service_not_authorized" {
		t.Fatalf("foreign update = %v, want service_not_authorized", err)
	}
	if err := tracker.RecordUpdate("alpha", ExecutionUpdateParams{ExecutionID: "", Message: "blank"}); err == nil || err.Code != "invalid_request" {
		t.Fatalf("blank update = %v, want invalid_request", err)
	}

	tracker.End(first)
	if err := tracker.RecordUpdate("alpha", ExecutionUpdateParams{ExecutionID: first, Message: "late"}); err == nil || err.Code != "execution_not_found" {
		t.Fatalf("late update = %v, want execution_not_found", err)
	}
	if got := len(tracker.Snapshot()); got != 1 {
		t.Fatalf("snapshot after end = %d, want 1", got)
	}
	tracker.End(first)
	tracker.End(second)
	if got := len(tracker.Snapshot()); got != 0 {
		t.Fatalf("snapshot after ending both = %d, want 0", got)
	}
}

type executionToolClient struct {
	fakeClient
	tools []ToolRegistration
	hook  func(ToolExecuteParams) (ToolExecuteResult, error)
}

func (c *executionToolClient) Tools() []ToolRegistration {
	return append([]ToolRegistration(nil), c.tools...)
}

func (c *executionToolClient) ExecuteTool(_ context.Context, params ToolExecuteParams) (ToolExecuteResult, error) {
	return c.hook(params)
}

func TestHostExecuteToolMintsScopedExecutionID(t *testing.T) {
	textResult := ToolExecuteResult{Result: toolresult.Result{
		Content: []toolresult.ContentPart{{Type: toolresult.ContentTypeText, Text: "done"}},
	}}
	var host *Host
	var seen []string
	client := &executionToolClient{
		fakeClient: fakeClient{id: "acme.lookup", status: Status{State: StateActive}},
		tools: []ToolRegistration{{
			ID:          "search",
			Description: "Search local plugin data",
			InputSchema: map[string]any{"type": "object"},
		}},
		hook: func(params ToolExecuteParams) (ToolExecuteResult, error) {
			seen = append(seen, params.ExecutionID)
			if params.ExecutionID == "" {
				t.Error("tool.execute params carry no execution_id")
			}
			if err := host.RecordExecutionUpdate("acme.lookup", ExecutionUpdateParams{ExecutionID: params.ExecutionID, Message: "working"}); err != nil {
				t.Errorf("update during execution: %v", err)
			}
			if err := host.RecordExecutionUpdate("other.plugin", ExecutionUpdateParams{ExecutionID: params.ExecutionID, Message: "intruder"}); err == nil || err.Code != "service_not_authorized" {
				t.Errorf("foreign update during execution = %v, want service_not_authorized", err)
			}
			return textResult, nil
		},
	}
	host = New(client)

	definitions := host.ToolDefinitions()
	if len(definitions) != 1 {
		t.Fatalf("definitions = %+v", definitions)
	}
	for i := 0; i < 2; i++ {
		if _, err := host.ExecuteTool(context.Background(), definitions[0].Name, ToolExecuteInput{Arguments: json.RawMessage(`{}`)}); err != nil {
			t.Fatalf("execute %d: %v", i, err)
		}
	}
	if len(seen) != 2 || seen[0] == "" || seen[0] == seen[1] {
		t.Fatalf("execution ids = %v, want two distinct non-empty ids", seen)
	}
	if got := len(host.ExecutionSnapshots()); got != 0 {
		t.Fatalf("live executions after return = %d, want 0", got)
	}
	if err := host.RecordExecutionUpdate("acme.lookup", ExecutionUpdateParams{ExecutionID: seen[0], Message: "late"}); err == nil || err.Code != "execution_not_found" {
		t.Fatalf("late update = %v, want execution_not_found", err)
	}
}

func TestHostInvokeCapabilityMintsScopedExecutionID(t *testing.T) {
	var host *Host
	var seen string
	client := &fakeCapabilityClient{
		fakeClient: &fakeClient{id: "acme.slow", status: Status{State: StateActive}},
		capabilities: []CapabilityDescriptor{{
			ID:      CapabilityAgentTurnCompleted,
			Version: 1,
		}},
		invoke: func(params CapabilityInvokeParams) (CapabilityInvokeResult, error) {
			seen = params.ExecutionID
			if params.ExecutionID == "" {
				t.Error("capability.invoke params carry no execution_id")
			}
			if err := host.RecordExecutionUpdate("acme.slow", ExecutionUpdateParams{ExecutionID: params.ExecutionID, Message: "compacting"}); err != nil {
				t.Errorf("update during capability execution: %v", err)
			}
			output, _ := json.Marshal(AgentTurnCompletedOutput{})
			return CapabilityInvokeResult{Output: output}, nil
		},
	}
	host = New(client)

	capability, ok := host.Capability("acme.slow", CapabilityAgentTurnCompleted)
	if !ok {
		t.Fatal("capability not registered")
	}
	if err := host.InvokeCapability(context.Background(), capability, AgentTurnCompletedInput{}, &AgentTurnCompletedOutput{}); err != nil {
		t.Fatalf("invoke: %v", err)
	}
	if seen == "" {
		t.Fatal("no execution id observed")
	}
	if err := host.RecordExecutionUpdate("acme.slow", ExecutionUpdateParams{ExecutionID: seen, Message: "late"}); err == nil || !strings.Contains(err.Message, "not live") {
		t.Fatalf("late update = %v, want execution_not_found", err)
	}
}

func TestExecutionTrackerToolScopeIsTrustedAndEndsWaiters(t *testing.T) {
	tracker := NewExecutionTracker()
	id := tracker.BeginTool("ask-user", context.Background(), ToolExecuteInput{
		SessionID: "session-1", ThreadID: "thread-1", TurnID: "turn-1",
		ActorID: "actor-1", CallID: "call-1", Tool: "plugin_ask_user",
	})
	scope, serviceErr := tracker.ResolveTool("ask-user", id)
	if serviceErr != nil {
		t.Fatal(serviceErr)
	}
	if scope.ThreadID != "thread-1" || scope.TurnID != "turn-1" || scope.CallID != "call-1" {
		t.Fatalf("scope = %+v", scope.ExecutionSnapshot)
	}
	if _, serviceErr := tracker.ResolveTool("other", id); serviceErr == nil || serviceErr.Code != "service_not_authorized" {
		t.Fatalf("foreign resolve = %#v", serviceErr)
	}

	tracker.End(id)
	select {
	case <-scope.Context.Done():
		if !IsUserQuestionErrorCode(context.Cause(scope.Context), "execution_cancelled") {
			t.Fatalf("scope cause = %v", context.Cause(scope.Context))
		}
	case <-time.After(time.Second):
		t.Fatal("End did not cancel the execution scope")
	}
	if _, serviceErr := tracker.ResolveTool("ask-user", id); serviceErr == nil || serviceErr.Code != "execution_not_found" {
		t.Fatalf("late resolve = %#v", serviceErr)
	}
}

func TestExecutionTrackerRejectsCapabilityAsQuestionOwner(t *testing.T) {
	tracker := NewExecutionTracker()
	id := tracker.Begin("ask-user")
	defer tracker.End(id)
	if _, serviceErr := tracker.ResolveTool("ask-user", id); serviceErr == nil || serviceErr.Code != "invalid_execution_scope" {
		t.Fatalf("capability resolve = %#v", serviceErr)
	}
}

func TestExecutionTrackerCancelAllKeepsGenerationCause(t *testing.T) {
	tracker := NewExecutionTracker()
	id := tracker.BeginTool("ask-user", context.Background(), ToolExecuteInput{ThreadID: "thread", TurnID: "turn", CallID: "call"})
	scope, serviceErr := tracker.ResolveTool("ask-user", id)
	if serviceErr != nil {
		t.Fatal(serviceErr)
	}
	tracker.CancelAll(&UserQuestionError{Code: "generation_closed", Message: "retired"})
	<-scope.Context.Done()
	if !IsUserQuestionErrorCode(context.Cause(scope.Context), "generation_closed") {
		t.Fatalf("scope cause = %v", context.Cause(scope.Context))
	}
	tracker.End(id)
}

package appserver

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/blueberrycongee/wuu/internal/pluginhost"
	"github.com/blueberrycongee/wuu/internal/providers"
	pluginapi "github.com/blueberrycongee/wuu/packages/plugin-go"
	"github.com/blueberrycongee/wuu/plugins/goal"
)

// Exercise the extension's public messages against the real session router and
// settled-turn broadcast; storage writes signal completion without polling.
type goalBridge struct {
	handler pluginapi.Handler
	server  *Server
	mu      sync.Mutex
	value   *string
	limited chan struct{}
	once    sync.Once
}

func (b *goalBridge) ID() string { return "goal" }
func (b *goalBridge) Status() pluginhost.Status {
	return pluginhost.Status{ID: b.ID(), State: pluginhost.StateActive}
}
func (b *goalBridge) Close(ctx context.Context) error { return b.handler.Shutdown(ctx) }
func (b *goalBridge) ProtocolVersion() int            { return pluginhost.CapabilityProtocolVersion }
func (b *goalBridge) Capabilities() []pluginhost.CapabilityDescriptor {
	raw, _ := json.Marshal(b.handler.Definition.Capabilities)
	var result []pluginhost.CapabilityDescriptor
	_ = json.Unmarshal(raw, &result)
	return result
}
func (b *goalBridge) InvokeCapability(ctx context.Context, call pluginhost.CapabilityInvokeParams) (pluginhost.CapabilityInvokeResult, error) {
	output, err := b.handler.InvokeCapability(ctx, b, pluginapi.CapabilityCall{Capability: call.Capability, Input: call.Input, Output: call.Output})
	return pluginhost.CapabilityInvokeResult{Output: output}, err
}
func (b *goalBridge) InitializeParams() pluginapi.InitializeParams {
	return pluginapi.InitializeParams{}
}
func (b *goalBridge) CallHost(ctx context.Context, _ string, input, out any) error {
	raw, _ := json.Marshal(input)
	var call struct {
		Service string
		Params  json.RawMessage
	}
	_ = json.Unmarshal(raw, &call)
	var result any
	var err error
	switch call.Service {
	case pluginapi.HostServiceStorageGet:
		b.mu.Lock()
		result = pluginapi.StorageGetResult{Value: b.value}
		b.mu.Unlock()
	case pluginapi.HostServiceStorageSet:
		var p pluginapi.StorageSetParams
		_ = json.Unmarshal(call.Params, &p)
		b.mu.Lock()
		b.value = &p.Value
		b.mu.Unlock()
		result = struct{}{}
		var state map[string]goal.Goal
		_ = json.Unmarshal([]byte(p.Value), &state)
		for _, g := range state {
			if g.Status == "budget_limited" {
				b.once.Do(func() { close(b.limited) })
			}
		}
	case pluginapi.HostServiceSessionSend:
		var p pluginhost.SessionSendParams
		_ = json.Unmarshal(call.Params, &p)
		result, err = b.server.sendPluginSession(ctx, b.ID(), p)
	case pluginapi.HostServiceSessionInspect:
		var p pluginhost.SessionInspectParams
		_ = json.Unmarshal(call.Params, &p)
		result, err = b.server.inspectPluginSession(ctx, b.ID(), p)
	case pluginapi.HostServiceSessionCancel:
		var p pluginhost.SessionCancelParams
		_ = json.Unmarshal(call.Params, &p)
		result, err = b.server.cancelPluginSession(ctx, b.ID(), p)
	default:
		return fmt.Errorf("unexpected service %s", call.Service)
	}
	if err != nil {
		return err
	}
	raw, err = json.Marshal(result)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, out)
}

func TestGoalExtensionContinuesUserSessionThroughRealTurnLifecycle(t *testing.T) {
	response := providersResponse("working")
	response.Usage = &providers.TokenUsage{InputTokens: 5, OutputTokens: 5}
	rt := newTestRuntime(t, &fakeClient{response: response})
	bridge := &goalBridge{handler: goal.Handler(), limited: make(chan struct{})}
	for _, capability := range bridge.Capabilities() {
		if err := pluginhost.ValidateCapabilityDescriptor(capability); err != nil {
			t.Fatal(err)
		}
	}
	rt.PluginHost = pluginhost.New(bridge)
	out := &lockedBuffer{}
	srv := New(rt, out)
	bridge.server = srv
	t.Cleanup(srv.Close)
	if err := bridge.handler.Initialize(context.Background(), bridge, pluginapi.InitializeParams{}); err != nil {
		t.Fatal(err)
	}
	if err := bridge.handler.Activate(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := srv.handleLine(context.Background(), []byte(`{"id":"goal-thread","method":"thread/start"}`)); err != nil {
		t.Fatal(err)
	}
	threadID := remarshal[ThreadStartResult](t, responseByID(t, parseOutput(t, out.String()), "goal-thread")["result"]).Thread.ID
	input, _ := json.Marshal(map[string]any{"method": "create_goal", "input": map[string]any{"thread_id": threadID, "objective": "Finish the bounded work", "token_budget": 15}})
	if _, err := bridge.handler.InvokeCapability(context.Background(), bridge, pluginapi.CapabilityCall{Capability: "plugin.client.request", Input: input}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-bridge.limited:
	case <-time.After(5 * time.Second):
		t.Fatalf("goal did not stop at budget; output=%s", out.String())
	}
	result, err := bridge.handler.ExecuteTool(context.Background(), bridge, pluginapi.ToolCall{ToolID: "get_goal", SessionID: threadID, Arguments: json.RawMessage(`{}`)})
	if err != nil {
		t.Fatal(err)
	}
	var snapshot struct{ Goal goal.Goal }
	if err := json.Unmarshal(result.StructuredContent, &snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot.Goal.TokensUsed != 20 || snapshot.Goal.Status != "budget_limited" {
		t.Fatalf("goal = %+v", snapshot.Goal)
	}
}

func TestSharedSessionPluginCanInspectAndCancelOnlyItsOwnWork(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	rt := newTestRuntime(t, &fakeClient{response: providersResponse("done"), onChat: func(_ int, _ providers.ChatRequest) { close(started); <-release }})
	out := &lockedBuffer{}
	srv := New(rt, out)
	t.Cleanup(func() { close(release); srv.Close() })
	if err := srv.handleLine(context.Background(), []byte(`{"id":"shared","method":"thread/start"}`)); err != nil {
		t.Fatal(err)
	}
	id := remarshal[ThreadStartResult](t, responseByID(t, parseOutput(t, out.String()), "shared")["result"]).Thread.ID
	send := func(plugin, request string) pluginhost.SessionSendResult {
		t.Helper()
		r, err := srv.sendPluginSession(context.Background(), plugin, pluginhost.SessionSendParams{RequestID: request, SessionID: id, Input: pluginhost.SessionInput{Prompt: "work"}})
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	running := send("scheduler", "first")
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("not started")
	}
	queued := send("workflow", "queued")
	inspected, err := srv.inspectPluginSession(context.Background(), "workflow", pluginhost.SessionInspectParams{SessionID: id, RequestID: "queued"})
	if err != nil || inspected.Turn == nil || inspected.Turn.QueueID != queued.QueueID {
		t.Fatalf("inspect = %+v %v", inspected, err)
	}
	other, err := srv.inspectPluginSession(context.Background(), "other", pluginhost.SessionInspectParams{SessionID: id, RequestID: "queued"})
	if err != nil || other.Turn != nil {
		t.Fatalf("leaked other plugin request: %+v %v", other, err)
	}
	if _, err := srv.cancelPluginSession(context.Background(), "workflow", pluginhost.SessionCancelParams{SessionID: id}); err == nil {
		t.Fatal("unscoped cancellation accepted")
	}
	if _, err := srv.cancelPluginSession(context.Background(), "workflow", pluginhost.SessionCancelParams{SessionID: id, TurnID: running.TurnID}); err == nil {
		t.Fatal("other plugin turn cancelled")
	}
	cancelled, err := srv.cancelPluginSession(context.Background(), "other", pluginhost.SessionCancelParams{SessionID: id, QueueID: queued.QueueID})
	if err != nil || cancelled.Cancelled {
		t.Fatal("other plugin queue cancelled")
	}
	cancelled, err = srv.cancelPluginSession(context.Background(), "workflow", pluginhost.SessionCancelParams{SessionID: id, QueueID: queued.QueueID})
	if err != nil || !cancelled.Cancelled {
		t.Fatalf("owned queue cancellation: %+v %v", cancelled, err)
	}
	cancelled, err = srv.cancelPluginSession(context.Background(), "scheduler", pluginhost.SessionCancelParams{SessionID: id, TurnID: running.TurnID})
	if err != nil || !cancelled.Cancelled {
		t.Fatalf("owned running cancellation: %+v %v", cancelled, err)
	}
}

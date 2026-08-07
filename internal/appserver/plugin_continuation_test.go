package appserver

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/blueberrycongee/wuu/internal/pluginhost"
	"github.com/blueberrycongee/wuu/internal/providers"
)

type continuationTestRuntime struct {
	id          string
	output      pluginhost.AgentContinuationOutput
	prepareOnly bool
	calls       []pluginhost.AgentContinuationInput
}

func (c *continuationTestRuntime) ID() string               { return c.id }
func (c *continuationTestRuntime) Hooks() []pluginhost.Hook { return nil }
func (c *continuationTestRuntime) Status() pluginhost.Status {
	return pluginhost.Status{ID: c.id, State: pluginhost.StateActive}
}
func (c *continuationTestRuntime) Invoke(context.Context, pluginhost.InvokeParams) (pluginhost.InvokeResult, error) {
	return pluginhost.InvokeResult{}, nil
}
func (c *continuationTestRuntime) Close(context.Context) error { return nil }
func (c *continuationTestRuntime) ProtocolVersion() int        { return pluginhost.CapabilityProtocolVersion }
func (c *continuationTestRuntime) Capabilities() []pluginhost.CapabilityDescriptor {
	return []pluginhost.CapabilityDescriptor{{ID: pluginhost.CapabilityAgentContinuation, Kind: "decision", Version: 1}}
}
func (c *continuationTestRuntime) InvokeCapability(_ context.Context, params pluginhost.CapabilityInvokeParams) (pluginhost.CapabilityInvokeResult, error) {
	var input pluginhost.AgentContinuationInput
	if err := json.Unmarshal(params.Input, &input); err != nil {
		return pluginhost.CapabilityInvokeResult{}, err
	}
	c.calls = append(c.calls, input)
	decision := c.output
	if c.prepareOnly && input.Phase == pluginhost.ContinuationPhaseProbe {
		decision = pluginhost.AgentContinuationOutput{}
	}
	output, err := json.Marshal(decision)
	return pluginhost.CapabilityInvokeResult{Output: output}, err
}

func TestPluginContinuationProbesAndPreparesRequestOnlyContext(t *testing.T) {
	srv, _, _ := newPluginStateTestServer(t)
	client := &continuationTestRuntime{id: "continuation-owner", output: pluginhost.AgentContinuationOutput{
		Continue: true,
		Blocks:   []pluginhost.AgentContinuationBlock{{Kind: "PLUGIN_WORK", Title: "Plugin work", Source: "plugin.test", Content: "continue this task"}},
	}}
	srv.rt.PluginHost = pluginhost.New(client)

	continued, segments, _, err := srv.pluginContinuation(context.Background(), "thread-1", pluginhost.ContinuationPhaseProbe)
	if err != nil || !continued || len(segments) != 0 {
		t.Fatalf("probe = continued %v segments %v err %v", continued, segments, err)
	}
	continued, segments, _, err = srv.pluginContinuation(context.Background(), "thread-1", pluginhost.ContinuationPhasePrepare)
	if err != nil || !continued || len(segments) != 1 || len(segments[0].Blocks) != 1 || segments[0].Blocks[0].Content != "continue this task" {
		t.Fatalf("prepare = continued %v segments %+v err %v", continued, segments, err)
	}
	if len(client.calls) != 2 || client.calls[0].Phase != pluginhost.ContinuationPhaseProbe || client.calls[1].Phase != pluginhost.ContinuationPhasePrepare {
		t.Fatalf("calls = %+v", client.calls)
	}
}

func TestPluginContinuationRejectsPreparedTurnWithoutContext(t *testing.T) {
	srv, _, _ := newPluginStateTestServer(t)
	srv.rt.PluginHost = pluginhost.New(&continuationTestRuntime{
		id: "bad-continuation", output: pluginhost.AgentContinuationOutput{Continue: true},
	})
	if continued, _, _, err := srv.pluginContinuation(context.Background(), "thread-1", pluginhost.ContinuationPhasePrepare); err == nil || continued {
		t.Fatalf("continued = %v, err = %v", continued, err)
	}
}

func TestPluginContinuationStartsInternalTurnWithPluginContext(t *testing.T) {
	client := &fakeClient{response: providers.ChatResponse{Content: "continued"}}
	rt := newTestRuntime(t, client)
	continuation := &continuationTestRuntime{id: "continuation-owner", prepareOnly: true, output: pluginhost.AgentContinuationOutput{
		Continue: true,
		Display:  &pluginhost.AgentContinuationDisplay{Text: "Goal 持续推进中", Name: "plugin-test-continuation"},
		Blocks: []pluginhost.AgentContinuationBlock{{
			Kind: "PLUGIN_WORK", Title: "Plugin work", Source: "plugin.test", Content: "opaque continuation context",
		}},
	}}
	rt.PluginHost = pluginhost.New(continuation)
	out := &lockedBuffer{}
	srv := New(rt, out)

	if err := srv.handleLine(context.Background(), []byte(`{"id":"1","method":"thread/start"}`)); err != nil {
		t.Fatalf("thread/start: %v", err)
	}
	threadID := remarshal[ThreadStartResult](t, responseByID(t, parseOutput(t, out.String()), "1")["result"]).Thread.ID
	started, err := srv.startPluginContinuationTurn(context.Background(), threadID)
	if err != nil {
		t.Fatalf("startPluginContinuationTurn: %v", err)
	}
	if !started {
		t.Fatal("expected plugin continuation turn to start")
	}
	waitForTurnCompletedForThread(t, out, threadID)
	var displayItem *ThreadItem
	for _, notification := range parseOutput(t, out.String()) {
		if notification["method"] != NotificationTurnStarted {
			continue
		}
		started := remarshal[TurnStartedNotification](t, notification["params"])
		if started.ThreadID != threadID || len(started.Turn.Items) == 0 {
			continue
		}
		displayItem = &started.Turn.Items[0]
	}
	if displayItem == nil || displayItem.Type != ThreadItemUserMessage || displayItem.Text != "Goal 持续推进中" || !displayItem.ReadOnly {
		t.Fatalf("continuation display item = %+v", displayItem)
	}

	client.mu.Lock()
	requests := append([]providers.ChatRequest(nil), client.requests...)
	client.mu.Unlock()
	if len(requests) != 1 {
		t.Fatalf("provider requests = %d, want 1", len(requests))
	}
	found := false
	for _, message := range requests[0].Messages {
		if strings.Contains(message.Content, "opaque continuation context") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("plugin continuation context missing from request: %+v", requests[0].Messages)
	}
	if len(continuation.calls) == 0 || continuation.calls[0].Phase != pluginhost.ContinuationPhasePrepare {
		t.Fatalf("continuation calls = %+v", continuation.calls)
	}
}

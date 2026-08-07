package appserver

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/blueberrycongee/wuu/internal/agent"
	"github.com/blueberrycongee/wuu/internal/pluginhost"
	"github.com/blueberrycongee/wuu/internal/providers"
	"github.com/blueberrycongee/wuu/internal/runtime"
	"github.com/blueberrycongee/wuu/internal/session"
)

type pluginTurnLifecycleClient struct {
	id    string
	calls chan pluginhost.AgentTurnLifecycleInput
}

func (c *pluginTurnLifecycleClient) ID() string               { return c.id }
func (c *pluginTurnLifecycleClient) Hooks() []pluginhost.Hook { return nil }
func (c *pluginTurnLifecycleClient) Status() pluginhost.Status {
	return pluginhost.Status{ID: c.id, State: pluginhost.StateActive}
}
func (c *pluginTurnLifecycleClient) Invoke(context.Context, pluginhost.InvokeParams) (pluginhost.InvokeResult, error) {
	return pluginhost.InvokeResult{}, nil
}
func (c *pluginTurnLifecycleClient) Close(context.Context) error { return nil }
func (c *pluginTurnLifecycleClient) ProtocolVersion() int {
	return pluginhost.CapabilityProtocolVersion
}
func (c *pluginTurnLifecycleClient) Capabilities() []pluginhost.CapabilityDescriptor {
	return []pluginhost.CapabilityDescriptor{{ID: pluginhost.CapabilityAgentTurnLifecycle, Kind: pluginhost.SeamObserve, Version: 1}}
}
func (c *pluginTurnLifecycleClient) InvokeCapability(_ context.Context, params pluginhost.CapabilityInvokeParams) (pluginhost.CapabilityInvokeResult, error) {
	var input pluginhost.AgentTurnLifecycleInput
	if err := json.Unmarshal(params.Input, &input); err != nil {
		return pluginhost.CapabilityInvokeResult{}, err
	}
	c.calls <- input
	return pluginhost.CapabilityInvokeResult{Output: json.RawMessage(`{}`)}, nil
}

func TestPluginTurnRequestContextIsBoundedAndRequestOnly(t *testing.T) {
	segments, err := pluginTurnRequestContext([]pluginhost.AgentContinuationBlock{{Content: "private trigger facts"}})
	if err != nil || len(segments) != 1 || len(segments[0].Blocks) != 1 {
		t.Fatalf("segments = %+v, err = %v", segments, err)
	}
	if segments[0].Lifecycle != agent.ContextSegmentRequestOnly || segments[0].Durable || segments[0].VisibleInUI {
		t.Fatalf("plugin context is not request-only: %+v", segments[0])
	}
	tooMany := make([]pluginhost.AgentContinuationBlock, pluginhost.MaxTurnSubmitContextBlocks+1)
	for index := range tooMany {
		tooMany[index].Content = "x"
	}
	if _, err := pluginTurnRequestContext(tooMany); err == nil {
		t.Fatal("oversized context block list was accepted")
	}
}

func TestPluginTurnSubmitCreatesOrdinaryDurableThreadAndTargetsLifecycle(t *testing.T) {
	client := &fakeClient{response: providersResponse("done")}
	rt := newTestRuntime(t, client)
	rt.PluginTurnRouter = runtime.NewPluginTurnRouter()
	owner := &pluginTurnLifecycleClient{id: "schedule", calls: make(chan pluginhost.AgentTurnLifecycleInput, 2)}
	other := &pluginTurnLifecycleClient{id: "other", calls: make(chan pluginhost.AgentTurnLifecycleInput, 1)}
	rt.PluginHost = pluginhost.New(owner, other)
	out := &lockedBuffer{}
	srv := New(rt, out)
	t.Cleanup(srv.Close)

	result, err := rt.PluginTurnRouter.Submit(context.Background(), owner.id, pluginhost.TurnSubmitParams{
		RequestID: "request-1", Prompt: "inspect the build",
		ContextBlocks: []pluginhost.AgentContinuationBlock{{Kind: "TRIGGER", Source: "schedule", Content: "fired at 09:00"}},
	})
	if err != nil || result.State != pluginhost.TurnLifecycleRunning || result.ThreadID == "" || result.TurnID == "" {
		t.Fatalf("submit = %+v, %v", result, err)
	}
	waitForTurnCompletedForThread(t, out, result.ThreadID)

	select {
	case lifecycle := <-owner.calls:
		if lifecycle.State != pluginhost.TurnLifecycleCompleted || lifecycle.RequestID != "request-1" || lifecycle.ThreadID != result.ThreadID || lifecycle.TurnID != result.TurnID {
			t.Fatalf("lifecycle = %+v", lifecycle)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("owner did not receive terminal lifecycle")
	}
	select {
	case lifecycle := <-other.calls:
		t.Fatalf("unrelated plugin received lifecycle: %+v", lifecycle)
	default:
	}

	persisted, ok, err := session.Find(rt.SessionDir, result.ThreadID)
	if err != nil || !ok || persisted.Source != "plugin:schedule" {
		t.Fatalf("persisted thread = %+v, %t, %v", persisted, ok, err)
	}
	records, err := session.LoadHistoryRecords(rt.SessionDir, result.ThreadID, true)
	if err != nil {
		t.Fatal(err)
	}
	for _, record := range records {
		if strings.Contains(record.Content, "fired at 09:00") {
			t.Fatalf("request-only context leaked into history: %+v", record)
		}
	}
	client.mu.Lock()
	requests := append([]providers.ChatRequest(nil), client.requests...)
	client.mu.Unlock()
	if len(requests) != 1 || !strings.Contains(requests[0].Messages[len(requests[0].Messages)-1].Content, "fired at 09:00") {
		t.Fatalf("provider request missing plugin context: %+v", requests)
	}
}

func TestPluginTurnSubmitQueuesBusyThreadAndReportsLaterTransitions(t *testing.T) {
	chatStarted := make(chan struct{})
	releaseFirst := make(chan struct{})
	client := &fakeClient{
		responses: []providers.ChatResponse{providersResponse("first"), providersResponse("second")},
		onChat: func(call int, _ providers.ChatRequest) {
			if call == 1 {
				close(chatStarted)
				<-releaseFirst
			}
		},
	}
	rt := newTestRuntime(t, client)
	rt.PluginTurnRouter = runtime.NewPluginTurnRouter()
	owner := &pluginTurnLifecycleClient{id: "schedule", calls: make(chan pluginhost.AgentTurnLifecycleInput, 8)}
	rt.PluginHost = pluginhost.New(owner)
	out := &lockedBuffer{}
	srv := New(rt, out)
	t.Cleanup(srv.Close)

	first, err := rt.PluginTurnRouter.Submit(context.Background(), owner.id, pluginhost.TurnSubmitParams{RequestID: "first", Prompt: "first"})
	if err != nil || first.State != pluginhost.TurnLifecycleRunning {
		t.Fatalf("first submit = %+v, %v", first, err)
	}
	select {
	case <-chatStarted:
	case <-time.After(3 * time.Second):
		t.Fatal("first turn did not start")
	}
	second, err := rt.PluginTurnRouter.Submit(context.Background(), owner.id, pluginhost.TurnSubmitParams{
		RequestID: "second", ThreadID: first.ThreadID, Prompt: "second",
	})
	if err != nil || second.State != pluginhost.TurnLifecycleQueued || second.QueueID == "" {
		t.Fatalf("second submit = %+v, %v", second, err)
	}
	close(releaseFirst)
	waitForTurnCompletedForThread(t, out, first.ThreadID)

	deadline := time.After(5 * time.Second)
	states := make([]string, 0, 2)
	for len(states) < 2 {
		select {
		case lifecycle := <-owner.calls:
			if lifecycle.RequestID == "second" {
				states = append(states, lifecycle.State)
				if lifecycle.QueueID != second.QueueID || lifecycle.ThreadID != first.ThreadID {
					t.Fatalf("second lifecycle = %+v", lifecycle)
				}
			}
		case <-deadline:
			t.Fatalf("second lifecycle states = %v", states)
		}
	}
	if strings.Join(states, ",") != pluginhost.TurnLifecycleRunning+","+pluginhost.TurnLifecycleCompleted {
		t.Fatalf("second lifecycle states = %v", states)
	}
}

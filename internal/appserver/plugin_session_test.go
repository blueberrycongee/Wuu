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

func (c *pluginTurnLifecycleClient) ID() string { return c.id }
func (c *pluginTurnLifecycleClient) Status() pluginhost.Status {
	return pluginhost.Status{ID: c.id, State: pluginhost.StateActive}
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
	segments, err := pluginTurnRequestContext([]pluginhost.SessionContextBlock{{Content: "private trigger facts"}})
	if err != nil || len(segments) != 1 || len(segments[0].Blocks) != 1 {
		t.Fatalf("segments = %+v, err = %v", segments, err)
	}
	if segments[0].Lifecycle != agent.ContextSegmentRequestOnly || segments[0].Durable || segments[0].VisibleInUI {
		t.Fatalf("plugin context is not request-only: %+v", segments[0])
	}
	tooMany := make([]pluginhost.SessionContextBlock, pluginhost.MaxSessionSendContextBlocks+1)
	for index := range tooMany {
		tooMany[index].Content = "x"
	}
	if _, err := pluginTurnRequestContext(tooMany); err == nil {
		t.Fatal("oversized context block list was accepted")
	}
}

func TestPluginSessionCreateAndSendPersistProvenanceAndTargetLifecycle(t *testing.T) {
	client := &fakeClient{response: providersResponse("done")}
	rt := newTestRuntime(t, client)
	rt.PluginSessionRouter = runtime.NewPluginSessionRouter()
	owner := &pluginTurnLifecycleClient{id: "schedule", calls: make(chan pluginhost.AgentTurnLifecycleInput, 2)}
	other := &pluginTurnLifecycleClient{id: "other", calls: make(chan pluginhost.AgentTurnLifecycleInput, 1)}
	rt.PluginHost = pluginhost.New(owner, other)
	out := &lockedBuffer{}
	srv := New(rt, out)
	t.Cleanup(srv.Close)

	created, err := rt.PluginSessionRouter.Create(context.Background(), owner.id, pluginhost.SessionCreateParams{
		RequestID: "create-1", Visibility: pluginhost.SessionVisibilityUser, ContextSource: pluginhost.SessionContextFresh,
	})
	if err != nil || created.SessionID == "" || !created.Created {
		t.Fatalf("create = %+v, %v", created, err)
	}
	result, err := rt.PluginSessionRouter.Send(context.Background(), owner.id, pluginhost.SessionSendParams{
		RequestID: "request-1", SessionID: created.SessionID,
		Input:        pluginhost.SessionInput{Prompt: "internal inspect prompt", ContextBlocks: []pluginhost.SessionContextBlock{{Kind: "TRIGGER", Source: "schedule", Content: "fired at 09:00"}}},
		Presentation: &pluginhost.SessionInputPresentation{Kind: pluginhost.SessionPresentationQueryBubble, Text: "后台任务已唤醒 Agent"}, Cause: "schedule:daily",
	})
	if err != nil || result.State != pluginhost.TurnLifecycleRunning || result.SessionID == "" || result.TurnID == "" {
		t.Fatalf("send = %+v, %v", result, err)
	}
	waitForTurnCompletedForThread(t, out, result.SessionID)

	select {
	case lifecycle := <-owner.calls:
		if lifecycle.State != pluginhost.TurnLifecycleCompleted || lifecycle.RequestID != "request-1" || lifecycle.ThreadID != result.SessionID || lifecycle.TurnID != result.TurnID || lifecycle.FinalOutput != "done" {
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

	persisted, ok, err := session.Find(rt.SessionDir, result.SessionID)
	if err != nil || !ok || persisted.Source != "plugin:schedule" || persisted.Owner != "plugin:schedule" || persisted.Visibility != pluginhost.SessionVisibilityUser {
		t.Fatalf("persisted thread = %+v, %t, %v", persisted, ok, err)
	}
	records, err := session.LoadHistoryRecords(rt.SessionDir, result.SessionID, true)
	if err != nil {
		t.Fatal(err)
	}
	for _, record := range records {
		if strings.Contains(record.Content, "fired at 09:00") {
			t.Fatalf("request-only context leaked into history: %+v", record)
		}
	}
	var generated *session.HistoryRecord
	for index := range records {
		if records[index].Origin == pluginhost.SessionInputPlugin {
			generated = &records[index]
			break
		}
	}
	if generated == nil || generated.OriginID != owner.id || generated.Cause != "schedule:daily" || generated.DisplayContent != "后台任务已唤醒 Agent" || generated.Content != "internal inspect prompt" || !generated.ReadOnly {
		t.Fatalf("generated query provenance = %+v", generated)
	}
	client.mu.Lock()
	requests := append([]providers.ChatRequest(nil), client.requests...)
	client.mu.Unlock()
	providerHasGeneratedQuery := false
	providerHasRequestContext := false
	if len(requests) == 1 {
		for _, message := range requests[0].Messages {
			providerHasGeneratedQuery = providerHasGeneratedQuery || (message.Role == "user" && message.Origin == pluginhost.SessionInputPlugin && strings.Contains(message.Content, "internal inspect prompt"))
			providerHasRequestContext = providerHasRequestContext || strings.Contains(message.Content, "fired at 09:00")
		}
	}
	if !providerHasGeneratedQuery || !providerHasRequestContext {
		t.Fatalf("provider request missing plugin context: %+v", requests)
	}
	loaded, err := srv.ensureThreadLoaded(result.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	loaded.mu.Lock()
	item := loaded.Turns[0].Items[0]
	loaded.mu.Unlock()
	if item.Text != "后台任务已唤醒 Agent" || strings.Contains(item.Text, "internal inspect prompt") || item.Origin != pluginhost.SessionInputPlugin || !item.ReadOnly || item.PresentationKind != pluginhost.SessionPresentationQueryBubble {
		t.Fatalf("query bubble projection = %+v", item)
	}
}

func TestPluginSessionListAndCancelAreOwnerScoped(t *testing.T) {
	chatStarted := make(chan struct{})
	release := make(chan struct{})
	rt := newTestRuntime(t, &fakeClient{response: providersResponse("done"), onChat: func(_ int, _ providers.ChatRequest) { close(chatStarted); <-release }})
	rt.PluginSessionRouter = runtime.NewPluginSessionRouter()
	srv := New(rt, &lockedBuffer{})
	t.Cleanup(func() { close(release); srv.Close() })
	created, err := rt.PluginSessionRouter.Create(context.Background(), "subagent", pluginhost.SessionCreateParams{RequestID: "owned", Name: "review_parser", Visibility: pluginhost.SessionVisibilityPlugin, ContextSource: pluginhost.SessionContextFresh})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rt.PluginSessionRouter.Send(context.Background(), "subagent", pluginhost.SessionSendParams{RequestID: "run", SessionID: created.SessionID, Input: pluginhost.SessionInput{Prompt: "work"}}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-chatStarted:
	case <-time.After(3 * time.Second):
		t.Fatal("turn did not start")
	}
	listed, err := rt.PluginSessionRouter.List(context.Background(), "subagent", pluginhost.SessionListParams{})
	if err != nil || len(listed.Sessions) != 1 || listed.Sessions[0].SessionID != created.SessionID || listed.Sessions[0].Name != "review_parser" || listed.Sessions[0].State != pluginhost.TurnLifecycleRunning {
		t.Fatalf("list = %+v, %v", listed, err)
	}
	other, err := rt.PluginSessionRouter.List(context.Background(), "other", pluginhost.SessionListParams{})
	if err != nil || len(other.Sessions) != 0 {
		t.Fatalf("other list = %+v, %v", other, err)
	}
	if _, err := rt.PluginSessionRouter.Cancel(context.Background(), "other", pluginhost.SessionCancelParams{SessionID: created.SessionID}); err == nil {
		t.Fatal("another plugin cancelled an owned session")
	}
	cancelled, err := rt.PluginSessionRouter.Cancel(context.Background(), "subagent", pluginhost.SessionCancelParams{SessionID: created.SessionID})
	if err != nil || !cancelled.Cancelled {
		t.Fatalf("cancel = %+v, %v", cancelled, err)
	}
}

func TestPluginSessionCreateIsIdempotentAndPrivateSessionsStayOutOfSearch(t *testing.T) {
	rt := newTestRuntime(t, &fakeClient{response: providersResponse("done")})
	rt.PluginSessionRouter = runtime.NewPluginSessionRouter()
	srv := New(rt, &lockedBuffer{})
	t.Cleanup(srv.Close)
	params := pluginhost.SessionCreateParams{RequestID: "same-create", Visibility: pluginhost.SessionVisibilityPlugin, ContextSource: pluginhost.SessionContextFresh}
	first, err := rt.PluginSessionRouter.Create(context.Background(), "dream", params)
	if err != nil || !first.Created {
		t.Fatalf("first create = %+v, %v", first, err)
	}
	second, err := rt.PluginSessionRouter.Create(context.Background(), "dream", params)
	if err != nil || second.Created || second.SessionID != first.SessionID {
		t.Fatalf("idempotent create = %+v, %v", second, err)
	}
	sources, err := srv.threadSearchSources()
	if err != nil {
		t.Fatal(err)
	}
	for _, source := range sources {
		if source.entry.thread.ID == first.SessionID {
			t.Fatalf("private plugin session leaked into ordinary search: %+v", source)
		}
	}
}

func TestPluginSessionSendQueuesBusyThreadAndReportsLaterTransitions(t *testing.T) {
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
	rt.PluginSessionRouter = runtime.NewPluginSessionRouter()
	owner := &pluginTurnLifecycleClient{id: "schedule", calls: make(chan pluginhost.AgentTurnLifecycleInput, 8)}
	rt.PluginHost = pluginhost.New(owner)
	out := &lockedBuffer{}
	srv := New(rt, out)
	t.Cleanup(srv.Close)

	created, err := rt.PluginSessionRouter.Create(context.Background(), owner.id, pluginhost.SessionCreateParams{RequestID: "create", Visibility: pluginhost.SessionVisibilityUser, ContextSource: pluginhost.SessionContextFresh})
	if err != nil {
		t.Fatal(err)
	}
	first, err := rt.PluginSessionRouter.Send(context.Background(), owner.id, pluginhost.SessionSendParams{RequestID: "first", SessionID: created.SessionID, Input: pluginhost.SessionInput{Prompt: "first"}})
	if err != nil || first.State != pluginhost.TurnLifecycleRunning {
		t.Fatalf("first submit = %+v, %v", first, err)
	}
	select {
	case <-chatStarted:
	case <-time.After(3 * time.Second):
		t.Fatal("first turn did not start")
	}
	second, err := rt.PluginSessionRouter.Send(context.Background(), owner.id, pluginhost.SessionSendParams{
		RequestID: "second", SessionID: first.SessionID, Input: pluginhost.SessionInput{Prompt: "second"},
	})
	if err != nil || second.State != pluginhost.TurnLifecycleQueued || second.QueueID == "" {
		t.Fatalf("second submit = %+v, %v", second, err)
	}
	close(releaseFirst)
	waitForTurnCompletedForThread(t, out, first.SessionID)

	deadline := time.After(5 * time.Second)
	states := make([]string, 0, 2)
	for len(states) < 2 {
		select {
		case lifecycle := <-owner.calls:
			if lifecycle.RequestID == "second" {
				states = append(states, lifecycle.State)
				if lifecycle.QueueID != second.QueueID || lifecycle.ThreadID != first.SessionID {
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

func TestQueuedUserWorkHasPriorityOverPluginWakeups(t *testing.T) {
	srv := &Server{pendingQueuedTurns: map[string][]queuedTurn{}}
	threadID := "priority-thread"
	srv.pendingQueuedTurns[threadID] = []queuedTurn{
		{id: "plugin", snapshot: turnRuntimeSnapshot{PluginTurn: &pluginTurnReference{PluginID: "goal", RequestID: "continue"}}},
		{id: "user"},
	}
	first, ok := srv.takeNextQueuedUserTurn(threadID)
	if !ok || first.id != "user" {
		t.Fatalf("first queued turn = %+v, %t", first, ok)
	}
	second, ok := srv.takeNextQueuedUserTurn(threadID)
	if !ok || second.id != "plugin" {
		t.Fatalf("second queued turn = %+v, %t", second, ok)
	}
}

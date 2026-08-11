package subagent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	pluginapi "github.com/blueberrycongee/wuu/packages/plugin-go"
)

type capturedCall struct {
	method string
	params map[string]any
}
type captureHost struct{ calls []capturedCall }

type lifecycleHost struct {
	captureHost
	record taskRecord
}

type interruptHost struct {
	captureHost
	record taskRecord
}

func (h *lifecycleHost) CallHost(ctx context.Context, method string, params, result any) error {
	switch method {
	case pluginapi.HostServiceStorageGet:
		encoded, _ := json.Marshal(h.record)
		value, _ := json.Marshal(string(encoded))
		return json.Unmarshal([]byte(`{"value":`+string(value)+`}`), result)
	case pluginapi.HostServiceSessionList:
		return decodeInto(pluginapi.SessionListResult{Sessions: []pluginapi.SessionSummary{{
			SessionID: h.record.SessionID, ParentSessionID: h.record.ParentSessionID, Name: h.record.Name, State: h.record.State,
		}}}, result)
	}
	return h.captureHost.CallHost(ctx, method, params, result)
}

func (h *interruptHost) CallHost(ctx context.Context, method string, params, result any) error {
	switch method {
	case pluginapi.HostServiceSessionList:
		return decodeInto(pluginapi.SessionListResult{Sessions: []pluginapi.SessionSummary{{
			SessionID: h.record.SessionID, ParentSessionID: h.record.ParentSessionID, Name: h.record.Name, State: h.record.State,
		}}}, result)
	case pluginapi.HostServiceStorageGet:
		encoded, _ := json.Marshal(h.record)
		value := string(encoded)
		return decodeInto(map[string]any{"value": &value}, result)
	case pluginapi.HostServiceSessionCancel:
		var input pluginapi.SessionCancelParams
		raw, _ := json.Marshal(params)
		_ = json.Unmarshal(raw, &input)
		if err := h.captureHost.CallHost(ctx, method, params, result); err != nil {
			return err
		}
		return decodeInto(pluginapi.SessionCancelResult{SessionID: input.SessionID, TurnID: input.TurnID, Cancelled: true}, result)
	default:
		return h.captureHost.CallHost(ctx, method, params, result)
	}
}

func (h *captureHost) InitializeParams() pluginapi.InitializeParams {
	return pluginapi.InitializeParams{}
}
func (h *captureHost) CallHost(_ context.Context, method string, params, result any) error {
	raw, _ := json.Marshal(params)
	var decoded map[string]any
	_ = json.Unmarshal(raw, &decoded)
	h.calls = append(h.calls, capturedCall{method: method, params: decoded})
	response := `{}`
	switch method {
	case pluginapi.HostServiceSessionCreate:
		response = `{"session_id":"child-1","created":true}`
	case pluginapi.HostServiceSessionSend:
		response = `{"state":"running","session_id":"child-1","turn_id":"turn-1"}`
	}
	return json.Unmarshal([]byte(response), result)
}

func TestHandlerOwnsSubagentToolsAndPrompt(t *testing.T) {
	handler := Handler()
	if len(handler.Definition.Tools) != 3 {
		t.Fatalf("tools = %+v", handler.Definition.Tools)
	}
	services := map[string]bool{}
	for _, service := range handler.Definition.RequiredHostServices {
		services[service.ID] = true
	}
	for _, want := range []string{pluginapi.HostServiceSessionCreate, pluginapi.HostServiceSessionSend, pluginapi.HostServiceSessionList, pluginapi.HostServiceSessionCancel} {
		if !services[want] {
			t.Fatalf("missing host service %s: %+v", want, handler.Definition.RequiredHostServices)
		}
	}
	for _, tool := range handler.Definition.Tools {
		if len(tool.ExecutionScopes) != 1 || tool.ExecutionScopes[0] != "root" {
			t.Fatalf("tool %q scopes = %v", tool.ID, tool.ExecutionScopes)
		}
	}
	raw, err := handler.InvokeCapability(context.Background(), &captureHost{}, pluginapi.CapabilityCall{Capability: capabilityPrompt})
	if err != nil || len(raw) == 0 {
		t.Fatalf("prompt capability = %s, %v", raw, err)
	}
}

func TestSpawnComposesPublicSessionServices(t *testing.T) {
	host := &captureHost{}
	result, err := executeTool(context.Background(), host, pluginapi.ToolCall{ToolID: "spawn_agent", SessionID: "parent-1", Arguments: json.RawMessage(`{"description":"Review parser","prompt":"Inspect and report.","subagent_type":"general-purpose","model":"cheap","run_in_background":true}`)})
	if err != nil {
		t.Fatal(err)
	}
	if len(host.calls) != 6 {
		t.Fatalf("calls = %+v", host.calls)
	}
	if host.calls[0].method != pluginapi.HostServiceSessionCreate || host.calls[0].params["parent_session_id"] != "parent-1" || host.calls[0].params["context_source"] != "fresh" || host.calls[0].params["model_alias"] != "cheap" {
		t.Fatalf("create = %+v", host.calls[0])
	}
	if host.calls[3].method != pluginapi.HostServiceSessionSend {
		t.Fatalf("send = %+v", host.calls)
	}
	if len(result.Content) != 1 || result.Content[0].Text == "" {
		t.Fatalf("result = %+v", result)
	}
}

func TestTerminalLifecycleDeliversFinalOutputToParentSession(t *testing.T) {
	host := &lifecycleHost{record: taskRecord{SessionID: "child-1", ParentSessionID: "parent-1", Name: "review_parser", RequestID: "turn-one"}}
	input, _ := json.Marshal(pluginapi.TurnLifecycleInput{RequestID: "turn-one", State: "completed", ThreadID: "child-1", FinalOutput: "parser is correct"})
	if _, err := invokeCapability(context.Background(), host, pluginapi.CapabilityCall{Capability: capabilityLifecycle, Input: input}); err != nil {
		t.Fatal(err)
	}
	var delivery *capturedCall
	for index := range host.calls {
		if host.calls[index].method == pluginapi.HostServiceSessionSend {
			delivery = &host.calls[index]
		}
	}
	if delivery == nil || delivery.params["session_id"] != "parent-1" || delivery.params["cause"] != "subagent.completion" {
		t.Fatalf("delivery = %+v, calls = %+v", delivery, host.calls)
	}
	inputValue, _ := delivery.params["input"].(map[string]any)
	if !strings.Contains(fmt.Sprint(inputValue["prompt"]), "parser is correct") {
		t.Fatalf("delivery prompt = %+v", inputValue)
	}
}

func TestParentTurnInterruptionCancelsOnlyItsCurrentChildTurn(t *testing.T) {
	host := &interruptHost{record: taskRecord{
		SessionID: "child-1", ParentSessionID: "parent-1", ParentTurnID: "parent-turn-1",
		Name: "review_parser", RequestID: "child-request", TurnID: "child-turn-1", State: "running",
	}}
	input, _ := json.Marshal(pluginapi.AgentTurnInterruptedInput{ThreadID: "parent-1", TurnID: "parent-turn-1"})
	if _, err := invokeCapability(context.Background(), host, pluginapi.CapabilityCall{Capability: capabilityInterrupt, Input: input}); err != nil {
		t.Fatal(err)
	}
	var cancel *capturedCall
	for index := range host.calls {
		if host.calls[index].method == pluginapi.HostServiceSessionCancel {
			cancel = &host.calls[index]
		}
	}
	if cancel == nil || cancel.params["session_id"] != "child-1" || cancel.params["turn_id"] != "child-turn-1" {
		t.Fatalf("cancel = %+v, calls = %+v", cancel, host.calls)
	}
}

func TestSendMessageRebindsChildTurnToCurrentParentTurn(t *testing.T) {
	host := &lifecycleHost{record: taskRecord{
		SessionID: "child-1", ParentSessionID: "parent-1", ParentTurnID: "parent-turn-1",
		Name: "review_parser", RequestID: "old-request", TurnID: "old-child-turn", State: "completed",
	}}
	_, err := sendMessage(context.Background(), host, pluginapi.ToolCall{
		SessionID: "parent-1", TurnID: "parent-turn-2", Arguments: json.RawMessage(`{"target":"child-1","message":"continue"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	var saved taskRecord
	for _, call := range host.calls {
		if call.method != pluginapi.HostServiceStorageSet || call.params["key"] != "session.child-1" {
			continue
		}
		if err := json.Unmarshal([]byte(fmt.Sprint(call.params["value"])), &saved); err != nil {
			t.Fatal(err)
		}
	}
	if saved.ParentTurnID != "parent-turn-2" || saved.TurnID != "turn-1" || saved.ParentSessionID != "parent-1" {
		t.Fatalf("saved task = %+v", saved)
	}
}

func decodeInto(value any, result any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, result)
}

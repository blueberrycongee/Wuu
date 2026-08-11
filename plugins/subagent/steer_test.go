package subagent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	pluginapi "github.com/blueberrycongee/wuu/packages/plugin-go"
)

type steerLifecycleHost struct {
	captureHost
	record taskRecord
}

func (h *steerLifecycleHost) CallHost(ctx context.Context, method string, params, result any) error {
	switch method {
	case pluginapi.HostServiceStorageGet:
		encoded, _ := json.Marshal(h.record)
		value := string(encoded)
		return decodeInto(map[string]any{"value": &value}, result)
	case pluginapi.HostServiceSessionList:
		return decodeInto(pluginapi.SessionListResult{Sessions: []pluginapi.SessionSummary{{
			SessionID: h.record.SessionID, ParentSessionID: h.record.ParentSessionID, Name: h.record.Name, State: h.record.State,
		}}}, result)
	case pluginapi.HostServiceSessionSend:
		raw, _ := json.Marshal(params)
		var decoded map[string]any
		_ = json.Unmarshal(raw, &decoded)
		h.calls = append(h.calls, capturedCall{method: method, params: decoded})
		return decodeInto(pluginapi.SessionSendResult{State: "running", SessionID: h.record.SessionID, TurnID: h.record.TurnID, Steered: true}, result)
	default:
		return h.captureHost.CallHost(ctx, method, params, result)
	}
}

func TestSendMessageSteersRunningChildWithoutReplacingLifecycleRequest(t *testing.T) {
	host := &steerLifecycleHost{record: taskRecord{
		SessionID: "child-1", ParentSessionID: "parent-1", ParentTurnID: "parent-turn-1",
		Name: "review_parser", RequestID: "original-request", TurnID: "child-turn-1", State: "running",
	}}
	result, err := sendMessage(context.Background(), host, pluginapi.ToolCall{
		SessionID: "parent-1", TurnID: "parent-turn-2", Arguments: json.RawMessage(`{"target":"child-1","message":"summarize now"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Content) != 1 || !strings.Contains(result.Content[0].Text, `"steered":true`) {
		t.Fatalf("result = %+v", result)
	}
	var saved taskRecord
	for _, call := range host.calls {
		if call.method == pluginapi.HostServiceSessionSend && call.params["if_running"] != pluginapi.SessionIfRunningSteer {
			t.Fatalf("send params = %+v", call.params)
		}
		if call.method != pluginapi.HostServiceStorageSet || call.params["key"] != "session.child-1" {
			continue
		}
		if err := json.Unmarshal([]byte(fmt.Sprint(call.params["value"])), &saved); err != nil {
			t.Fatal(err)
		}
	}
	if saved.RequestID != "original-request" || saved.ParentTurnID != "parent-turn-2" || saved.TurnID != "child-turn-1" {
		t.Fatalf("saved task = %+v", saved)
	}
}

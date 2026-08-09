package goal

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	pluginapi "github.com/blueberrycongee/wuu/packages/plugin-go"
)

type memoryHost struct {
	values map[string]string
	sends  []pluginapi.SessionSendParams
}

func (h *memoryHost) InitializeParams() pluginapi.InitializeParams {
	return pluginapi.InitializeParams{PluginID: "goal"}
}
func (h *memoryHost) CallHost(_ context.Context, method string, params, result any) error {
	raw, _ := json.Marshal(params)
	var input struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}
	_ = json.Unmarshal(raw, &input)
	switch method {
	case "host.storage.get":
		value, ok := h.values[input.Key]
		var response struct {
			Value *string `json:"value"`
		}
		if ok {
			response.Value = &value
		}
		raw, _ = json.Marshal(response)
		return json.Unmarshal(raw, result)
	case "host.storage.set":
		h.values[input.Key] = input.Value
		return nil
	case "host.storage.delete":
		delete(h.values, input.Key)
		return nil
	case pluginapi.HostServiceSessionSend:
		var send pluginapi.SessionSendParams
		if err := json.Unmarshal(raw, &send); err != nil {
			return err
		}
		h.sends = append(h.sends, send)
		response, _ := json.Marshal(pluginapi.SessionSendResult{State: "queued", SessionID: send.SessionID, QueueID: "queue-1"})
		return json.Unmarshal(response, result)
	default:
		return fmt.Errorf("unexpected method %s", method)
	}
}

func TestGoalPluginOwnsToolsStorageAndGeneratedQueryContinuation(t *testing.T) {
	host := &memoryHost{values: map[string]string{}}
	handler := Handler()
	created, err := handler.ExecuteTool(context.Background(), host, pluginapi.ToolCall{
		ToolID: "create_goal", ThreadID: "thread-1", Arguments: json.RawMessage(`{"objective":"ship the plugin"}`),
	})
	if err != nil || len(created.Content) != 1 {
		t.Fatalf("create = %+v, err = %v", created, err)
	}
	now := time.Now().UTC()
	_, err = handler.InvokeCapability(context.Background(), host, pluginapi.CapabilityCall{
		Capability: capabilityTurnCompleted,
		Input: json.RawMessage(fmt.Sprintf(`{"thread_id":"thread-1","turn_id":"turn-1","started_at":%q,"completed_at":%q,"succeeded":true}`,
			now.Add(-time.Second).Format(time.RFC3339Nano), now.Format(time.RFC3339Nano))),
	})
	if err != nil || len(host.sends) != 1 || host.sends[0].SessionID != "thread-1" || host.sends[0].Input.Prompt == "" || host.sends[0].Presentation == nil || host.sends[0].Presentation.Text != "Goal 持续推进中" {
		t.Fatalf("generated continuation = %+v, err = %v", host.sends, err)
	}
	if _, err := handler.ExecuteTool(context.Background(), host, pluginapi.ToolCall{
		ToolID: "update_goal", ThreadID: "thread-1", Arguments: json.RawMessage(`{"status":"complete"}`),
	}); err != nil {
		t.Fatal(err)
	}
	_, err = handler.InvokeCapability(context.Background(), host, pluginapi.CapabilityCall{
		Capability: capabilityTurnCompleted,
		Input: json.RawMessage(fmt.Sprintf(`{"thread_id":"thread-1","turn_id":"turn-2","started_at":%q,"completed_at":%q,"succeeded":true}`,
			now.Format(time.RFC3339Nano), now.Add(time.Second).Format(time.RFC3339Nano))),
	})
	if err != nil || len(host.sends) != 1 {
		t.Fatalf("completed goal sent another continuation: %+v, err = %v", host.sends, err)
	}
}

func TestGoalPluginClientRequestsStayOpaqueToHost(t *testing.T) {
	host := &memoryHost{values: map[string]string{}}
	handler := Handler()
	call := func(method, input string) json.RawMessage {
		raw, err := handler.InvokeCapability(context.Background(), host, pluginapi.CapabilityCall{
			Capability: capabilityClientRequest,
			Input:      json.RawMessage(fmt.Sprintf(`{"method":%q,"input":%s}`, method, input)),
		})
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	call("goal.set", `{"thread_id":"thread-2","objective":"finish UI"}`)
	summary := call("summary.get", `{"thread_id":"thread-2"}`)
	if !contains(string(summary), "finish UI", `"status":"active"`) {
		t.Fatalf("summary = %s", summary)
	}
}

func TestGoalPluginAccountsCompletedTurnFromGenericObservation(t *testing.T) {
	host := &memoryHost{values: map[string]string{}}
	handler := Handler()
	if _, err := handler.ExecuteTool(context.Background(), host, pluginapi.ToolCall{
		ToolID: "create_goal", ThreadID: "thread-usage", Arguments: json.RawMessage(`{"objective":"measure work"}`),
	}); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	input, _ := json.Marshal(map[string]any{
		"thread_id": "thread-usage", "turn_id": "turn-1", "started_at": now.Add(-2 * time.Second),
		"completed_at": now, "succeeded": true, "input_tokens": 4, "output_tokens": 3,
	})
	if _, err := handler.InvokeCapability(context.Background(), host, pluginapi.CapabilityCall{
		Capability: capabilityTurnCompleted, Input: input,
	}); err != nil {
		t.Fatal(err)
	}
	result, err := handler.ExecuteTool(context.Background(), host, pluginapi.ToolCall{ToolID: "get_goal", ThreadID: "thread-usage", Arguments: json.RawMessage(`{}`)})
	if err != nil || len(result.Content) != 1 || !contains(result.Content[0].Text, `"tokens_used":7`, `"time_used_ms":`) {
		t.Fatalf("goal after observation = %+v, err = %v", result, err)
	}
}

func contains(value string, parts ...string) bool {
	for _, part := range parts {
		if !strings.Contains(value, part) {
			return false
		}
	}
	return true
}

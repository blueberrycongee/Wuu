package subagent

import (
	"context"
	"encoding/json"
	"testing"

	pluginapi "github.com/blueberrycongee/wuu/packages/plugin-go"
)

type captureHost struct {
	method string
	params map[string]any
}

func (h *captureHost) InitializeParams() pluginapi.InitializeParams {
	return pluginapi.InitializeParams{}
}
func (h *captureHost) CallHost(_ context.Context, method string, params, result any) error {
	h.method = method
	raw, _ := json.Marshal(params)
	_ = json.Unmarshal(raw, &h.params)
	encoded := json.RawMessage(`{"id":"child-1","status":"running"}`)
	*(result.(*json.RawMessage)) = encoded
	return nil
}

func TestHandlerOwnsSubagentToolsAndPrompt(t *testing.T) {
	handler := Handler()
	if len(handler.Definition.Tools) != 5 {
		t.Fatalf("tools = %+v", handler.Definition.Tools)
	}
	if len(handler.Definition.RequiredHostServices) != 1 || handler.Definition.RequiredHostServices[0].ID != hostChildSession {
		t.Fatalf("host services = %+v", handler.Definition.RequiredHostServices)
	}
	for _, tool := range handler.Definition.Tools {
		want := "root"
		if tool.ID == "agent_report" {
			want = "child"
		}
		if len(tool.ExecutionScopes) != 1 || tool.ExecutionScopes[0] != want {
			t.Fatalf("tool %q execution scopes = %v, want %q", tool.ID, tool.ExecutionScopes, want)
		}
	}
	raw, err := handler.InvokeCapability(context.Background(), &captureHost{}, pluginapi.CapabilityCall{Capability: capabilityPrompt})
	if err != nil || len(raw) == 0 {
		t.Fatalf("prompt capability = %s, %v", raw, err)
	}
}

func TestSpawnForwardsNeutralChildSessionRequest(t *testing.T) {
	host := &captureHost{}
	result, err := executeTool(context.Background(), host, pluginapi.ToolCall{
		ToolID: "spawn_agent", ActorID: "parent-1", ActorPath: "/root",
		Arguments: json.RawMessage(`{"description":"Review parser","prompt":"Inspect and report.","subagent_type":"general-purpose","run_in_background":true}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if host.method != hostChildSession || host.params["action"] != "spawn" || host.params["actor_id"] != "parent-1" {
		t.Fatalf("host call = %q %+v", host.method, host.params)
	}
	if len(result.Content) != 1 || result.Content[0].Text == "" {
		t.Fatalf("result = %+v", result)
	}
}

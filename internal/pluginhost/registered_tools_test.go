package pluginhost

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/blueberrycongee/wuu/internal/toolresult"
)

type registeredToolTestClient struct {
	fakeClient
	tools  []ToolRegistration
	params ToolExecuteParams
	result ToolExecuteResult
	err    error
}

func (c *registeredToolTestClient) Tools() []ToolRegistration {
	return append([]ToolRegistration(nil), c.tools...)
}

func (c *registeredToolTestClient) ExecuteTool(_ context.Context, params ToolExecuteParams) (ToolExecuteResult, error) {
	c.params = params
	return c.result, c.err
}

func TestHostRegistersNamespacedToolsAndExecutesStructuredResult(t *testing.T) {
	client := &registeredToolTestClient{
		fakeClient: fakeClient{id: "acme.lookup"},
		tools: []ToolRegistration{{
			ID:          "search",
			Description: "Search local plugin data",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]any{"type": "string"},
				},
			},
		}},
		result: ToolExecuteResult{Result: toolresult.Result{
			Content:           []toolresult.ContentPart{{Type: toolresult.ContentTypeText, Text: "found"}},
			StructuredContent: json.RawMessage(`{"count":1}`),
		}},
	}
	host := New(client)
	definitions := host.ToolDefinitions()
	if len(definitions) != 1 {
		t.Fatalf("definitions = %+v", definitions)
	}
	publicName := definitions[0].Name
	if !strings.HasPrefix(publicName, "plugin_acme_lookup_search_") || publicName == "search" {
		t.Fatalf("public name = %q", publicName)
	}
	definitions[0].InputSchema["type"] = "array"
	if got := host.ToolDefinitions()[0].InputSchema["type"]; got != "object" {
		t.Fatalf("host schema mutated through returned definition: %v", got)
	}

	result, err := host.ExecuteTool(context.Background(), publicName, ToolExecuteInput{
		CallID:    "call-1",
		Tool:      "ignored",
		Arguments: json.RawMessage(`{"query":"wuu"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.TextProjection() != "found" || string(result.StructuredContent) != `{"count":1}` {
		t.Fatalf("result = %+v", result)
	}
	if client.params.ToolID != "search" || client.params.Tool != publicName || client.params.CallID != "call-1" {
		t.Fatalf("params = %+v", client.params)
	}
	if err := host.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if host.SupportsTool(publicName) || len(host.ToolDefinitions()) != 0 {
		t.Fatal("tool registration survived host close")
	}
}

func TestHostRejectsInvalidToolRegistrationAndExecutionErrors(t *testing.T) {
	invalid := &registeredToolTestClient{
		fakeClient: fakeClient{id: "invalid"},
		tools: []ToolRegistration{{
			ID:          "bad id",
			Description: "Invalid",
			InputSchema: map[string]any{"type": "object"},
		}},
	}
	if definitions := New(invalid).ToolDefinitions(); len(definitions) != 0 {
		t.Fatalf("invalid definitions = %+v", definitions)
	}

	failing := &registeredToolTestClient{
		fakeClient: fakeClient{id: "failing"},
		tools: []ToolRegistration{{
			ID:          "run",
			Description: "Fail intentionally",
			InputSchema: map[string]any{"type": "object"},
		}},
		err: errors.New("boom"),
	}
	host := New(failing)
	name := host.ToolDefinitions()[0].Name
	_, err := host.ExecuteTool(context.Background(), name, ToolExecuteInput{Arguments: json.RawMessage(`{}`)})
	if err == nil || !strings.Contains(err.Error(), `plugin "failing"`) || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("error = %v", err)
	}
}

var _ ToolClient = (*registeredToolTestClient)(nil)

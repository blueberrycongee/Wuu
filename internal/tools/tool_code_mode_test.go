package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/blueberrycongee/wuu/internal/codemode"
	"github.com/blueberrycongee/wuu/internal/providers"
)

func newCodeModeTestToolkit(t *testing.T) *Toolkit {
	t.Helper()
	kit, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New toolkit: %v", err)
	}
	service, err := codemode.NewService(codemode.ServiceConfig{
		Executable: "/tmp/wuu-code-mode-host",
		SessionID:  "test-session",
	})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	kit.SetCodeModeService(service)
	return kit
}

func names(defs []providers.ToolDefinition) []string {
	out := make([]string, 0, len(defs))
	for _, d := range defs {
		out = append(out, d.Name)
	}
	return out
}

func contains(name string, defs []providers.ToolDefinition) bool {
	for _, d := range defs {
		if d.Name == name {
			return true
		}
	}
	return false
}

func TestCodeModeEntryToolsRegisteredWithService(t *testing.T) {
	kit := newCodeModeTestToolkit(t)
	defs := kit.Definitions()
	if !contains(codeModeExecToolName, defs) {
		t.Fatalf("Definitions() missing %q: %v", codeModeExecToolName, names(defs))
	}
	if !contains(codeModeWaitToolName, defs) {
		t.Fatalf("Definitions() missing %q: %v", codeModeWaitToolName, names(defs))
	}
	// Without a service the entry tools must not exist.
	bare, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New bare toolkit: %v", err)
	}
	if contains(codeModeExecToolName, bare.Definitions()) {
		t.Fatal("bare toolkit advertises exec without a code-mode service")
	}
}

func TestCodeModeEntriesSurviveModelProfilesAndThreadClones(t *testing.T) {
	for _, model := range []string{"gpt-5-codex", "gpt-5", "claude-sonnet-4", "generic-model"} {
		t.Run(model, func(t *testing.T) {
			kit := newCodeModeTestToolkit(t)
			kit.ConfigureSurfaceForProviderModel("test", model, true)
			clone, err := kit.CloneForRoot(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			for _, active := range []*Toolkit{kit, clone} {
				for _, name := range []string{"exec", "wait"} {
					if !contains(name, active.Definitions()) || !active.SupportsTool(name) {
						t.Fatalf("%s is unavailable with model %s", name, model)
					}
					if err := active.ensureToolAvailableForExecution(name); err != nil {
						t.Fatal(err)
					}
					if _, ok := active.ActiveSurface().Tools[name]; !ok {
						t.Fatalf("active surface omitted %s", name)
					}
				}
			}
			kit.DisableTools("exec")
			if contains("exec", kit.Definitions()) || kit.SupportsTool("exec") {
				t.Fatal("disabled exec remains available")
			}
			clone.SetCodeModeService(nil)
			if contains("exec", clone.Definitions()) || clone.SupportsTool("exec") {
				t.Fatal("detached runtime still advertises exec")
			}
		})
	}
}

func TestCodeModeExecIsOrchestrator(t *testing.T) {
	kit := newCodeModeTestToolkit(t)
	metadata, ok := kit.ToolMetadata(providers.ToolCall{Name: codeModeExecToolName})
	if !ok {
		t.Fatal("ToolMetadata did not resolve exec")
	}
	if !metadata.Orchestrator {
		t.Fatal("exec metadata is not an orchestrator")
	}
	metadata, ok = kit.ToolMetadata(providers.ToolCall{Name: "read_file"})
	if !ok || metadata.Orchestrator {
		t.Fatalf("read_file metadata = %+v (found=%v), want non-orchestrator", metadata, ok)
	}
}

func TestCodeModeOnlyHidesButKeepsUnderlyingTools(t *testing.T) {
	kit := newCodeModeTestToolkit(t)
	kit.SetCodeModeOnly(true)
	defs := names(kit.Definitions())
	if len(defs) != 2 || defs[0] != codeModeExecToolName || defs[1] != codeModeWaitToolName {
		t.Fatalf("CodeModeOnly Definitions() = %v, want [exec wait]", defs)
	}
	// The nested surface still lists the underlying tools: they are hidden
	// from the model, not disabled.
	surface, err := kit.CodeModeNestedSurface()
	if err != nil {
		t.Fatalf("CodeModeNestedSurface: %v", err)
	}
	if !contains("read_file", codeModeDefsToProviderDefs(surface)) {
		t.Fatal("nested surface lost read_file in CodeModeOnly")
	}
}

func codeModeDefsToProviderDefs(defs []codemode.ToolDefinition) []providers.ToolDefinition {
	out := make([]providers.ToolDefinition, 0, len(defs))
	for _, d := range defs {
		out = append(out, providers.ToolDefinition{Name: d.Name, Description: d.Description})
	}
	return out
}

func TestCodeModeNestedSurfaceExcludesEntriesAndDisabled(t *testing.T) {
	kit := newCodeModeTestToolkit(t)
	kit.DisableTools("grep")
	surface, err := kit.CodeModeNestedSurface()
	if err != nil {
		t.Fatalf("CodeModeNestedSurface: %v", err)
	}
	defs := codeModeDefsToProviderDefs(surface)
	if contains(codeModeExecToolName, defs) || contains(codeModeWaitToolName, defs) {
		t.Fatal("nested surface includes the code-mode entry tools")
	}
	if contains("grep", defs) {
		t.Fatal("nested surface includes a disabled tool")
	}
	if !contains("read_file", defs) {
		t.Fatal("nested surface missing read_file")
	}
	// Schemas are marshaled, not nil.
	for _, d := range surface {
		if len(d.InputSchema) == 0 || !json.Valid(d.InputSchema) {
			t.Fatalf("tool %q has invalid input schema %s", d.Name, d.InputSchema)
		}
		if d.ToolName.Name != d.Name {
			t.Fatalf("tool %q ToolName = %+v", d.Name, d.ToolName)
		}
	}
}

func TestCodeModeExecRequiresOrchestratorScope(t *testing.T) {
	kit := newCodeModeTestToolkit(t)
	exec := NewCodeModeExecTool(kit)
	call := providers.ToolCall{
		ID:        "model-call-1",
		Name:      codeModeExecToolName,
		Arguments: `{"source":"1 + 1"}`,
	}
	result, err := exec.ExecuteResultCall(context.Background(), call)
	if err == nil || !strings.Contains(err.Error(), "orchestrator execution scope") {
		t.Fatalf("exec without orchestrator scope: result=%+v err=%v", result, err)
	}
}

func TestCodeModeWaitRequiresCell(t *testing.T) {
	kit := newCodeModeTestToolkit(t)
	wait := NewCodeModeWaitTool(kit)
	result, err := wait.ExecuteResult(context.Background(), `{}`)
	if err == nil || !strings.Contains(err.Error(), "cell_id") {
		t.Fatalf("wait without cell_id: result=%+v err=%v", result, err)
	}
}

func TestCodeModeExecResponseShape(t *testing.T) {
	response := codemode.Response{
		State:          "Yielded",
		CellID:         "cell-1",
		Content:        []codemode.ContentItem{{Type: "input_text", Text: "hi"}},
		HostDurationNS: 42,
	}
	raw := codeModeResponseResult(response)
	if raw.IsError {
		t.Fatal("response result is an error")
	}
	var decoded struct {
		State  string `json:"state"`
		CellID string `json:"cell_id"`
	}
	if err := json.Unmarshal([]byte(raw.TextProjection()), &decoded); err != nil {
		t.Fatalf("response result is not the wire shape: %v (%s)", err, raw.TextProjection())
	}
	if decoded.State != "Yielded" || decoded.CellID != "cell-1" {
		t.Fatalf("decoded response = %+v", decoded)
	}
}

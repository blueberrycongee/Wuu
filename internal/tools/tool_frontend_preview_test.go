package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/blueberrycongee/wuu/internal/modelprofile"
	"github.com/blueberrycongee/wuu/internal/providers"
)

const validFrontendPreviewArgs = `{"version":1,"title":"Loading button","html":"<button id=\"save\">Save</button>","css":"button:hover { transform: scale(1.05); }","javascript":"document.querySelector('#save').addEventListener('click', () => {});","viewport":{"height":320}}`

func TestFrontendPreviewToolValidatesAndReturnsCompactAcknowledgement(t *testing.T) {
	tool := NewFrontendPreviewTool()
	if err := tool.ValidateInput(validFrontendPreviewArgs); err != nil {
		t.Fatalf("ValidateInput: %v", err)
	}
	result, err := tool.ExecuteResult(context.Background(), validFrontendPreviewArgs)
	if err != nil {
		t.Fatalf("ExecuteResult: %v", err)
	}
	if result.TextProjection() != "Frontend preview ready." {
		t.Fatalf("unexpected model projection: %q", result.TextProjection())
	}
	if strings.Contains(result.JSONProjection(), "Loading button") || strings.Contains(result.JSONProjection(), "querySelector") {
		t.Fatalf("result must not echo the preview payload: %s", result.JSONProjection())
	}
	var meta struct {
		Presentation struct {
			Kind    string `json:"kind"`
			Version int    `json:"version"`
			Digest  string `json:"digest"`
		} `json:"presentation"`
	}
	if err := json.Unmarshal(result.Meta, &meta); err != nil {
		t.Fatalf("decode meta: %v", err)
	}
	if meta.Presentation.Kind != "frontend_preview" || meta.Presentation.Version != 1 || !strings.HasPrefix(meta.Presentation.Digest, "sha256:") {
		t.Fatalf("unexpected acknowledgement metadata: %+v", meta.Presentation)
	}
}

func TestFrontendPreviewToolRejectsUnsafeOrUnboundedInput(t *testing.T) {
	tool := NewFrontendPreviewTool()
	tests := []struct {
		name string
		args string
		want string
	}{
		{name: "unknown field", args: `{"version":1,"title":"x","html":"","extra":true}`, want: "unknown field"},
		{name: "missing html", args: `{"version":1,"title":"x"}`, want: "missing_html"},
		{name: "unsupported version", args: `{"version":2,"title":"x","html":""}`, want: "unsupported_version"},
		{name: "external image", args: `{"version":1,"title":"x","html":"<img src=\"https://example.com/x.png\">"}`, want: "unsafe_html"},
		{name: "inline handler", args: `{"version":1,"title":"x","html":"<button onclick=\"alert(1)\">x</button>"}`, want: "unsafe_html"},
		{name: "script element", args: `{"version":1,"title":"x","html":"<script>alert(1)</script>"}`, want: "unsafe_html"},
		{name: "css import", args: `{"version":1,"title":"x","html":"","css":"@import 'https://example.com/x.css';"}`, want: "unsafe_css"},
		{name: "invalid viewport", args: `{"version":1,"title":"x","html":"","viewport":{"height":900}}`, want: "invalid_viewport"},
		{name: "nested unknown field", args: `{"version":1,"title":"x","html":"","viewport":{"height":320,"width":640}}`, want: "unknown field"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tool.ValidateInput(tt.args)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("ValidateInput error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestFrontendPreviewToolIsCapabilityGatedAndMainOnly(t *testing.T) {
	kit, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	kit.SetActiveProfile(modelprofile.Resolve("openai", "gpt-5-codex"), true)
	if containsProfileDef(kit.Definitions(), frontendPreviewToolName) {
		t.Fatal("new toolkit must keep frontend preview disabled")
	}
	kit.SetFrontendPreviewEnabled(true)
	if !containsProfileDef(kit.Definitions(), frontendPreviewToolName) {
		t.Fatalf("enabled main toolkit missing %s", frontendPreviewToolName)
	}

	worker, err := kit.CloneForRoot(t.TempDir())
	if err != nil {
		t.Fatalf("CloneForRoot: %v", err)
	}
	worker.SetActiveProfile(modelprofile.Resolve("openai", "gpt-5-codex"), false)
	if containsProfileDef(worker.Definitions(), frontendPreviewToolName) {
		t.Fatal("worker surface must not expose frontend previews")
	}

	kit.SetFrontendPreviewEnabled(false)
	if _, err := kit.Execute(context.Background(), providers.ToolCall{Name: frontendPreviewToolName, Arguments: validFrontendPreviewArgs}); err == nil || !strings.Contains(err.Error(), "disabled in this session") {
		t.Fatalf("disabled frontend preview execution error = %v", err)
	}
}

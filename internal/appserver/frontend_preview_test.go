package appserver

import (
	"context"
	"testing"
)

func TestServerInitializeNegotiatesFrontendPreviewTool(t *testing.T) {
	rt := newTestRuntime(t, &fakeClient{})
	out := &lockedBuffer{}
	srv := New(rt, out)
	request := `{"id":"1","method":"initialize","params":{"protocol_version":"wuu-app-server/v0.1","capabilities":{"presentations":{"frontend_preview_versions":[1]}}}}`

	if err := srv.handleLine(context.Background(), []byte(request)); err != nil {
		t.Fatalf("initialize: %v", err)
	}
	result := remarshal[InitializeResult](t, responseByID(t, parseOutput(t, out.String()), "1")["result"])
	if !result.Features.FrontendPreview {
		t.Fatalf("frontend preview feature not negotiated: %+v", result.Features)
	}
	if !toolDefinitionNames(rt.Toolkit.Definitions())["render_frontend_preview"] {
		t.Fatal("negotiated runtime did not expose render_frontend_preview")
	}
}

func TestServerInitializeKeepsFrontendPreviewDisabledWithoutCapability(t *testing.T) {
	rt := newTestRuntime(t, &fakeClient{})
	out := &lockedBuffer{}
	srv := New(rt, out)
	if err := srv.handleLine(context.Background(), []byte(`{"id":"1","method":"initialize","params":{}}`)); err != nil {
		t.Fatalf("initialize: %v", err)
	}
	result := remarshal[InitializeResult](t, responseByID(t, parseOutput(t, out.String()), "1")["result"])
	if result.Features.FrontendPreview {
		t.Fatalf("unexpected frontend preview negotiation: %+v", result.Features)
	}
	if toolDefinitionNames(rt.Toolkit.Definitions())["render_frontend_preview"] {
		t.Fatal("unsupported shell must not expose render_frontend_preview")
	}
}

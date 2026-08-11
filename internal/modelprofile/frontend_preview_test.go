package modelprofile

import (
	"testing"

	"github.com/blueberrycongee/wuu/internal/capability"
)

func TestFrontendPreviewCapabilityIsMainConversationOnly(t *testing.T) {
	profile := Resolve("openai", "gpt-5-codex")
	main := DefaultCompiler{}.Compile(profile, SurfaceMain)
	if got := main.Tools["render_frontend_preview"]; got != capability.CapabilityPresentationFrontend {
		t.Fatalf("main frontend preview capability = %q", got)
	}
	for name, surface := range map[string]capability.Surface{
		"worker":      DefaultCompiler{}.Compile(profile, SurfaceWorker),
		"named-agent": DefaultCompiler{}.Compile(profile, SurfaceNamedAgent),
	} {
		if _, ok := surface.Tools["render_frontend_preview"]; ok {
			t.Fatalf("%s surface must not expose render_frontend_preview", name)
		}
	}
}

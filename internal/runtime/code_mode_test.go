package runtime

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/blueberrycongee/wuu/internal/config"
	"github.com/blueberrycongee/wuu/internal/providers"
	"github.com/blueberrycongee/wuu/internal/toolctx"
	"github.com/blueberrycongee/wuu/internal/toolresult"
	"github.com/blueberrycongee/wuu/internal/tools"
)

func TestSessionCodeModeDefaultAndOptOut(t *testing.T) {
	for _, tc := range []struct{ name, mode, workspaceID string }{
		{"desktop-default", "", "code-mode-workspace"},
		{"path-only-default", "", ""},
		{"code-only", "code_only", "code-mode-workspace"},
		{"mixed", "code", "code-mode-workspace"},
		{"direct", "direct", "code-mode-workspace"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mode := tc.mode
			root, home := t.TempDir(), t.TempDir()
			t.Setenv("WUU_HOME", filepath.Join(home, "state"))
			t.Setenv("TEST_WUU_KEY", "test")
			host := os.Getenv("WUU_CODE_MODE_HOST")
			realHost := host != ""
			if !realHost {
				host = filepath.Join(home, "wuu-code-mode-host")
			}
			t.Setenv("WUU_CODE_MODE_HOST", host)
			session, err := NewSession(Options{
				RootDir: root, HomeDir: home, WorkspaceID: tc.workspaceID,
				Config: config.Config{
					DefaultProvider: "test",
					Providers: map[string]config.ProviderConfig{"test": {
						Type: "openai-compatible", BaseURL: "https://example.test/v1",
						APIKeyEnv: "TEST_WUU_KEY", Model: "gpt-5",
					}},
					CodeMode: config.CodeModeConfig{Mode: mode},
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _, _ = session.Cleanup() })
			thread, err := session.NewThreadRuntime("conversation")
			if err != nil {
				t.Fatal(err)
			}
			for _, kit := range []*tools.Toolkit{session.Toolkit, thread.Toolkit} {
				enabled := mode != "direct"
				if kit.SupportsTool("exec") != enabled || kit.SupportsTool("wait") != enabled {
					t.Fatalf("mode %q: code-mode availability does not match configuration", mode)
				}
				if kit.CodeModeService() != session.CodeMode {
					t.Fatal("thread lost session runtime")
				}
				defs := kit.Definitions()
				found := false
				for _, d := range defs {
					if d.Name == "exec" {
						found = true
					}
				}
				if found != enabled {
					t.Fatalf("mode %q: model cannot discover configured runtime", mode)
				}
				for _, d := range defs {
					if (mode == "" || mode == "code_only") && d.Name == "read_file" {
						t.Fatal("code_only exposed leaf tools")
					}
				}
			}
			if !realHost || mode == "direct" {
				return
			}
			// Exercise the real host through the normal thread toolkit. The adapter
			// keeps this a provider-free integration test; agent tests cover scheduling.
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err := os.WriteFile(filepath.Join(root, "sample.txt"), []byte("code-mode-read-ok"), 0600); err != nil {
				t.Fatal(err)
			}
			ctx = toolctx.WithNestedExecutor(ctx, codeModeTestScope{thread.Toolkit})
			args, _ := json.Marshal(map[string]any{
				"source": `text(await tools.read_file({path: "sample.txt"}));`, "yield_time_ms": 1,
			})
			result, err := thread.Toolkit.ExecuteResult(ctx, providers.ToolCall{ID: "code-mode-read", Name: "exec", Arguments: string(args)})
			var output strings.Builder
			for err == nil && !result.IsError {
				output.WriteString(result.TextProjection())
				var response struct {
					State  string `json:"state"`
					CellID string `json:"cell_id"`
				}
				if decodeErr := json.Unmarshal([]byte(result.TextProjection()), &response); decodeErr != nil {
					t.Fatal(decodeErr)
				}
				if response.State != "Yielded" {
					break
				}
				waitArgs, _ := json.Marshal(map[string]any{"cell_id": response.CellID, "yield_time_ms": 1000})
				result, err = thread.Toolkit.ExecuteResult(ctx, providers.ToolCall{ID: "code-mode-wait", Name: "wait", Arguments: string(waitArgs)})
			}
			if err != nil || result.IsError || !strings.Contains(output.String(), "code-mode-read-ok") {
				t.Fatalf("code-mode read: result=%s err=%v", result.TextProjection(), err)
			}
		})
	}
}

type codeModeTestScope struct {
	executor interface {
		ExecuteResult(context.Context, providers.ToolCall) (toolresult.Result, error)
	}
}

func (s codeModeTestScope) OutlivingNested() toolctx.NestedExecutor { return s }
func (s codeModeTestScope) Invoke(ctx context.Context, call providers.ToolCall) (toolresult.Result, error) {
	return s.executor.ExecuteResult(ctx, call)
}

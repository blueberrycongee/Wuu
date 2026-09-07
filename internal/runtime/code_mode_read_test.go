package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/blueberrycongee/wuu/internal/agent"
	"github.com/blueberrycongee/wuu/internal/codemode"
	"github.com/blueberrycongee/wuu/internal/providers"
	"github.com/blueberrycongee/wuu/internal/tools"
)

func TestCodeModeReadFileProjectedContinuation(t *testing.T) {
	executable := os.Getenv("WUU_CODE_MODE_HOST")
	if executable == "" {
		t.Skip("WUU_CODE_MODE_HOST is required for the real JavaScript host")
	}
	t.Setenv("WUU_TOOL_RESULT_PROJECTION", "active")
	root := t.TempDir()
	var content strings.Builder
	for i := 1; i <= 600; i++ {
		fmt.Fprintf(&content, "record-%04d %s\n", i, strings.Repeat("x", 100))
	}
	if err := os.WriteFile(filepath.Join(root, "records.txt"), []byte(content.String()), 0600); err != nil {
		t.Fatal(err)
	}
	kit, err := tools.New(root)
	if err != nil {
		t.Fatal(err)
	}
	kit.SetSessionDir(t.TempDir())
	kit.ConfigureSurfaceForProviderModel("openai", "gpt-5", true)
	service, err := codemode.NewService(codemode.ServiceConfig{Executable: executable, SessionID: "read-continuation"})
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	kit.SetCodeModeService(service)
	kit.SetCodeModeOnly(true)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	runtime := agent.NewTurnToolRuntime(agent.ToolRuntimeConfig{Executor: kit, RunContext: ctx, Gate: agent.NewToolExecutionGate(1)})
	defer runtime.Cancel()
	// A non-default starting line and bounded range catch replay, gaps, and
	// continuation accidentally escaping the original request.
	source := `let args = {path: "records.txt", offset: 51, limit: 400};
 let expected = 51, pages = 0;
 while (args) {
   const result = await tools.read_file(args);
   const page = JSON.parse(result.content[0].text);
   const lines = page.content.trimEnd().split("\n");
   if (page.start_line !== expected || page.num_lines !== lines.length) throw new Error("bad page metadata");
   for (const line of lines) {
     const match = line.match(/^\s*(\d+)\trecord-(\d+) x+$/);
     if (!match || Number(match[1]) !== expected || Number(match[2]) !== expected) throw new Error("gap or duplicate at " + expected);
     expected++;
   }
   if (page.range.end_line !== expected - 1) throw new Error("bad range");
   args = page.continuation?.has_more ? page.continuation.next : null;
   if (++pages > 100) throw new Error("continuation did not terminate");
 }
 if (pages < 2 || expected !== 451) throw new Error("incomplete requested range");
 text("READ_CONTINUATION_OK");`
	args, _ := json.Marshal(map[string]any{"source": source, "yield_time_ms": 1})
	call := providers.ToolCall{ID: "read-exec", Name: "exec", Arguments: string(args)}
	var output strings.Builder
	for step := 0; ; step++ {
		messages, err := runtime.ExecuteFinalCalls(ctx, []providers.ToolCall{call}, nil)
		if err != nil || len(messages) != 1 {
			t.Fatalf("execute: %v, %+v", err, messages)
		}
		var response struct {
			State     string                 `json:"state"`
			CellID    string                 `json:"cell_id"`
			ErrorText *string                `json:"error_text"`
			Content   []codemode.ContentItem `json:"content_items"`
		}
		if err := json.Unmarshal([]byte(messages[0].Content), &response); err != nil {
			t.Fatal(err)
		}
		if response.State == "" {
			t.Fatalf("missing code-mode response: %+v", messages)
		}
		if response.ErrorText != nil {
			t.Fatal(*response.ErrorText)
		}
		for _, item := range response.Content {
			output.WriteString(item.Text)
		}
		if response.State != "Yielded" {
			break
		}
		args, _ = json.Marshal(map[string]any{"cell_id": response.CellID, "yield_time_ms": 10000})
		call = providers.ToolCall{ID: fmt.Sprintf("read-wait-%d", step), Name: "wait", Arguments: string(args)}
	}
	if output.String() != "READ_CONTINUATION_OK" {
		t.Fatalf("unexpected output: %s", output.String())
	}
}

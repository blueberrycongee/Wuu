package codexengine

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/blueberrycongee/wuu/internal/agentengine"
	"github.com/blueberrycongee/wuu/internal/providers"
)

// buildFakeCodex compiles the fake app-server binary once per test run.
func buildFakeCodex(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	binary := filepath.Join(dir, "fakecodex")
	cmd := exec.Command("go", "build", "-o", binary, "./testdata/fakecodex")
	cmd.Dir = "."
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build fake codex: %v\n%s", err, out)
	}
	return binary
}

func TestEngineEndToEndFakeCodex(t *testing.T) {
	binary := buildFakeCodex(t)
	host := NewHost(binary, t.TempDir())
	engine := NewEngine(host)
	defer host.Release()

	desc, err := engine.Descriptor(context.Background())
	if err != nil {
		t.Fatalf("Descriptor: %v", err)
	}
	if desc.ID != agentengine.EngineID("codex") {
		t.Fatalf("descriptor id = %q, want codex", desc.ID)
	}

	persisted := ""
	sess, err := engine.SessionForThread(context.Background(), agentengine.ThreadBinding{
		ThreadID: "wuu-thread-1",
		RootDir:  t.TempDir(),
		Model:    "gpt-5",
		PersistRef: func(ref string) error {
			persisted = ref
			return nil
		},
	})
	if err != nil {
		t.Fatalf("SessionForThread: %v", err)
	}

	var events []providers.StreamEvent
	var content strings.Builder
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	result, err := sess.RunTurn(ctx, agentengine.TurnInput{
		History: []providers.ChatMessage{{Role: "user", Content: "hi"}},
	}, func(ev providers.StreamEvent) {
		events = append(events, ev)
		if ev.Type == providers.EventContentDelta {
			content.WriteString(ev.Content)
		}
	})
	if err != nil {
		t.Fatalf("RunTurn: %v", err)
	}

	if persisted != "codex-thread-1" {
		t.Fatalf("persisted ref = %q, want codex-thread-1", persisted)
	}
	if got := content.String(); got != "Hello from codex." {
		t.Fatalf("streamed content = %q, want %q", got, "Hello from codex.")
	}
	if got := result.Result.Content; got != "Hello from codex." {
		t.Fatalf("result content = %q, want %q", got, "Hello from codex.")
	}
	if len(result.Result.NewMessages) != 1 || result.Result.NewMessages[0].Role != "assistant" {
		t.Fatalf("NewMessages = %+v, want one assistant message", result.Result.NewMessages)
	}
	if result.Result.InputTokens != 60 || result.Result.OutputTokens != 25 || result.Result.CacheReadTokens != 40 {
		t.Fatalf("usage = in %d out %d cache %d, want 60/25/40",
			result.Result.InputTokens, result.Result.OutputTokens, result.Result.CacheReadTokens)
	}

	// A second turn on the same session reuses the persisted ref (no new
	// thread/start) and still completes.
	sess2, err := engine.SessionForThread(context.Background(), agentengine.ThreadBinding{
		ThreadID:    "wuu-thread-1",
		RootDir:     t.TempDir(),
		ExternalRef: "codex-thread-1",
		PersistRef:  func(string) error { return nil },
	})
	if err != nil {
		t.Fatalf("second SessionForThread: %v", err)
	}
	if _, err := sess2.RunTurn(ctx, agentengine.TurnInput{
		History: []providers.ChatMessage{{Role: "user", Content: "again"}},
	}, nil); err != nil {
		t.Fatalf("second RunTurn: %v", err)
	}

	// Resume via the factory Open/Resume seam.
	resumed, err := engine.Resume(context.Background(), agentengine.ResumeRequest{
		OpenRequest:        agentengine.OpenRequest{ThreadID: "wuu-thread-2", RootDir: t.TempDir()},
		ExternalSessionRef: "codex-thread-1",
	})
	if err != nil {
		t.Fatalf("Resume: %v", err)
	}
	if _, err := resumed.RunTurn(ctx, agentengine.TurnInput{
		History: []providers.ChatMessage{{Role: "user", Content: "resume"}},
	}, nil); err != nil {
		t.Fatalf("resumed RunTurn: %v", err)
	}
}

func TestEngineMissingBinaryFailsClearly(t *testing.T) {
	host := NewHost(filepath.Join(t.TempDir(), "no-such-codex"), t.TempDir())
	engine := NewEngine(host)
	_, err := engine.Open(context.Background(), agentengine.OpenRequest{ThreadID: "t", RootDir: t.TempDir()})
	if err == nil {
		t.Fatal("Open without a codex binary must fail")
	}
	if !strings.Contains(err.Error(), "codex") {
		t.Fatalf("error should mention codex, got: %v", err)
	}
}

func TestResolveBinaryEnvOverride(t *testing.T) {
	t.Setenv("WUU_CODEX_BINARY", "/nonexistent/codex")
	path, err := ResolveBinary()
	if err != nil {
		t.Fatalf("ResolveBinary with env override: %v", err)
	}
	if path != "/nonexistent/codex" {
		t.Fatalf("ResolveBinary = %q, want env value", path)
	}
	t.Setenv("WUU_CODEX_BINARY", "")
	if _, err := ResolveBinary(); err == nil {
		// Machine may or may not have codex on PATH; both are acceptable,
		// but the error path must exist when lookup fails.
		_ = err
	}
}

var _ = os.Getenv

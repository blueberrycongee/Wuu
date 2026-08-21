package claudeengine

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

func buildFakeClaude(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	binary := filepath.Join(dir, "fakeclaude")
	cmd := exec.Command("go", "build", "-o", binary, "./testdata/fakeclaude")
	cmd.Dir = "."
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build fake claude: %v\n%s", err, out)
	}
	return binary
}

func TestEngineEndToEndFakeClaude(t *testing.T) {
	binary := buildFakeClaude(t)
	engine := NewEngine(binary, t.TempDir())

	desc, err := engine.Descriptor(context.Background())
	if err != nil {
		t.Fatalf("Descriptor: %v", err)
	}
	if desc.ID != agentengine.EngineID("claude") {
		t.Fatalf("descriptor id = %q, want claude", desc.ID)
	}
	if !strings.Contains(desc.Version, "2.1.226") {
		t.Fatalf("descriptor version = %q, want fake 2.1.226", desc.Version)
	}

	persisted := ""
	sess, err := engine.SessionForThread(context.Background(), agentengine.ThreadBinding{
		ThreadID: "wuu-thread-1",
		RootDir:  t.TempDir(),
		PersistRef: func(ref string) error {
			persisted = ref
			return nil
		},
	})
	if err != nil {
		t.Fatalf("SessionForThread: %v", err)
	}

	var content strings.Builder
	var thinking strings.Builder
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	result, err := sess.RunTurn(ctx, agentengine.TurnInput{
		History: []providers.ChatMessage{{Role: "user", Content: "hi"}},
	}, func(ev providers.StreamEvent) {
		switch ev.Type {
		case providers.EventContentDelta:
			content.WriteString(ev.Content)
		case providers.EventThinkingDelta:
			thinking.WriteString(ev.Content)
		}
	})
	if err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	if persisted != "fake-session-1" {
		t.Fatalf("persisted ref = %q, want fake-session-1", persisted)
	}
	if got := content.String(); got != "Hello from claude." {
		t.Fatalf("content = %q, want %q", got, "Hello from claude.")
	}
	if !strings.Contains(thinking.String(), "let me think") {
		t.Fatalf("thinking = %q, want it to contain thinking delta", thinking.String())
	}
	if result.Result.InputTokens != 80 || result.Result.OutputTokens != 12 || result.Result.CacheReadTokens != 30 {
		t.Fatalf("usage = in %d out %d cache %d, want 80/12/30",
			result.Result.InputTokens, result.Result.OutputTokens, result.Result.CacheReadTokens)
	}
	if len(result.Result.NewMessages) != 1 || result.Result.NewMessages[0].Role != "assistant" {
		t.Fatalf("NewMessages = %+v, want one assistant message", result.Result.NewMessages)
	}

	// Second turn on the same session resumes with the persisted id.
	sess2, err := engine.SessionForThread(context.Background(), agentengine.ThreadBinding{
		ThreadID:    "wuu-thread-1",
		RootDir:     t.TempDir(),
		ExternalRef: "fake-session-1",
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

	// Resume through the factory seam.
	resumed, err := engine.Resume(context.Background(), agentengine.ResumeRequest{
		OpenRequest:        agentengine.OpenRequest{ThreadID: "wuu-thread-2", RootDir: t.TempDir()},
		ExternalSessionRef: "fake-session-1",
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
	engine := NewEngine(filepath.Join(t.TempDir(), "no-such-claude"), t.TempDir())
	// Open is lazy (the child spawns on the first turn), so the missing
	// binary surfaces as a clear RunTurn error.
	sess, err := engine.Open(context.Background(), agentengine.OpenRequest{ThreadID: "t", RootDir: t.TempDir()})
	if err != nil {
		t.Fatalf("Open must not fail before the first turn: %v", err)
	}
	_, err = sess.RunTurn(context.Background(), agentengine.TurnInput{
		History: []providers.ChatMessage{{Role: "user", Content: "hi"}},
	}, nil)
	if err == nil {
		t.Fatal("RunTurn without a claude binary must fail")
	}
	if !strings.Contains(err.Error(), "claude") {
		t.Fatalf("error should mention claude, got: %v", err)
	}
}

func TestResolveBinaryEnvOverride(t *testing.T) {
	t.Setenv("WUU_CLAUDE_BINARY", "/nonexistent/claude")
	path, err := ResolveBinary()
	if err != nil {
		t.Fatalf("ResolveBinary with env override: %v", err)
	}
	if path != "/nonexistent/claude" {
		t.Fatalf("ResolveBinary = %q, want env value", path)
	}
}

var _ = os.Getenv

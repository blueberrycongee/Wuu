package tools

import (
	"context"
	"path/filepath"
	"testing"

	proc "github.com/blueberrycongee/wuu/internal/process"
	"github.com/blueberrycongee/wuu/internal/providers"
)

func startBackgroundForRootThreadTest(t *testing.T, arguments string, bind func(kit *Toolkit)) proc.Process {
	t.Helper()
	root := t.TempDir()
	kit, err := New(root)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	manager, err := proc.NewManager(root, filepath.Join(t.TempDir(), "runtime"))
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	t.Cleanup(func() { _ = manager.CleanupSession() })
	kit.SetProcessManager(manager)
	bind(kit)

	if _, err := kit.Execute(context.Background(), providers.ToolCall{Name: "bash", Arguments: arguments}); err != nil {
		t.Fatalf("start background: %v", err)
	}

	list, err := manager.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected exactly one command session, got %d", len(list))
	}
	return list[0]
}

// The record has to name the conversation that owns the command, so a later
// thread-delete cascade has something to key off.
func TestStartBackgroundStampsBoundThreadOnRecord(t *testing.T) {
	got := startBackgroundForRootThreadTest(t,
		`{"action":"start_background","command":"sleep 60"}`,
		func(kit *Toolkit) { kit.SetSessionID("thread-abc") },
	)
	if got.RootThreadID != "thread-abc" {
		t.Fatalf("RootThreadID = %q, want thread-abc", got.RootThreadID)
	}
}

// root_thread_id is host state, not a model argument. A model that invents one
// must not be able to point the record at another conversation and escape its
// owner's cleanup.
func TestStartBackgroundIgnoresModelSuppliedRootThread(t *testing.T) {
	got := startBackgroundForRootThreadTest(t,
		`{"action":"start_background","command":"sleep 60","root_thread_id":"someone-elses-thread"}`,
		func(kit *Toolkit) { kit.SetSessionID("thread-abc") },
	)
	if got.RootThreadID != "thread-abc" {
		t.Fatalf("RootThreadID = %q, want the bound thread-abc, not the model-supplied value", got.RootThreadID)
	}
}

// A toolkit with no bound conversation (headless or workspace-level) records an
// empty owner rather than a fabricated one.
func TestStartBackgroundLeavesRootThreadEmptyWithoutSession(t *testing.T) {
	got := startBackgroundForRootThreadTest(t,
		`{"action":"start_background","command":"sleep 60"}`,
		func(kit *Toolkit) {},
	)
	if got.RootThreadID != "" {
		t.Fatalf("RootThreadID = %q, want empty for an unbound toolkit", got.RootThreadID)
	}
}

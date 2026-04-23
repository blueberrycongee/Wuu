package context

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSnapshotCacheHit(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root := cwd
	for {
		if _, err := os.Stat(filepath.Join(root, ".git")); err == nil {
			break
		}
		parent := filepath.Dir(root)
		if parent == root {
			t.Skip("not in a git repo")
		}
		root = parent
	}

	// Clear cache.
	snapshotCache.mu.Lock()
	snapshotCache.captured = time.Time{}
	snapshotCache.mu.Unlock()

	// First call warms cache.
	info1 := Snapshot(root)
	if info1.CWD == "" {
		t.Fatal("expected non-empty CWD")
	}

	// Second call should hit cache.
	info2 := Snapshot(root)
	if info2.GitBranch != info1.GitBranch {
		t.Fatalf("cache miss: branch changed from %q to %q", info1.GitBranch, info2.GitBranch)
	}
	if info2.GitStatus != info1.GitStatus {
		t.Fatalf("cache miss: status changed from %q to %q", info1.GitStatus, info2.GitStatus)
	}
}

func TestSnapshotCacheDifferentCWD(t *testing.T) {
	// Ensure cache stores per-CWD.
	snapshotCache.mu.Lock()
	snapshotCache.captured = time.Time{}
	snapshotCache.mu.Unlock()

	_ = Snapshot("/tmp/fake-a")
	info2 := Snapshot("/tmp/fake-b")

	// Second call with different CWD must not reuse first CWD.
	if info2.CWD != "/tmp/fake-b" {
		t.Fatalf("expected CWD /tmp/fake-b, got %q", info2.CWD)
	}
}

package tools

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/blueberrycongee/wuu/internal/processsandbox"
)

func TestProcessSandboxPolicyMapsWorkspaceBoundary(t *testing.T) {
	if !processsandbox.Supported() {
		t.Skip("filesystem process sandbox backend is not available on this platform")
	}
	root := t.TempDir()
	env := &Env{RootDir: root}
	policy, tempDir, err := env.processSandboxPolicy(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if policy == nil || policy.Mode != processsandbox.ModeWorkspaceWrite {
		t.Fatalf("default policy = %#v, want workspace-write", policy)
	}
	wantRoot, _ := filepath.EvalSymlinks(root)
	wantTemp, _ := filepath.EvalSymlinks(tempDir)
	normalized := make(map[string]bool)
	for _, candidate := range policy.WritableRoots {
		if evaluated, err := filepath.EvalSymlinks(candidate); err == nil {
			candidate = evaluated
		}
		normalized[candidate] = true
	}
	if !normalized[wantRoot] || !normalized[wantTemp] {
		t.Fatalf("writable roots = %#v, want workspace %q and temp %q", policy.WritableRoots, wantRoot, wantTemp)
	}

	env.boundaryConfigured = true
	env.AllowMutations = false
	policy, _, err = env.processSandboxPolicy(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if policy == nil || policy.Mode != processsandbox.ModeReadOnly || len(policy.WritableRoots) != 0 {
		t.Fatalf("read-only policy = %#v", policy)
	}

	env.Unconfined = true
	if policy, _, err := env.processSandboxPolicy(context.Background()); err != nil || policy != nil {
		t.Fatalf("unconfined policy = %#v, want nil", policy)
	}
}

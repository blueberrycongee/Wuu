package tools

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/blueberrycongee/wuu/internal/processsandbox"
)

type policyTestSandboxProvider struct{}

func (policyTestSandboxProvider) Confine(context.Context, []string, processsandbox.Policy) (processsandbox.ConfinedCommand, error) {
	return processsandbox.ConfinedCommand{}, nil
}

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

func TestProcessSandboxPolicyFailsClosedWithoutBackend(t *testing.T) {
	env := &Env{RootDir: t.TempDir()}
	policy, tempDir, err := env.processSandboxPolicyWithBuiltIn(context.Background(), false)
	if !errors.Is(err, processsandbox.ErrUnavailable) || policy != nil || tempDir != "" {
		t.Fatalf("missing backend = policy %#v temp %q error %v", policy, tempDir, err)
	}

	env.Unconfined = true
	if policy, _, err := env.processSandboxPolicyWithBuiltIn(context.Background(), false); err != nil || policy != nil {
		t.Fatalf("explicit unconfined mode must bypass confinement: policy %#v error %v", policy, err)
	}

	env.Unconfined = false
	env.ProcessSandboxProvider = policyTestSandboxProvider{}
	policy, _, err = env.processSandboxPolicyWithBuiltIn(context.Background(), false)
	if err != nil || policy == nil || policy.Mode != processsandbox.ModeWorkspaceWrite {
		t.Fatalf("custom provider policy = %#v error %v", policy, err)
	}
}

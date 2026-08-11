package tools

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/blueberrycongee/wuu/internal/processsandbox"
)

// processSandboxPolicy translates the existing workspace boundary into an OS
// filesystem policy. It never asks for per-call approval: policy changes remain
// a user-owned session/workspace setting. Platforms without Wuu's built-in
// backend may still confine processes through a configured provider.
func (e *Env) processSandboxPolicy(ctx context.Context) (*processsandbox.Policy, string, error) {
	return e.processSandboxPolicyWithBuiltIn(ctx, processsandbox.Supported())
}

func (e *Env) processSandboxPolicyWithBuiltIn(ctx context.Context, builtInAvailable bool) (*processsandbox.Policy, string, error) {
	if e == nil || e.Unconfined {
		return nil, "", nil
	}
	if e.ProcessSandboxProvider == nil && !builtInAvailable {
		return nil, "", fmt.Errorf("%w: no built-in backend is available; configure sandbox.process@1 or explicitly use unconfined mode", processsandbox.ErrUnavailable)
	}
	if e.boundaryConfigured && !e.AllowMutations {
		return &processsandbox.Policy{Mode: processsandbox.ModeReadOnly}, "", nil
	}

	roots := e.FileScopeRoots
	if len(roots) == 0 {
		roots = []string{e.RootDir}
	} else {
		roots = append([]string(nil), roots...)
	}
	if execRoot, err := e.ExecRootDir(ctx); err == nil && strings.TrimSpace(execRoot) != "" {
		replaced := false
		for i, root := range roots {
			if sameRuntimeFileScopePath(root, e.RootDir) {
				roots[i] = execRoot
				replaced = true
				break
			}
		}
		if !replaced {
			roots = append(roots, execRoot)
		}
	}
	tempDir, err := e.ensureProcessSandboxTempDir()
	if err != nil {
		return nil, "", err
	}
	roots = append(roots, tempDir)
	return &processsandbox.Policy{
		Mode:          processsandbox.ModeWorkspaceWrite,
		WritableRoots: roots,
	}, tempDir, nil
}

func (e *Env) ensureProcessSandboxTempDir() (string, error) {
	base := strings.TrimSpace(e.SessionDir)
	if base == "" {
		base = strings.TrimSpace(e.StateDir)
	}
	var tempDir string
	if base != "" {
		tempDir = filepath.Join(base, "process-tmp")
	} else {
		identity := strings.TrimSpace(e.RootDir) + "\x00" + strings.TrimSpace(e.SessionID)
		sum := sha256.Sum256([]byte(identity))
		tempDir = filepath.Join(os.TempDir(), "wuu-process-sandbox", fmt.Sprintf("%x", sum[:8]))
	}
	if err := os.MkdirAll(tempDir, 0o700); err != nil {
		return "", fmt.Errorf("create private process temp directory: %w", err)
	}
	if err := os.Chmod(tempDir, 0o700); err != nil {
		return "", fmt.Errorf("secure private process temp directory: %w", err)
	}
	return tempDir, nil
}

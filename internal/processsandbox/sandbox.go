// Package processsandbox confines subprocess filesystem effects to the same
// preselected workspace boundary used by Wuu's in-process tools.
package processsandbox

import (
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

type Mode string

const (
	ModeReadOnly       Mode = "read-only"
	ModeWorkspaceWrite Mode = "workspace-write"
)

var ErrUnavailable = errors.New("filesystem process sandbox is unavailable")

var denialSignatures = []string{
	"operation not permitted",
	"read-only file system",
}

// Policy is the complete filesystem-effect policy for one subprocess. Network
// access and process visibility are deliberately outside this contract.
type Policy struct {
	Mode          Mode
	WritableRoots []string
}

// Apply rewrites cmd so the original command runs under the host filesystem
// sandbox. Implementations must either install confinement or return an error;
// silently running the original command is forbidden.
func Apply(cmd *exec.Cmd, policy Policy) error {
	if cmd == nil {
		return errors.New("filesystem process sandbox requires a command")
	}
	if err := validatePolicy(policy); err != nil {
		return err
	}
	return applyPlatform(cmd, normalizedPolicy(policy))
}

// Supported reports whether this build has a filesystem process sandbox
// backend. Wuu currently ships the backend on macOS, its desktop platform.
func Supported() bool { return platformSupported() }

// IsDenied classifies a failed confined execution using the diagnostic dialect
// produced by the shipped backends. It is a result fact, not an approval
// trigger.
func IsDenied(exitCode int, output string) bool {
	if exitCode == 0 {
		return false
	}
	lower := strings.ToLower(output)
	for _, signature := range denialSignatures {
		if strings.Contains(lower, signature) {
			return true
		}
	}
	return false
}

// IsRunnerFailure distinguishes a failed sandbox launcher from a command that
// ran and was denied. The original command did not execute on this path.
func IsRunnerFailure(exitCode int, output string) bool {
	return exitCode != 0 && strings.Contains(strings.ToLower(output), "sandbox-exec:")
}

func validatePolicy(policy Policy) error {
	switch policy.Mode {
	case ModeReadOnly, ModeWorkspaceWrite:
		return nil
	default:
		return fmt.Errorf("unknown filesystem process sandbox mode %q", policy.Mode)
	}
}

func normalizedPolicy(policy Policy) Policy {
	if policy.Mode != ModeWorkspaceWrite {
		policy.WritableRoots = nil
		return policy
	}

	seen := make(map[string]struct{}, len(policy.WritableRoots))
	roots := make([]string, 0, len(policy.WritableRoots))
	for _, root := range policy.WritableRoots {
		root = strings.TrimSpace(root)
		if root == "" {
			continue
		}
		abs, err := filepath.Abs(root)
		if err != nil {
			continue
		}
		abs = filepath.Clean(abs)
		if evaluated, err := filepath.EvalSymlinks(abs); err == nil {
			abs = evaluated
		}
		if _, ok := seen[abs]; ok {
			continue
		}
		seen[abs] = struct{}{}
		roots = append(roots, abs)
	}
	sort.Strings(roots)
	policy.WritableRoots = roots
	return policy
}

func seatbeltString(value string) string {
	replacer := strings.NewReplacer(
		`\`, `\\`,
		`"`, `\"`,
		"\n", `\n`,
		"\r", `\r`,
		"\t", `\t`,
	)
	return `"` + replacer.Replace(value) + `"`
}

func seatbeltProfile(policy Policy) string {
	forms := []string{
		"(version 1)",
		"(allow default)",
		"(deny file-write*)",
		`(allow file-write* (literal "/dev/null"))`,
		`(allow file-write* (literal "/dev/ptmx"))`,
		`(allow file-write* (regex #"^/dev/ttys[0-9]+$"))`,
	}
	if policy.Mode == ModeWorkspaceWrite {
		for _, root := range policy.WritableRoots {
			forms = append(forms, fmt.Sprintf("(allow file-write* (subpath %s))", seatbeltString(root)))
		}
	}
	return strings.Join(forms, "\n")
}

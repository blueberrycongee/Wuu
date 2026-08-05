package processsandbox

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestNormalizedPolicyCanonicalizesAndDeduplicatesWritableRoots(t *testing.T) {
	root := t.TempDir()
	policy := normalizedPolicy(Policy{
		Mode:          ModeWorkspaceWrite,
		WritableRoots: []string{root, filepath.Join(root, "."), ""},
	})
	if len(policy.WritableRoots) != 1 {
		t.Fatalf("writable roots = %#v, want one canonical root", policy.WritableRoots)
	}
	if !filepath.IsAbs(policy.WritableRoots[0]) {
		t.Fatalf("writable root is not absolute: %q", policy.WritableRoots[0])
	}
}

func TestSeatbeltProfileExpressesOnlyFileWritePolicy(t *testing.T) {
	root := filepath.Join(t.TempDir(), "quoted\"root")
	profile := seatbeltProfile(normalizedPolicy(Policy{
		Mode:          ModeWorkspaceWrite,
		WritableRoots: []string{root},
	}))
	for _, required := range []string{
		"(allow default)",
		"(deny file-write*)",
		`(literal "/dev/null")`,
		"(subpath ",
	} {
		if !strings.Contains(profile, required) {
			t.Fatalf("profile missing %q:\n%s", required, profile)
		}
	}
	if strings.Contains(profile, "deny network") || strings.Contains(profile, "deny process") {
		t.Fatalf("filesystem-only profile overclaims another boundary:\n%s", profile)
	}
}

func TestSandboxFailureClassification(t *testing.T) {
	if !IsDenied(1, "bash: x: Operation not permitted") {
		t.Fatal("Seatbelt denial was not classified")
	}
	if IsDenied(0, "Operation not permitted") {
		t.Fatal("successful command was classified as denied")
	}
	if !IsRunnerFailure(1, "sandbox-exec: sandbox_init: invalid profile") {
		t.Fatal("runner failure was not classified")
	}
}

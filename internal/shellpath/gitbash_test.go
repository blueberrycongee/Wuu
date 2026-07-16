package shellpath

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

func chainEnv(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}

func existsSet(paths ...string) func(string) bool {
	set := make(map[string]bool, len(paths))
	for _, p := range paths {
		set[p] = true
	}
	return func(path string) bool { return set[path] }
}

func noLookPath(string) (string, error) { return "", errors.New("not found") }

func TestFindGitBashEnvOverrideWins(t *testing.T) {
	override := filepath.Join("D:", "tools", "bash.exe")
	got, err := findGitBash(
		chainEnv(map[string]string{"WUU_GIT_BASH_PATH": override}),
		existsSet(override),
		noLookPath,
	)
	if err != nil {
		t.Fatalf("findGitBash: %v", err)
	}
	if got != override {
		t.Fatalf("got %q, want %q", got, override)
	}
}

func TestFindGitBashEnvOverrideMissingFails(t *testing.T) {
	_, err := findGitBash(
		chainEnv(map[string]string{"WUU_GIT_BASH_PATH": "X:\\missing\\bash.exe"}),
		existsSet(),
		noLookPath,
	)
	if err == nil || !strings.Contains(err.Error(), "WUU_GIT_BASH_PATH") {
		t.Fatalf("want override error, got %v", err)
	}
}

func TestFindGitBashStandardInstallRoots(t *testing.T) {
	programFiles := filepath.Join("C:", "Program Files")
	installed := filepath.Join(programFiles, "Git", "bin", "bash.exe")
	got, err := findGitBash(
		chainEnv(map[string]string{"ProgramFiles": programFiles}),
		existsSet(installed),
		noLookPath,
	)
	if err != nil {
		t.Fatalf("findGitBash: %v", err)
	}
	if got != installed {
		t.Fatalf("got %q, want %q", got, installed)
	}
}

func TestFindGitBashDerivesFromGitExe(t *testing.T) {
	gitExe := filepath.Join(string(filepath.Separator)+"custom", "Git", "cmd", "git.exe")
	derived := filepath.Join(string(filepath.Separator)+"custom", "Git", "bin", "bash.exe")
	got, err := findGitBash(
		chainEnv(nil),
		existsSet(derived),
		func(name string) (string, error) {
			if name == "git" {
				return gitExe, nil
			}
			return "", errors.New("not found")
		},
	)
	if err != nil {
		t.Fatalf("findGitBash: %v", err)
	}
	if got != derived {
		t.Fatalf("got %q, want %q", got, derived)
	}
}

func TestFindGitBashRejectsRelativeLookPath(t *testing.T) {
	_, err := findGitBash(
		chainEnv(nil),
		func(string) bool { return true },
		func(name string) (string, error) {
			// A current-directory match must never be trusted.
			return name + ".exe", nil
		},
	)
	if err == nil {
		t.Fatal("want resolution failure for relative LookPath results")
	}
}

func TestFindGitBashSkipsSystemRootBash(t *testing.T) {
	systemRoot := string(filepath.Separator) + filepath.Join("Windows")
	wslBash := filepath.Join(systemRoot, "System32", "bash.exe")
	_, err := findGitBash(
		chainEnv(map[string]string{"SystemRoot": systemRoot}),
		existsSet(),
		func(name string) (string, error) {
			if name == "bash" {
				return wslBash, nil
			}
			return "", errors.New("not found")
		},
	)
	if err == nil || !strings.Contains(err.Error(), "Git Bash not found") {
		t.Fatalf("want install hint after skipping WSL shim, got %v", err)
	}
}

func TestFindGitBashAcceptsPathBashOutsideSystemRoot(t *testing.T) {
	msysBash := string(filepath.Separator) + filepath.Join("msys64", "usr", "bin", "bash.exe")
	got, err := findGitBash(
		chainEnv(map[string]string{"SystemRoot": string(filepath.Separator) + "Windows"}),
		existsSet(),
		func(name string) (string, error) {
			if name == "bash" {
				return msysBash, nil
			}
			return "", errors.New("not found")
		},
	)
	if err != nil {
		t.Fatalf("findGitBash: %v", err)
	}
	if got != msysBash {
		t.Fatalf("got %q, want %q", got, msysBash)
	}
}

func TestGitBashCommandEnvPrependsToolDirsOnce(t *testing.T) {
	bash := filepath.Join(string(filepath.Separator)+"git-install", "bin", "bash.exe")
	env := gitBashCommandEnv([]string{"Path=C:\\Windows", "HOME=C:\\Users\\dev"}, bash)

	var pathEntries []string
	for _, entry := range env {
		key, value, _ := strings.Cut(entry, "=")
		if strings.EqualFold(key, "PATH") {
			pathEntries = append(pathEntries, value)
		}
	}
	if len(pathEntries) != 1 {
		t.Fatalf("want exactly one PATH entry, got %d in %v", len(pathEntries), env)
	}
	usrBin := filepath.Join(string(filepath.Separator)+"git-install", "usr", "bin")
	if !strings.HasPrefix(pathEntries[0], usrBin) {
		t.Fatalf("PATH %q does not lead with %q", pathEntries[0], usrBin)
	}
	if !strings.HasSuffix(pathEntries[0], "C:\\Windows") {
		t.Fatalf("PATH %q lost original value", pathEntries[0])
	}

	assertHasEnv(t, env, "MSYS_NO_PATHCONV", "1")
	assertHasEnv(t, env, "MSYS2_ARG_CONV_EXCL", "*")
}

func TestGitBashCommandEnvKeepsExistingMsysSettings(t *testing.T) {
	bash := filepath.Join(string(filepath.Separator)+"git-install", "bin", "bash.exe")
	env := gitBashCommandEnv([]string{"MSYS_NO_PATHCONV=0"}, bash)
	assertHasEnv(t, env, "MSYS_NO_PATHCONV", "0")
	for _, entry := range env {
		if entry == "MSYS_NO_PATHCONV=1" {
			t.Fatalf("existing MSYS_NO_PATHCONV overridden: %v", env)
		}
	}
}

func assertHasEnv(t *testing.T, env []string, key, want string) {
	t.Helper()
	for _, entry := range env {
		gotKey, value, _ := strings.Cut(entry, "=")
		if strings.EqualFold(gotKey, key) {
			if value != want {
				t.Fatalf("%s = %q, want %q", key, value, want)
			}
			return
		}
	}
	t.Fatalf("env missing %s: %v", key, env)
}

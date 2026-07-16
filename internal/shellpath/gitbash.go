package shellpath

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

// gitBashInstallHint is surfaced whenever Windows command execution cannot
// find a bash interpreter.
const gitBashInstallHint = "Git Bash not found: install Git for Windows (https://git-scm.com/downloads/win) or set WUU_GIT_BASH_PATH to a bash.exe"

// findGitBash resolves the Git Bash interpreter on Windows. The chain is
// dependency-injected so the logic itself stays testable on every
// platform: env override → standard install roots → derive from git.exe →
// bare bash.exe on PATH (excluding the WSL shim under the system root,
// which cannot run commands against Windows paths).
func findGitBash(getenv func(string) string, fileExists func(string) bool, lookPath func(string) (string, error)) (string, error) {
	if override := strings.TrimSpace(getenv("WUU_GIT_BASH_PATH")); override != "" {
		if fileExists(override) {
			return override, nil
		}
		return "", fmt.Errorf("WUU_GIT_BASH_PATH %q does not exist", override)
	}
	roots := []string{
		getenv("ProgramFiles"),
		getenv("ProgramFiles(x86)"),
	}
	if localAppData := strings.TrimSpace(getenv("LOCALAPPDATA")); localAppData != "" {
		roots = append(roots, filepath.Join(localAppData, "Programs"))
	}
	for _, root := range roots {
		if strings.TrimSpace(root) == "" {
			continue
		}
		candidate := filepath.Join(root, "Git", "bin", "bash.exe")
		if fileExists(candidate) {
			return candidate, nil
		}
	}
	// Derive <install>\bin\bash.exe from a git.exe on PATH
	// (<install>\cmd\git.exe or <install>\bin\git.exe). LookPath results
	// that are not absolute (current-directory matches) are never trusted.
	if gitPath, err := lookPath("git"); err == nil && filepath.IsAbs(gitPath) {
		candidate := filepath.Join(filepath.Dir(filepath.Dir(gitPath)), "bin", "bash.exe")
		if fileExists(candidate) {
			return candidate, nil
		}
	}
	if bashPath, err := lookPath("bash"); err == nil && filepath.IsAbs(bashPath) {
		if !pathWithin(getenv("SystemRoot"), bashPath) {
			return bashPath, nil
		}
	}
	return "", errors.New(gitBashInstallHint)
}

// pathWithin reports whether path sits under root, comparing
// case-insensitively because Windows paths are.
func pathWithin(root, path string) bool {
	root = strings.TrimSpace(root)
	if root == "" {
		return false
	}
	rel, err := filepath.Rel(strings.ToLower(root), strings.ToLower(path))
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

// gitBashCommandEnv layers the Git Bash runtime requirements onto env:
// the install's usr/bin and mingw64/bin lead PATH so coreutils resolve
// deterministically without a login shell, and MSYS argument/path
// conversion is disabled so native-style arguments (cmd /c, taskkill
// /pid, C:\ paths) reach programs verbatim.
func gitBashCommandEnv(env []string, bashPath string) []string {
	installRoot := filepath.Dir(filepath.Dir(bashPath))
	prepend := strings.Join([]string{
		filepath.Join(installRoot, "usr", "bin"),
		filepath.Join(installRoot, "mingw64", "bin"),
	}, string(filepath.ListSeparator))

	out := make([]string, 0, len(env)+2)
	sawPath := false
	sawNoPathConv := false
	sawArgConvExcl := false
	for _, entry := range env {
		key, value, ok := strings.Cut(entry, "=")
		if !ok {
			out = append(out, entry)
			continue
		}
		switch {
		case strings.EqualFold(key, "PATH"):
			if !sawPath {
				entry = key + "=" + prepend + string(filepath.ListSeparator) + value
				sawPath = true
			}
		case strings.EqualFold(key, "MSYS_NO_PATHCONV"):
			sawNoPathConv = true
		case strings.EqualFold(key, "MSYS2_ARG_CONV_EXCL"):
			sawArgConvExcl = true
		}
		out = append(out, entry)
	}
	if !sawPath {
		out = append(out, "PATH="+prepend)
	}
	if !sawNoPathConv {
		out = append(out, "MSYS_NO_PATHCONV=1")
	}
	if !sawArgConvExcl {
		out = append(out, "MSYS2_ARG_CONV_EXCL=*")
	}
	return out
}

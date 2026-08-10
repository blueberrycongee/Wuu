package tools

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/blueberrycongee/wuu/internal/statepath"
)

func sensitivePathReason(path string) (string, bool) {
	normalized := filepath.ToSlash(strings.TrimSpace(path))
	if normalized == "" {
		return "", false
	}
	lower := strings.ToLower(normalized)
	parts := strings.Split(lower, "/")
	for _, part := range parts {
		part = strings.Trim(part, `"'`)
		switch {
		case part == ".git" || part == ".hg" || part == ".svn":
			return "version-control metadata", true
		case part == ".wuu" || part == ".wuu-state" || part == ".wuu-home":
			return "wuu runtime state", true
		case part == ".env" || strings.HasPrefix(part, ".env.") || strings.Contains(part, ".env"):
			return ".env file", true
		case part == ".netrc":
			return ".netrc credentials", true
		case part == ".npmrc" || part == ".pypirc" || part == ".pgpass":
			return "credential configuration", true
		case strings.Contains(part, "credential") || strings.Contains(part, "secret"):
			return "credential or secret path", true
		case part == "id_rsa" || part == "id_ed25519" || part == "id_ecdsa":
			return "SSH private key", true
		case strings.Contains(part, "private") && strings.Contains(part, "key"):
			return "private key path", true
		}
	}
	return "", false
}

func isSensitivePath(path string) bool {
	_, ok := sensitivePathReason(path)
	return ok
}

// isNamedAgentIdentityNotebookPath reports whether an absolute path belongs
// to a collaboration named agent's identity notebook. This is the only core
// file-tool exception under WUU_HOME: user and session memory belongs to the
// Memory plugin and must be reached through that plugin's tools.
func isNamedAgentIdentityNotebookPath(absPath string) bool {
	if strings.TrimSpace(absPath) == "" {
		return false
	}
	home, err := statepath.Home("")
	if err != nil {
		return false
	}
	agentsDir := filepath.Join(home, "channels", "agents")
	rel, err := filepath.Rel(agentsDir, absPath)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return false
	}
	parts := strings.Split(filepath.ToSlash(rel), "/")
	return len(parts) >= 2 && strings.TrimSpace(parts[0]) != "" && parts[1] == "memory"
}

// wuuCredentialFileNames are the app's own credential files at the root of
// the wuu home directory. They are floor-protected in every permission
// mode, including unconfined: no agent tool may read or write them. The
// agent never needs their contents to do its job, and the runtime-metadata
// exemption below exists for the memory notebook and session artifacts —
// not for these files.
var wuuCredentialFileNames = map[string]struct{}{
	"auth.json":        {},
	"credentials.json": {},
	"remote.json":      {},
	"phone.json":       {},
}

// isWuuCredentialPath reports whether absPath is one of the app's own
// credential files directly under the wuu home directory.
func isWuuCredentialPath(absPath string) bool {
	if strings.TrimSpace(absPath) == "" {
		return false
	}
	if _, ok := wuuCredentialFileNames[filepath.Base(filepath.Clean(absPath))]; !ok {
		return false
	}
	home, err := statepath.Home("")
	if err != nil {
		return false
	}
	return filepath.Dir(filepath.Clean(absPath)) == filepath.Clean(home)
}

func wuuCredentialRefusal(toolName, action, absPath string) error {
	return fmt.Errorf("%s refuses to %s wuu credential file %q: it stores the app's own login credentials and is never accessible to the agent in any permission mode. Ask the user to manage it outside the session", toolName, action, absPath)
}

// redactSensitiveReadContent masks credential values in content read from a
// sensitive path while unconfined. Confined modes never reach this helper:
// the read itself is refused by rejectSensitiveReadPath instead.
func redactSensitiveReadContent(env *Env, absPath, content string) string {
	if content == "" || env == nil || !env.BypassToolHardProtections() {
		return content
	}
	if _, ok := sensitivePathReason(env.NormalizeDisplayPath(absPath)); !ok {
		return content
	}
	return redactToolOutput(content)
}

func rejectSensitiveReadPath(env *Env, toolName, absPath string) error {
	if isWuuCredentialPath(absPath) {
		return wuuCredentialRefusal(toolName, "read", absPath)
	}
	if env.BypassToolHardProtections() {
		return nil
	}
	if env.AllowMutations && isNamedAgentIdentityNotebookPath(absPath) {
		return nil
	}
	displayPath := env.NormalizeDisplayPath(absPath)
	if reason, ok := sensitivePathReason(displayPath); ok {
		return fmt.Errorf("%s refuses to read sensitive path %q (%s). Use a safer metadata command or ask the user for explicit secret handling", toolName, displayPath, reason)
	}
	return nil
}

func rejectSensitiveToolPath(env *Env, toolName, action, absPath string) error {
	if isWuuCredentialPath(absPath) {
		return wuuCredentialRefusal(toolName, action, absPath)
	}
	// Sensitive-path writes stay blocked in every mode, including
	// unconfined: lifting the path boundary does not lift secret guards.
	// A named agent's identity notebook is explicit collaboration state and is
	// included in that agent's file scope. Other WUU_HOME state stays behind
	// dedicated core or plugin APIs.
	if env.AllowMutations && isNamedAgentIdentityNotebookPath(absPath) {
		return nil
	}
	displayPath := env.NormalizeDisplayPath(absPath)
	if reason, ok := sensitivePathReason(displayPath); ok {
		return fmt.Errorf("%s refuses to %s sensitive path %q (%s). Use dedicated metadata-safe tools or ask the user for explicit secret handling", toolName, action, displayPath, reason)
	}
	return nil
}

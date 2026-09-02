package grokbuild

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/blueberrycongee/wuu/internal/grokbuildspec"
)

type credentialSource struct {
	explicitToken string
	home          string
	reuseCLI      bool
}

type authEntry struct {
	Key         string `json:"key"`
	Token       string `json:"token"`
	AccessToken string `json:"access_token"`
}

func (s credentialSource) token() (string, error) {
	if token := strings.TrimSpace(s.explicitToken); token != "" {
		return token, nil
	}
	if !s.reuseCLI {
		return "", errors.New("no Grok Build credential configured; set reuse_grok_credentials to true to read the local Grok CLI login")
	}
	return loadCLIToken(s.home)
}

func loadCLIToken(home string) (string, error) {
	path, err := cliAuthPath(home)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read Grok CLI credentials: %w", err)
	}
	var entries map[string]authEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return "", fmt.Errorf("parse Grok CLI credentials: %w", err)
	}
	var entry authEntry
	found := false
	for _, scope := range []string{grokbuildspec.OAuthCredentialScope, grokbuildspec.CredentialIssuer} {
		if entry, found = entries[scope]; found {
			break
		}
	}
	if !found {
		return "", errors.New("Grok CLI credentials do not contain a supported xAI login; run `grok login`")
	}
	token := firstNonEmpty(entry.Key, entry.Token, entry.AccessToken)
	if token == "" {
		return "", errors.New("Grok CLI login token is empty; run `grok login`")
	}
	return token, nil
}

func cliAuthPath(home string) (string, error) {
	grokHome := strings.TrimSpace(os.Getenv("GROK_HOME"))
	if grokHome == "" {
		home = strings.TrimSpace(home)
		if home == "" {
			home = strings.TrimSpace(os.Getenv("HOME"))
		}
		if home == "" {
			return "", errors.New("home directory is required for Grok Build credentials")
		}
		grokHome = filepath.Join(home, ".grok")
	}
	return filepath.Join(grokHome, "auth.json"), nil
}

// LocalOAuthStatus reports whether a reusable Grok CLI login is available.
// It never refreshes or writes the CLI-owned credential file.
func LocalOAuthStatus(home string) (string, error) {
	if _, err := loadCLIToken(home); err != nil {
		return "", err
	}
	return "grok-cli", nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

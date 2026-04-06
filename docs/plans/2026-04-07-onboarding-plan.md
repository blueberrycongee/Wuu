# First-Run Onboarding Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add an interactive TUI-based onboarding flow that runs on first launch, letting users configure their provider and theme without editing JSON.

**Architecture:** Onboarding is a separate Bubble Tea program that runs before the main TUI when no config is found. It writes three files: `~/.config/wuu/auth.json` (API keys), `.wuu.json` (project config), and `~/.config/wuu/config.json` (global prefs). The provider factory gains a third key resolution fallback from auth.json.

**Tech Stack:** Go, Bubble Tea (bubbletea + bubbles textarea), lipgloss, existing config package.

---

### Task 1: Auth Store — `internal/config/auth.go`

**Files:**
- Create: `internal/config/auth.go`
- Test: `internal/config/auth_test.go`

**Step 1: Write the failing test**

```go
// internal/config/auth_test.go
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAuthStore_RoundTrip(t *testing.T) {
	home := t.TempDir()

	// Save a key.
	if err := SaveAuthKey(home, "openai", "sk-test-key-123"); err != nil {
		t.Fatalf("SaveAuthKey: %v", err)
	}

	// Load it back.
	key, err := LoadAuthKey(home, "openai")
	if err != nil {
		t.Fatalf("LoadAuthKey: %v", err)
	}
	if key != "sk-test-key-123" {
		t.Fatalf("expected sk-test-key-123, got %q", key)
	}

	// Check file permissions.
	path := filepath.Join(home, ".config", "wuu", "auth.json")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat auth.json: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("expected 0600 permissions, got %o", info.Mode().Perm())
	}
}

func TestAuthStore_UnknownProvider(t *testing.T) {
	home := t.TempDir()
	_, err := LoadAuthKey(home, "nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown provider")
	}
}

func TestAuthStore_MultipleProviders(t *testing.T) {
	home := t.TempDir()

	SaveAuthKey(home, "openai", "sk-openai")
	SaveAuthKey(home, "anthropic", "sk-ant-xxx")

	k1, _ := LoadAuthKey(home, "openai")
	k2, _ := LoadAuthKey(home, "anthropic")

	if k1 != "sk-openai" {
		t.Fatalf("openai key mismatch: %q", k1)
	}
	if k2 != "sk-ant-xxx" {
		t.Fatalf("anthropic key mismatch: %q", k2)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/blueberrycongee/wuu && go test ./internal/config/ -run TestAuthStore -v`
Expected: FAIL — `SaveAuthKey` and `LoadAuthKey` undefined.

**Step 3: Write minimal implementation**

```go
// internal/config/auth.go
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const authRelativePath = ".config/wuu/auth.json"

type authStore struct {
	Keys map[string]string `json:"keys"`
}

// SaveAuthKey persists an API key for the given provider name.
func SaveAuthKey(home, providerName, apiKey string) error {
	path := filepath.Join(home, authRelativePath)
	store, _ := loadAuthStore(path)
	if store.Keys == nil {
		store.Keys = make(map[string]string)
	}
	store.Keys[providerName] = apiKey
	return writeAuthStore(path, store)
}

// LoadAuthKey reads the API key for the given provider name.
func LoadAuthKey(home, providerName string) (string, error) {
	path := filepath.Join(home, authRelativePath)
	store, err := loadAuthStore(path)
	if err != nil {
		return "", err
	}
	key, ok := store.Keys[providerName]
	if !ok || key == "" {
		return "", fmt.Errorf("no auth key for provider %q", providerName)
	}
	return key, nil
}

func loadAuthStore(path string) (authStore, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return authStore{Keys: make(map[string]string)}, err
	}
	var store authStore
	if err := json.Unmarshal(data, &store); err != nil {
		return authStore{Keys: make(map[string]string)}, err
	}
	if store.Keys == nil {
		store.Keys = make(map[string]string)
	}
	return store, nil
}

func writeAuthStore(path string, store authStore) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create auth dir: %w", err)
	}
	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o600)
}
```

**Step 4: Run test to verify it passes**

Run: `cd /Users/blueberrycongee/wuu && go test ./internal/config/ -run TestAuthStore -v`
Expected: PASS (3 tests).

**Step 5: Commit**

```bash
git add internal/config/auth.go internal/config/auth_test.go
git commit -m "feat: add global auth key store (~/.config/wuu/auth.json)"
```

---

### Task 2: Global Config — `internal/config/global.go`

**Files:**
- Create: `internal/config/global.go`
- Test: `internal/config/global_test.go`

**Step 1: Write the failing test**

```go
// internal/config/global_test.go
package config

import "testing"

func TestGlobalConfig_RoundTrip(t *testing.T) {
	home := t.TempDir()

	gc := GlobalConfig{Theme: "dark", HasCompletedOnboarding: true}
	if err := SaveGlobalConfig(home, gc); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := LoadGlobalConfig(home)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.Theme != "dark" {
		t.Fatalf("theme mismatch: %q", loaded.Theme)
	}
	if !loaded.HasCompletedOnboarding {
		t.Fatal("expected onboarding completed")
	}
}

func TestGlobalConfig_DefaultsWhenMissing(t *testing.T) {
	home := t.TempDir()
	gc, err := LoadGlobalConfig(home)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if gc.Theme != "" {
		t.Fatalf("expected empty theme, got %q", gc.Theme)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/blueberrycongee/wuu && go test ./internal/config/ -run TestGlobalConfig -v`
Expected: FAIL — types undefined.

**Step 3: Write minimal implementation**

```go
// internal/config/global.go
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const globalConfigRelPath = ".config/wuu/config.json"

// GlobalConfig stores user-wide preferences.
type GlobalConfig struct {
	Theme                  string `json:"theme,omitempty"`
	HasCompletedOnboarding bool   `json:"has_completed_onboarding,omitempty"`
}

// LoadGlobalConfig reads ~/.config/wuu/config.json. Returns zero value if missing.
func LoadGlobalConfig(home string) (GlobalConfig, error) {
	path := filepath.Join(home, globalConfigRelPath)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return GlobalConfig{}, nil
		}
		return GlobalConfig{}, err
	}
	var gc GlobalConfig
	if err := json.Unmarshal(data, &gc); err != nil {
		return GlobalConfig{}, fmt.Errorf("parse global config: %w", err)
	}
	return gc, nil
}

// SaveGlobalConfig writes ~/.config/wuu/config.json.
func SaveGlobalConfig(home string, gc GlobalConfig) error {
	path := filepath.Join(home, globalConfigRelPath)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create global config dir: %w", err)
	}
	data, err := json.MarshalIndent(gc, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}
```

**Step 4: Run test to verify it passes**

Run: `cd /Users/blueberrycongee/wuu && go test ./internal/config/ -run TestGlobalConfig -v`
Expected: PASS (2 tests).

**Step 5: Commit**

```bash
git add internal/config/global.go internal/config/global_test.go
git commit -m "feat: add global config store for theme and onboarding state"
```

---

### Task 3: Provider Factory — Add auth.json Fallback

**Files:**
- Modify: `internal/providerfactory/factory.go:80-98`
- Test: `internal/providerfactory/factory_test.go`

**Step 1: Read existing factory_test.go to understand test patterns**

Run: `cat internal/providerfactory/factory_test.go`

**Step 2: Write the failing test**

Add to `internal/providerfactory/factory_test.go`:

```go
func TestResolveAPIKey_AuthStoreFallback(t *testing.T) {
	home := t.TempDir()
	// Save key to auth store.
	if err := config.SaveAuthKey(home, "myapi", "sk-from-auth-store"); err != nil {
		t.Fatalf("save auth key: %v", err)
	}

	provider := config.ProviderConfig{
		Type:    "openai-compatible",
		BaseURL: "https://example.com/v1",
		Model:   "test",
		// No APIKey, no APIKeyEnv set.
	}

	key, err := ResolveAPIKeyWithHome(provider, "myapi", home)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if key != "sk-from-auth-store" {
		t.Fatalf("expected sk-from-auth-store, got %q", key)
	}
}
```

**Step 3: Run test to verify it fails**

Run: `cd /Users/blueberrycongee/wuu && go test ./internal/providerfactory/ -run TestResolveAPIKey_AuthStoreFallback -v`
Expected: FAIL — `ResolveAPIKeyWithHome` undefined.

**Step 4: Implement the fallback**

Modify `internal/providerfactory/factory.go`:

1. Export a new `ResolveAPIKeyWithHome` function that takes `home` as parameter.
2. Add auth.json fallback as step 3 in key resolution.
3. Update `resolveAPIKey` to call `ResolveAPIKeyWithHome` with `os.Getenv("HOME")`.

```go
// ResolveAPIKeyWithHome resolves API key with explicit home directory.
func ResolveAPIKeyWithHome(provider config.ProviderConfig, providerName, home string) (string, error) {
	// 1. Explicit api_key in config.
	if strings.TrimSpace(provider.APIKey) != "" {
		return strings.TrimSpace(provider.APIKey), nil
	}

	// 2. Environment variable.
	envKey := strings.TrimSpace(provider.APIKeyEnv)
	if envKey == "" {
		envKey = defaultAPIKeyEnv(normalizeType(provider.Type))
	}
	if envKey != "" {
		value := strings.TrimSpace(os.Getenv(envKey))
		if value != "" {
			return value, nil
		}
	}

	// 3. Global auth store.
	if home != "" {
		key, err := config.LoadAuthKey(home, providerName)
		if err == nil && key != "" {
			return key, nil
		}
	}

	return "", fmt.Errorf("no API key found for provider %q (set api_key, %s env var, or run wuu init)", providerName, envKey)
}
```

Update `resolveAPIKey` to delegate:

```go
func resolveAPIKey(provider config.ProviderConfig) (string, error) {
	return ResolveAPIKeyWithHome(provider, "", os.Getenv("HOME"))
}
```

Also update `BuildClient` and `BuildStreamClient` to pass provider name through. This requires changing their signatures to accept `providerName string` or extracting it. The simplest approach: add a `ResolveAPIKeyForProvider` variant that `main.go` can call with the resolved name.

**Step 5: Run all factory tests**

Run: `cd /Users/blueberrycongee/wuu && go test ./internal/providerfactory/ -v`
Expected: PASS.

**Step 6: Commit**

```bash
git add internal/providerfactory/factory.go internal/providerfactory/factory_test.go
git commit -m "feat: add auth.json fallback to API key resolution"
```

---

### Task 4: Onboarding TUI Model — `internal/tui/onboarding.go`

This is the largest task. It creates the interactive onboarding screen.

**Files:**
- Create: `internal/tui/onboarding.go`
- Test: `internal/tui/onboarding_test.go`

**Step 1: Write the failing test for OnboardingResult generation**

```go
// internal/tui/onboarding_test.go
package tui

import "testing"

func TestOnboardingResult(t *testing.T) {
	m := OnboardingModel{
		providerType: "openai",
		baseURL:      "https://api.openai.com/v1",
		apiKey:       "sk-test",
		model:        "gpt-4.1",
		theme:        "dark",
		step:         stepDone,
	}

	result := m.Result()
	if result.ProviderType != "openai" {
		t.Fatalf("provider type: %q", result.ProviderType)
	}
	if result.BaseURL != "https://api.openai.com/v1" {
		t.Fatalf("base url: %q", result.BaseURL)
	}
	if result.APIKey != "sk-test" {
		t.Fatalf("api key: %q", result.APIKey)
	}
	if result.Model != "gpt-4.1" {
		t.Fatalf("model: %q", result.Model)
	}
	if result.Theme != "dark" {
		t.Fatalf("theme: %q", result.Theme)
	}
	if !result.Completed {
		t.Fatal("expected completed")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/blueberrycongee/wuu && go test ./internal/tui/ -run TestOnboardingResult -v`
Expected: FAIL — types undefined.

**Step 3: Write the OnboardingModel**

Create `internal/tui/onboarding.go` with:

- `onboardingStep` enum: `stepProviderType`, `stepBaseURL`, `stepAPIKey`, `stepModel`, `stepTheme`, `stepDone`
- `OnboardingModel` struct with fields: step, cursor, providerType, baseURL, apiKey, model, theme, textInput, width, height, err
- `OnboardingResult` struct returned when done
- `NewOnboardingModel() OnboardingModel`
- `Init() tea.Cmd` — returns nil
- `Update(msg tea.Msg) (tea.Model, tea.Cmd)` — handles key events per step:
  - List steps (providerType, model, theme): up/down to move cursor, enter to select
  - Text steps (baseURL, apiKey): textarea input, enter to confirm
  - Esc: go back one step (or quit on first step)
- `View() string` — renders current step as a centered card
- `Result() OnboardingResult` — extracts collected values
- Helper: `providerTypeOptions()`, `modelOptions(providerType)`, `themeOptions()`
- Helper: `defaultBaseURL(providerType)` returns pre-filled URL

Provider type options:
- "openai" → display "OpenAI", base URL "https://api.openai.com/v1"
- "anthropic" → display "Anthropic", base URL "https://api.anthropic.com"
- "openai-compatible" → display "OpenAI-Compatible (third-party)", base URL "" (user input)

Model options per provider:
- openai: ["gpt-4.1", "gpt-4.1-mini", "gpt-4.1-nano"]
- anthropic: ["claude-sonnet-4-20250514", "claude-3-5-haiku-latest"]
- openai-compatible: [] (text input only)

Theme options: ["dark", "light"]

API key input: use textarea with custom rendering that masks all but last 4 chars.

**Step 4: Run test to verify it passes**

Run: `cd /Users/blueberrycongee/wuu && go test ./internal/tui/ -run TestOnboardingResult -v`
Expected: PASS.

**Step 5: Write step transition test**

```go
func TestOnboardingStepTransitions(t *testing.T) {
	m := NewOnboardingModel()
	if m.step != stepProviderType {
		t.Fatalf("initial step: %d", m.step)
	}

	// Simulate selecting "openai" (cursor 0) and pressing enter.
	// After provider type, should move to baseURL.
	m.cursor = 0
	m.selectCurrentOption()
	if m.step != stepBaseURL {
		t.Fatalf("after provider select, step: %d", m.step)
	}
	if m.providerType != "openai" {
		t.Fatalf("provider type: %q", m.providerType)
	}
	if m.baseURL != "https://api.openai.com/v1" {
		t.Fatalf("base url not pre-filled: %q", m.baseURL)
	}
}
```

**Step 6: Run test, implement `selectCurrentOption` if needed**

Run: `cd /Users/blueberrycongee/wuu && go test ./internal/tui/ -run TestOnboardingStep -v`
Expected: PASS.

**Step 7: Commit**

```bash
git add internal/tui/onboarding.go internal/tui/onboarding_test.go
git commit -m "feat: add onboarding TUI model with step-by-step provider and theme setup"
```

---

### Task 5: Wire Onboarding into main.go

**Files:**
- Modify: `cmd/wuu/main.go:55-85` (runInit)
- Modify: `cmd/wuu/main.go:176-276` (runTUI)

**Step 1: Modify `runTUI` to detect missing config and run onboarding**

In `runTUI`, after `config.LoadFrom` fails:

```go
cfg, configPath, err := config.LoadFrom(rootDir, os.Getenv("HOME"))
if err != nil {
	// No config found — run onboarding.
	result, onboardErr := runOnboarding()
	if onboardErr != nil {
		return onboardErr
	}
	if !result.Completed {
		return nil // user cancelled
	}

	// Write config files from onboarding result.
	if writeErr := writeOnboardingResult(rootDir, os.Getenv("HOME"), result); err != nil {
		return writeErr
	}

	// Reload config.
	cfg, configPath, err = config.LoadFrom(rootDir, os.Getenv("HOME"))
	if err != nil {
		return fmt.Errorf("config still invalid after onboarding: %w", err)
	}
}
```

**Step 2: Implement `runOnboarding` helper**

```go
func runOnboarding() (tui.OnboardingResult, error) {
	m := tui.NewOnboardingModel()
	p := tea.NewProgram(m, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return tui.OnboardingResult{}, fmt.Errorf("onboarding: %w", err)
	}
	om, ok := finalModel.(tui.OnboardingModel)
	if !ok {
		return tui.OnboardingResult{}, fmt.Errorf("unexpected model type")
	}
	return om.Result(), nil
}
```

**Step 3: Implement `writeOnboardingResult` helper**

```go
func writeOnboardingResult(rootDir, home string, r tui.OnboardingResult) error {
	// 1. Save API key to global auth store.
	providerName := r.ProviderType
	if providerName == "openai-compatible" {
		providerName = "custom"
	}
	if err := config.SaveAuthKey(home, providerName, r.APIKey); err != nil {
		return fmt.Errorf("save auth key: %w", err)
	}

	// 2. Write .wuu.json (no API key in it).
	cfg := config.Config{
		DefaultProvider: providerName,
		Providers: map[string]config.ProviderConfig{
			providerName: {
				Type:    r.ProviderType,
				BaseURL: r.BaseURL,
				Model:   r.Model,
			},
		},
		Agent: config.Default().Agent,
	}
	configPath := filepath.Join(rootDir, ".wuu.json")
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(configPath, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	// 3. Save global preferences.
	gc := config.GlobalConfig{
		Theme:                  r.Theme,
		HasCompletedOnboarding: true,
	}
	return config.SaveGlobalConfig(home, gc)
}
```

**Step 4: Update `runInit` to also use onboarding**

Replace the current `runInit` body to run the onboarding TUI instead of writing a template:

```go
func runInit(args []string) error {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	force := fs.Bool("force", false, "overwrite existing .wuu.json")
	if err := fs.Parse(args); err != nil {
		return err
	}

	workdir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get current directory: %w", err)
	}
	configPath := filepath.Join(workdir, ".wuu.json")

	if !*force {
		if _, err := os.Stat(configPath); err == nil {
			return fmt.Errorf("%s already exists (use --force to overwrite)", configPath)
		}
	}

	result, err := runOnboarding()
	if err != nil {
		return err
	}
	if !result.Completed {
		fmt.Println("setup cancelled")
		return nil
	}

	return writeOnboardingResult(workdir, os.Getenv("HOME"), result)
}
```

**Step 5: Build and smoke test**

Run: `cd /Users/blueberrycongee/wuu && go build -o wuu ./cmd/wuu/`
Expected: Compiles without errors.

Manual test: `cd /tmp && mkdir test-wuu && cd test-wuu && /Users/blueberrycongee/wuu/wuu`
Expected: Onboarding TUI appears (no .wuu.json in /tmp/test-wuu).

**Step 6: Commit**

```bash
git add cmd/wuu/main.go
git commit -m "feat: wire onboarding into wuu init and first-run TUI launch"
```

---

### Task 6: Run Full Test Suite and Fix

**Step 1: Run all tests**

Run: `cd /Users/blueberrycongee/wuu && go test ./... -v`
Expected: All tests pass.

**Step 2: Fix any failures**

Address compilation errors or test failures from the integration.

**Step 3: Build final binary**

Run: `cd /Users/blueberrycongee/wuu && go build -o wuu ./cmd/wuu/`
Expected: Clean build.

**Step 4: Commit if any fixes were needed**

```bash
git add -A
git commit -m "fix: resolve test failures from onboarding integration"
```

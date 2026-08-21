package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/blueberrycongee/wuu/internal/securefs"
)

// EnginesConfig configures external agent engines (codex, claude) in the
// desktop settings. A nil section means auto-detection: the engine is
// available when its CLI binary is found. The settings are machine-local and
// live in the settings JSON next to the rest of the user config.
type EnginesConfig struct {
	// DefaultEngine is the engine id used for new threads when the caller
	// does not request one explicitly. Empty means the built-in wuu engine.
	DefaultEngine string              `json:"default_engine,omitempty"`
	Codex         *EngineBinaryConfig `json:"codex,omitempty"`
	Claude        *EngineBinaryConfig `json:"claude,omitempty"`
}

// EngineBinaryConfig configures one external engine's binary.
type EngineBinaryConfig struct {
	// Enabled nil = auto (binary found => available), false = explicitly
	// disabled, true = explicitly enabled.
	Enabled *bool `json:"enabled,omitempty"`
	// BinaryPath overrides the PATH lookup. Empty uses the default lookup.
	BinaryPath string `json:"binary_path,omitempty"`
}

// EngineBinaryUpdate is the mutable view of EngineBinaryConfig from the UI.
type EngineBinaryUpdate struct {
	Enabled    *bool   `json:"enabled,omitempty"`
	BinaryPath *string `json:"binary_path,omitempty"`
}

// EnginesSettingsUpdate is the engine/update request body.
type EnginesSettingsUpdate struct {
	DefaultEngine *string             `json:"default_engine,omitempty"`
	Codex         *EngineBinaryUpdate `json:"codex,omitempty"`
	Claude        *EngineBinaryUpdate `json:"claude,omitempty"`
}

// UpdateEnginesSettings persists engine settings into the config JSON,
// following the same raw-map merge pattern as UpdateGeneralSettings.
func UpdateEnginesSettings(configPath string, update EnginesSettingsUpdate) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	engines, _ := raw["engines"].(map[string]any)
	if engines == nil {
		engines = make(map[string]any)
		raw["engines"] = engines
	}
	applyEngineBinaryUpdate(engines, "codex", update.Codex)
	applyEngineBinaryUpdate(engines, "claude", update.Claude)
	if update.DefaultEngine != nil {
		defaultEngine := strings.TrimSpace(*update.DefaultEngine)
		if defaultEngine == "" || defaultEngine == "wuu" {
			delete(engines, "default_engine")
		} else {
			engines["default_engine"] = defaultEngine
		}
	}
	resetDisabledDefaultEngine(engines, "codex", update.Codex)
	resetDisabledDefaultEngine(engines, "claude", update.Claude)
	if len(engines) == 0 {
		delete(raw, "engines")
	}

	out, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	var cfg Config
	if err := json.Unmarshal(out, &cfg); err != nil {
		return err
	}
	applyDefaults(&cfg)
	if err := cfg.Validate(); err != nil {
		return err
	}
	return securefs.WriteFileAtomic(configPath, append(out, '\n'))
}

func applyEngineBinaryUpdate(engines map[string]any, name string, update *EngineBinaryUpdate) {
	if update == nil {
		return
	}
	entry, _ := engines[name].(map[string]any)
	if entry == nil {
		entry = make(map[string]any)
		engines[name] = entry
	}
	if update.Enabled != nil {
		if *update.Enabled {
			entry["enabled"] = true
		} else {
			entry["enabled"] = false
		}
	}
	if update.BinaryPath != nil {
		if path := strings.TrimSpace(*update.BinaryPath); path != "" {
			entry["binary_path"] = path
		} else {
			delete(entry, "binary_path")
		}
	}
	if len(entry) == 0 {
		delete(engines, name)
	}
}

func resetDisabledDefaultEngine(engines map[string]any, name string, update *EngineBinaryUpdate) {
	if update == nil || update.Enabled == nil || *update.Enabled {
		return
	}
	if current, _ := engines["default_engine"].(string); strings.TrimSpace(current) == name {
		delete(engines, "default_engine")
	}
}

// EngineEnabled reports whether the engine is explicitly enabled (nil config
// means auto, which the caller resolves against binary availability).
func (c *EngineBinaryConfig) EngineEnabled() (enabled bool, explicit bool) {
	if c == nil || c.Enabled == nil {
		return true, false
	}
	return *c.Enabled, true
}

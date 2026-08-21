package appserver

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/blueberrycongee/wuu/internal/agentengine"
	"github.com/blueberrycongee/wuu/internal/claudeengine"
	"github.com/blueberrycongee/wuu/internal/codexengine"
	"github.com/blueberrycongee/wuu/internal/config"
)

// handleEngineList reports the engine inventory and the persisted engine
// settings for the settings UI.
func (s *Server) handleEngineList(req Request) error {
	if s.rt == nil {
		return s.writeResponse(req.ID, nil, errors.New("runtime is not initialized"))
	}
	result := EngineListResult{
		Engines:  s.engineInventory(),
		Settings: s.engineSettingsFromConfig(),
	}
	return s.writeResponse(req.ID, result, nil)
}

// handleEngineUpdate persists engine settings and applies them to the live
// runtime (enable/disable engines, binary path overrides, default engine).
func (s *Server) handleEngineUpdate(req Request) error {
	var params EngineUpdateParams
	if err := decodeParams(req.Params, &params); err != nil {
		return s.writeResponse(req.ID, nil, err)
	}
	if s.rt == nil {
		return s.writeResponse(req.ID, nil, errors.New("runtime is not initialized"))
	}
	update := config.EnginesSettingsUpdate{
		DefaultEngine: strings.TrimSpace(params.DefaultEngine),
		Codex:         params.Codex,
		Claude:        params.Claude,
	}
	if err := config.UpdateEnginesSettings(s.rt.ConfigPath, update); err != nil {
		return s.writeResponse(req.ID, nil, err)
	}
	s.applyEngineSettingsToRuntime()
	return s.writeResponse(req.ID, EngineListResult{
		Engines:  s.engineInventory(),
		Settings: s.engineSettingsFromConfig(),
	}, nil)
}

// engineInventory lists the registered engines with binary status.
func (s *Server) engineInventory() []EngineInfo {
	if s.rt == nil {
		return nil
	}
	var out []EngineInfo
	// Built-in engine is always present.
	out = append(out, EngineInfo{
		ID:       string(agentengine.EngineWuu),
		Version:  "1",
		Enabled:  true,
		BinaryOK: true,
	})
	if s.rt.Engines() == nil {
		return out
	}
	for _, desc := range s.rt.Engines().Descriptors() {
		info := EngineInfo{
			ID:           string(desc.ID),
			Version:      desc.Version,
			Capabilities: append([]string(nil), desc.Capabilities...),
			Enabled:      true,
		}
		switch desc.ID {
		case agentengine.EngineID("codex"):
			path, err := s.codexBinaryPath()
			info.BinaryPath = path
			info.BinaryOK, info.Error = binaryStatus(path, err)
		case agentengine.EngineID("claude"):
			path, err := s.claudeBinaryPath()
			info.BinaryPath = path
			info.BinaryOK, info.Error = binaryStatus(path, err)
		}
		out = append(out, info)
	}
	return out
}

// claudeBinaryPath resolves the claude binary: settings override, then env
// override, then PATH lookup.
func (s *Server) claudeBinaryPath() (string, error) {
	path := ""
	if cfg := s.engineSettingsFromConfig(); cfg != nil && cfg.Claude != nil {
		path = strings.TrimSpace(cfg.Claude.BinaryPath)
	}
	if path == "" {
		path = strings.TrimSpace(os.Getenv("WUU_CLAUDE_BINARY"))
	}
	if path != "" {
		return path, nil
	}
	return exec.LookPath("claude")
}

// engineSettingsFromConfig returns the persisted engine settings section.
func (s *Server) engineSettingsFromConfig() *config.EnginesConfig {
	if s.rt == nil {
		return nil
	}
	cfg, _, err := s.rt.LoadEffectiveConfig()
	if err != nil || cfg.Engines == nil {
		return nil
	}
	return cfg.Engines
}

// applyEngineSettingsToRuntime pushes persisted settings into the live
// runtime: rebuilds the codex engine around the configured binary path and
// updates the default engine for new threads.
func (s *Server) applyEngineSettingsToRuntime() {
	if s.rt == nil {
		return
	}
	cfg, _, err := s.rt.LoadEffectiveConfig()
	if err != nil {
		return
	}
	codexEnabled := true
	codexBinary, _ := codexengine.ResolveBinary()
	claudeEnabled := true
	claudeBinary, _ := claudeengine.ResolveBinary()
	defaultEngine := agentengine.EngineWuu
	if engineCfg := cfg.Engines; engineCfg != nil {
		if trimmed := strings.TrimSpace(engineCfg.DefaultEngine); trimmed != "" {
			defaultEngine = agentengine.NormalizeEngineID(trimmed)
		}
		if codexCfg := engineCfg.Codex; codexCfg != nil {
			enabled, explicit := codexCfg.EngineEnabled()
			if explicit {
				codexEnabled = enabled
			}
			if path := strings.TrimSpace(codexCfg.BinaryPath); path != "" {
				codexBinary = path
			}
		}
		if claudeCfg := engineCfg.Claude; claudeCfg != nil {
			enabled, explicit := claudeCfg.EngineEnabled()
			if explicit {
				claudeEnabled = enabled
			}
			if path := strings.TrimSpace(claudeCfg.BinaryPath); path != "" {
				claudeBinary = path
			}
		}
	}
	s.rt.DefaultEngine = defaultEngine
	s.rt.RebuildCodexEngine(codexEnabled, codexBinary)
	s.rt.RebuildClaudeEngine(claudeEnabled, claudeBinary)
}

// codexBinaryPath resolves the codex binary: settings override, then env
// override, then PATH lookup.
func (s *Server) codexBinaryPath() (string, error) {
	path := ""
	if cfg := s.engineSettingsFromConfig(); cfg != nil && cfg.Codex != nil {
		path = strings.TrimSpace(cfg.Codex.BinaryPath)
	}
	if path == "" {
		path = strings.TrimSpace(os.Getenv("WUU_CODEX_BINARY"))
	}
	if path != "" {
		return path, nil
	}
	return exec.LookPath("codex")
}

// binaryStatus reports whether a resolved binary path exists and is
// executable, or whether the resolution failed.
func binaryStatus(path string, resolveErr error) (bool, string) {
	if resolveErr != nil {
		return false, resolveErr.Error()
	}
	if strings.TrimSpace(path) == "" {
		return false, "no binary path resolved"
	}
	if info, err := os.Stat(path); err != nil || info.IsDir() {
		return false, "binary not found at " + path
	}
	if !filepath.IsAbs(path) {
		return false, "binary path is not absolute: " + path
	}
	return true, ""
}

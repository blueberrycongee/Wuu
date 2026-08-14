package appserver

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/blueberrycongee/wuu/internal/config"
	"github.com/blueberrycongee/wuu/internal/modelcatalog"
	"github.com/blueberrycongee/wuu/internal/modelroles"
	"github.com/blueberrycongee/wuu/internal/modelvariant"
	"github.com/blueberrycongee/wuu/internal/providerfactory"
	"github.com/blueberrycongee/wuu/internal/providers"
)

const configWatchInterval = 500 * time.Millisecond

func (s *Server) startConfigWatch() {
	if s == nil || s.rt == nil || s.rt.WuuHome == "" || s.startupErr != nil || s.out == nil {
		return
	}
	s.startBackground(func() {
		ticker := time.NewTicker(configWatchInterval)
		defer ticker.Stop()
		for !s.closed.Load() {
			<-ticker.C
			if err := s.refreshConfigIfChanged(); err != nil {
				providers.DebugLogf("refresh observed config change: %v", err)
			}
		}
	})
}

func (s *Server) refreshConfigIfChanged() error {
	if s == nil || s.rt == nil || s.closed.Load() {
		return nil
	}
	next, err := s.effectiveConfigFingerprint()
	if err != nil {
		return err
	}
	s.configRefreshMu.Lock()
	defer s.configRefreshMu.Unlock()
	if next == s.configFingerprint {
		return nil
	}
	// First observation only establishes the baseline. Later changes hot-apply
	// the effective config and then publish a config/changed notification.
	if s.configFingerprint == "" {
		s.configFingerprint = next
		return nil
	}
	cfg, _, err := s.rt.LoadEffectiveConfig()
	if err != nil {
		return err
	}
	if err := s.applyExternalConfigChange(cfg); err != nil {
		// Keep the previous fingerprint so the watcher retries after the user
		// fixes the file instead of permanently swallowing the broken edit.
		return err
	}
	s.configFingerprint = next
	if s.out == nil {
		return nil
	}
	return s.writeNotification(NotificationConfigChanged, s.currentConfigChangedNotification())
}

func (s *Server) applyExternalConfigChange(cfg config.Config) error {
	if s == nil || s.rt == nil {
		return errors.New("runtime session is required")
	}
	providerCfg, resolvedName, err := cfg.ResolveProvider("")
	if err != nil {
		return err
	}
	model := strings.TrimSpace(providerCfg.Model)
	if model == "" {
		return errors.New("default provider has no model")
	}
	variant := strings.TrimSpace(cfg.Agent.Variant)
	effort := strings.TrimSpace(cfg.Agent.Effort)
	ruleProviderName, ruleProviderCfg := modelcatalog.EnrichProvider(resolvedName, providerCfg, model)

	previousProvider := s.rt.ProviderName
	previousRuleProviderCfg := s.rt.ModelRoles.Main.RuleProviderConfig
	if previousRuleProviderCfg.Type == "" {
		previousRuleProviderCfg = s.rt.ModelRoles.Main.ProviderConfig
	}
	providerClientChanged := providerClientConfigChanged(previousRuleProviderCfg, ruleProviderCfg)
	connectionChanged := resolvedName != previousProvider || providerClientChanged

	selection := modelvariant.ResolveForProvider(ruleProviderName, ruleProviderCfg, model, variant, effort)
	roleSelections, err := modelroles.Resolve(cfg, modelroles.ResolveOptions{
		ProviderName:   resolvedName,
		ProviderConfig: providerCfg,
		Model:          model,
		Effort:         selection.LegacyEffort,
		Variant:        selection.Variant,
	})
	if err != nil {
		return err
	}

	var client providers.StreamClient
	if resolvedName != previousProvider || connectionChanged || providerClientChanged {
		client, err = providerfactory.BuildStreamClient(ruleProviderCfg, resolvedName)
		if err != nil {
			return err
		}
	}

	return s.applyModelSelectionToRuntime(cfg, resolvedName, model, ruleProviderName, ruleProviderCfg, selection, roleSelections, connectionChanged, providerClientChanged, previousProvider, client, true)
}

func (s *Server) effectiveConfigFingerprint() (string, error) {
	if s == nil || s.rt == nil {
		return "", errors.New("runtime session is required")
	}
	cfg, _, err := s.rt.LoadEffectiveConfig()
	if err != nil {
		return "", err
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func (s *Server) currentConfigChangedNotification() ConfigChangedNotification {
	return ConfigChangedNotification{
		Provider:     s.rt.ProviderName,
		Model:        s.rt.Model,
		Effort:       s.currentDisplayEffort(),
		Variant:      s.currentVariant(),
		ModelRoles:   s.currentModelRoleSummaries(),
		ModelAliases: s.currentModelAliasSummaries(),
		Providers:    s.providerSummaries(),
	}
}

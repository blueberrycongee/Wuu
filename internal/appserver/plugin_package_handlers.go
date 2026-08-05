package appserver

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/blueberrycongee/wuu/internal/config"
	"github.com/blueberrycongee/wuu/internal/extensions"
	pluginpkg "github.com/blueberrycongee/wuu/internal/plugin"
	"github.com/blueberrycongee/wuu/internal/statepath"
)

func (s *Server) handlePluginPackageInspect(req Request) error {
	var params PluginPackageInspectParams
	if err := decodeParams(req.Params, &params); err != nil {
		return s.writeResponse(req.ID, nil, err)
	}
	inspected, err := pluginpkg.InspectPackage(params.Path)
	if err != nil {
		return s.writeResponse(req.ID, nil, fmt.Errorf("inspect plugin package: %w", err))
	}
	return s.writeResponse(req.ID, PluginPackageInspectResult{
		Package: pluginPackageMetadata(inspected),
	}, nil)
}

func (s *Server) handlePluginPackageInstall(req Request) error {
	var params PluginPackageInstallParams
	if err := decodeParams(req.Params, &params); err != nil {
		return s.writeResponse(req.ID, nil, err)
	}
	if err := s.requireIdlePluginPackageRuntime("install"); err != nil {
		return s.writeResponse(req.ID, nil, err)
	}

	inspected, err := pluginpkg.InspectPackage(params.Path)
	if err != nil {
		return s.writeResponse(req.ID, nil, fmt.Errorf("install plugin package: %w", err))
	}
	installedPath := filepath.Join(s.rt.WuuHome, "plugins", inspected.ID)
	if _, statErr := os.Lstat(installedPath); statErr == nil {
		pending, err := pluginpkg.StagePackageUpdate(s.rt.WuuHome, params.Path)
		if err != nil {
			return s.writeResponse(req.ID, nil, fmt.Errorf("stage plugin package update: %w", err))
		}
		return s.writeResponse(req.ID, PluginPackageInstallResult{
			Package:            pluginPackageMetadata(pending.Package),
			Pending:            true,
			ActiveFingerprint:  pending.ActiveFingerprint,
			ExtensionInventory: s.currentExtensionInventory(),
			Skills:             skillSummaries(s.rt.Skills),
		}, nil)
	} else if !os.IsNotExist(statErr) {
		return s.writeResponse(req.ID, nil, fmt.Errorf("inspect installed plugin package %q: %w", inspected.ID, statErr))
	}

	installed, err := pluginpkg.InstallPackage(s.rt.WuuHome, params.Path)
	if err != nil {
		return s.writeResponse(req.ID, nil, fmt.Errorf("install plugin package: %w", err))
	}
	inventory, skills, err := s.refreshPluginPackages()
	if err != nil {
		return s.writeResponse(req.ID, nil, fmt.Errorf("plugin %q was installed, but extension refresh failed: %w", installed.Package.ID, err))
	}
	return s.writeResponse(req.ID, PluginPackageInstallResult{
		Package:            pluginPackageMetadata(installed.Package),
		Replaced:           installed.Replaced,
		Pending:            false,
		ExtensionInventory: inventory,
		Skills:             skills,
	}, nil)
}

func (s *Server) handlePluginPackageRemove(req Request) error {
	var params PluginPackageRemoveParams
	if err := decodeParams(req.Params, &params); err != nil {
		return s.writeResponse(req.ID, nil, err)
	}
	if err := s.requireIdlePluginPackageRuntime("remove"); err != nil {
		return s.writeResponse(req.ID, nil, err)
	}

	pending, pendingErr := pluginpkg.ReadPendingUpdate(s.rt.WuuHome, params.ID)
	if pendingErr != nil && !errors.Is(pendingErr, pluginpkg.ErrPendingUpdateNotFound) {
		return s.writeResponse(req.ID, nil, fmt.Errorf("remove plugin package: inspect pending update: %w", pendingErr))
	}
	removed, err := pluginpkg.UninstallPackage(s.rt.WuuHome, params.ID)
	if err != nil {
		return s.writeResponse(req.ID, nil, fmt.Errorf("remove plugin package: %w", err))
	}
	if removed.Removed {
		if pendingErr == nil {
			if err := pluginpkg.RemovePendingUpdate(s.rt.WuuHome, params.ID, pending.Package.Fingerprint); err != nil {
				return s.writeResponse(req.ID, nil, fmt.Errorf("plugin %q was removed, but its pending update could not be cleared: %w", removed.ID, err))
			}
		}
		configPath, err := statepath.ConfigPath(s.rt.HomeDir)
		if err != nil {
			return s.writeResponse(req.ID, nil, fmt.Errorf("plugin %q was removed, but its policy path could not be resolved: %w", removed.ID, err))
		}
		if _, err := config.UpdateExtensionSettings(configPath, func(settings *extensions.Settings) error {
			settings.Revoke(extensions.SubjectID("user", removed.ID))
			return nil
		}); err != nil {
			return s.writeResponse(req.ID, nil, fmt.Errorf("plugin %q was removed, but its policy could not be cleared: %w", removed.ID, err))
		}
	}
	inventory, skills, err := s.refreshPluginPackages()
	if err != nil {
		if removed.Removed {
			return s.writeResponse(req.ID, nil, fmt.Errorf("plugin %q was removed, but extension refresh failed: %w", removed.ID, err))
		}
		return s.writeResponse(req.ID, nil, fmt.Errorf("refresh extensions after checking plugin %q: %w", removed.ID, err))
	}
	return s.writeResponse(req.ID, PluginPackageRemoveResult{
		ID:                 removed.ID,
		Removed:            removed.Removed,
		ExtensionInventory: inventory,
		Skills:             skills,
	}, nil)
}

func (s *Server) handlePendingPluginUpdate(req Request, params ExtensionPackageUpdateParams, selected pluginpkg.Plugin) error {
	if selected.Source != "user" {
		return s.writeResponse(req.ID, nil, errors.New("pending updates can only be managed for installed user plugins"))
	}
	pending, err := pluginpkg.ReadPendingUpdate(s.rt.WuuHome, selected.ID)
	if err != nil {
		return s.writeResponse(req.ID, nil, fmt.Errorf("read pending plugin update: %w", err))
	}
	if strings.TrimSpace(params.Fingerprint) == "" || params.Fingerprint != pending.Package.Fingerprint {
		return s.writeResponse(req.ID, nil, errors.New("pending plugin update changed; refresh inventory before updating it"))
	}
	if params.Action == ExtensionPackageRejectUpdate {
		if err := pluginpkg.RejectPendingUpdate(s.rt.WuuHome, selected.ID, params.Fingerprint); err != nil {
			return s.writeResponse(req.ID, nil, fmt.Errorf("reject pending plugin update: %w", err))
		}
		return s.writeResponse(req.ID, ExtensionPackageUpdateResult{ExtensionInventory: s.currentExtensionInventory()}, nil)
	}
	if _, err := pluginpkg.PromotePendingUpdate(s.rt.WuuHome, selected.ID, params.Fingerprint); err != nil {
		return s.writeResponse(req.ID, nil, fmt.Errorf("promote pending plugin update: %w", err))
	}

	configPath, err := statepath.ConfigPath(s.rt.HomeDir)
	if err != nil {
		return s.writeResponse(req.ID, nil, fmt.Errorf("plugin update was promoted, but its policy path could not be resolved: %w", err))
	}
	settings, err := config.UpdateExtensionSettings(configPath, func(settings *extensions.Settings) error {
		scope := extensions.GrantScopeUser
		if prior, ok := settings.Grants[selected.SubjectID]; ok && prior.Scope != "" {
			scope = prior.Scope
		}
		return settings.RecordGrant(extensions.Grant{
			SubjectID:   selected.SubjectID,
			Fingerprint: pending.Package.Fingerprint,
			Scope:       scope,
			Permissions: append([]string(nil), pending.Package.EffectivePermissions...),
			ApprovedAt:  time.Now().UTC(),
		})
	})
	if err != nil {
		_ = s.rt.RefreshExtensions(s.currentExtensionConfig())
		s.resetThreadRuntimesForGeneralSettings("")
		return s.writeResponse(req.ID, nil, fmt.Errorf("plugin update was promoted, but its approval could not be persisted: %w", err))
	}
	cfg := s.currentExtensionConfig()
	cfg.Extensions = &settings
	if err := s.rt.RefreshExtensions(cfg); err != nil {
		return s.writeResponse(req.ID, nil, fmt.Errorf("plugin update was approved, but extension refresh failed: %w", err))
	}
	s.resetThreadRuntimesForGeneralSettings("")
	return s.writeResponse(req.ID, ExtensionPackageUpdateResult{ExtensionInventory: s.currentExtensionInventory()}, nil)
}

func (s *Server) requireIdlePluginPackageRuntime(action string) error {
	if s == nil || s.rt == nil {
		return errors.New("runtime is not initialized")
	}
	if strings.TrimSpace(s.rt.WuuHome) == "" {
		return errors.New("runtime Wuu home is not configured")
	}
	if s.hasRunningThread() {
		return fmt.Errorf("cannot %s a plugin package while a turn is running", action)
	}
	return nil
}

func (s *Server) refreshPluginPackages() ([]ExtensionInventoryRecord, []SkillSummary, error) {
	if err := s.rt.RefreshExtensions(s.currentExtensionConfig()); err != nil {
		return nil, nil, err
	}
	s.resetThreadRuntimesForGeneralSettings("")
	return s.currentExtensionInventory(), skillSummaries(s.rt.Skills), nil
}

func pluginPackageMetadata(item pluginpkg.PackageInspection) PluginPackageMetadata {
	return PluginPackageMetadata{
		ID:                   item.ID,
		Name:                 item.Name,
		Version:              item.Version,
		Description:          item.Description,
		SourceKind:           PluginPackageSourceKind(item.SourceKind),
		ArchiveRoot:          item.ArchiveRoot,
		ManifestPath:         item.ManifestPath,
		FileCount:            item.FileCount,
		UnpackedSize:         item.UnpackedSize,
		Fingerprint:          item.Fingerprint,
		RequestedPermissions: append([]string(nil), item.RequestedPermissions...),
		EffectivePermissions: append([]string(nil), item.EffectivePermissions...),
		UnsupportedFields:    append([]string(nil), item.UnsupportedFields...),
	}
}

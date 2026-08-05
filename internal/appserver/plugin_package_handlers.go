package appserver

import (
	"errors"
	"fmt"
	"strings"

	pluginpkg "github.com/blueberrycongee/wuu/internal/plugin"
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

	removed, err := pluginpkg.UninstallPackage(s.rt.WuuHome, params.ID)
	if err != nil {
		return s.writeResponse(req.ID, nil, fmt.Errorf("remove plugin package: %w", err))
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

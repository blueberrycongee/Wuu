package runtime

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/blueberrycongee/wuu/internal/agent"
	"github.com/blueberrycongee/wuu/internal/config"
	"github.com/blueberrycongee/wuu/internal/hooks"
	"github.com/blueberrycongee/wuu/internal/mcp"
	pluginpkg "github.com/blueberrycongee/wuu/internal/plugin"
	"github.com/blueberrycongee/wuu/internal/pluginhost"
	"github.com/blueberrycongee/wuu/internal/providers"
	"github.com/blueberrycongee/wuu/internal/skills"
	"github.com/blueberrycongee/wuu/internal/tools"
)

// PluginGeneration is a complete, inactive replacement for the plugin-owned
// surfaces of a Session. It owns every process and private package snapshot.
type PluginGeneration struct {
	settings          config.Config
	plugins           []pluginpkg.Plugin
	active            []pluginpkg.Plugin
	host              *pluginhost.Host
	hooks             *hooks.Dispatcher
	skills            []skills.Skill
	mcp               *mcp.Manager
	mcpBinding        map[string]tools.MCPActivityBinding
	requestTransforms *agent.RequestTransformChain
	systemPrompts     *agent.SystemPromptAssembler
	compactions       *agent.CompactionRegistry
	ownedRoots        []string
}

// PreflightExtensions discovers and builds a replacement without changing the
// live Session.
func (s *Session) PreflightExtensions(cfg config.Config) (*PluginGeneration, error) {
	if s == nil {
		return nil, errors.New("runtime is not initialized")
	}
	return s.buildPluginGeneration(cfg, discoverPlugins(s.RootDir, s.WuuHome), nil, nil, startPluginClient)
}

// PreflightExtensionPolicy builds a replacement from the current package set
// without persisting the proposed grant/enable decisions.
func (s *Session) PreflightExtensionPolicy(cfg config.Config) (*PluginGeneration, error) {
	if s == nil {
		return nil, errors.New("runtime is not initialized")
	}
	return s.buildPluginGeneration(cfg, s.Plugins, nil, nil, startPluginClient)
}

// PreflightPluginRemoval builds the generation that will remain after one
// installed user package is removed, while the current package is still
// available for rollback.
func (s *Session) PreflightPluginRemoval(cfg config.Config, id string) (*PluginGeneration, error) {
	if s == nil {
		return nil, errors.New("runtime is not initialized")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, errors.New("plugin id is required")
	}
	discovered := make([]pluginpkg.Plugin, 0, len(s.Plugins))
	found := false
	for _, item := range s.Plugins {
		if item.Source == "user" && item.ID == id {
			found = true
			continue
		}
		discovered = append(discovered, item)
	}
	if !found {
		return nil, fmt.Errorf("installed user plugin %q was not found", id)
	}
	return s.buildPluginGeneration(cfg, discovered, nil, nil, startPluginClient)
}

// PreflightPluginUpdate builds an exact approved pending package as part of a
// complete replacement generation. The private snapshot keeps registrations
// valid after the pending directory is published and removed.
func (s *Session) PreflightPluginUpdate(cfg config.Config, id, fingerprint, packageRoot, manifestPath string) (*PluginGeneration, error) {
	if s == nil {
		return nil, errors.New("runtime is not initialized")
	}
	id = strings.TrimSpace(id)
	fingerprint = strings.TrimSpace(fingerprint)
	if id == "" || fingerprint == "" {
		return nil, errors.New("plugin id and fingerprint are required")
	}
	snapshot, err := snapshotPluginPackage(packageRoot)
	if err != nil {
		return nil, fmt.Errorf("snapshot plugin %q candidate: %w", id, err)
	}
	cleanupSnapshot := true
	defer func() {
		if cleanupSnapshot {
			_ = os.RemoveAll(snapshot)
		}
	}()

	manifest, err := packageManifestPath(snapshot, manifestPath)
	if err != nil {
		return nil, fmt.Errorf("resolve plugin %q candidate manifest: %w", id, err)
	}
	candidate, err := pluginpkg.LoadManifestWithOptions(manifest, pluginpkg.LoadOptions{Source: "user"})
	if err != nil {
		return nil, fmt.Errorf("load plugin %q candidate: %w", id, err)
	}
	if candidate.ID != id || candidate.Fingerprint != fingerprint {
		return nil, fmt.Errorf("plugin %q candidate changed during activation", id)
	}

	discovered := append([]pluginpkg.Plugin(nil), s.Plugins...)
	found := false
	for index := range discovered {
		if discovered[index].ID != id {
			continue
		}
		if discovered[index].Source != "user" || discovered[index].SubjectID != candidate.SubjectID {
			return nil, fmt.Errorf("plugin %q candidate does not replace the installed user package", id)
		}
		discovered[index] = candidate
		found = true
		break
	}
	if !found {
		return nil, fmt.Errorf("installed user plugin %q was not found", id)
	}
	sort.Slice(discovered, func(i, j int) bool { return discovered[i].ID < discovered[j].ID })

	required := map[string]bool{id: true}
	if s.PluginHost != nil {
		for _, status := range s.PluginHost.Statuses() {
			if status.State == pluginhost.StateActive {
				required[status.ID] = true
			}
		}
	}
	generation, err := s.buildPluginGeneration(cfg, discovered, required, []string{snapshot}, startPluginClient)
	if err != nil {
		return nil, err
	}
	if !containsPluginGeneration(generation.active, id, fingerprint) {
		_ = generation.close()
		return nil, fmt.Errorf("plugin %q candidate is not approved for its exact fingerprint", id)
	}
	cleanupSnapshot = false
	return generation, nil
}

func (s *Session) buildPluginGeneration(cfg config.Config, discovered []pluginpkg.Plugin, required map[string]bool, ownedRoots []string, start pluginClientStarter) (*PluginGeneration, error) {
	var active []pluginpkg.Plugin
	if !s.SafeMode {
		activationPlan, err := ResolvePluginActivationPlan(cfg, discovered)
		if err != nil {
			return nil, err
		}
		active = activationPlan.Plugins
	}
	host, err := buildPluginHost(active, s.RootDir, s.WuuHome, s.StateDir, required, start, s.PluginSessionRouter)
	if err != nil {
		for _, root := range ownedRoots {
			_ = os.RemoveAll(root)
		}
		return nil, err
	}
	systemPrompts, compactions, err := buildPluginAgentCapabilities(context.Background(), host, s.ProviderName, s.Model, s.RootDir)
	if err != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		closeErr := host.Close(ctx)
		cancel()
		for _, root := range ownedRoots {
			_ = os.RemoveAll(root)
		}
		return nil, errors.Join(err, closeErr)
	}
	generation := &PluginGeneration{
		settings:          cfg,
		plugins:           append([]pluginpkg.Plugin(nil), discovered...),
		active:            append([]pluginpkg.Plugin(nil), active...),
		host:              host,
		hooks:             buildHookDispatcher(cfg, active, s.TitleClient, s.Model, nil),
		skills:            discoverSkills(s.RootDir, s.HomeDir, s.WuuHome, active),
		mcpBinding:        mcpActivityBindingsFromPlugins(active),
		requestTransforms: buildPluginRequestTransforms(host, s.ProviderName, "", s.RootDir),
		systemPrompts:     systemPrompts,
		compactions:       compactions,
		ownedRoots:        append([]string(nil), ownedRoots...),
	}
	generation.mcp, err = startMCPManager(cfg, active, required)
	if err != nil {
		_ = generation.close()
		return nil, err
	}
	return generation, nil
}

// ActivatePluginGeneration swaps a prebuilt candidate into the Session. commit
// runs while the old generation is retained. Its failure restores the old
// bindings and closes the candidate; success closes the old generation.
func (s *Session) ActivatePluginGeneration(candidate *PluginGeneration, commit func() error) error {
	if s == nil {
		return errors.New("runtime is not initialized")
	}
	if candidate == nil || candidate.host == nil || candidate.hooks == nil {
		return errors.New("plugin candidate generation is not initialized")
	}
	s.pluginGenerationMu.Lock()
	defer s.pluginGenerationMu.Unlock()
	old := s.pluginGeneration
	if old == nil {
		old = s.capturePluginGeneration()
	}
	s.applyPluginGeneration(candidate)
	if commit != nil {
		if err := commit(); err != nil {
			s.applyPluginGeneration(old)
			return errors.Join(err, candidate.close())
		}
	}
	s.pluginGeneration = candidate
	if err := candidate.host.Activate(context.Background()); err != nil {
		providers.DebugLogf("activate committed plugin generation: %v", err)
	}
	if err := old.close(); err != nil {
		providers.DebugLogf("plugin generation cleanup: %v", err)
	}
	return nil
}

func (s *Session) capturePluginGeneration() *PluginGeneration {
	var manager *mcp.Manager
	if s.Toolkit != nil {
		manager = s.Toolkit.MCPManager()
	}
	hookSnapshot := hooks.NewDispatcher(nil)
	hookSnapshot.Replace(s.HookDispatcher)
	generation := &PluginGeneration{
		settings:          config.Config{Extensions: s.ExtensionSettings},
		plugins:           append([]pluginpkg.Plugin(nil), s.Plugins...),
		active:            append([]pluginpkg.Plugin(nil), s.ActivePlugins...),
		host:              s.PluginHost,
		hooks:             hookSnapshot,
		skills:            append([]skills.Skill(nil), s.Skills...),
		mcp:               manager,
		mcpBinding:        mcpActivityBindingsFromPlugins(s.ActivePlugins),
		requestTransforms: buildPluginRequestTransforms(s.PluginHost, s.ProviderName, "", s.RootDir),
		systemPrompts:     s.systemPrompts,
	}
	if s.StreamRunner != nil {
		generation.compactions = s.StreamRunner.CompactionRegistry
	}
	return generation
}

func (s *Session) applyPluginGeneration(generation *PluginGeneration) {
	if generation == nil {
		return
	}
	s.Plugins = append([]pluginpkg.Plugin(nil), generation.plugins...)
	s.ActivePlugins = append([]pluginpkg.Plugin(nil), generation.active...)
	s.ExtensionSettings = generation.settings.Extensions
	s.PluginHost = generation.host
	s.systemPrompts = generation.systemPrompts
	if s.StreamRunner != nil {
		inner := s.StreamRunner.Tools
		if previous, ok := inner.(*pluginToolExecutor); ok {
			inner = previous.inner
		}
		s.StreamRunner.Tools = newPluginToolExecutor(inner, generation.host, "", s.RootDir)
		s.StreamRunner.BeforeRequest = pluginRequestInterceptorWithTransforms(generation.host, generation.requestTransforms, s.ProviderName, "", s.RootDir)
		s.StreamRunner.CompactionRegistry = generation.compactions
	}
	if s.HookDispatcher == nil {
		s.HookDispatcher = generation.hooks
	} else if s.HookDispatcher != generation.hooks {
		s.HookDispatcher.Replace(generation.hooks)
	}
	s.Skills = append([]skills.Skill(nil), generation.skills...)
	if s.Toolkit != nil {
		s.Toolkit.SetSkills(s.Skills)
		s.Toolkit.SetMCPActivityBindings(generation.mcpBinding)
		s.Toolkit.SetMCPManager(generation.mcp)
	}
	s.RefreshSystemPrompt(s.ProviderName, s.Model)
}

func (g *PluginGeneration) close() error {
	if g == nil {
		return nil
	}
	var err error
	if g.mcp != nil {
		err = errors.Join(err, g.mcp.Close())
		g.mcp = nil
	}
	if g.systemPrompts != nil {
		g.systemPrompts.Clear()
		g.systemPrompts = nil
	}
	if g.compactions != nil {
		g.compactions.Clear()
		g.compactions = nil
	}
	if g.host != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		err = errors.Join(err, g.host.Close(ctx))
		cancel()
		g.host = nil
	}
	g.requestTransforms = nil
	for index := len(g.ownedRoots) - 1; index >= 0; index-- {
		err = errors.Join(err, os.RemoveAll(g.ownedRoots[index]))
	}
	g.ownedRoots = nil
	return err
}

func containsPluginGeneration(plugins []pluginpkg.Plugin, id, fingerprint string) bool {
	for _, item := range plugins {
		if item.ID == id && item.Fingerprint == fingerprint {
			return true
		}
	}
	return false
}

func packageManifestPath(root, manifestPath string) (string, error) {
	manifestPath = filepath.Clean(strings.TrimSpace(manifestPath))
	if manifestPath == "." || filepath.IsAbs(manifestPath) || manifestPath == ".." || strings.HasPrefix(manifestPath, ".."+string(filepath.Separator)) {
		return "", errors.New("manifest path escapes the package")
	}
	return filepath.Join(root, manifestPath), nil
}

func snapshotPluginPackage(source string) (string, error) {
	source = filepath.Clean(strings.TrimSpace(source))
	info, err := os.Stat(source)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", errors.New("candidate package root is not a directory")
	}
	destination, err := os.MkdirTemp("", "wuu-plugin-generation-")
	if err != nil {
		return "", err
	}
	if err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(source, path)
		if err != nil || rel == "." {
			return err
		}
		target := filepath.Join(destination, rel)
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return os.MkdirAll(target, info.Mode().Perm())
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("candidate package contains unsupported entry %s", rel)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		out, openErr := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, info.Mode().Perm())
		if openErr == nil {
			_, openErr = io.Copy(out, in)
		}
		closeErr := in.Close()
		if out != nil {
			closeErr = errors.Join(closeErr, out.Close())
		}
		return errors.Join(openErr, closeErr)
	}); err != nil {
		_ = os.RemoveAll(destination)
		return "", err
	}
	return destination, nil
}

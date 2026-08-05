package plugin

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
)

// PluginScope binds a Plugin to its active Generation and typed Registries.
// It is created when a plugin is activated and destroyed when the plugin
// is deactivated, deleted, or upgraded. The scope owns the generation;
// disposing the scope disposes the generation, which atomically withdraws
// all registrations.
type PluginScope struct {
	Plugin     Plugin
	Generation *Generation
	Registries *PluginRegistries

	mu     sync.Mutex
	active bool
}

// NewPluginScope creates an active scope for a plugin. It generates a
// unique generation ID and creates empty registries. The scope is
// immediately active; call Dispose to withdraw all registrations.
func NewPluginScope(p Plugin) *PluginScope {
	genID := newGenerationID(p.ID)
	gen := NewGeneration(genID, p.ID, p.Version, p.Fingerprint)
	return &PluginScope{
		Plugin:     p,
		Generation: gen,
		Registries: NewPluginRegistries(),
		active:     true,
	}
}

// Active reports whether the scope is still active (not disposed).
func (s *PluginScope) Active() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.active && !s.Generation.Disposed()
}

// Dispose withdraws all registrations and marks the scope inactive.
// It is safe to call multiple times.
func (s *PluginScope) Dispose() error {
	s.mu.Lock()
	if !s.active {
		s.mu.Unlock()
		return nil
	}
	s.active = false
	s.mu.Unlock()
	return s.Generation.Dispose()
}

// RegisterInScope contributes a value to a typed registry within this scope.
// Returns a Disposer that can withdraw this single contribution.
//
// Safety-kernel enforcement: registrations targeting host-owned seams
// (host.plugin.*, host.safe_mode, etc.) are rejected. Plugins may only
// contribute to public seams defined in the SeamCatalog.
func RegisterInScope(scope *PluginScope, kind EffectKind, key string, value any, dependsOn map[string]DependencyRule, priority int) (Disposer, error) {
	// P0 safety-kernel guard: reject any registration that targets a
	// host-owned safety-kernel seam. The key is the seam or registration
	// name (e.g. "host.plugin.install" or "agent.tool.my-tool").
	if IsSafetyKernelSeam(key) {
		return nil, fmt.Errorf("plugin scope: %q is a host-owned safety-kernel seam and cannot be contributed to by plugins", key)
	}

	reg := scope.Registries.RegistryFor(kind)
	if reg == nil {
		return nil, fmt.Errorf("plugin scope: unknown effect kind %s", kind)
	}
	entry := RegistryEntry[any]{
		Value:      value,
		PluginID:   scope.Plugin.ID,
		Generation: scope.Generation,
		DependsOn:  dependsOn,
		Priority:   priority,
	}
	return reg.Register(key, entry)
}

// newGenerationID generates a unique generation identifier for a plugin.
func newGenerationID(pluginID string) string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%s-gen-%s", pluginID, hex.EncodeToString(b))
}

// ScopeManager tracks all active plugin scopes. It provides lookup by
// plugin ID and bulk disposal (e.g., on shutdown or safe mode).
type ScopeManager struct {
	mu     sync.RWMutex
	scopes map[string]*PluginScope // keyed by plugin ID
}

// NewScopeManager creates an empty scope manager.
func NewScopeManager() *ScopeManager {
	return &ScopeManager{scopes: make(map[string]*PluginScope)}
}

// Activate creates and tracks a new scope for the given plugin.
// If a scope already exists for this plugin, it is disposed first.
func (m *ScopeManager) Activate(p Plugin) *PluginScope {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Dispose old scope atomically before creating the new one.
	if old, ok := m.scopes[p.ID]; ok {
		_ = old.Dispose()
	}
	scope := NewPluginScope(p)
	m.scopes[p.ID] = scope
	return scope
}

// Deactivate disposes and removes the scope for the given plugin ID.
// Returns nil if no scope existed.
func (m *ScopeManager) Deactivate(pluginID string) error {
	m.mu.Lock()
	scope, ok := m.scopes[pluginID]
	if ok {
		delete(m.scopes, pluginID)
	}
	m.mu.Unlock()

	if !ok {
		return nil
	}
	return scope.Dispose()
}

// Get returns the active scope for a plugin, or nil.
func (m *ScopeManager) Get(pluginID string) *PluginScope {
	m.mu.RLock()
	defer m.mu.RUnlock()
	scope, ok := m.scopes[pluginID]
	if !ok || !scope.Active() {
		return nil
	}
	return scope
}

// List returns all active scopes.
func (m *ScopeManager) List() []*PluginScope {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*PluginScope, 0, len(m.scopes))
	for _, scope := range m.scopes {
		if scope.Active() {
			out = append(out, scope)
		}
	}
	return out
}

// DisposeAll disposes all active scopes. Used for shutdown and safe mode.
// Errors are collected and joined.
func (m *ScopeManager) DisposeAll() error {
	m.mu.Lock()
	scopes := make([]*PluginScope, 0, len(m.scopes))
	for id, scope := range m.scopes {
		scopes = append(scopes, scope)
		delete(m.scopes, id)
	}
	m.mu.Unlock()

	var errs []error
	for _, scope := range scopes {
		if err := scope.Dispose(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("scope manager dispose: %w", joinErrors(errs))
	}
	return nil
}

// ActiveCount returns the number of active scopes.
func (m *ScopeManager) ActiveCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	count := 0
	for _, scope := range m.scopes {
		if scope.Active() {
			count++
		}
	}
	return count
}

// joinErrors is a simple error join for Go versions before errors.Join.
func joinErrors(errs []error) error {
	if len(errs) == 0 {
		return nil
	}
	msgs := make([]string, len(errs))
	for i, err := range errs {
		msgs[i] = err.Error()
	}
	return fmt.Errorf("%s", joinStrings(msgs, "; "))
}

func joinStrings(ss []string, sep string) string {
	if len(ss) == 0 {
		return ""
	}
	result := ss[0]
	for _, s := range ss[1:] {
		result += sep + s
	}
	return result
}

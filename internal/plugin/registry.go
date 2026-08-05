package plugin

import (
	"errors"
	"fmt"
	"sync"
)

// DependencyRule governs how a registry entry relates to other entries.
type DependencyRule int

const (
	// DepRequired: the dependency must be present or registration fails.
	DepRequired DependencyRule = iota

	// DepOptional: the dependency is used if present but registration
	// succeeds without it. Optional dependencies enable graceful
	// composition without tight coupling.
	DepOptional

	// DepConflicts: the named entry must NOT be present or registration fails.
	// Use for mutually exclusive contributions (e.g. two compaction providers).
	DepConflicts
)

// RegistryEntry describes one contribution with its dependency constraints.
type RegistryEntry[V any] struct {
	// Value is the contribution.
	Value V

	// PluginID identifies the owning plugin.
	PluginID string

	// Generation is the owning generation.
	Generation *Generation

	// DependsOn maps dependency keys to dependency rules.
	DependsOn map[string]DependencyRule

	// Priority influences ordering when multiple entries exist.
	// Higher priority entries execute first in transform chains
	// and are preferred in decision seams.
	Priority int
}

// Registry is a typed, generation-scoped collection of named contributions.
// Each registration belongs to exactly one Generation and is atomically
// withdrawn when that generation is disposed.
//
// Registry is safe for concurrent use.
type Registry[V any] struct {
	mu      sync.RWMutex
	entries map[string]*RegistryEntry[V]
	order   []string // insertion order for stable iteration
}

// NewRegistry creates an empty registry.
func NewRegistry[V any]() *Registry[V] {
	return &Registry[V]{
		entries: make(map[string]*RegistryEntry[V]),
	}
}

// Register adds an entry to the registry. It validates dependency
// constraints, registers the entry, attaches a disposer to the
// owning generation, and returns a Disposer that can be called
// independently.
//
// Returns an error if:
//   - The key is already registered.
//   - A required dependency is missing.
//   - A conflict dependency is present.
//   - The owning generation has been disposed.
func (r *Registry[V]) Register(key string, entry RegistryEntry[V]) (Disposer, error) {
	if key == "" {
		return nil, errors.New("registry: key must not be empty")
	}
	if entry.Generation == nil {
		return nil, errors.New("registry: generation is required")
	}
	if entry.Generation.Disposed() {
		return nil, fmt.Errorf("registry: generation %s is disposed", entry.Generation.ID)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Validate dependency constraints.
	for depKey, rule := range entry.DependsOn {
		existing, exists := r.entries[depKey]
		switch rule {
		case DepRequired:
			if !exists {
				return nil, fmt.Errorf("registry: %s requires %s which is not registered", key, depKey)
			}
			if existing.Generation.Disposed() {
				return nil, fmt.Errorf("registry: %s requires %s whose generation is disposed", key, depKey)
			}
		case DepConflicts:
			if exists && !existing.Generation.Disposed() {
				return nil, fmt.Errorf("registry: %s conflicts with existing %s", key, depKey)
			}
		case DepOptional:
			// Always ok.
		}
	}

	if _, exists := r.entries[key]; exists {
		return nil, fmt.Errorf("registry: %s is already registered", key)
	}

	r.entries[key] = &entry
	r.order = append(r.order, key)

	// Attach cleanup to the owning generation.
	dispose := func() error {
		r.mu.Lock()
		defer r.mu.Unlock()
		delete(r.entries, key)
		// Remove from order slice.
		for i, k := range r.order {
			if k == key {
				r.order = append(r.order[:i], r.order[i+1:]...)
				break
			}
		}
		return nil
	}

	if err := entry.Generation.Register(dispose); err != nil {
		// Clean up the entry we just added (generation was disposed
		// between our check above and the Register call).
		delete(r.entries, key)
		for i, k := range r.order {
			if k == key {
				r.order = append(r.order[:i], r.order[i+1:]...)
				break
			}
		}
		return nil, fmt.Errorf("registry: failed to attach to generation: %w", err)
	}

	return dispose, nil
}

// Get returns the entry for key, if registered and its generation is alive.
func (r *Registry[V]) Get(key string) (*RegistryEntry[V], bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	entry, ok := r.entries[key]
	if !ok || entry.Generation.Disposed() {
		return nil, false
	}
	return entry, true
}

// List returns all alive entries in insertion order.
func (r *Registry[V]) List() []RegistryEntry[V] {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]RegistryEntry[V], 0, len(r.entries))
	for _, key := range r.order {
		entry, ok := r.entries[key]
		if !ok || entry.Generation.Disposed() {
			continue
		}
		out = append(out, *entry)
	}
	return out
}

// ListByPlugin returns all alive entries belonging to the given plugin.
func (r *Registry[V]) ListByPlugin(pluginID string) []RegistryEntry[V] {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []RegistryEntry[V]
	for _, entry := range r.entries {
		if entry.Generation.Disposed() {
			continue
		}
		if entry.PluginID == pluginID {
			out = append(out, *entry)
		}
	}
	return out
}

// Keys returns all alive entry keys in insertion order.
func (r *Registry[V]) Keys() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	keys := make([]string, 0, len(r.entries))
	for _, key := range r.order {
		if entry, ok := r.entries[key]; ok && !entry.Generation.Disposed() {
			keys = append(keys, key)
		}
	}
	return keys
}

// Count returns the number of alive entries.
func (r *Registry[V]) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	n := 0
	for _, entry := range r.entries {
		if !entry.Generation.Disposed() {
			n++
		}
	}
	return n
}

// PluginRegistries holds the typed registries for a plugin scope.
// Each registry corresponds to a specific EffectKind and Seam.
type PluginRegistries struct {
	Tools         *Registry[any]
	SystemPrompts *Registry[any]
	Contexts      *Registry[any]
	Providers     *Registry[any]
	Compactions   *Registry[any]
	Continuations *Registry[any]
	Subagents     *Registry[any]
	Permissions   *Registry[any]
	Commands      *Registry[any]

	// Desktop
	Views     *Registry[any]
	Themes    *Registry[any]
	Settings  *Registry[any]
	Storages  *Registry[any]
	Layouts   *Registry[any]
	Renderers *Registry[any]
	Shells    *Registry[any]
}

// NewPluginRegistries creates all typed registries for a plugin scope.
// Each registry starts empty; they are populated as plugin generations
// activate and cleared as they dispose.
func NewPluginRegistries() *PluginRegistries {
	return &PluginRegistries{
		Tools:         NewRegistry[any](),
		SystemPrompts: NewRegistry[any](),
		Contexts:      NewRegistry[any](),
		Providers:     NewRegistry[any](),
		Compactions:   NewRegistry[any](),
		Continuations: NewRegistry[any](),
		Subagents:     NewRegistry[any](),
		Permissions:   NewRegistry[any](),
		Commands:      NewRegistry[any](),
		Views:         NewRegistry[any](),
		Themes:        NewRegistry[any](),
		Settings:      NewRegistry[any](),
		Storages:      NewRegistry[any](),
		Layouts:       NewRegistry[any](),
		Renderers:     NewRegistry[any](),
		Shells:        NewRegistry[any](),
	}
}

// RegistryFor returns the typed registry for the given effect kind, or nil.
func (r *PluginRegistries) RegistryFor(kind EffectKind) *Registry[any] {
	switch kind {
	case EffectTool:
		return r.Tools
	case EffectSystemPrompt:
		return r.SystemPrompts
	case EffectContext:
		return r.Contexts
	case EffectProvider:
		return r.Providers
	case EffectCompaction:
		return r.Compactions
	case EffectContinuation:
		return r.Continuations
	case EffectSubagent:
		return r.Subagents
	case EffectPermission:
		return r.Permissions
	case EffectCommand:
		return r.Commands
	case EffectView:
		return r.Views
	case EffectTheme:
		return r.Themes
	case EffectSetting:
		return r.Settings
	case EffectStorage:
		return r.Storages
	case EffectLayout:
		return r.Layouts
	case EffectRenderer:
		return r.Renderers
	case EffectShell:
		return r.Shells
	default:
		return nil
	}
}

// ValidateDependencies checks that all required dependencies for entries
// in this scope are satisfied. It returns nil if the dependency graph
// is valid, or an error describing the first violation found.
func ValidateDependencies[V any](reg *Registry[V]) error {
	reg.mu.RLock()
	defer reg.mu.RUnlock()

	for key, entry := range reg.entries {
		if entry.Generation.Disposed() {
			continue
		}
		for depKey, rule := range entry.DependsOn {
			existing, exists := reg.entries[depKey]
			switch rule {
			case DepRequired:
				if !exists || existing.Generation.Disposed() {
					return fmt.Errorf("%s requires %s which is missing or disposed", key, depKey)
				}
			case DepConflicts:
				if exists && !existing.Generation.Disposed() {
					return fmt.Errorf("%s conflicts with %s", key, depKey)
				}
			}
		}
	}
	return nil
}

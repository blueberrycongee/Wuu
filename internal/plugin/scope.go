// Package plugin implements the Wuu plugin platform: discovery, lifecycle,
// capability contract, typed seams, registries, and generation-scoped
// registration with atomic publish/withdraw semantics.
//
// ## Generation scope model
//
// Every plugin contribution lives in a Generation — a unique, atomic registration
// scope bound to one plugin version+activation. The generation guarantees:
//
//   - Atomic publish: all registrations activate together on successful activation.
//   - Atomic withdraw: all registrations dispose together on disable, delete, or update.
//   - Failure isolation: a partial activation failure rolls back the entire generation.
//   - No lingering state: after disposal, no registration from that generation remains.
//   - Disposers are async-aware and respect a settled signal before the host proceeds.
//
// ## Safety kernel
//
// The following capabilities are owned exclusively by the Wuu host process and are
// never exposed to plugins through registries or seams:
//
//   - Plugin inspection, approval, enable, disable, upgrade, and delete.
//   - Safe mode, crash recovery, and emergency restart.
//   - Permission and trust prompt final boundaries.
//   - Native window and app-server lifecycle.
//   - Plugin generation error isolation and disposal enforcement.
//   - User escape paths: settings access, plugin disable, and default UI restoration.
//
// Product capabilities outside this kernel should prefer public extension paths.
package plugin

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

// Generation is a unique, atomic registration scope. Every plugin contribution
// is registered within a single Generation. When the generation is disposed,
// all registrations are withdrawn in reverse order with best-effort semantics.
//
// Generations are created by the plugin host and handed to activation code.
// Once Dispose is called the generation is permanently gone; re-enabling a
// plugin creates a new Generation.
type Generation struct {
	// ID is a unique opaque identifier for this generation instance.
	ID string
	// PluginID is the plugin this generation belongs to.
	PluginID string
	// Version is the plugin version active in this generation.
	Version string
	// Fingerprint is the package fingerprint at activation time.
	Fingerprint string
	// CreatedAt records when the generation was created.
	CreatedAt time.Time

	mu        sync.Mutex
	disposed  bool
	disposers []func() error
	errs      []error
}

// NewGeneration creates a generation bound to a specific plugin activation.
func NewGeneration(id, pluginID, version, fingerprint string) *Generation {
	return &Generation{
		ID:          id,
		PluginID:    pluginID,
		Version:     version,
		Fingerprint: fingerprint,
		CreatedAt:   time.Now(),
	}
}

// Register attaches a disposer to this generation. The disposer is called
// during Dispose in reverse registration order (LIFO).
//
// Register returns an error if the generation has already been disposed.
// Callers must check this error and avoid using the registration result
// when the generation is gone — this prevents leaked registrations from
// outliving their owning generation.
func (g *Generation) Register(dispose func() error) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.disposed {
		return fmt.Errorf("plugin generation %s is already disposed", g.ID)
	}
	g.disposers = append(g.disposers, dispose)
	return nil
}

// Dispose withdraws all registrations in reverse order. Each disposer runs
// to completion even if earlier disposers fail. All errors are collected
// and returned as a joined error.
//
// Dispose is idempotent: calling it more than once is safe and returns the
// same error set.
func (g *Generation) Dispose() error {
	g.mu.Lock()
	if g.disposed {
		g.mu.Unlock()
		return errors.Join(g.errs...)
	}
	g.disposed = true
	disposers := make([]func() error, len(g.disposers))
	copy(disposers, g.disposers)
	g.mu.Unlock()

	// Run in reverse order (LIFO) so later registrations are cleaned up
	// before earlier ones — this matches the natural dependency graph.
	for i := len(disposers) - 1; i >= 0; i-- {
		if err := disposers[i](); err != nil {
			g.mu.Lock()
			g.errs = append(g.errs, err)
			g.mu.Unlock()
		}
	}
	return errors.Join(g.errs...)
}

// Disposed reports whether the generation has been disposed.
func (g *Generation) Disposed() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.disposed
}

// RegistrationCount returns the number of registrations in this generation.
func (g *Generation) RegistrationCount() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return len(g.disposers)
}

// Disposer is a cleanup function returned by a registry when a contribution
// is registered. The host calls it to atomically withdraw a single registration
// without tearing down the whole generation.
type Disposer func() error

// NoopDisposer is a disposer that always succeeds. Use it for registrations
// that have no cleanup requirements.
func NoopDisposer() error { return nil }

// EffectKind classifies what a registration contributes to the runtime.
// Each effect kind maps to a specific registry and dispatch seam.
type EffectKind string

const (
	// Agent runtime effects.
	EffectTool         EffectKind = "tool"
	EffectSystemPrompt EffectKind = "system_prompt"
	EffectContext      EffectKind = "context"
	EffectProvider     EffectKind = "provider"
	EffectCompaction   EffectKind = "compaction"
	EffectContinuation EffectKind = "continuation"
	EffectSubagent     EffectKind = "subagent"
	EffectPermission   EffectKind = "permission"
	EffectCommand      EffectKind = "command"

	// Desktop workbench effects.
	EffectView     EffectKind = "view"
	EffectTheme    EffectKind = "theme"
	EffectSetting  EffectKind = "setting"
	EffectStorage  EffectKind = "storage"
	EffectLayout   EffectKind = "layout"
	EffectRenderer EffectKind = "renderer"
	EffectShell    EffectKind = "shell"
)

// String returns the effect kind identifier.
func (e EffectKind) String() string { return string(e) }

// IsAgentRuntime reports whether this effect belongs to the agent runtime layer.
func (e EffectKind) IsAgentRuntime() bool {
	switch e {
	case EffectTool, EffectSystemPrompt, EffectContext, EffectProvider,
		EffectCompaction, EffectContinuation, EffectSubagent, EffectPermission,
		EffectCommand:
		return true
	default:
		return false
	}
}

// IsDesktopWorkbench reports whether this effect belongs to the desktop workbench layer.
func (e EffectKind) IsDesktopWorkbench() bool {
	switch e {
	case EffectView, EffectTheme, EffectSetting, EffectStorage,
		EffectLayout, EffectRenderer, EffectShell:
		return true
	default:
		return false
	}
}

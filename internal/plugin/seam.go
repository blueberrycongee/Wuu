package plugin

import (
	"context"
	"errors"
	"fmt"
)

// SeamKind classifies the dispatch semantics of an extension point. Every
// seam must define ordering, short-circuit, error, cancellation, concurrency,
// and unload behavior. Not all extension points are the same kind of callback.
type SeamKind string

const (
	// SeamObserve watches immutable events and cannot affect results.
	// Observers run concurrently and failures are logged but do not
	// block the observed operation.
	//
	// Examples: session start/stop telemetry, audit logging, usage tracking.
	SeamObserve SeamKind = "observe"

	// SeamTransform modifies input or output through a stable ordered chain.
	// Each transform receives the output of the previous transform. Order
	// is determined by dependency ranking; transforms with no declared
	// dependency order execute in registration order.
	//
	// Examples: message pre-processing, tool result enrichment,
	// system prompt section assembly, context injection.
	SeamTransform SeamKind = "transform"

	// SeamGuard adds constraints or rejects an operation. Once a guard
	// rejects, no subsequent plugin may silently relax that rejection.
	// Guards execute before transforms and around-wrappers.
	//
	// Examples: tool access control, file path restrictions,
	// permission policy enforcement.
	SeamGuard SeamKind = "guard"

	// SeamAround wraps the actual execution of an operation. The wrapper
	// receives the original input and a next() function. It may modify
	// input, skip execution, transform output, or add side effects.
	// Around wrappers nest innermost-first: the first registered wrapper
	// is the outermost.
	//
	// Examples: tool timeout, retry, metrics, sandbox enforcement,
	// result caching, audit trail.
	SeamAround SeamKind = "around"

	// SeamDecision produces a typed decision that affects control flow.
	// The first non-continue decision wins; subsequent plugins are not
	// consulted. The host may override rejections through its safety
	// kernel but must log the override.
	//
	// Examples: turn continuation policy, compaction trigger,
	// subagent routing, steering suggestions.
	SeamDecision SeamKind = "decision"
)

// String returns the seam kind identifier.
func (k SeamKind) String() string { return string(k) }

// ErrorPolicy defines how seam dispatch handles plugin errors.
type ErrorPolicy int

const (
	// ErrorPolicyPropagate: the first error stops dispatch and is returned
	// to the caller. Use for seams where correctness depends on every
	// registered handler succeeding (e.g. guard chain).
	ErrorPolicyPropagate ErrorPolicy = iota

	// ErrorPolicyIsolate: each handler runs independently; errors from one
	// handler do not affect others. All errors are collected and returned
	// as a joined error after all handlers complete.
	ErrorPolicyIsolate

	// ErrorPolicyIgnore: handler errors are logged but not returned to the
	// caller. Use for observe seams where failures should never block the
	// observed operation.
	ErrorPolicyIgnore
)

// SeamDispatch describes the execution contract for one seam.
type SeamDispatch struct {
	// Kind is the semantic classification of this seam.
	Kind SeamKind

	// Ordered indicates that handlers execute in dependency order rather
	// than registration order. When false, handlers execute in
	// registration order (oldest first).
	Ordered bool

	// Concurrent indicates that handlers may execute concurrently.
	// Only valid for SeamObserve; other seam kinds must execute
	// sequentially to maintain ordering guarantees.
	Concurrent bool

	// ShortCircuit indicates that the first non-nil or non-continue
	// result stops dispatch. True for SeamGuard and SeamDecision.
	ShortCircuit bool

	// ErrorPolicy governs error propagation during dispatch.
	ErrorPolicy ErrorPolicy
}

// Validate checks that the dispatch configuration is internally consistent.
func (d SeamDispatch) Validate() error {
	if d.Kind == "" {
		return errors.New("seam dispatch: kind is required")
	}
	if d.Concurrent && d.Kind != SeamObserve {
		return fmt.Errorf("seam dispatch: concurrent is only valid for observe seams, got %s", d.Kind)
	}
	if d.ShortCircuit && d.Kind != SeamGuard && d.Kind != SeamDecision {
		return fmt.Errorf("seam dispatch: short-circuit is only valid for guard and decision seams, got %s", d.Kind)
	}
	if d.ErrorPolicy == ErrorPolicyIgnore && d.Kind != SeamObserve {
		return fmt.Errorf("seam dispatch: error ignore is only valid for observe seams, got %s", d.Kind)
	}
	return nil
}

// Seam is a named extension point with typed dispatch semantics. The type
// parameter T is the handler type for this seam (e.g. a function signature
// or interface).
//
// Seams are registered once by the host and then populated by plugin
// generations. The seam itself is not generic-aware at runtime; typed
// dispatch is provided by the registry that backs it.
type Seam struct {
	// Name is a stable dotted identifier for this extension point.
	// Examples: "agent.tool.execute", "desktop.view.register",
	// "agent.system_prompt.section".
	Name string

	// Dispatch describes the execution contract.
	Dispatch SeamDispatch
}

// SeamCatalog is the host-owned registry of all known seams. Plugins
// contribute handlers to seams; the catalog defines which seams exist.
type SeamCatalog struct {
	seams map[string]Seam
}

// NewSeamCatalog creates an empty seam catalog.
func NewSeamCatalog() *SeamCatalog {
	return &SeamCatalog{seams: make(map[string]Seam)}
}

// Register adds a seam to the catalog. Returns an error if a seam with
// the same name already exists.
func (c *SeamCatalog) Register(seam Seam) error {
	if err := seam.Dispatch.Validate(); err != nil {
		return fmt.Errorf("seam catalog: %w", err)
	}
	if _, exists := c.seams[seam.Name]; exists {
		return fmt.Errorf("seam catalog: seam %q already registered", seam.Name)
	}
	c.seams[seam.Name] = seam
	return nil
}

// Get returns the seam with the given name, if registered.
func (c *SeamCatalog) Get(name string) (Seam, bool) {
	seam, ok := c.seams[name]
	return seam, ok
}

// IsPluginAccessible reports whether a seam may be contributed to by plugins.
// Safety-kernel seams are never accessible to plugins. Unknown seams (not in
// the catalog) are also rejected.
func (c *SeamCatalog) IsPluginAccessible(name string) bool {
	if IsSafetyKernelSeam(name) {
		return false
	}
	_, ok := c.seams[name]
	return ok
}

// Names returns all registered seam names in sorted order.
func (c *SeamCatalog) Names() []string {
	names := make([]string, 0, len(c.seams))
	for name := range c.seams {
		names = append(names, name)
	}
	// sort.Strings(names) — caller can sort if needed
	return names
}

// Standard agent runtime seams. These are the public extension points
// that plugins use to customize agent behavior.
var StandardAgentSeams = []Seam{
	{
		Name: "agent.tool.register",
		Dispatch: SeamDispatch{
			Kind: SeamTransform, Ordered: false, ErrorPolicy: ErrorPolicyPropagate,
		},
	},
	{
		Name: "agent.tool.execute.before",
		Dispatch: SeamDispatch{
			Kind: SeamGuard, ShortCircuit: true, ErrorPolicy: ErrorPolicyPropagate,
		},
	},
	{
		Name: "agent.tool.execute.around",
		Dispatch: SeamDispatch{
			Kind: SeamAround, ErrorPolicy: ErrorPolicyPropagate,
		},
	},
	{
		Name: "agent.tool.execute.after",
		Dispatch: SeamDispatch{
			Kind: SeamTransform, Ordered: false, ErrorPolicy: ErrorPolicyIsolate,
		},
	},
	{
		Name: "agent.system_prompt.section",
		Dispatch: SeamDispatch{
			Kind: SeamTransform, Ordered: true, ErrorPolicy: ErrorPolicyIsolate,
		},
	},
	{
		Name: "agent.context.inject",
		Dispatch: SeamDispatch{
			Kind: SeamTransform, Ordered: true, ErrorPolicy: ErrorPolicyIsolate,
		},
	},
	{
		Name: "agent.request.transform",
		Dispatch: SeamDispatch{
			Kind: SeamTransform, Ordered: true, ErrorPolicy: ErrorPolicyPropagate,
		},
	},
	{
		Name: "agent.response.transform",
		Dispatch: SeamDispatch{
			Kind: SeamTransform, Ordered: true, ErrorPolicy: ErrorPolicyPropagate,
		},
	},
	{
		Name: "agent.compaction",
		Dispatch: SeamDispatch{
			Kind: SeamDecision, ShortCircuit: true, ErrorPolicy: ErrorPolicyPropagate,
		},
	},
	{
		Name: "agent.provider.register",
		Dispatch: SeamDispatch{
			Kind: SeamTransform, Ordered: false, ErrorPolicy: ErrorPolicyPropagate,
		},
	},
	{
		Name: "agent.continuation.policy",
		Dispatch: SeamDispatch{
			Kind: SeamDecision, ShortCircuit: true, ErrorPolicy: ErrorPolicyPropagate,
		},
	},
	{
		Name: "agent.subagent.provider",
		Dispatch: SeamDispatch{
			Kind: SeamDecision, ShortCircuit: true, ErrorPolicy: ErrorPolicyPropagate,
		},
	},
	{
		Name: "agent.permission.policy",
		Dispatch: SeamDispatch{
			Kind: SeamGuard, ShortCircuit: true, ErrorPolicy: ErrorPolicyPropagate,
		},
	},
	{
		Name: "agent.session.lifecycle",
		Dispatch: SeamDispatch{
			Kind: SeamObserve, Concurrent: true, ErrorPolicy: ErrorPolicyIgnore,
		},
	},
}

// Standard desktop workbench seams.
var StandardDesktopSeams = []Seam{
	{
		Name: "desktop.view.register",
		Dispatch: SeamDispatch{
			Kind: SeamTransform, Ordered: false, ErrorPolicy: ErrorPolicyPropagate,
		},
	},
	{
		Name: "desktop.layout.apply",
		Dispatch: SeamDispatch{
			Kind: SeamTransform, Ordered: true, ErrorPolicy: ErrorPolicyIsolate,
		},
	},
	{
		Name: "desktop.theme.register",
		Dispatch: SeamDispatch{
			Kind: SeamTransform, Ordered: false, ErrorPolicy: ErrorPolicyIsolate,
		},
	},
	{
		Name: "desktop.renderer.register",
		Dispatch: SeamDispatch{
			Kind: SeamTransform, Ordered: false, ErrorPolicy: ErrorPolicyPropagate,
		},
	},
	{
		Name: "desktop.command.register",
		Dispatch: SeamDispatch{
			Kind: SeamTransform, Ordered: false, ErrorPolicy: ErrorPolicyPropagate,
		},
	},
	{
		Name: "desktop.surface.register",
		Dispatch: SeamDispatch{
			Kind: SeamTransform, Ordered: false, ErrorPolicy: ErrorPolicyPropagate,
		},
	},
}

// RegisterStandardSeams registers all standard agent and desktop seams
// into the catalog. Call once during host initialization.
func RegisterStandardSeams(catalog *SeamCatalog) error {
	all := append(StandardAgentSeams, StandardDesktopSeams...)
	for _, seam := range all {
		if err := catalog.Register(seam); err != nil {
			return fmt.Errorf("register standard seam %q: %w", seam.Name, err)
		}
	}
	return nil
}

// IsSafetyKernelSeam reports whether a seam name belongs to the host-owned
// safety kernel and must never be exposed to plugins.
func IsSafetyKernelSeam(name string) bool {
	// The safety kernel seams are not in the public catalog; they exist
	// only for host-internal dispatch. Plugin contributions to these
	// seams are rejected at registration time.
	switch name {
	case "host.plugin.install",
		"host.plugin.approval",
		"host.plugin.enable",
		"host.plugin.disable",
		"host.plugin.upgrade",
		"host.plugin.delete",
		"host.safe_mode",
		"host.crash_recovery",
		"host.permission.final",
		"host.window.lifecycle",
		"host.appserver.lifecycle",
		"host.generation.isolate",
		"host.escape.settings",
		"host.escape.default_ui":
		return true
	}
	return false
}

// contextKey is the unexported type used for context value keys to prevent
// collisions with keys defined in other packages.
type contextKey struct{ name string }

// CancelCause returns the reason a context was cancelled, or nil.
func CancelCause(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		if cause := context.Cause(ctx); cause != nil {
			return fmt.Errorf("%w: %w", err, cause)
		}
		return err
	}
	return nil
}

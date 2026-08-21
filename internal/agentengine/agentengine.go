// Package agentengine defines the boundary between the Wuu App Server and an
// Agent Engine: a system that owns its own agent loop, model access,
// authentication, tools, native session state, and resume semantics.
//
// The built-in "wuu" engine is the native StreamRunner loop. Claude and Codex
// are integrated as separate engines behind the same Factory/Session seam;
// Wuu only unifies the upper product semantics (thread, turn, run, events,
// status, stop, resume, desktop presentation) and never reaches into an
// external engine's internals.
package agentengine

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/blueberrycongee/wuu/internal/agent"
	"github.com/blueberrycongee/wuu/internal/providers"
)

// EngineID identifies the agent engine a thread is bound to. A thread binds
// its engine at creation and never silently switches.
type EngineID string

const (
	// EngineWuu is the built-in engine: the native StreamRunner loop that
	// has always driven Wuu conversations. Threads created before engine
	// binding existed default to it.
	EngineWuu EngineID = "wuu"
)

// ErrUnknownEngine reports a thread bound to an engine this build cannot host.
var ErrUnknownEngine = errors.New("unknown agent engine")

// ErrNoActiveTurn reports an Interrupt call with no in-flight turn.
var ErrNoActiveTurn = errors.New("no active turn on this engine session")

// NormalizeEngineID treats an empty id as the built-in wuu engine, which is
// the legacy default for threads persisted before engine binding.
func NormalizeEngineID(id string) EngineID {
	if strings.TrimSpace(id) == "" {
		return EngineWuu
	}
	return EngineID(strings.TrimSpace(id))
}

// IsKnownEngine reports whether this build can host the engine.
func IsKnownEngine(id EngineID) bool {
	return id == EngineWuu
}

// KnownEngineIDs returns the engines this build can host, in stable order.
func KnownEngineIDs() []EngineID {
	return []EngineID{EngineWuu}
}

// CheckEngine returns an explicit error when the engine is not known to this
// build, so callers surface a clear message instead of silently running the
// native loop for a thread bound elsewhere.
func CheckEngine(id EngineID) error {
	if !IsKnownEngine(id) {
		return fmt.Errorf("%w: %q", ErrUnknownEngine, id)
	}
	return nil
}

// Descriptor describes one engine for capability display and selection.
type Descriptor struct {
	ID           EngineID
	Version      string
	Capabilities []string
}

// OpenRequest carries what a Factory needs to start a new engine-bound thread.
// The fields are engine-agnostic; each engine interprets them in its own
// native terms (model, auth, session reference).
type OpenRequest struct {
	ThreadID  string
	SessionID string
	RootDir   string
}

// ResumeRequest is OpenRequest plus the persisted binding for resumption.
// The wuu engine resumes from Wuu's canonical history, so it carries no
// adapter state; external engines use their native session reference.
type ResumeRequest struct {
	OpenRequest
	AdapterVersion     string
	ProtocolVersion    string
	ExternalSessionRef string
}

// TurnInput is the normalized user input for one turn.
type TurnInput struct {
	// History is the Wuu canonical message history for the turn. The wuu
	// engine consumes it directly; external engines receive it as the
	// seed for their own native context.
	History []providers.ChatMessage
}

// EventSink receives engine events converted to Wuu's unified event shape.
// The wuu engine emits native StreamEvents; external engine adapters
// translate their native events into the same shape at this boundary.
type EventSink func(providers.StreamEvent)

// TurnResult is the outcome of one engine turn.
type TurnResult struct {
	// Result is the native loop result of the built-in wuu engine. External
	// engines produce their own opaque results that adapters translate into
	// Wuu items and notifications.
	Result agent.LoopResult
}

// Session is one engine-bound conversation handle.
type Session interface {
	RunTurn(context.Context, TurnInput, EventSink) (TurnResult, error)
	Interrupt(context.Context, string) error
	Close(context.Context) error
}

// Factory creates and resumes engine sessions.
type Factory interface {
	Descriptor(context.Context) (Descriptor, error)
	Open(context.Context, OpenRequest) (Session, error)
	Resume(context.Context, ResumeRequest) (Session, error)
}

// Registry maps engine ids to factories. The runtime registers the built-in
// wuu engine at startup; external engines register as they are added.
type Registry struct {
	factories map[EngineID]Factory
}

// ThreadBinding carries the app-server-side state an engine needs to bind a
// session to an existing thread runtime (the live path with cached runtimes).
type ThreadBinding struct {
	ThreadID string
	RootDir  string
	Model    string
	// ExternalRef is the engine's persisted native session reference (for
	// example a codex thread id); empty means the engine must create one.
	ExternalRef string
	// PersistRef stores a newly created native session reference back into
	// the thread's persisted binding.
	PersistRef func(ref string) error
}

// ThreadBoundFactory is an optional factory extension for engines that bind
// to an existing app-server thread runtime instead of being opened fresh.
// The built-in wuu engine is the prime example; external engines implement
// it to route turns for threads already bound to them.
type ThreadBoundFactory interface {
	SessionForThread(context.Context, ThreadBinding) (Session, error)
}

// FailedSession returns a session that fails every turn with err. It is used
// when an engine cannot produce a real session (missing binary, auth failure,
// protocol error) so the turn terminates with the specific error instead of a
// generic unavailable message.
func FailedSession(err error) Session {
	return &failedSession{err: err}
}

type failedSession struct {
	err error
}

func (f *failedSession) RunTurn(context.Context, TurnInput, EventSink) (TurnResult, error) {
	if f == nil || f.err == nil {
		return TurnResult{}, errors.New("engine session unavailable")
	}
	return TurnResult{}, f.err
}

func (f *failedSession) Interrupt(context.Context, string) error {
	return ErrNoActiveTurn
}

func (f *failedSession) Close(context.Context) error {
	return nil
}

// NewRegistry returns an empty engine registry.
func NewRegistry() *Registry {
	return &Registry{factories: make(map[EngineID]Factory)}
}

// Register installs a factory under its own id. It replaces a previous
// factory with the same id.
func (r *Registry) Register(f Factory) error {
	if r == nil || f == nil {
		return errors.New("engine factory is required")
	}
	desc, err := f.Descriptor(context.Background())
	if err != nil {
		return fmt.Errorf("describe engine: %w", err)
	}
	id := EngineID(strings.TrimSpace(string(desc.ID)))
	if id == "" {
		return errors.New("engine descriptor id is required")
	}
	r.factories[id] = f
	return nil
}

// Lookup returns the factory for an engine id, normalized for legacy threads.
func (r *Registry) Lookup(id EngineID) (Factory, bool) {
	if r == nil {
		return nil, false
	}
	f, ok := r.factories[NormalizeEngineID(string(id))]
	return f, ok
}

// Unregister removes a factory by id. Settings-driven enable/disable uses
// this to take an engine out of rotation; threads already bound to it keep
// their history readable but fail turns with an explicit error.
func (r *Registry) Unregister(id EngineID) {
	if r == nil {
		return
	}
	delete(r.factories, NormalizeEngineID(string(id)))
}

// Descriptors lists all registered engines in stable order.
func (r *Registry) Descriptors() []Descriptor {
	if r == nil {
		return nil
	}
	ids := make([]string, 0, len(r.factories))
	for id := range r.factories {
		ids = append(ids, string(id))
	}
	sort.Strings(ids)
	out := make([]Descriptor, 0, len(ids))
	for _, id := range ids {
		if f, ok := r.factories[EngineID(id)]; ok {
			if desc, err := f.Descriptor(context.Background()); err == nil {
				out = append(out, desc)
			}
		}
	}
	return out
}

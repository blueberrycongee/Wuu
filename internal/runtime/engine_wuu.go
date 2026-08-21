package runtime

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/blueberrycongee/wuu/internal/agent"
	"github.com/blueberrycongee/wuu/internal/agentengine"
)

// WuuEngine is the built-in agent engine: the native StreamRunner loop that
// has always driven Wuu conversations. It implements agentengine.Factory so
// the app-server talks to it through the same seam it will use for external
// engines (Claude, Codex).
type WuuEngine struct {
	session *Session
}

// Descriptor describes the built-in engine.
func (e *WuuEngine) Descriptor(context.Context) (agentengine.Descriptor, error) {
	return agentengine.Descriptor{
		ID:      agentengine.EngineWuu,
		Version: "1",
		Capabilities: []string{
			"native-tool-loop",
			"native-session-resume",
		},
	}, nil
}

// Open creates a fresh thread runtime for a new conversation and returns it
// as an engine session. The wuu engine keeps running the existing
// constructor path; only the entry point moves behind the seam.
func (e *WuuEngine) Open(ctx context.Context, req agentengine.OpenRequest) (agentengine.Session, error) {
	if e == nil || e.session == nil {
		return nil, errors.New("wuu engine is not bound to a runtime session")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	threadID := req.ThreadID
	if threadID == "" {
		threadID = req.SessionID
	}
	rootDir := req.RootDir
	if rootDir == "" {
		rootDir = e.session.RootDir
	}
	rt, err := e.session.NewThreadRuntimeForRoot(threadID, rootDir)
	if err != nil {
		return nil, err
	}
	rt.EngineID = agentengine.EngineWuu
	return &wuuEngineSession{runner: rt.StreamRunner}, nil
}

// Resume reopens an existing conversation. The wuu engine restores context
// from Wuu's canonical history, so resume is the same construction path;
// external engines will use their native session references here.
func (e *WuuEngine) Resume(ctx context.Context, req agentengine.ResumeRequest) (agentengine.Session, error) {
	return e.Open(ctx, req.OpenRequest)
}

// SessionForThread binds the built-in engine to an existing thread runtime.
// The app-server keeps a cache of live runtimes and reuses them across turns;
// this is the live-path equivalent of Open.
func (e *WuuEngine) SessionForThread(rt *ThreadRuntime) agentengine.Session {
	if rt == nil || rt.StreamRunner == nil {
		return nil
	}
	return &wuuEngineSession{runner: rt.StreamRunner}
}

// SessionForRunner binds the built-in engine to a workspace-level runner
// (the fallback used when a turn has no thread runtime).
func (e *WuuEngine) SessionForRunner(runner *agent.StreamRunner) agentengine.Session {
	if runner == nil {
		return nil
	}
	return &wuuEngineSession{runner: runner}
}

// EngineAvailable reports whether an engine id resolves in the registry.
// thread creation validates its engine binding against this; the static
// agentengine.CheckEngine only knows the built-in wuu engine.
func (s *Session) EngineAvailable(id agentengine.EngineID) bool {
	if s == nil || s.engines == nil {
		return id == agentengine.EngineWuu
	}
	_, ok := s.engines.Lookup(id)
	return ok
}

// WuuEngine returns the built-in engine factory. The built-in engine only
// needs the native runner, so when the registry is absent (directly
// constructed runtimes, e.g. in tests) a stateless adapter still works.
func (s *Session) WuuEngine() *WuuEngine {
	if s != nil && s.engines != nil {
		if f, ok := s.engines.Lookup(agentengine.EngineWuu); ok {
			if w, ok := f.(*WuuEngine); ok {
				return w
			}
		}
	}
	return &WuuEngine{session: s}
}

// EngineSessionForThread returns the engine session bound to an existing
// thread runtime. The thread's persisted engine selects the implementation;
// a thread bound to an engine this build cannot host gets a session that
// fails every turn with an explicit error (history stays readable).
func (s *Session) EngineSessionForThread(ctx context.Context, rt *ThreadRuntime, binding agentengine.ThreadBinding) agentengine.Session {
	if rt == nil {
		return nil
	}
	id := agentengine.NormalizeEngineID(string(rt.EngineID))
	if err := agentengine.CheckEngine(id); err != nil {
		return &unavailableEngineSession{id: id}
	}
	if id == agentengine.EngineWuu {
		if rt.StreamRunner == nil {
			return nil
		}
		if w := s.WuuEngine(); w != nil {
			return w.SessionForThread(rt)
		}
		return &unavailableEngineSession{id: id}
	}
	if s == nil || s.engines == nil {
		return &unavailableEngineSession{id: id}
	}
	f, ok := s.engines.Lookup(id)
	if !ok {
		return &unavailableEngineSession{id: id}
	}
	tbf, ok := f.(agentengine.ThreadBoundFactory)
	if !ok {
		return &unavailableEngineSession{id: id}
	}
	sess, err := tbf.SessionForThread(ctx, binding)
	if err != nil {
		return agentengine.FailedSession(err)
	}
	if sess == nil {
		return &unavailableEngineSession{id: id}
	}
	return sess
}

// wuuEngineSession adapts a StreamRunner to the engine Session contract.
// RunTurn is exactly StreamRunner.RunWithCallback: the native loop, streamed
// through the caller's event sink. Interrupt and Close are host-owned for the
// built-in engine (the app-server cancels turn contexts and cleans up thread
// runtimes itself), so they only guard the per-turn context this session
// creates.
type wuuEngineSession struct {
	mu     sync.Mutex
	runner *agent.StreamRunner
	cancel context.CancelFunc
}

func (w *wuuEngineSession) RunTurn(ctx context.Context, input agentengine.TurnInput, sink agentengine.EventSink) (agentengine.TurnResult, error) {
	if w == nil || w.runner == nil {
		return agentengine.TurnResult{}, errors.New("wuu engine session has no runner")
	}
	ctx, cancel := context.WithCancel(ctx)
	w.mu.Lock()
	w.cancel = cancel
	w.mu.Unlock()
	defer func() {
		w.mu.Lock()
		w.cancel = nil
		w.mu.Unlock()
		cancel()
	}()
	res, err := w.runner.RunWithCallback(ctx, input.History, agent.StreamCallback(sink))
	return agentengine.TurnResult{Result: res}, err
}

func (w *wuuEngineSession) Interrupt(_ context.Context, _ string) error {
	if w == nil {
		return errors.New("wuu engine session is nil")
	}
	w.mu.Lock()
	cancel := w.cancel
	w.mu.Unlock()
	if cancel == nil {
		return agentengine.ErrNoActiveTurn
	}
	cancel()
	return nil
}

// Close releases per-turn state. The host owns the runner and thread runtime
// lifecycle for the built-in engine, so there is nothing to shut down here.
func (w *wuuEngineSession) Close(context.Context) error {
	w.mu.Lock()
	cancel := w.cancel
	w.cancel = nil
	w.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	return nil
}

// EngineUnavailableSession returns an engine session that fails every turn
// with an explicit error naming the engine. It is the last-resort session the
// app-server uses when no engine implementation can be resolved at all, so a
// turn terminates with a clear error instead of panicking.
func EngineUnavailableSession(id agentengine.EngineID) agentengine.Session {
	return &unavailableEngineSession{id: id}
}

// unavailableEngineSession fails turns for threads bound to an engine this
// build cannot host. The failure is explicit so the UI can explain it instead
// of silently running the native loop.
type unavailableEngineSession struct {
	id agentengine.EngineID
}

func (u *unavailableEngineSession) RunTurn(context.Context, agentengine.TurnInput, agentengine.EventSink) (agentengine.TurnResult, error) {
	if u == nil {
		return agentengine.TurnResult{}, errors.New("engine session is nil")
	}
	return agentengine.TurnResult{}, fmt.Errorf("%w: %q is not available in this build", agentengine.ErrUnknownEngine, u.id)
}

func (u *unavailableEngineSession) Interrupt(context.Context, string) error {
	return agentengine.ErrNoActiveTurn
}

func (u *unavailableEngineSession) Close(context.Context) error {
	return nil
}

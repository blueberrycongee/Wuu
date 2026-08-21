package runtime

import (
	"context"
	"errors"
	"testing"

	"github.com/blueberrycongee/wuu/internal/agent"
	"github.com/blueberrycongee/wuu/internal/agentengine"
)

func TestWuuEngineDescriptor(t *testing.T) {
	e := &WuuEngine{}
	desc, err := e.Descriptor(context.Background())
	if err != nil {
		t.Fatalf("Descriptor: %v", err)
	}
	if desc.ID != agentengine.EngineWuu {
		t.Fatalf("descriptor id = %q, want wuu", desc.ID)
	}
	if desc.Version == "" {
		t.Fatal("descriptor version must not be empty")
	}
}

func TestWuuEngineSessionNilRunner(t *testing.T) {
	sess := &wuuEngineSession{}
	if _, err := sess.RunTurn(context.Background(), agentengine.TurnInput{}, nil); err == nil {
		t.Fatal("RunTurn on a session without a runner must fail")
	}
}

func TestWuuEngineSessionInterrupt(t *testing.T) {
	sess := &wuuEngineSession{}
	if err := sess.Interrupt(context.Background(), "stop"); !errors.Is(err, agentengine.ErrNoActiveTurn) {
		t.Fatalf("Interrupt without a turn = %v, want ErrNoActiveTurn", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sess.mu.Lock()
	sess.cancel = cancel
	sess.mu.Unlock()
	if err := sess.Interrupt(context.Background(), "stop"); err != nil {
		t.Fatalf("Interrupt with an active turn: %v", err)
	}
	if ctx.Err() == nil {
		t.Fatal("Interrupt must cancel the active turn context")
	}
	if err := sess.Close(context.Background()); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestSessionForThreadNilHandling(t *testing.T) {
	e := &WuuEngine{}
	if sess := e.SessionForThread(nil); sess != nil {
		t.Fatal("SessionForThread(nil) must return nil")
	}
	rt := &ThreadRuntime{}
	if sess := e.SessionForThread(rt); sess != nil {
		t.Fatal("SessionForThread with nil runner must return nil")
	}
	if sess := e.SessionForRunner(nil); sess != nil {
		t.Fatal("SessionForRunner(nil) must return nil")
	}
}

func TestEngineSessionForThreadUnavailable(t *testing.T) {
	s := &Session{engines: agentengine.NewRegistry()}
	rt := &ThreadRuntime{
		StreamRunner: &agent.StreamRunner{},
		EngineID:     agentengine.EngineWuu,
	}
	sess := s.EngineSessionForThread(context.Background(), rt, agentengine.ThreadBinding{})
	if sess == nil {
		t.Fatal("wuu thread must resolve to an engine session")
	}
	rt.EngineID = agentengine.EngineID("claude")
	sess = s.EngineSessionForThread(context.Background(), rt, agentengine.ThreadBinding{})
	if sess == nil {
		t.Fatal("unavailable engine must still resolve to an error session")
	}
	if _, err := sess.RunTurn(context.Background(), agentengine.TurnInput{}, nil); !errors.Is(err, agentengine.ErrUnknownEngine) {
		t.Fatalf("RunTurn on unavailable engine = %v, want ErrUnknownEngine", err)
	}
	if err := sess.Close(context.Background()); err != nil {
		t.Fatalf("Close on unavailable session: %v", err)
	}
	// A codex-bound runtime without a registered factory also fails closed.
	rt.EngineID = agentengine.EngineID("codex")
	sess = s.EngineSessionForThread(context.Background(), rt, agentengine.ThreadBinding{})
	if sess == nil {
		t.Fatal("unregistered engine must still resolve to an error session")
	}
	if _, err := sess.RunTurn(context.Background(), agentengine.TurnInput{}, nil); !errors.Is(err, agentengine.ErrUnknownEngine) {
		t.Fatalf("RunTurn on unregistered engine = %v, want ErrUnknownEngine", err)
	}
}

func TestEngineSessionForThreadUsesRegisteredExternalFactory(t *testing.T) {
	registry := agentengine.NewRegistry()
	factory := &testThreadBoundEngine{id: agentengine.EngineID("claude")}
	if err := registry.Register(factory); err != nil {
		t.Fatalf("register external engine: %v", err)
	}
	s := &Session{engines: registry}
	rt := &ThreadRuntime{EngineID: factory.id}
	binding := agentengine.ThreadBinding{ThreadID: "thread-1", RootDir: "/workspace"}

	sess := s.EngineSessionForThread(context.Background(), rt, binding)
	if sess != factory.session {
		t.Fatalf("resolved session = %T, want registered external session", sess)
	}
	if factory.binding.ThreadID != binding.ThreadID || factory.binding.RootDir != binding.RootDir {
		t.Fatalf("binding = %+v, want %+v", factory.binding, binding)
	}
}

func TestEngineUnavailableSession(t *testing.T) {
	sess := EngineUnavailableSession(agentengine.EngineID("codex"))
	if _, err := sess.RunTurn(context.Background(), agentengine.TurnInput{}, nil); !errors.Is(err, agentengine.ErrUnknownEngine) {
		t.Fatalf("RunTurn = %v, want ErrUnknownEngine", err)
	}
}

type testThreadBoundEngine struct {
	id      agentengine.EngineID
	binding agentengine.ThreadBinding
	session *testEngineSession
}

func (e *testThreadBoundEngine) Descriptor(context.Context) (agentengine.Descriptor, error) {
	return agentengine.Descriptor{ID: e.id, Version: "test"}, nil
}

func (e *testThreadBoundEngine) Open(context.Context, agentengine.OpenRequest) (agentengine.Session, error) {
	return nil, errors.New("unexpected Open")
}

func (e *testThreadBoundEngine) Resume(context.Context, agentengine.ResumeRequest) (agentengine.Session, error) {
	return nil, errors.New("unexpected Resume")
}

func (e *testThreadBoundEngine) SessionForThread(_ context.Context, binding agentengine.ThreadBinding) (agentengine.Session, error) {
	e.binding = binding
	if e.session == nil {
		e.session = &testEngineSession{}
	}
	return e.session, nil
}

type testEngineSession struct{}

func (*testEngineSession) RunTurn(context.Context, agentengine.TurnInput, agentengine.EventSink) (agentengine.TurnResult, error) {
	return agentengine.TurnResult{}, nil
}

func (*testEngineSession) Interrupt(context.Context, string) error { return nil }
func (*testEngineSession) Close(context.Context) error             { return nil }

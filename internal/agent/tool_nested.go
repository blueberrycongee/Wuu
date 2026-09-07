package agent

import (
	"context"
	"crypto/rand"
	"errors"
	"strings"
	"sync"

	"github.com/blueberrycongee/wuu/internal/providers"
	"github.com/blueberrycongee/wuu/internal/toolctx"
	"github.com/blueberrycongee/wuu/internal/toolresult"
)

func toolIsOrchestrator(executor ToolExecutor, call providers.ToolCall) bool {
	provider, ok := executor.(ToolMetadataProvider)
	if !ok {
		return false
	}
	metadata, found := provider.ToolMetadata(call)
	return found && metadata.Orchestrator
}

type nestedToolScope struct {
	runtime *TurnToolRuntime
	parent  *toolRun
	ctx     context.Context
	mu      sync.Mutex
	calls   map[string]*toolRun
}

func (s *nestedToolScope) Invoke(ctx context.Context, call providers.ToolCall) (toolresult.Result, error) {
	if strings.TrimSpace(call.ID) == "" || strings.TrimSpace(call.Name) == "" {
		return toolresult.Result{}, errors.New("nested tool requires a local call ID and name")
	}
	if s.parent.depth >= 8 {
		return toolresult.Result{}, errors.New("nested tool depth limit reached")
	}
	// Both the owner and this invocation can cancel a child; neither can keep
	// it alive after the other has ended. Observation detachment is separate.
	childCtx, cancel := context.WithCancel(s.ctx)
	stop := context.AfterFunc(ctx, cancel)
	defer stop()
	defer cancel()
	if err := ctx.Err(); err != nil {
		return toolresult.Result{}, err
	}
	if err := s.ctx.Err(); err != nil {
		return toolresult.Result{}, err
	}
	s.mu.Lock()
	r := s.runtime
	r.mu.Lock()
	if r.canceled || s.ctx.Err() != nil {
		r.mu.Unlock()
		s.mu.Unlock()
		return toolresult.Result{}, context.Canceled
	}
	run := s.calls[call.ID]
	if run != nil {
		if run.call.Name != call.Name || run.call.Arguments != call.Arguments || run.call.Kind != call.Kind {
			r.mu.Unlock()
			s.mu.Unlock()
			return toolresult.Result{}, errors.New("nested call ID reused with different arguments")
		}
	} else {
		if len(s.calls) >= 1024 {
			r.mu.Unlock()
			s.mu.Unlock()
			return toolresult.Result{}, errors.New("nested tool call limit reached")
		}
		localID := call.ID
		// Never trust a child's local key as a global/provider call identity.
		call = providers.ToolCall{ID: "nested-" + rand.Text(), Name: call.Name, Arguments: call.Arguments, Kind: call.Kind}
		run = r.runForCallLocked(call)
		run.parent, run.depth = s.parent, s.parent.depth+1
		s.calls[localID] = run
		r.startRunLocked(childCtx, run, false)
	}
	r.mu.Unlock()
	s.mu.Unlock()
	return r.awaitRunResult(childCtx, run)
}

// OutlivingNested returns a scope whose cancellation follows the owning turn
// runtime instead of this orchestrator tool call. A code-mode exec tool call
// yields and returns while its cell keeps invoking tools from later model
// steps; those children must still route through the same policy, shared
// scheduling gate, parent linkage, and ledger, and must stop when the turn
// runtime is canceled. Call IDs stay scope-local to the new scope.
func (s *nestedToolScope) OutlivingNested() toolctx.NestedExecutor {
	base := s.ctx
	if s.runtime.runContext != nil {
		base = s.runtime.runContext
	}
	if base == nil {
		// Non-streaming runtimes have no turn-scoped base context. The scope
		// still enforces cancellation through r.canceled; the absence of a
		// context only means cell termination follows the session lifecycle.
		base = context.Background()
	}
	return &nestedToolScope{runtime: s.runtime, parent: s.parent, ctx: base, calls: make(map[string]*toolRun)}
}

// Done exposes the outliving scope's lifetime: it closes when the owning turn
// runtime ends, including through cancellation. A yielded code-mode cell can
// be terminated as soon as its owning turn no longer exists.
func (s *nestedToolScope) Done() <-chan struct{} {
	if s.ctx == nil {
		closed := make(chan struct{})
		close(closed)
		return closed
	}
	return s.ctx.Done()
}

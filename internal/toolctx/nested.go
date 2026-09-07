package toolctx

import (
	"context"

	"github.com/blueberrycongee/wuu/internal/providers"
	"github.com/blueberrycongee/wuu/internal/toolresult"
)

// NestedExecutor routes child calls through the owning agent's execution
// pipeline, including policy, scheduling and durable recording. Call IDs are
// idempotency keys local to this scope, not provider/global execution IDs.
// A repeated ID must carry the same tool and arguments. The scope is revoked
// when its owner ends; retaining this interface does not extend its lifetime.
type NestedExecutor interface {
	Invoke(context.Context, providers.ToolCall) (toolresult.Result, error)
}

type nestedExecutorKey struct{}

func WithNestedExecutor(ctx context.Context, executor NestedExecutor) context.Context {
	return context.WithValue(ctx, nestedExecutorKey{}, executor)
}

// Nested returns the scoped bridge, if this invocation is an orchestrator.
// Ordinary leaf tools do not receive one and must not bypass the scheduler.
func Nested(ctx context.Context) (NestedExecutor, bool) {
	executor, ok := ctx.Value(nestedExecutorKey{}).(NestedExecutor)
	return executor, ok && executor != nil
}

// OutlivingScopeSource is implemented by orchestrator scopes that can hand out
// an executor whose lifetime follows the owning turn runtime rather than the
// orchestrator tool call itself. Code-mode cells need this: a yielded cell
// keeps making nested tool calls in later model steps, after the exec tool
// call has returned, while its child executions must still stop when the turn
// is cancelled, the extension is disabled, or the session tears down.
type OutlivingScopeSource interface {
	OutlivingNested() NestedExecutor
}

// OutlivingNested returns an executor that outlives the current orchestrator
// tool call, or false when the owning scope does not support it. The returned
// executor inherits the same parent linkage, policy, scheduling gate, and
// durable recording as the orchestrator's own scope.
func OutlivingNested(ctx context.Context) (NestedExecutor, bool) {
	source, ok := ctx.Value(nestedExecutorKey{}).(OutlivingScopeSource)
	if !ok {
		return nil, false
	}
	executor := source.OutlivingNested()
	return executor, executor != nil
}

// ScopedExecutor exposes the owning scope's lifetime. Holders of long-lived
// cells use Done to stop the cell when the scope ends: the turn that owns the
// cell has finished or been cancelled.
type ScopedExecutor interface {
	NestedExecutor
	Done() <-chan struct{}
}

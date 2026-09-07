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

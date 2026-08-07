# First-Party Migration Proof — Phase B

This document audits whether the public plugin contract is sufficient for real
first-party Wuu features. A feature is not proven migrated merely because its
Tool, prompt, or UI moved under `plugins/`; its complete product workflow must run
without a product-specific host seam.

## Principle

> 如果一项功能只能通过修改 Agent loop 实现，应先判断缺少的是哪一个公共能力，而不是直接增加产品专用分支。

## Example 1: Custom Tool Permission Guard

**Current implementation**: Tool permission checks are embedded in the tool execution path within `internal/agent/tool_runtime.go`.

**Public seam migration**:

```go
// Register via the capability contract as a plugin would:
// seam: agent.tool.execute.before (guard)
// priority: 100 (executes before other guards)

func GuardFilePathAccess(ctx context.Context, input ToolExecuteInput) (ToolExecuteInput, error) {
    // Check if the tool is attempting to access paths outside the workspace.
    // Reject before execution reaches the tool runtime.
    if isOutsideWorkspace(input.Arguments) {
        return input, ErrPathAccessDenied
    }
    return input, nil
}
```

**Why this works**: The `agent.tool.execute.before` seam is a guard — it short-circuits on rejection. The permission policy plugin registers at high priority and blocks unauthorized access before any other guard runs.

## Boundary Example 2: Plan Stays in the Core Loop

Plan is the standard execution state of the main Agent loop. `update_plan`, the
current task's plan state, lifecycle events, and standard presentation remain in
the core. Plan must not grow into cross-Turn scheduling, automatic continuation,
or a durable Goal system.

Plugins may observe or present the standard plan state where a future public
contract permits it, but they do not replace its core semantics. The earlier
proposal to prove Plan migration with an `agent.system_prompt.section` was the
wrong boundary: moving one prompt paragraph would not migrate the actual plan
state or Tool lifecycle.

## Example 3: Goal Uses Generic Session Delivery

**Current implementation**: Goal owns its state machine, storage, Tool, prompt,
and Desktop contribution. It observes `agent.turn.completed` and calls
`host.session.send` on the same Session with a full continuation prompt and a
safe read-only query summary such as “Goal 持续推进中”.

**Target public composition**:

```go
// Observe the settled Turn, check plugin-owned goal state, then submit another
// ordinary Turn to the same Session.
session.Send(SessionSendRequest{
    SessionID: parent,
    Input: continuationPrompt(goal),
    Presentation: GeneratedQuery("Goal 持续推进中"),
    Cause: "plugin.goal.continue",
    RequestID: stableAttemptID,
})
```

The host owns idempotency, durable queue admission, execution leases, user-work
priority, cancellation, and recovery. The plugin owns the decision to continue
and the two forms of content: full model input and concise user presentation.
This path is now proven; `agent.turn.continuation` and its host polling have
been deleted.

## Example 4: Subagent Uses Private Child Sessions

**Current implementation**: The plugin owns the model-visible Tools, prompting,
and UI, while `host.child_session.request` still switches over
`spawn/send/close/list/await/report` and the core still understands Subagent
product vocabulary. This is a facade extraction, not a complete vertical
migration.

**Target public composition**:

```go
child := session.Create(SessionCreateRequest{
    Owner: "plugin.subagent",
    Visibility: PluginPrivate,
    Parent: parent,
    Context: FreshOrForkParent,
    Workspace: SharedOrWorktree,
})
session.Send(child.ID, taskPrompt)
// On child terminal event, wake the parent through the same generated-query
// delivery used by Goal.
session.Send(parent, completionPayload, GeneratedQuery("子任务已更新"))
```

The existing concurrency, persistence, cancellation, recovery, and worktree
code remains valuable, but must become a product-neutral Session/resource
engine. Task names, worker types, `spawn_agent`, reporting format, and completion
prompting belong to the plugin. HelpMe is deleted rather than migrated.

## Example 5: Compaction Strategy

**Current implementation**: `compact.CompactWithBudgetAndOptions` is called directly from `Runner.RunWithUsage`.

**Public seam migration**:

```go
// seam: agent.compaction (decision)
// key: "wuu.summary-compaction"
// priority: 1 (default; custom strategies use higher priority)

type SummaryCompactionProvider struct{}

func (p *SummaryCompactionProvider) CompactionKey() string { return "wuu.summary-compaction" }
func (p *SummaryCompactionProvider) CompactionPriority() int { return 1 }
func (p *SummaryCompactionProvider) Compact(ctx context.Context, model string, messages []providers.ChatMessage) ([]providers.ChatMessage, error) {
    return compact.CompactWithBudgetAndOptions(ctx, messages, client, model, budget, opts)
}
```

**Why this works**: `CompactionRegistry.Resolve()` returns the highest-priority provider. A plugin can register a custom compaction strategy at priority 100 to override the default.

## Verification

The plugin examples should ultimately use only:

- the versioned public plugin SDK;
- documented capability and host-service contracts;
- generation-owned Tool, prompt, event, storage, Session, and Desktop contributions.

They must not import or require:

- any `internal/agent`, `internal/agentcontrol`, or `internal/subagent` package;
- private app-server RPCs or product-specific host action switches;
- private React state or internal class names.

Goal is complete only after the continuation seam is gone. Subagent is complete
only after the child-session action switch and core product branches are gone.
Until then, these are migration targets rather than proof that the current public
contract is already sufficient.

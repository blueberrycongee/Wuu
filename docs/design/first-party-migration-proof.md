# First-Party Migration Proof — Phase B

This document audits whether the public plugin contract is sufficient for real
first-party Wuu features. A feature is not proven migrated merely because its
Tool, prompt, or UI moved under `plugins/`; its complete product workflow must run
without a product-specific host seam.

## Principle

> 如果一项功能只能通过修改 Agent loop 实现，应先判断缺少的是哪一个公共能力，而不是直接增加产品专用分支。

## Boundary Example 1: Tool Permission Policy Stays Host-Owned

**Current implementation**: Tool permission checks are embedded in the tool execution path within `internal/agent/tool_runtime.go`.

This is intentionally not a public plugin seam. The former
`tool.execute.before/after` RuntimeHook branch had no distributed consumer and
duplicated Tool registration and capability dispatch, so it was deleted. Final
permission decisions remain in the host safety boundary. A plugin can register
and execute its own Tool, but cannot wrap every other Tool or override the
host's final policy.

## Boundary Example 2: Plan Is Temporarily Core, Not a Permanent Kernel Rule

Plan is currently implemented as standard execution state of the main Agent
loop. `update_plan`, the current task's plan state, lifecycle events, and
standard presentation therefore remain in the core today. This is an
implementation fact, not a permanent Plugin Kernel boundary. Plan must not grow
into cross-Turn scheduling, automatic continuation, or a durable Goal system.

Plugins may observe or present the standard plan state where a future public
contract permits it. After the Agent Loop Driver and generic collaboration-state
contracts exist, the default Plan implementation should be reconsidered as a
bundled first-party plugin paired with the default driver. The earlier proposal
to prove Plan migration with only an `agent.system_prompt.section` was still the
wrong boundary: moving one prompt paragraph would not migrate the actual plan
state, Tool lifecycle, recovery, or presentation.

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
task records, completion delivery, and UI. It composes
`host.session.create/send/list/cancel`; the former
`host.child_session.request` action switch has been deleted.

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

The host stamps generation-bound ownership, hides plugin-private Sessions from
ordinary history and search, enforces cancellation and worktree boundaries,
and returns final output only to the submitting plugin's lifecycle observer.
Task names, worker prompts, `spawn_agent`, status presentation, and completion
prompting belong to the plugin. HelpMe is deleted rather than migrated.

Proactive delegation is also plugin-owned. The Subagent runtime stores its
setting through namespaced storage and contributes a dynamic
`agent.request.transform`; its Desktop module contributes the composer control.
The core has no Ultra config field, turn snapshot, policy injection, CLI/API
switch, IPC state, or native Composer treatment.

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

The five distributed first-party plugins now use only:

- the versioned public plugin SDK;
- documented capability and host-service contracts;
- generation-owned Tool, prompt, event, storage, Session, and Desktop contributions.

They must not import or require:

- any `internal/agent`, `internal/agentcontrol`, or `internal/subagent` package;
- private app-server RPCs or product-specific host action switches;
- private React state or internal class names.

The current proof matrix is:

| Plugin | Runtime proof | Desktop proof | Shutdown and recovery proof |
| --- | --- | --- | --- |
| Goal | Tool, prompt, storage, settled-Turn observation and generated-query delivery use public contracts | Real bundled module registers its View, Slot and settings entry | Disable removes contributions; a new generation restores plugin-owned state |
| Subagent | Tool, private Session lifecycle, completion delivery and proactive request transform use public contracts, including a real subprocess protocol test | Real bundled module registers status, settings and composer-toolbar contributions | Disable removes contributions; generation ownership isolates registrations and storage |
| Automation | Cron parser, Timer, records, prompt and Session delivery live in the plugin | Real bundled module owns navigation, View and settings | Shutdown joins the Timer loop; a new generation restores namespaced state |
| Memory | User/workspace/session files, Tools, prompt and management Session live in the plugin | Real bundled module owns navigation, View and settings | Disable removes prompt, Tools and UI; workspace state is recovered through plugin storage/files |
| Dream | Candidate selection, Timer, retry state, prompt and private Session live in the plugin | Real bundled module owns navigation, View and settings | Shutdown joins the Timer loop; a new generation restores state; writes require the Memory plugin Tool |

`FirstPartyPluginLifecycle.test.ts` executes all five real bundled Desktop modules
through the production `WorkbenchController`/`PluginHost` path. It verifies atomic
generation activation and replacement, and that disabling a plugin removes every
View, Slot, navigation item, settings entry, locale, and style contribution.
Ordinary Agent sessions remain usable when any or all of these plugins are absent.

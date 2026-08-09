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

## Boundary Example 2: Plan Uses Semantic Tool Facts

Plan is a bundled first-party plugin. Its runtime owns the model-visible Tool,
argument validation, and result contract; its Desktop module owns the Tool
Activity presenter and Inspector section. The core no longer registers
`update_plan`, stores mutable plan state, restores it into a Toolkit, injects a
stale-plan reminder, or renders a native plan section.

The host persists ordinary Tool call/result facts and projects the public
`display.capability = "plan"` semantic into the versioned stream and Inspector
snapshots. It does not recognize the plugin's hashed public Tool name. This keeps
recovery in the causal transcript and lets disabling the plugin remove its Tool
and UI without a compatibility implementation in core. Plan remains bounded to
the current Turn; cross-Turn continuation and durable goals belong to different
plugins.

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

`agent.compaction` is connected to the live stream and the host validates the
returned Tool history, but it remains Experimental. No distributed first-party
plugin consumes it, and the developer-loop sample no longer registers a
pass-through implementation merely to make the seam appear proven. The default
compactor remains the fallback until a real second strategy demonstrates the
public contract.

## Verification

The six distributed first-party plugins now use only:

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
| Plan | Tool schema, validation and result contract live in the plugin; host stores ordinary Tool facts | Real bundled module owns the semantic Tool presenter and Inspector section | Disable removes Tool, presenter, section and style; the transcript remains the recoverable source of truth |

`FirstPartyPluginLifecycle.test.ts` executes all six real bundled Desktop modules
through the production `WorkbenchController`/`PluginHost` path. It verifies atomic
generation activation and replacement, and that disabling a plugin removes every
View, Slot, navigation item, settings entry, Inspector section, Presenter,
locale, and style contribution.
Ordinary Agent sessions remain usable when any or all of these plugins are absent.

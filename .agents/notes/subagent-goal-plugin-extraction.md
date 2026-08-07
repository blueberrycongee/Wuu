# Subagent and Goal first-party plugin extraction

## Acceptance boundary

This is a full vertical extraction, not a wrapper around the existing backend.
Each first-party plugin owns its:

- model prompt and request-only context;
- model-visible tools and tool copy;
- product state and persistence schema;
- app-server-facing operations and product events;
- Desktop components, commands, interaction state, settings, locales, and styles;
- product tests and documentation.

Disabling or removing either plugin must leave a usable core Agent. Core
production code must not retain `goal`, `subagent`, `spawn_agent`, or equivalent
product branches. The host may retain only product-neutral primitives used by
plugins.

## Existing coupling that must be removed

- `internal/tools/toolkit.go` registers Goal and Subagent tools directly.
- `internal/runtime/session.go` constructs `GoalRuntime` and `AgentControl` for
  every thread and injects their product context.
- `internal/context/context.go` recognizes Goal continuation and Subagent
  completion envelopes by product-specific names and tags.
- `internal/appserver/protocol.go` publishes Goal methods/types and Subagent
  notifications as core protocol.
- `internal/appserver/goal_handlers.go` owns Goal CRUD and UI projections.
- `internal/appserver/turn_handlers.go` owns Goal continuation scheduling.
- `internal/appserver/agent_threads.go` maps Subagent lifecycle into thread and
  turn state.
- `desktop/src/renderer/App.tsx` owns Goal fetching/mutations and Subagent UI
  state directly.
- `prompts/system_main.md` contains Subagent-specific completion guidance.
- `packages/plugin-sdk/src/index.ts` currently advertises product-specific
  `host.subagent.*` services; these are not acceptable generic plugin seams.

## Minimum host seams forced by the extraction

These are hypotheses to validate against both real migrations. Do not add one
until the extraction reaches its existing call site.

1. Generation-bound Desktop-to-runtime plugin calls with opaque plugin-owned
   method/input/result payloads.
2. Plugin-owned events delivered to its Desktop generation without adding
   product event names to the core app-server protocol.
3. Product-neutral child-session lifecycle operations and snapshots. Scheduling,
   execution leases, cancellation, and durable thread integrity remain host
   responsibilities; worker presets, prompts, reports, and presentation belong
   to the Subagent plugin.
4. Product-neutral continuation requests for a thread. The host arbitrates user
   work, active turns, background processes, and execution leases; the Goal
   plugin decides whether/why to continue and supplies request-only context.
5. Plugin-scoped durable storage keyed by thread/session, rather than a core
   Goal persistence schema.
6. Public renderer React hooks and host primitives required to implement real
   stateful first-party plugin UI without importing private renderer state.

## Migration order

1. Finish full-stack call-chain audits and baseline tests.
2. Add only the shared seams reached by the Subagent extraction.
3. Move the complete Subagent vertical slice and prove core operation with it
   disabled.
4. Reuse those seams for Goal, adding only continuation/storage gaps proven by
   Goal's existing call sites.
5. Delete old protocol, prompt, runtime, app-server, Desktop, SDK, test, and doc
   product paths.
6. Search core production paths for product names and run targeted, affected,
   disabled-plugin, generation rollback, and Desktop tests.

## Workspace guard

Pre-existing edits in plugin documentation and `internal/appserver/server_test.go`
belong to other work. Do not overwrite or fold them into this extraction.

## Progress

### Shared seam 1: generation-bound client requests

Implemented the first extraction-forced seam across pluginhost, app-server,
Desktop preload/main, renderer PluginHost, the shared protocol, and the public
SDK:

- capability: `plugin.client.request`;
- app-server method: `plugin/client/request`;
- Desktop API: `requestPluginRuntime`;
- plugin renderer API: `invokeRuntime(method, input)`.

The host validates the installed plugin identity, active fingerprint, enabled
state, exact capability owner, JSON framing, and one-MiB input/output bounds.
The method and payload remain plugin-owned. Replaced generations cannot call the
runtime. Public host-owned React hooks are also typed so first-party plugin UI
does not import Wuu's private React state or bundle another React instance.

Verification completed:

- targeted Go pluginhost/app-server tests pass;
- Desktop typecheck passes;
- PluginHost generation request tests pass;
- public plugin SDK typecheck passes.

### Shared seam 2: generation-bound host events

The Desktop host now forwards its existing product-neutral server event stream
to active plugin generations through `onHostEvent(handler)`. The subscription is
owned by the generation and is disposed on replacement/disable. Goal can refresh
its private summary after ordinary turn/thread events; Subagent can refresh its
child-session projection. This intentionally avoids plugin-specific app-server
notifications and another backend event bus.

Verification completed:

- Desktop typecheck passes;
- PluginHost and DesktopPluginRuntime tests pass (19 tests);
- public plugin SDK typecheck passes.

### Audit corrections and runtime packaging

The audit proposed several seams that current code already provides. Do not add
duplicates: `composer.above` is the Goal strip slot; locales, settings, tool
registration, renderers, and plugin storage already exist. Opaque
`plugin.client.request` replaces custom app-server method registration.

Official first-party plugins use `internal/plugin/bundled` and independent helper
processes. The bundled helper placeholder is being generalized from MCP commands
to `runtime.command`, so Goal/Subagent implementations can live in separate
plugin processes and avoid importing Agent/Desktop private implementations.

### Shared seam 3: continuation request (in progress)

Added the negotiated `agent.turn.continuation` decision contract. It has two
phases: a side-effect-free `probe` before speculative lease acquisition and a
`prepare` recheck after the host owns the thread lease. The plugin returns
request-only context blocks; the host remains solely responsible for queue gates,
execution leases, turn creation, and retries.

### Goal plugin vertical slice (in progress)

- Moved the complete pure Goal state machine, store/runtime code, and tests from
  `internal/goalruntime` to `plugins/goal/runtime` (temporary core imports remain
  only to preserve behavior until the external runtime cutover).
- Added the public standard-library-only `packages/plugin-go` process SDK. It
  serves initialize/tool/capability/shutdown and supports synchronous reverse
  host-service calls without importing any Wuu internal package.
- Added `plugins/goal` plus `cmd/wuu-goal-plugin`. The independent process owns
  `get_goal`, `create_goal`, `update_goal`, plugin-storage persistence, opaque UI
  methods, and the continuation prompt/decision.
- Added a real subprocess integration test proving capability negotiation, tool
  execution, reverse storage RPC, and continuation context across the JSON-lines
  protocol.
- Added the bundled official Goal manifest and independent helper resolution for
  `runtime.command`. Desktop/core packaging now discovers all `cmd/wuu-*-plugin`
  helpers generically and includes them as resources.
- Added a self-contained Goal desktop module using `composer.above`, host React,
  generation-bound runtime requests/events, plugin locales, and plugin-owned CSS.
  The generic composer slot context now includes `threadId` and `translate`.

Verification completed:

- Goal state/plugin/process tests pass;
- public Go plugin SDK tests pass;
- bundled manifest/helper resolution test passes;
- generic Desktop core build produces `wuu-goal-plugin`;
- Goal desktop module syntax check and Desktop typecheck pass.

Next cutover: replace app-server Goal continuation with
`agent.turn.continuation`, remove core Goal tool/runtime/handler/protocol/UI paths,
then run disabled-plugin behavior before starting the child-session extraction.

## Execution checkpoint — 2026-08-07

- Goal runtime/tool/continuation/Desktop contribution extraction is implemented.
- Neutral `host.child_session.request` plus tool actor identity are implemented across host, Go SDK, and TypeScript SDK.
- Bundled Subagent helper owns `spawn_agent`, `send_message`, `close_agent`, `agent_report`, `helpme`, its static model prompt section, and a `composer.above` Desktop status contribution.
- Real process integration verifies plugin tools crossing the helper protocol and calling the neutral child-session service.
- Core toolkit registration has been removed for all five Subagent-family tools; stale core tests still assert the old registry and must be rewritten or removed.
- Core Composer Subagent status is removed; handoff rendering/state, dynamic active-task context, and legacy runtime/persistence remain to migrate.
- Latest verified gates: Desktop typecheck, affected Go build, Subagent process integration, and `git diff --check`. Full runtime/tools tests currently fail on stale product-owned expectations.

# Wuu plugin platform architecture

## Status

Accepted direction for `feat/plugin-platform`. This document converts the Cordis comparison, Wuu code audit, frontend proposal, and security review into implementation contracts.

## Product goal

Wuu should be a complete product whose extension ecosystem can safely reshape agent behavior, workflows, tools, providers, and presentation. The host remains usable without plugins; plugins compose through public contracts rather than core switches or DOM reach-through.

## What Wuu adopts from Cordis

1. **Stable service seams instead of concrete imports.** A consumer asks for a capability contract, not a package implementation.
2. **Scoped ownership.** Registrations belong to application, workspace, session, or agent scope.
3. **Disposable registrations.** Every command, hook, provider, slot, setting, subscription, and process has one owner and a deterministic disposer.
4. **Typed event modes.** Observation, ordered transformation, parallel work, and short-circuit policy are distinct contracts rather than one generic callback.
5. **Dependency-aware activation.** Required services and plugin dependencies determine readiness; incidental filesystem order does not.
6. **Host-owned slots.** The host owns rendering and layout. Contributions register into typed `single`, `list`, `keyed`, or `chain` slots with stable ordering and fail-loud collision behavior.

## What Wuu does not copy

- Same-realm third-party JavaScript loading in Electron.
- TypeScript declaration merging or implicit prototype augmentation as the public ABI.
- Cordis `isolate` as a security boundary; it is a service namespace, not process confinement.
- Raw plugin CSS, arbitrary DOM access, or direct access to `window.wuu`.
- A single global service locator with registrations that outlive their owner.

## Trust tiers

### Tier 0 — Declarative metadata

The host may parse and display a bounded manifest before approval. Contributions contain data only and are rendered by Wuu. Initial public surfaces are package metadata and `prompt_template` commands. Discovery never implies execution.

### Tier 1 — Granted capability

An approved package may activate declared subprocess hooks, MCP servers, runtime actions, and future sandboxed UI. The host validates the exact package fingerprint and enforces the granted permission set at every registration and RPC boundary.

### Tier 2 — Trusted executable

Bundled or explicitly trusted native code executes with the user's OS authority. Subprocesses improve crash and lifecycle isolation but are not a security sandbox. Diagnostics must state the execution form and residual filesystem/network risk honestly.

There is no public, ordinary-marketplace tier that injects React into Wuu's renderer. A future developer-only in-process tier must remain explicit and non-default.

## Package identity and precedence

The package subject identifier is `plugin:<scope>:<id>`, where scope is `bundled`, `user`, or `project`. Discovery retains all candidates for diagnostics. Activation resolves one candidate per logical plugin id using current precedence: official bundled, then project, then user. Shadowing is visible; an untrusted package can never impersonate an official bundled package.

The manifest id is stable package identity, not display text. Contribution ids are namespaced as `<plugin-id>.<local-id>`.

## Discovery and activation

Discovery is side-effect free:

1. locate candidate roots;
2. parse the manifest with size and path limits;
3. normalize and validate contributions;
4. calculate package fingerprint and requested permission union;
5. publish inventory metadata.

Discovery must not start a process, connect MCP, register a hook, load a skill into model context, execute a command, or inject UI code.

Activation requires all of the following:

- the candidate wins precedence;
- platform and minimum Wuu version are compatible;
- no dependency is missing or cyclic;
- the package is official bundled or has an exact matching grant;
- it is not disabled or rejected;
- every executable surface declares the permissions required by its contract.

Project and user packages default to `pending`. A changed fingerprint moves an approved package to `changed`, disposes active contributions, and requires a new grant.

## Package fingerprint

One aggregate fingerprint protects the user's package-level approval. It covers:

- canonical normalized manifest fields;
- runtime protocol, command, arguments, declared environment names, and timeout;
- hook, MCP, skill, command, settings, theme, and UI descriptors;
- requested permissions;
- hashes of executable or prompt entry files that resolve inside the plugin root;
- package API and minimum-Wuu-version requirements.

Secret values are excluded so credential rotation does not demand reapproval; secret names and access permissions remain covered. External executable paths can be fingerprinted by declaration but not proven immutable, so the inventory labels them trusted external executables.

## Policy persistence

Existing `extensions.grants` remains the approval record and matches exact subject id plus fingerprint. The settings model gains package policy that is distinct from approval:

- disabled preserves a valid grant but prevents activation;
- rejected records the rejected fingerprint and avoids repeated prompting;
- a different fingerprint invalidates the old decision and becomes `changed` or `pending` as appropriate.

Policy is user-owned configuration. Plugin manifests and project repositories cannot grant themselves authority.

## Inventory contract

The existing extension inventory remains the single fact source. It evolves rather than gaining a parallel plugin-list API.

A root package record contains:

- package subject id, logical id, metadata, provenance, trust tier, and source path;
- aggregate fingerprint and requested permission union;
- `approval_state`: `official`, `pending`, `granted`, `changed`, or `rejected`;
- `runtime_state`: `inactive`, `starting`, `active`, `failed`, `stopping`, or `stopped`;
- enabled/disabled state and user-safe diagnostic;
- contribution counts and compatibility diagnostics.

Child records identify MCP, hook, command, skill, setting, theme, and future UI surfaces with `parent_id`. They are diagnostic details, not separate approval prompts. Standalone configured MCP servers and hooks remain independent subjects.

## Lifecycle manager

The platform introduces a manager that owns an immutable active snapshot and swaps it transactionally:

1. discover and validate a candidate snapshot;
2. resolve dependencies and grants;
3. prepare new processes, MCP clients, hooks, commands, and registries without publishing them;
4. publish the complete snapshot atomically;
5. dispose removed registrations and processes in reverse dependency order.

If preparation fails, the prior valid snapshot remains active. Disable and revoke remove all owned registrations. Existing sessions reference stable registry/host handles so a snapshot swap does not leave captured stale pointers.

Every registration returns a disposer. Package unload runs child disposers in reverse registration order under a deadline, then force-terminates remaining processes. Runtime failure is isolated and visible; it does not corrupt the inventory or silently remove other plugins.

## Permission enforcement

Manifest permissions are validated against a public catalog. Unknown permissions fail activation rather than being ignored.

Initial required permissions include:

| Surface | Required permission |
|---|---|
| start external runtime | `process.spawn` |
| read message/model request content | `session.read` |
| transform message/model request | `session.write` |
| contribute or rewrite tool definitions | `tools.define` |
| intercept tool execution | `tools.intercept` |
| mutate shell environment | `shell.env` |
| connect remote MCP/runtime endpoint | `network.connect` |
| invoke a runtime-backed command | `commands.execute` plus runtime permission |

Initialization rejects hook/action registrations whose required permissions are absent from both the manifest request and exact grant. Payloads are minimized by contract; plugin identity comes from the host-owned connection, never a caller-supplied plugin id.

Plugin processes no longer inherit `os.Environ()`. The host supplies a documented cross-platform baseline needed to launch software and only explicitly approved variable names beyond it. Wuu secrets are never included implicitly.

Native subprocesses retain the user's filesystem and network authority unless an OS sandbox is explicitly active. Version one must not claim otherwise.

## Declarative command contract

The first end-to-end UI contribution is a command descriptor:

```json
{
  "id": "summarize-selection",
  "title": "Summarize selection",
  "description": "Ask the active agent to summarize selected text",
  "kind": "prompt_template",
  "prompt": "prompts/summarize-selection.md",
  "contexts": ["conversation", "channel"]
}
```

`prompt_template` loads a bounded UTF-8 file inside the plugin root and produces a normal host-owned composer action. It does not start plugin code. The host namespaces ids, validates contexts, resolves collisions fail-loud, and unregisters commands when the package is disabled or changed.

`runtime_action` is reserved in the schema but unavailable until its request/response contract, permission, timeout, cancellation, and audit behavior are implemented. An unsupported runtime action is diagnostic, never silently treated as a prompt.

## Frontend architecture

Electron receives validated descriptors through the app-server protocol and exposes no generic privileged plugin bridge.

The renderer owns `PluginContributionRegistry` with typed registries for commands, settings, theme tokens, and future slots. Updates replace one package's contribution set transactionally and return a disposer. Core features and plugin descriptors use the same registry contract so future core extraction does not require separate switches.

The existing Skills/Extensions catalog becomes the package management surface: package state, permissions, grant/reject/revoke, enable/disable, diagnostics, provenance, and expandable child contributions. Plugin-provided schema settings appear in Settings only after activation; lifecycle management stays in Extensions.

Future rich UI runs in a separate sandboxed frame or WebContents without preload access. The main process mints an opaque capability handle bound to frame identity, package fingerprint, grants, and expiry. The host validates every call by handle; the renderer never submits trusted `{pluginId, permissions}` objects.

Themes are validated namespaced design tokens. Raw global CSS is not a public surface.

## Compatibility

- Manifest and runtime protocol versions are explicit and negotiated.
- Additive descriptor fields are allowed only when old hosts can safely ignore them; executable unknown fields fail closed.
- Minimum Wuu version is checked before activation.
- Stable ids and wire fields are documented; Go internals are not the public plugin ABI.
- The TypeScript SDK and JSON schema are generated or checked against the same contract fixtures.
- Fingerprint changes intentionally require renewed approval when executable behavior or requested authority changes.

## First PR vertical slice

The first PR delivers a complete safety and declarative-extension path:

1. package aggregation, fingerprinting, compatibility diagnostics, and permission catalog;
2. discovery/activation separation and grant-gated runtime/MCP/hook/skill activation;
3. minimal process environment and fail-loud hook permission validation;
4. grant, reject, revoke, enable, and disable app-server mutations;
5. root/child extension inventory with approval and runtime states;
6. Extensions management UI using the existing catalog;
7. manifest-to-renderer `prompt_template` command registry;
8. transactional reload/disposal for affected contributions;
9. architecture, threat-model, migration notes, diagnostics, and adversarial contract fixtures.

Out of scope for this PR: marketplace installation/update, arbitrary renderer code, raw CSS, sandbox claims that are not enforced, WASM runtime, per-capability partial grants, and wholesale conversion of every Wuu core subsystem into a plugin.

## Later phases

1. Schema-rendered plugin settings and namespaced secret references.
2. Workflow/provider/tool service registries and agent-scoped contexts.
3. Sandboxed rich UI and typed host slots.
4. Signed packages, provenance verification, installation, update, and rollback.
5. Optional OS sandbox backends and WASM for genuinely constrained code.
6. Extraction of selected core features into bundled plugins after public contracts prove stable.


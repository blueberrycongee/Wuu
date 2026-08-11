# Local Plugin Customization Acceptance Tracker

This tracker measures working product paths, not type declarations or planned APIs. A capability is complete only when a plugin outside Wuu can use the public contract and an automated test proves activation, use, unload, and failure recovery.

## Current acceptance state

| # | Acceptance criterion | State | Current evidence and remaining work |
|---|---|---|---|
| 1 | Install, approve, enable, disable, and remove from a local directory | Complete | The package lifecycle and CLI cover directory/zip install, exact-fingerprint approval, enable/disable, pending update, and removal. Inspection and discovery reject unsupported platforms, future minimum-Wuu requirements, and unavailable runtime executables before approval. |
| 2 | Rebuild and reload after a save in development mode | Complete | `wuu plugin dev` runs the package build, validates a candidate, publishes an atomic generation, waits for active generation leases, and preserves the previous generation on build or activation failure. CLI tests cover the normal, blocked, and failed refresh paths. |
| 3 | Register and execute a Tool | Complete | External Tools are model-visible and execute through `pluginToolExecutor`; registration, execution scope, structured results, failure, and unload are generation-owned. Final permission policy remains Host-owned rather than a global plugin wrapper. |
| 4 | Contribute a system prompt or context section | Complete | Protocol v2 exposes `agent.system_prompt.section`; candidate activation evaluates it before swap, live streams consume the generation registry, and failed candidates preserve the active prompt. The public SDK and standalone example register the typed contract. |
| 5 | Replace compaction or register a model Provider | Experimental | `agent.compaction` is connected to the live stream and validates Tool history, but it has no distributed first-party consumer. Provider adapter remains proposed; neither contract is stable evidence yet. |
| 6 | Add a persistent custom workbench view and replace semantic UI surfaces | Complete | Desktop consumes generation-scoped views and semantic presenters for Tool Activity, complete reasoning/tool process regions, conversation items, Composer, conversation/workspace headers, primary navigation, file preview, status, and Settings. The standalone renderer executes multi-surface replacements and Host Actions, reads settings, restores namespaced state, writes updates, proves failed-generation rollback and render fallback, and disposes every registration on unload. |
| 7 | Change workbench layout and the complete theme language | Complete | The public SDK limits default placement to host-owned `main`, `sidebar`, and `auxiliary` regions; candidate activation rejects unsupported regions and unknown Views, while Workbench preserves priority, user dismissal, durable state, and host recovery. One generated registry keeps manifest, SDK, and Desktop token validation aligned, legacy names map to current semantic tokens, and contract tests require every public theme and syntax token to have a real host consumer. Host-owned UI recipes, the protected Layer Host, shared Modal, and semantic component anchors keep protected chrome and fallback behavior under host control. |
| 8 | Read and write plugin settings and namespaced storage | Complete | Core-owned user/workspace stores are exposed through typed app-server APIs, Workbench host actions, and bidirectional runtime host services. Ownership, generation, type, scope, key, entry, and value limits are enforced; declarative Settings renders boolean, string, number, and enum controls. |
| 9 | Preserve a recoverable host after update, activation failure, or render failure | Complete | Pending package updates do not execute before approval, failed candidate generations keep the old generation, development reload preserves the previous package, and Desktop error boundaries retain disable and Settings escape paths. Generation swap and close tests prove old registrations and host services unload. |
| 10 | Build without importing Wuu private source or private React state | Complete | `@wuu/plugin-sdk` publishes the runtime and host-owned React contracts. The standalone example uses the public package version, builds against an `npm pack` artifact, contains no repository-relative dependency, and does not bundle React. |
| 11 | Pass public SDK contract tests | Complete | `wuu plugin test` starts the configured executable runtime, negotiates protocol v3, validates capabilities and Tools, and exits non-zero on failed checks. The standalone example additionally executes its built renderer through activation, host use, storage recovery, and generation disposal. |
| 12 | Continue working across compatible Wuu minor releases | Blocked | Protocol, manifest, platform, minimum-Wuu, and runtime-executable compatibility gates exist. However, `v0.14.0` and earlier releases predate the public SDK package, so there is no legitimate previous-minor SDK artifact for a released-host matrix. This remains blocked until the first SDK-bearing release can serve as the preserved previous-minor input; an unreleased historical commit is not release evidence. |

## Open composition runtime

This matrix records product paths that an independently developed plugin can
use. A bundled implementation is useful evidence only when it goes through the
same public process boundary and SDK; internal Go registries or type declarations
do not count on their own.

| Capability | State | Product evidence and boundary |
|---|---|---|
| Versioned plugin services | Complete | Plugins publish and consume generation-owned services through the public process protocol. Dream reads Memory through that path. Missing providers, incompatible major versions, stopped providers, and failed providers return typed `service_unavailable`; registry inspection exposes only names, versions, providers, methods, and generation identity. |
| Exact execution identity, progress, and cancellation | Complete | Tool, capability, and kernel-to-plugin service dispatches carry one execution ID. Both public runtime SDKs expose that identity, progress, and immediate handler cancellation; TypeScript also returns exact Turn/queue cancellation results. Progress is owner-checked, cancellation targets only that live dispatch, and late updates cannot reopen it. Session lineage remains descriptive and does not create a host-owned recursive cancellation tree. |
| Plugin-owned Agent topology | Partial | Public Session services let a plugin create, send to, list, and precisely cancel private Sessions. The bundled Subagent proves the same public path, but an independently maintained plugin has not yet demonstrated a DAG, worker pool, detached worker policy, and custom cancellation propagation. No additional topology API is justified until that consumer exists. |
| Replaceable loop driver | Partial | The bundled single-pass driver runs in a plugin process, calls model-loop and checkpoint kernel gateways through services, and resumes only from a compatible checkpoint. Missing drivers fail closed without making stored history unreadable. It still imports private Driver DTOs, selection is process-scoped rather than a durable per-Session fact, and no independently built plugin proves the contract, so this is not yet a public replacement surface. |
| Third-party model provider adapter | Not opened | Wuu has internal provider factories and stream validation, but no public cross-process provider contract and no real external provider consumer. A provider wire contract must not expose internal messages or be added solely to complete this matrix. The first real adapter must drive the contract and must pass Tool call/result ordering through the kernel gateway. |
| Plugin-owned durable Session events and projections | Not opened | Plugins currently persist namespaced state and ordinary transcript facts. No real plugin requires a separate durable event → projection contract yet. When one does, ownership, schema versioning, bounded payloads, Presenter input, generic read-only fallback, and model-input receipts must be designed together. |
| Functional UI and themes compose independently | Complete | Public regions, UI Kit primitives, semantic tokens, Presenter fallback, and generation cleanup are live. The [composition contract test](../../desktop/src/renderer/plugins/ThirdPartyComposition.test.tsx) loads the real Deep UI and Manga Studio example packages together and proves that disabling the theme removes its tokens and CSS without removing the functional surface. |
| Headless install, replace, and rollback | Partial | `wuu plugin` covers inspect, install, approve, enable, update, disable, and remove; `wuu plugin dev` preserves the previous generation after build or activation failure; `wuu exec` can use installed Tools without Desktop. One black-box flow still needs to exercise install → invoke → replace → failed activation rollback end to end against the same development package. |
| Observer, Presenter, and background failure isolation | Complete | Turn observers run outside terminal publication with bounded delivery, renderer failures preserve host fallback, and plugin background delivery cannot reopen or block a committed terminal state. |
| First-party/public-entry parity | Complete | Distributed first-party plugins use the public SDK, process protocol, host services, UI regions, and Presenter APIs. Production lifecycle tests prove generation replacement and full removal without private React state or product-specific host actions. |

## Completion rules

- A public interface, registry, manifest field, or protocol struct alone counts as **Partial**, never **Complete**.
- Every executable contribution must be owned by one plugin generation and disappear after disable, uninstall, upgrade, failed activation, or development reload.
- Every renderer must have a host fallback that preserves Settings, plugin disable, and default UI recovery.
- Development authorization is directory-specific and must never transfer to a normal downloaded package.
- The acceptance plugin must build and test outside the Wuu source tree using only the public SDK and documented commands.
- Marketplace, hosted publishing, remote automatic update, ranking, and remote dependency resolution are not acceptance requirements.

## Implementation lanes

1. Preserve the public SDK artifact from the first SDK-bearing release, then add it as the previous-minor input to the next release's host compatibility matrix.
2. Keep the standalone acceptance plugin in release checks for runtime and renderer contracts.
3. Run a real Desktop install, restart, upgrade, failed activation, failed render, disable, and removal smoke test before each release.

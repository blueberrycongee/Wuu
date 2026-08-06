# Local Plugin Customization Acceptance Tracker

This tracker measures working product paths, not type declarations or planned APIs. A capability is complete only when a plugin outside Wuu can use the public contract and an automated test proves activation, use, unload, and failure recovery.

## Current acceptance state

| # | Acceptance criterion | State | Current evidence and remaining work |
|---|---|---|---|
| 1 | Install, approve, enable, disable, and remove from a local directory | Complete | The package lifecycle and CLI cover directory/zip install, exact-fingerprint approval, enable/disable, pending update, and removal. |
| 2 | Rebuild and reload after a save in development mode | Not complete | The watcher detects changes, but the product path still needs build, validation, atomic generation replacement, failure preservation, and diagnostics. |
| 3 | Register a Tool and wrap its execution policy | Partial | External Tools execute through the runtime host. The public guard/around/after capability chain is not yet connected to external capability registration. |
| 4 | Contribute a system prompt or context section | Partial | Core registries exist, but an external plugin cannot yet register and execute these providers through the process protocol. |
| 5 | Replace compaction or register a model Provider | Partial | Core registries exist and are concurrency-safe. External capability negotiation and generation-scoped registration are not connected. |
| 6 | Add a persistent custom workbench view | Partial | Generation-scoped view snapshots now publish and unload atomically. View instances, host actions, layout placement, and durable state still need product integration. |
| 7 | Change workbench layout and the complete theme language | Partial | Declarative color/syntax themes work. Layout contributions and the wider typography, spacing, density, border, elevation, motion, and content token contract are not complete. |
| 8 | Read and write plugin settings and namespaced storage | Partial | Stores and protocol types exist. The runtime and Desktop host APIs still need production dispatch, workspace scoping, validation, and change delivery. |
| 9 | Preserve a recoverable host after update, activation failure, or render failure | Partial | Core pending-update rollback and current Surface error boundaries work. Workbench views and development reload still need the same fallback and escape-path coverage. |
| 10 | Build without importing Wuu private source or private React state | Partial | A public SDK exists, but generated projects still need a standalone install/build path and a typed host-owned React contract. |
| 11 | Pass public SDK contract tests | Not complete | The current contract helper does not start the configured runtime in its normal path and can report failed checks without failing the test. |
| 12 | Continue working across compatible Wuu minor releases | Not complete | Compatibility anchors exist, but no previous-minor/current-minor contract matrix proves them. |

## Completion rules

- A public interface, registry, manifest field, or protocol struct alone counts as **Partial**, never **Complete**.
- Every executable contribution must be owned by one plugin generation and disappear after disable, uninstall, upgrade, failed activation, or development reload.
- Every renderer must have a host fallback that preserves Settings, plugin disable, and default UI recovery.
- Development authorization is directory-specific and must never transfer to a normal downloaded package.
- The acceptance plugin must build and test outside the Wuu source tree using only the public SDK and documented commands.
- Marketplace, hosted publishing, remote automatic update, ranking, and remote dependency resolution are not acceptance requirements.

## Implementation lanes

1. Connect capability negotiation and generation-scoped external registrations to the live Agent runtime.
2. Consume workbench view, layout, renderer, theme, command, settings, storage, locale, and status registrations in Desktop.
3. Make `create`, `build`, `test`, and `dev` a runnable standalone development loop.
4. Add a standalone acceptance plugin and a compatibility test matrix.
5. Run real Desktop install, restart, upgrade, failed activation, failed render, disable, and removal verification before marking the platform complete.

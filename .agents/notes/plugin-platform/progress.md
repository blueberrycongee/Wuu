# Plugin platform progress

## 2026-08-04 — Research phase started

- Created runtime goal `goal-20260805-005605-4070a772d3675c75`.
- Announced architecture/integration ownership and requested parallel frontend and security/testing proposals in the group room.
- Created and started group task `Cordis 对照与插件平台架构`.
- Created `.worktrees/plugin-platform` and branch `feat/plugin-platform` from `origin/main` at `f1c3b549`.
- Confirmed the original main worktree contains unrelated concurrent changes and will remain untouched.
- Started a read-only audit of Cordis and Wuu’s current extension surfaces.
- Initial finding: Wuu already has manifest discovery, plugin subprocess hosting, typed interception hooks, plugin tools, bundled plugins, and tests. The likely project is an architectural upgrade and productization, not a greenfield loader.
- Le claimed the frontend extension-surface research track and will return a proposal to the integration branch rather than creating a competing worktree.
- Baseline verification passed for `internal/plugin`, `internal/pluginhost`, `internal/extensions`, and all plugin-specific `internal/runtime` tests.
- The full `internal/runtime` baseline has one unrelated failure in `TestApplyGeneralConfigRefreshesPromptAndMemory`: the current test environment injects a WUU attribution trailer despite the fixture disabling attribution. This pre-existing noise is recorded and will not be fixed opportunistically in the plugin PR.

## Current phase

First vertical slice implemented; security review and release-quality verification complete.

## 2026-08-04 — First vertical slice implementation

- Added a closed permission catalog with compatibility aliases; unknown permissions fail manifest loading.
- Added aggregate secret-free package contracts, deterministic fingerprints, canonical package subject IDs, entry-content hashes, and effective permission unions.
- Separated inert discovery from activation. Only provenance-verified official packages or exact fingerprint grants enter plugin host, MCP, hooks, and plugin-skill surfaces.
- Added package policy state (`official`, `pending`, `granted`, `changed`, `rejected`), runtime state, declarative command contributions, and package-level inventory records.
- Added the user-owned `extension/package/update` mutation RPC for exact grant/reject plus revoke/enable/disable. It rejects stale fingerprints and atomically refreshes current runtime surfaces while no turn is running.
- Hardened plugin processes with a minimal cross-platform environment allowlist, bounded JSON-lines responses, serialized calls, and bounded stderr capture.
- Added prompt-template command contributions and fail-closed renderer registration: only enabled, approved, active packages register; collisions and runtime actions do not register.
- Added extension management UI states and actions and wired Electron main/preload/App inventory replacement.
- Security review hardened real-path confinement, expanded fingerprints across skill/prompt assets and behavior-sensitive arguments/hook options, and removed all package policy from shared/project-owned config layers.

### Verification

- Plugin, pluginhost, extensions, hooks, config, package-mutation app-server, and plugin-specific runtime suites pass.
- Plugin-specific runtime and activation suites pass. The full runtime suite still has only the pre-existing attribution fixture failure recorded above.
- Race checks pass for `internal/pluginhost`, `internal/extensions`, and `internal/hooks`.
- Windows amd64 cross-compilation passes for pluginhost, plugin discovery, and app-server packages.
- Desktop TypeScript typecheck and production build pass.
- Targeted renderer suites pass: 5 files, 192 tests covering registry, slash commands, management UI, app state, and styles.
- The full app-server suite also encounters `TestGoalPauseResumeClearRuntimeGoal` (two provider requests instead of one); this reproduces from an untouched `origin/main` archive and does not intersect the plugin changes.

## Next actions

1. Commit and push `feat/plugin-platform`.
2. Open the review PR with architecture, threat model, compatibility notes, and both reproduced baseline failures.

## Blockers

None.

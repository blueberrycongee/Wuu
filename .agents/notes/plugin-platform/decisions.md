# Plugin platform decisions

This is the durable decision log for the unattended implementation. A decision is provisional until evidence and group discussion have converged.

## Accepted

### D-001: Isolate all work from main

Implementation lives in `.worktrees/plugin-platform` on `feat/plugin-platform`, based on the remote main branch. The shared main worktree is not modified.

### D-002: Evolve the existing plugin substrate

Wuu already discovers plugin manifests and hosts external runtimes. The design will preserve and evolve these foundations rather than introduce an unrelated second plugin system.

### D-003: Treat trust and permission enforcement as architecture, not polish

Plugin loading, host APIs, UI contributions, filesystem/process/network access, diagnostics, and revocation must share one explicit capability model. A manifest declaration alone is not enforcement.

## Provisional

### P-001: Use scoped contexts rather than a global service locator

Adopt Cordis's strongest idea—hierarchical contexts with owned registrations and deterministic disposal—while implementing it in a Go-appropriate form. Avoid copying TypeScript decorators or implicit prototype augmentation.

### P-002: Separate declarative contributions from executable runtimes

Skills, themes, settings schemas, commands, and static render metadata should load without granting arbitrary code execution. Executable backend and renderer contributions require explicit trust and permission grants.

### P-003: Ship a narrow vertical slice first

The first PR should establish contracts and prove one end-to-end extension path with lifecycle, permissions, diagnostics, tests, SDK/example, and desktop visibility. Additional agent-core and UI seams should build on the same contracts rather than land as ad hoc hooks.

## Rejected

None yet.


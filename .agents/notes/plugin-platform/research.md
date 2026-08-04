# Plugin platform research

## Objective

Design a Wuu plugin platform that lets third-party developers extend agent behavior, workflows, tools, providers, and the desktop experience without coupling every extension to Wuu core.

## Baseline

- Branch: `feat/plugin-platform`
- Worktree: `.worktrees/plugin-platform`
- Base: `origin/main` at `f1c3b549`

## Preliminary evidence

Cordis models applications as contexts composed from plugins. Its public APIs center on scoped contexts, services, typed events, and disposable effects.

Wuu already has a useful first-generation plugin substrate:

- manifest discovery from bundled, user, and project roots;
- skills, hooks, MCP servers, requested permissions, activity kinds, and external runtimes in the manifest;
- a versioned JSON-lines subprocess protocol;
- deterministic runtime hook chaining for messages, model requests, tool definitions/execution, shell environment, and session lifecycle;
- fail-visible runtime status and reverse-order teardown.

The gap is therefore not "add plugins from zero." The project needs to turn the existing manifest/runtime interception layer into a coherent public platform: lifecycle and dependency semantics, scoped service registries, permission enforcement, user control and diagnostics, renderer extension surfaces, stable SDKs, and compatibility/versioning policy.

## Research questions

1. Which extension surfaces must be typed services versus ordered events/hooks?
2. Which registrations are global, workspace-scoped, session-scoped, or agent-scoped?
3. How are plugin dependencies ordered and disposed without leaking state?
4. Which capabilities can safely execute in-process, subprocess, worker, or renderer sandbox?
5. How are requested permissions granted, persisted, revoked, and enforced at every host API boundary?
6. How can UI plugins contribute commands, settings, panels, message renderers, and themes without arbitrary renderer DOM access?
7. What compatibility contract can Wuu sustain across Go, JSON-RPC, Electron preload, and a TypeScript SDK?
8. What is the smallest vertical slice that proves the architecture without prematurely exposing every core seam?

## Pending inputs

- Cordis architecture evidence audit.
- Wuu extension-surface and security audit.
- Frontend extension proposal from the group.
- Independent security/testing review from the group.


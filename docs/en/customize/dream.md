# Background memory consolidation (Dream)

Dream is Wuu's background memory consolidation mechanism: after conversation turns, it
checks the workspace's finished sessions and consolidates stable facts worth keeping
long term into the workspace's [project_memory](memory.md#workspace-memory). It is off
by default and must be enabled explicitly.

## What problem it solves

Conversations are short-lived: when a session ends, the architecture decisions,
discovered conventions, and lessons learned in it stay in history, and the next
session does not know about them automatically. Dream periodically reads finished
sessions in the background and writes such stable facts into workspace memory, so
later sessions can read them directly.

## Enable and configure

After installing and enabling the first-party Dream plugin, open **Settings → Plugins
→ Dream**, where you can:

- enable or disable Dream;
- set the run interval (days);
- choose an optional model alias (empty inherits the current session's model).

Settings are stored in the plugin's own workspace storage and are no longer written to
the core `memory.dream` configuration. Field reference:

| Field | Meaning |
| --- | --- |
| `enabled` | Whether background consolidation is enabled, default `false` |
| `interval_days` | Minimum days since the last run, default `7` |
| `min_sessions` | Minimum accumulated completed sessions before a trigger, default `5` |
| `model_alias` | Optional model alias; empty inherits the parent session |

Changes made through the plugin settings take effect immediately. Disabling, upgrading,
or uninstalling the plugin stops the Timer and produces no more background wake-ups;
re-enabling resumes from the plugin's persisted state, including candidates, last
result, and failure backoff.

## When it runs

The plugin observes ordinary turn-completion events, and its own Timer checks whether
all of the following conditions hold:

- Dream is enabled;
- at least `interval_days` days have passed since the last run (or it has never run
  successfully);
- the number of completed sessions since the last consolidation reaches `min_sessions`
  (default `5`).

When the conditions hold, the plugin creates a private fork session from the most
recently completed parent session, then delivers the consolidation query through
`host.session.send` without blocking the current conversation. Only one consolidation
task runs at a time; if the previous run failed, it retries after about an hour. When
the plugin generation restarts, unfinished state is folded into a failure so it does
not get stuck forever.

## What it does and does not do

Dream's private session inherits the fork context and reads/updates `project_memory`
through the Memory plugin's `session_memory` tool. The consolidation prompt explicitly
forbids saving keys, raw conversations, temporary progress, PR numbers, commit SHAs, or
short-term facts, and forbids modifying workspace source files; when there is nothing
worth saving it returns `Nothing to dream`. The core no longer owns Dream's prompt,
tool allowlist, timeout, step count, or AfterTurn hooks.

## Privacy and cost

- Dream sends the text of finished sessions to the model service and may incur
  additional model requests and cost;
- a dedicated model can avoid contending with the current conversation's provider,
  but it also produces independent calls;
- the workspace memory written by consolidation enters later agent contexts like any
  other memory; before handling sensitive content, check the [security
  model](../reference/security-model.md) and the memory sources;
- if you do not want a certain kind of content in memory, delete the corresponding
  entries in the memory panel; Dream does not restore deleted content.

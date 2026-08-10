# Wuu configuration model

Wuu splits configuration into two kinds: "user-owned" and "project-supplemented".
The core principle: when you open a repository, the repository can describe how it
likes to work, but it cannot decide where your credentials go, read files outside the
workspace, or expand local permissions for you.

## Configuration sources and order

A normal startup reads the user configuration first, then layers project sources on
top, in order:

1. `~/.wuu/config.json`, or `WUU_HOME/config.json`
2. Legacy `~/.config/wuu/config.json` (compatibility and migration only)
3. Project `.wuu.json`, or `wuu.json` when the former does not exist
4. Project `.wuu/settings.json`
5. Project `.wuu/settings.local.json`

Objects are merged recursively; scalars and arrays are replaced by the later source.
After loading, the writable configuration path returned is always the user
configuration path, so desktop settings and model switching do not accidentally
rewrite repository files.

## Fields that always belong to the user

A normal startup ignores the following fields in all project sources and prints a
note to standard error:

- `default_provider`
- `providers`
- `instructions` (the legacy `memory` instruction-discovery field follows the same
  boundary)
- `agent.model_roles`
- `agent.model_aliases`
- `agent.permission_mode`

These fields respectively control the default provider, endpoint and credential
sources, out-of-workspace instruction discovery, model routing for background roles,
stable model aliases the agent can select explicitly, and Wuu's local permission
boundary. Field names follow JSON's case-matching rules, so renaming them to
`Providers`, `Memory`, or `Permission_Mode` does not bypass the restriction.

Other project behaviors still layer on normally, such as
`agent.append_system_prompt`. Project configuration must conform to the full
configuration structure; unknown fields are reported as errors so typos are not
silently ignored.

## Instructions, Memory, and Dream

Long-term rules that the whole team should follow belong in the repository's
`AGENTS.md` or project documentation. `instructions` only controls the core's generic
instruction-file discovery, so a project cannot redirect it outside the workspace.
The legacy top-level `memory` only migrates the instruction-discovery fields within
the read boundary; it no longer configures or toggles any core memory product.

User, workspace, and session memory are managed by the [Memory
plugin](../customize/memory.md); background consolidation is managed by the [Dream
plugin](../customize/dream.md). Their settings live in the plugins' own namespaces and
are not written into the core configuration. Disabling a plugin removes the
corresponding prompts, tools, background timers, and UI together.

## Explicitly trusting a full configuration

Automation scenarios can explicitly opt into a full configuration:

- `wuu exec --config <path>`: reads and trusts only the specified file.
- `wuu exec --ignore-user-config`: ignores the user configuration and reads and
  trusts the project `.wuu.json` (or `wuu.json`) plus the two project settings layers.

Both approaches accept the provider endpoints, credential environment variable names,
instruction paths, hooks, and MCP servers in the files, so only use them with files
you control. Ordinary desktop and CLI startups do not gain this trust through implicit
conditions such as an empty `HOME`.

## Anonymous worker concurrency and proactive delegation

The core only stores a generic execution capacity:

```json
{
  "agent": {
    "max_parallel": 5
  }
}
```

| Field | How to fill | Default | Semantics |
| --- | --- | --- | --- |
| `agent.max_parallel` | Non-negative integer; `0` equals omitted | `5` | Controls how many anonymous workers can execute at the same time; excess async executions enter `queued`. |

`queued` and `waiting_children` states do not occupy execution slots. A child result
waking the parent worker for integration is not a new spawn, so it does not pass the
spawn queue gate; while integration starts, the actually running count may briefly
exceed `max_parallel`. Negative values are invalid. `initialize`, `config/read`,
`config/model/update`, and the `session_configured` event of `wuu exec --json` all
read back the effective `max_parallel`.

Proactive delegation is not a core configuration or app-server mode. The Subagent
plugin stores its switch in its own namespace, appends a sourced, persistable hidden
message to subsequent model steps through `agent.pre_step`, and provides an A+
control in the Composer toolbar. The core has no `agent.ultra_mode`, turn snapshot,
`ultra` protocol field, or `wuu exec --ultra`; disabling the Subagent plugin removes
the delegation tool, prompts, state, and UI entry together.

## Migrating from legacy project configuration

If an old project keeps providers in `.wuu.json`:

1. Run `wuu init` to create a user configuration.
2. Move `default_provider`, `providers`, `instructions`, `agent.model_roles`,
   `agent.model_aliases`, and `agent.permission_mode` into the user configuration.
3. Keep in the project files the prompts and other project behavior that genuinely
   belong to the repository.

`WUU_HOME` moves the user configuration, authentication, sessions, memory, and log
directories as a whole. For example, with `WUU_HOME=/data/wuu` the user configuration
path becomes `/data/wuu/config.json`; this path still works even when the `HOME`
environment variable is not set.

## The shell on Windows

Bash syntax is the contract for command execution on every platform. On Windows, wuu
resolves Git Bash: it prefers the `WUU_GIT_BASH_PATH` environment variable; otherwise
it probes the standard install locations (`%ProgramFiles%\Git`,
`%ProgramFiles(x86)%\Git`, `%LOCALAPPDATA%\Programs\Git`), then derives `bash.exe`
from a `git.exe` on PATH. If all of that fails, command execution reports an error and
prompts to install Git for Windows. TTY-mode background processes are not available on
Windows and automatically fall back to pipe mode.

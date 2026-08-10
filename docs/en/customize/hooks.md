# Hooks

A Hook lets Wuu run a check or an automatic action when a lifecycle event such as a
tool call happens. It suits enforcing team rules, blocking dangerous commands,
recording tool activity, or supplementing the agent with check results.

Hooks execute commands or call models on Wuu's machine; they are not an
operating-system sandbox. Only configure Hooks you understand and trust; do not enable
unknown commands provided by a repository or a third party.

## Currently available scope

The current runtime fires the following events:

| Event | When | Main input | How the result is handled |
| --- | --- | --- | --- |
| `PreToolUse` | Before a tool executes | Tool name and arguments | Can allow, block, or replace tool arguments |
| `PermissionRequest` | Before tool permission judgment | Tool name and arguments | Can block tool execution |
| `PostToolUse` | After a tool succeeds | Tool name, arguments, and result text | Can add context for the agent; cannot undo completed operations |
| `PostToolUseFailure` | After a tool fails | Tool name, arguments, and error | For recording or notification; a Hook error does not override the original tool error |
| `PreCompact` | Before conversation compaction | Compaction reason | Can block this compaction |
| `PostCompact` | After the compaction implementation returns | Compaction reason and optional error | Can refuse to adopt the compaction result |
| `UserPromptSubmit` | Before a user prompt enters a model turn | Prompt text | Can block this turn |
| `SubagentStart` | Before a subagent turn starts | Subagent ID | Can block the subagent turn |
| `SubagentStop` | After a subagent turn ends | Subagent ID | A failure fails that subagent turn |
| `SessionStart` | After session binding completes | Session ID | A failure fails the session binding |
| `SessionEnd` | Before session resources close | Session ID | A failure returns with the cleanup error |
| `Stop` | When a model turn wraps up | Session ID | Can mark this turn as failed |
| `FileChanged` | After Wuu's file tools successfully write or edit a file | Absolute file path | For recording or triggering follow-up actions; the output currently does not change agent behavior |

`FileChanged` only tracks writes done through Wuu's file tools. When commands, external
programs, or users modify files directly, this event is not guaranteed to fire.

## Configuration location

The most direct way is to add `hooks` at the top level of the user configuration
`~/.wuu/config.json`. With `WUU_HOME` set, the user configuration lives at
`$WUU_HOME/config.json`.

```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "bash",
        "type": "command",
        "command": "python3 ~/.wuu/hooks/check-shell.py",
        "timeout": 10
      }
    ]
  }
}
```

Merge this into the existing file; do not delete your original model service and agent
configuration. After changing it, restart the current Wuu runtime; already-running
sessions do not re-read the file automatically.

In automation scenarios, `wuu exec --config <path>` loads the specified file as the
full configuration; `wuu exec --ignore-user-config` explicitly trusts the project
configuration. Both may enable the Hooks in the configuration, so only use them with
files you control. For the load order and trust boundary of a normal startup, see the
[configuration model](../reference/configuration.md).

Enabled plugins can also declare Hooks. A third-party plugin's Hooks carry the same
risk as running third-party local commands, so check the source, the commands, and the
authorization state first.

> The `hooks` field in Skill frontmatter currently does not register Hooks. See
> [writing and installing Skills](skill-authoring.md) for the available fields.

## Configuration fields

Each event maps to an array of Hooks, executed in configuration order. After the first
`PreToolUse` Hook that blocks or fails, the remaining Hooks and the target tool do not
execute.

| Field | Meaning |
| --- | --- |
| `matcher` | Matches the tool name. Empty or `*` matches everything; other values match the tool name case-insensitively and exactly, with no wildcard expressions |
| `type` | `command` or `prompt`; defaults to `command` |
| `command` | The shell command a `command` Hook runs |
| `prompt` | The judgment requirement for a `prompt` Hook; `$ARGUMENTS` inserts the event input JSON |
| `model` | The model a `prompt` Hook uses; empty uses the currently configured default tool model |
| `timeout` | Timeout in seconds for a single Hook; empty or `<= 0` means 30 seconds |

`matcher` mainly applies to the three tool events. Events without a tool name can only
use an empty matcher or `*`. The real tool names in the input can be seen in the Hook
logs; do not assume the UI name equals the internal name.

## Write a command Hook

A command Hook is started through a shell. Wuu writes a JSON object to the command's
standard input and reads an optional JSON object from standard output. The command
inherits the Wuu process's environment, but should not assume the process happens to
start in the workspace; when it needs the workspace path, read `cwd` from the input and
change directory explicitly.

The following script blocks `bash` calls containing `rm -rf`:

```python
#!/usr/bin/env python3
import json
import sys

event = json.load(sys.stdin)
tool_input = event.get("tool_input") or {}
command = tool_input.get("command", "")

if "rm -rf" in command:
    json.dump({
        "decision": "block",
        "reason": "Project rules do not allow running rm -rf through the agent"
    }, sys.stdout)
else:
    json.dump({}, sys.stdout)
```

For example, save it to `~/.wuu/hooks/check-shell.py` and use the `PreToolUse`
configuration above. The script does not need the executable bit, because the example
starts it with `python3`.

This is a minimal example to make the protocol understandable, not a complete command
security policy. Real rules should parse tool arguments instead of relying on simple
string matching to judge shell semantics.

## Input protocol

A command Hook receives the following structure on standard input. Besides the first
three common fields, Wuu only fills the fields relevant to the current event:

```json
{
  "hook_event_name": "PreToolUse",
  "session_id": "...",
  "cwd": "/path/to/workspace",
  "tool_name": "bash",
  "tool_input": {
    "command": "go test ./..."
  },
  "tool_response": "...",
  "error": "...",
  "prompt": "...",
  "file_path": "/path/to/workspace/file.go",
  "compact_reason": "proactive",
  "agent_id": "worker-id"
}
```

- `tool_input` is the raw JSON arguments of the target tool.
- `tool_response` is a stable textual projection of a successful result; it is not
  guaranteed to contain all internal data of rich-media results.
- `error` is only used for tool failure events.
- `file_path` is only used for `FileChanged`.
- `prompt` is only used for `UserPromptSubmit`.
- `compact_reason` is used for `PreCompact` and `PostCompact`.
- `agent_id` is used for `SubagentStart` and `SubagentStop`.
- Some execution paths may not fill `session_id`; do not treat a non-empty session ID
  as a precondition for Hooks to work.

## Output and exit codes

A Hook can write one JSON object to standard output:

```json
{
  "continue": true,
  "decision": "block",
  "reason": "explain why it was blocked",
  "updated_input": {
    "command": "go test ./internal/..."
  },
  "additional_context": "extra information for the agent"
}
```

Every field is optional:

- `decision: "block"` or `continue: false` means block;
- `reason` explains the block or judgment;
- `updated_input` replaces this tool call's arguments only in `PreToolUse` and must
  match the target tool's argument structure;
- `additional_context` is given to the agent after a successful `PostToolUse`;
- if you only need side effects, you can output nothing and exit with status 0.

Exit code 0 means continue; exit code 2 means block. When exit code 2 is used without
a `reason` in the JSON, Wuu prefers standard error as the reason. Any other non-zero
exit code counts as a Hook execution failure.

Only when the entire standard output is valid JSON is it parsed. Logs and debug
information should go to standard error so they are not mixed with the JSON on
standard output.

## Use a prompt Hook

A prompt Hook hands the event to a model for judgment, which suits soft rules that are
hard to express as a deterministic script:

```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "bash",
        "type": "prompt",
        "prompt": "Decide whether the following tool call could delete user data. Allow it only if you are sure it is safe: $ARGUMENTS",
        "timeout": 20
      }
    ]
  }
}
```

The model returns `ok` and a reason; `ok: false` blocks the operation and the reason is
also added as supplementary context. When the model request fails or returns an
unparseable result, the current implementation lets the operation through, so a prompt
Hook **cannot be the only security boundary**. Rules that must be enforced should use
deterministic command Hooks and the Wuu permission system.

A prompt Hook produces extra model requests, latency, and cost, and the event content
is also sent to the selected model service.

## Check and troubleshoot

When writing a Hook for the first time, start with a side-effect-free command Hook
that appends standard input to a temporary log you control, then trigger one clearly
scoped tool call. After confirming the fields and tool names, delete the logging Hook
so source code, command arguments, and tool results are not recorded long term.

Common problems:

- **The Hook does not run:** confirm the event name's case, that `matcher` uses the
  internal tool name, and restart the runtime.
- **All tools fail after configuration:** check whether the command exists, the Wuu
  process can read the script, and the script is not waiting for interactive input
  beyond the end of standard input.
- **The output has no effect:** make sure standard output contains only one valid JSON
  object, and logs go to standard error.
- **Manual modifications in the workspace do not trigger:** `FileChanged` is not a
  general file-system watcher.
- **The prompt Hook always allows:** check the current model service, model name, and
  the returned format; on model or parse failure the current policy does not block.

## Security boundary

- A command Hook runs with the Wuu process's local permissions and may read files,
  access the network, or modify system state;
- Hook input may contain prompts, source code, commands, paths, and tool results; do
  not upload or record it unintentionally or long term;
- Hook output affects the agent or tool arguments; treat it as untrusted input and
  guard against prompt injection;
- do not put API keys in Hook commands, project files, or logs;
- Hooks are an extended execution path outside the permission system; they do not
  automatically become read-only just because the agent is in read-only mode.

Before working with untrusted repositories or third-party extensions, continue with
[permission modes](../reference/permissions.md) and the [security
model](../reference/security-model.md).

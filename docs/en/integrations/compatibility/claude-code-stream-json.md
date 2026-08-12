# Protocol Comparison: `wuu exec --json` vs Claude Code `--output-format stream-json`

Status: reference / design note. This document only compares the two protocols and
sketches a compatibility mode. It does not change any code.

## Why this document exists

wuu is GUI-first. Its CLI (`wuu exec`) exists mainly so other agents and orchestration
frameworks can drive the same Go core the desktop app uses. A large fraction of the
existing harness ecosystem is written against Claude Code's (cc) `-p
--output-format stream-json` protocol, and LLM priors lean the same way. Before deciding
whether to add a cc-compatible output mode to wuu, we need a precise, code-sourced map of
how far apart the two event streams are.

All claims below cite code. Paths are relative to the repo root
(`<repo-root>`). The cc side is read from the vendored
source map under `thirdparty/claude-code-sourcemap/src` (a decompiled dump; treat it as
"best available", not upstream-authoritative). Anything not verifiable in code is marked
**(unconfirmed)**.

---

## 1. wuu side — `wuu exec --json`

### 1.1 Entry point and emission mechanism

- CLI flags are defined by `addExecFlags` in `cmd/wuu/main.go:1875-1906`. Relevant ones:
  - `--json` (bool) → `cmd/wuu/main.go:1898` ("emit machine-readable JSONL to stdout").
  - `--output-schema <schema.json>` → `cmd/wuu/main.go:1904`.
  - `--output-last-message <file>` → `cmd/wuu/main.go:1901`.
  - `--max-turns <n>` → `cmd/wuu/main.go:1903`.
  - Usage text: `cmd/wuu/main.go:2360-2508`.
  Line numbers drift as the file evolves; re-anchor with `grep -n addExecFlags
  cmd/wuu/main.go` when in doubt.
- The run loop is `exec.Run` in `internal/exec/runner.go:52-185`.
- Every JSONL line is written by `emitJSON` (`internal/exec/runner.go:1004-1010`):
  `json.NewEncoder(opts.Stdout).Encode(payload)` — one JSON object per line, newline
  delimited. **It is a no-op unless `opts.JSON` is true** (`runner.go:1005`), so in the
  default (non-JSON) mode none of these events are produced; stdout gets only the final
  message and stderr gets metadata (`runner.go:176-183`, and the non-JSON branches in
  `emitSessionConfigured`, `emitThreadEvent`, and `emitTurnStarted`, `runner.go:486-520`).
- Design contract (human-authored, not the emitter): `docs/en/automation/jsonl-events.md`, `docs/en/automation/exec.md`.
  Note: the **Required Event Families** section of `docs/en/automation/jsonl-events.md`
  lists the event family set; the list below records what the code **actually emits**
  today, including fields and conditional cases such as the narrowly scoped `error` event.

### 1.2 Identity model

wuu has **no `session_id`**. It identifies work with:

- `thread_id` — a persistent wuu session id, formatted like `20260618-120000-abcdef`
  (see the `thread_started` shape in `docs/en/automation/jsonl-events.md`;
  `appserver.Thread.ID`, `internal/appserver/protocol.go:1860`).
- `turn_id` — one agent turn within a thread (`appserver.Turn.ID`,
  `internal/appserver/protocol.go:1898`). One execution Run may contain multiple
  turn ids because of structured-output correction or other automatic continuation.
- `item_id` — one thread item (tool call, message, …) within a turn
  (`appserver.ThreadItem.ID`, `internal/appserver/protocol.go:1977`).

Almost every event carries `thread_id` and usually `turn_id`.

### 1.3 Event catalog (actual emit sites)

Every payload also has a `type` string. Fields listed are exactly the map keys passed to
`emitJSON`.

| Event `type` | Emit site | Fields |
|---|---|---|
| `session_configured` | `emitSessionConfigured` | `protocol_version`, `provider`, `model`, `effort`, `variant`, `max_parallel`, `workspace_root`, `permissions` |
| `thread_started` / `thread_resumed` / `thread_forked` | `emitThreadEvent`, `runner.go:507-513`; selected at `runner.go:120-127` | `thread_id`, `model`, `provider`, `cwd` |
| `turn_started` | `emitTurnStarted`, `runner.go:515-520` | `thread_id`, `turn_id` |
| `agent_message_delta` | `runner.go:312-320` | `thread_id`, `turn_id`, `delta` (token-level text, also accumulated into `finalMessage`) |
| `agent_message_final` | `runner.go:321-329` (from `AgentMessageReplace`) | `thread_id`, `turn_id`, `message` |
| `tool_started` | `emitItemStarted`, `runner.go:523-536` | `thread_id`, `turn_id`, `item_id`, `name`, `arguments` (a JSON **string**) |
| `tool_output_delta` | `runner.go:348-355` | `thread_id`, `turn_id`, `item_id`, `delta` |
| `tool_completed` | `emitItemCompleted`, `runner.go:539-560` | `thread_id`, `turn_id`, `item_id`, `name`, `status`, `error` |
| `command_started` | `runner.go:529-532` (only for command tools) | command payload (`runner.go:680-696`): `thread_id`, `turn_id`, `item_id`, `name`, `arguments`, plus `command` / `process_id` extracted from args |
| `command_output_delta` | `runner.go:564-571` | command payload + `delta` |
| `command_completed` | `runner.go:548-553` | command payload + `status`, `error` |
| `file_changed` | `runner.go:555-556` (`fileChangeEventsFromToolResult`, `runner.go:715+`) | `thread_id`, `turn_id`, `item_id`, `tool_name`, `path`, `action`, `old_file_sha`, `new_file_sha`, `workspace_revision`, plus tool-specific move/journal fields |
| `subagent_started` / `subagent_updated` / `subagent_completed` | `emitSubagentUpdated`, `runner.go:574-610` | `thread_id`, `agent_id`, `agent_type`, `status`, `task_name`, `agent_profile`, `agent_path`, `parent_id`, `description`, `result`, `result_path`, `result_bytes`, `result_truncated`, `error`, `input_tokens`, `output_tokens`, `cache_creation_tokens`, `cache_read_tokens` |
| `usage_updated` | `runner.go:362-367` | `thread_id`, `turn_id`, cumulative `input_tokens`, `output_tokens`, `cache_creation_tokens`, `cache_read_tokens` |
| `todo_updated` | `runner.go:859-863` | `thread_id`, `turn_id`, `todo` |
| `request_context` | `runner.go:863-905` | large diagnostic snapshot: `step_index`, byte/segment counts, hashes, `prompt_cache_key`, … |
| `provider_state` | `runner.go:906-934` | large transport snapshot: `provider`, `protocol`, `transport`, `replay_mode`, fallback state, input-item counts, … |
| `turn_completed` | `runner.go:374-392` | `thread_id`, `turn_id`, `input_tokens`, `output_tokens`, `trace_path`, `awaiting_auto_continuation` |
| `turn_failed` | `runner.go:441-468` | `thread_id`, `turn_id`, `error` |
| `turn_interrupted` | `runner.go:441-459`, `612-618` | `thread_id`, `turn_id`, `reason` (`"timeout"` \| `"interrupted"`) |
| `error` | `emitStructuredOutputValidation`, `runner.go:938-955` | `thread_id`, `turn_id`, `run_id`, `error`, `retrying` — **only a terminal CLI/app-server structured-output settlement mismatch**, not normal correction turns or a general error channel |
| `result` | `emitResult`, `runner.go:957-976` | `status`, `thread_id`, `turn_id`, `run_id`, `final_message`, `trace_path`, `error` (if any), `structured_result` (if `--output-schema` validated) |

### 1.4 First and last events

- **First after preflight succeeds:** `session_configured` (`runner.go:113`), then one of
  `thread_started`/`thread_resumed`/`thread_forked` (`runner.go:120-127`), then
  `turn_started` when the app-server accepts the first turn (`runner.go:300-311`). A
  preflight failure can instead produce a single terminal `result` with empty ids.
- **Last:** exactly one `result` event ends every run (the `emitResult`/`finishRunError`
  paths at `runner.go:957-1001`; contract in the **Rules** section of
  `docs/en/automation/jsonl-events.md`).

### 1.5 `result.status` values and error expression

- `status` ∈ `completed`, `failed`, `permission_denied`, `timeout`, `interrupted`
  (settled by `Run`, its `run/updated` handler, and `finishRunError`; documented under
  the `result` event in `docs/en/automation/jsonl-events.md`).
- Failures are expressed at several layers:
  1. Per-turn: `turn_failed` event (`runner.go:461`) then a `result` with the failing
     `status` and an `error` string.
  2. Setup failures and some Run-settlement failures produce only the terminal `result`;
     structured-output retry exhaustion is one such case.
  3. The optional `error` event only covers a final structured-output validation
     mismatch after the app-server reported successful Run settlement.
- There is **no** `total_cost_usd`, `usage` block, `num_turns`, or `duration_ms` on the
  `result`. Token usage lives only in separate `usage_updated` / `turn_completed` events.

### 1.6 Exit codes (granular)

Defined in `internal/execution/exitcodes.go:9-20` and re-exported by
`internal/exec/types.go:16-27` (documented in the **Exit Codes** section of
`docs/en/automation/exec.md`):

| Code | Const | Meaning |
|---|---|---|
| 0 | `ExitOK` | completed |
| 1 | `ExitTurnFailed` | agent turn failed |
| 2 | `ExitInvalidInput` | CLI/config/input validation failed |
| 3 | `ExitPermissionDenied` | permission denied / no approval handler |
| 4 | `ExitTimeout` | timeout |
| 5 | `ExitInterrupted` | interrupted / cancelled |
| 6 | `ExitProtocol` | app-server protocol error |
| 7 | `ExitProviderModelError` | provider/model error |
| 8 | `ExitToolFailed` | tool execution failed, unrecovered |
| 9 | `ExitConflict` | target thread already has a running turn |

Classification comes from the terminal execution Run and the setup/protocol/context
helpers in `internal/exec/runner.go`.

### 1.7 `--output-schema` behavior

Implemented across the CLI's schema loader and the app-server execution Run:

- `loadOutputSchema` reads and compiles a JSON Schema using Draft 2020
  (`internal/exec/structured_output.go:21-59`).
- The app-server compiles the same raw schema and prepends its instruction to the
  first prompt (`internal/appserver/run_handlers.go:56-70`;
  `internal/structuredoutput/validator.go:21-54`).
- A failed final answer creates up to two correction turns inside the **same** Run
  (`structuredoutput.MaxRetries = 2`; `run_handlers.go:303-317`, `385-434`). The
  preceding `turn_completed` has `awaiting_auto_continuation:true`; normal correction
  turns do not emit `error` events.
- Exhausting retries settles the Run as failed, producing `result.status:"failed"` and
  exit `ExitTurnFailed`.
- After a successful Run settlement, the CLI parses the final answer once more to
  populate `structured_result` (`runner.go:154-167`). A disagreement at that final
  boundary emits `error` with `retrying:false` before the failed `result`.

### 1.8 `--output-last-message` behavior

Writes the final agent message text to a file after a successful run
(`runner.go:169-173`, `writeLastMessage` at `runner.go:1061+`). It is **not** part of the
JSONL stream and does not add or change any event.

### 1.9 Session start / resume / fork are subcommands, not flags

wuu selects thread lifecycle via subcommands (see **Resume** in
`docs/en/automation/exec.md`), e.g.
`wuu exec resume --last`, `wuu exec resume <thread-id>`, `wuu exec fork <thread-id>`.
There is **no** `--session-id`, `--resume`, or `--output-format` flag on `wuu exec`.
Provider/model are chosen with `--provider` / `--model` / `--effort` / `--variant`
(`cmd/wuu/main.go:1840-1843`).

### 1.10 Permission flow

`wuu exec` makes allow-or-deny decisions without an interactive approval
exchange. It has no approval handler or socket, and its app-server client treats
server-initiated requests as protocol errors.

---

## 2. cc side — `claude -p --output-format stream-json`

### 2.1 Entry point and emission mechanism

- Headless entry: `runHeadless` in `cli/print.ts:455-974`.
- Requires print mode: `-p, --print` (`main.tsx:976`). Input must come from a prompt arg,
  stdin, or a resumable session (`cli/print.ts:779-785`).
- `stream-json` output **requires `--verbose`** in print mode; otherwise it errors and
  exits 1 (`cli/print.ts:787-793`).
- A stdout guard diverts stray non-JSON writes to stderr so the NDJSON stays clean
  (`installStreamJsonStdoutGuard`, `cli/print.ts:594-596`).
- In `stream-json` + `--verbose`, **every** `SDKMessage` produced by
  `runHeadlessStreaming` is written to stdout (`cli/print.ts:884-886`) via
  `structuredIO.write` → `ndjsonSafeStringify(message) + '\n'`
  (`cli/structuredIO.ts:465-467`).
- `--output-format` choices: `text` (default), `json`, `stream-json`
  (`main.tsx:976`). `text` prints only the final `result.result`; `json` prints the final
  result object (or the full message array with `--verbose`); `stream-json` streams
  everything (`cli/print.ts:917-957`).

### 2.2 Identity model

cc has a single `session_id` on **every** message (`getSessionId()`), and it is expected
to be a valid UUID — `--session-id <uuid>` "must be a valid UUID" (`main.tsx:1000`);
resume validity is checked with `validateUuid` (`cli/print.ts:774-776`). There is no
thread/turn/item split on the wire; turns are counted (`num_turns`) but not addressed.

### 2.3 SDKMessage union

Authoritative schema union: `entrypoints/sdk/coreSchemas.ts:1854-1881`. The members most
relevant to a headless stream:

**`system` / `init`** (first message) — schema `coreSchemas.ts:1457-1494`, built by
`buildSystemInitMessage` (`utils/messages/systemInit.ts:53-96`):
`type:"system"`, `subtype:"init"`, `cwd`, `session_id`, `tools[]` (names),
`mcp_servers[]` (`{name,status}`), `model`, `permissionMode`, `slash_commands[]`,
`apiKeySource`, `betas[]`, `claude_code_version`, `output_style`, `agents[]`, `skills[]`,
`plugins[]`, `uuid`, `fast_mode_state`.

**`assistant`** — schema `coreSchemas.ts:1347-1356`. Fields: `type:"assistant"`,
`message` (a full Anthropic **API assistant message**: `role`, `content[]` blocks,
`stop_reason`, `usage`, …), `parent_tool_use_id` (nullable), `error?`, `uuid`,
`session_id`. Yielded once per assistant message via `normalizeMessage`
(`QueryEngine.ts:761-770`). **Tool calls are `tool_use` content blocks inside this
message**, not separate events.

**`user`** — schema `coreSchemas.ts:1273-1295`. Fields: `type:"user"`, `message` (API user
message; carries `tool_result` blocks for tool outputs), `parent_tool_use_id`,
`isSynthetic?`, `tool_use_result?`, `priority?`, `timestamp?`, `uuid?`, `session_id?`.
User messages are emitted both for the prompt and for tool results (yields at
`QueryEngine.ts:739-748`, `881-891`). Replay variant adds `isReplay:true`
(`coreSchemas.ts:1297-1303`).

**`stream_event`** (partial assistant) — schema `coreSchemas.ts:1496-1504`. Fields:
`type:"stream_event"`, `event` (a raw Anthropic streaming event, e.g.
`content_block_delta`), `parent_tool_use_id`, `uuid`, `session_id`. **Only emitted with
`--include-partial-messages`** (`QueryEngine.ts:820-825`; flag `main.tsx:976`).

**`result`** (last message) — success schema `coreSchemas.ts:1407-1426`, error schema
`coreSchemas.ts:1428-1451`:

- Common: `type:"result"`, `subtype`, `duration_ms`, `duration_api_ms`, `is_error`,
  `num_turns`, `stop_reason` (nullable), `total_cost_usd`, `usage`, `modelUsage`
  (per-model map), `permission_denials[]`, `fast_mode_state?`, `uuid`, `session_id`.
- Success adds `result` (final text string) and `structured_output?`.
- Error adds `errors[]` (string array) instead of `result`.
- `subtype` values:
  - `success` (`QueryEngine.ts:618-638`, `1135-1155`)
  - `error_during_execution` (`QueryEngine.ts:1082-1117`)
  - `error_max_turns` (`QueryEngine.ts:851-873`)
  - `error_max_budget_usd` (`QueryEngine.ts:981-1001`)
  - `error_max_structured_output_retries` (`QueryEngine.ts:1024-1046`)
- `num_turns` = count of user messages in the run (`QueryEngine.ts:753-755`).

**Other `system` subtypes and side-channel messages** (all in the union at
`coreSchemas.ts:1854-1881`, most gated behind flags/features): `compact_boundary`,
`status`, `post_turn_summary`, `api_retry`, `local_command_output`, `hook_started` /
`hook_progress` / `hook_response` (only with `--include-hook-events`,
`cli/print.ts:628-674`), `tool_progress`, `auth_status`, `task_notification` /
`task_started` / `task_progress`, `session_state_changed`, files-persisted,
`tool_use_summary`, `rate_limit_event`, `elicitation_complete`, `prompt_suggestion`.

**Control protocol** (not `SDKMessage`, but interleaved on the same stdio when
`--input-format stream-json`): `control_request` / `control_response` /
`control_cancel_request` carry `can_use_tool` permission prompts, `hook_callback`,
`mcp_message`, and `elicitation` (`cli/structuredIO.ts:333-531`). Permission prompts are
**requests to the client**, answered on stdin — a fundamentally different model from
wuu's non-interactive allow-or-deny decisions.

### 2.4 Exit codes (binary)

`runHeadless` ends with `gracefulShutdownSync(result.is_error ? 1 : 0)`
(`cli/print.ts:971-973`; `gracefulShutdownSync` sets `process.exitCode`,
`utils/gracefulShutdown.ts:336-347`). Startup/validation failures call
`gracefulShutdownSync(1)` (e.g. `cli/print.ts:569,576,584,608,749,761,783,791`).
So cc exit codes are effectively **0 = success, 1 = any error** — no granular taxonomy.

### 2.5 Headless flags (from `main.tsx:976-1000`)

| Flag | Notes |
|---|---|
| `-p, --print` | headless mode (required for all below) |
| `--output-format <text\|json\|stream-json>` | `main.tsx:976` |
| `--input-format <text\|stream-json>` | `main.tsx:976`; stream-json input requires stream-json output (`main.tsx:1825`) |
| `--include-partial-messages` | needs `--print` + stream-json (`main.tsx:1850`); enables `stream_event` |
| `--include-hook-events` | needs stream-json; enables hook `system` messages |
| `--verbose` | required for stream-json in print mode |
| `--max-turns <n>` | early-exits with `error_max_turns` |
| `--max-budget-usd <amount>` | `error_max_budget_usd` |
| `--json-schema <schema>` | structured output validation |
| `--permission-mode <mode>` | choices `PERMISSION_MODES` |
| `--permission-prompt-tool <tool>` | MCP tool for permission prompts |
| `--allowedTools` / `--allowed-tools <tools...>`, `--disallowedTools`, `--tools` | tool policy |
| `--system-prompt[-file]`, `--append-system-prompt[-file]` | |
| `-c, --continue`, `-r, --resume [value]`, `--fork-session`, `--resume-session-at` | session lifecycle |
| `--session-id <uuid>`, `-n, --name` | must be a valid UUID |
| `--agents <json>`, `--agent <agent>`, `--fallback-model`, `--betas` | |
| `--dangerously-skip-permissions` | bypass all permission checks |
| `--replay-user-messages` | re-emit stdin user messages (stream-json both ways) |
| `--mcp-config`, `--strict-mcp-config`, `--add-dir`, `--settings`, `--setting-sources` | |

---

## 3. Field-by-field mapping

Legend: **=** direct map · **~** needs transform/synthesis · **wuu∅** wuu has no source ·
**cc∅** cc has no equivalent.

### 3.1 Lifecycle / envelope

| Concept | wuu | cc | Rel. | Notes |
|---|---|---|---|---|
| Session identity | `thread_id` (date-string) | `session_id` (UUID) | ~ | Different id spaces; cc consumers may assume UUID (`main.tsx:1000`). See §5. |
| Turn identity | `turn_id` | none on wire; `num_turns` counter | ~ | wuu addresses turns; cc only counts. |
| Item identity | `item_id` | `tool_use`/`tool_result` block ids inside messages | ~ | Different granularity. |
| First event | `session_configured` + `thread_started` | `system`/`init` | ~ | Fields overlap partially — see §3.2. |
| Last event | `result` | `result` | ~ | Field sets differ hugely — see §3.4. |
| Per-message stamp | `thread_id`(+`turn_id`) on each | `session_id` (+`uuid`) on each | ~ | cc adds a per-message `uuid`; wuu has none. |

### 3.2 Init / configuration

| cc `system/init` field | wuu source | Rel. |
|---|---|---|
| `cwd` | `thread_started.cwd` (`runner.go:509`) / `session_configured.workspace_root` | = |
| `model` | `session_configured.model` (`runner.go:492`) | = |
| `permissionMode` | `session_configured.permissions` (`runner.go:497`) | ~ (shape differs; wuu carries a `PermissionSummary`) |
| `tools[]` | none emitted at init | wuu∅ (tools appear only when used via `tool_started`) |
| `slash_commands[]`, `agents[]`, `skills[]`, `plugins[]`, `mcp_servers[]` | none at init | wuu∅ |
| `apiKeySource`, `betas`, `claude_code_version`, `output_style`, `fast_mode_state` | none | wuu∅ (cc-specific) |
| `session_id`, `uuid` | `thread_id` / none | ~ / wuu∅ |
| — | `session_configured.provider`, `effort`, `variant`, `max_parallel`, `protocol_version` | cc∅ |

### 3.3 Streaming content

| Concept | wuu | cc | Rel. |
|---|---|---|---|
| Assistant text (incremental) | `agent_message_delta.delta` (`runner.go:319`) | `stream_event` w/ `content_block_delta` (`--include-partial-messages`) | ~ |
| Assistant text (final/whole) | `agent_message_final.message` (`runner.go:328`) | `assistant.message.content[]` text blocks | ~ (cc ships a full API message; wuu ships plain text) |
| Reasoning / thinking | intentionally omitted from the automation stream | `assistant.message.content[]` `thinking` blocks (+ partial `stream_event`) | wuu∅ |
| Tool call start | `tool_started` (`runner.go:528`) | `tool_use` block inside `assistant.message.content[]` | ~ (event vs content block) |
| Tool output (incremental) | `tool_output_delta` (`runner.go:354`) | none as text; final result only | ~ / cc∅ streaming |
| Tool result (final) | `tool_completed` (`runner.go:546`) | `tool_result` block inside a `user` message | ~ |
| Command tools | `command_*` events (`runner.go:529-571`) | folded into the same `tool_use`/`tool_result` blocks | ~ (wuu splits them out) |
| File change | `file_changed` (`runner.go:555-556`) | none (implicit in tool result) | cc∅ |
| TODO | `todo_updated` (`runner.go:862`) | none (TodoWrite tool result) | cc∅ |
| Subagents | `subagent_*` (`runner.go:574-610`) | `parent_tool_use_id` threading on `assistant`/`user` msgs | ~ |
| User prompt echo | none emitted | `user` message | wuu∅ |
| Provider/request diagnostics | `provider_state`, `request_context` (`runner.go:863-934`) | none | cc∅ |
| Token usage (cumulative turn snapshot) | `usage_updated` (`runner.go:367`) | none per-message; only in final `result.usage` | ~ |
| Rate limits | none | `rate_limit_event` | cc∅ |

### 3.4 Result

| cc `result` field | wuu source | Rel. |
|---|---|---|
| `subtype` (`success` / `error_*`) | `result.status` (`runner.go:963`) | ~ (see mapping table §5.2) |
| `is_error` | derive from `status != "completed"` | ~ |
| `result` (final text) | `result.final_message` (`runner.go:967`) | = |
| `structured_output` | `result.structured_result` (`runner.go:974`) | = |
| `errors[]` | `result.error` string (`runner.go:971`) | ~ (string → array) |
| `stop_reason` | not on wuu `result` (available on `TurnCompletedNotification.StopReason`, `protocol.go:1760`, but not emitted) | wuu∅ on result |
| `num_turns` | derivable by counting `turn_started` | ~ |
| `duration_ms` / `duration_api_ms` | wall duration exists on app-server `Turn.DurationMS` but is not emitted to JSONL; no API-time split | ~ / wuu∅ |
| `usage` (aggregate) | sum the latest cumulative `usage_updated` snapshot per turn; use `turn_completed` input/output as the final fallback | ~ |
| `modelUsage` (per-model map w/ cost) | no per-model cost tracking | wuu∅ |
| **`total_cost_usd`** | **not computed anywhere in wuu** | **wuu∅ — hard blocker, see §5.3** |
| `permission_denials[]` | no direct event field | wuu∅ |
| `session_id`, `uuid` | `thread_id` / none | ~ / wuu∅ |
| `fast_mode_state`, `apiKeySource` | none | wuu∅ (cc-specific) |

### 3.5 Errors and permissions

| Concept | wuu | cc | Rel. |
|---|---|---|---|
| Turn failure | `turn_failed` + `result(status:"failed")` | `result(subtype:"error_during_execution")` | ~ |
| Timeout | `turn_interrupted(reason:"timeout")` + `result(status:"timeout")`, exit 4 | no dedicated subtype/exit | cc∅ |
| Interrupt/cancel | `turn_interrupted(reason:"interrupted")` + `result(status:"interrupted")`, exit 5 | SIGINT → graceful shutdown, exit 0 (`runHeadlessStreaming` `sigintHandler`, `cli/print.ts:1027-1034`) | ~ |
| Max turns | generic turn/Run failure after the configured step cap; no distinct result subtype | `result(subtype:"error_max_turns")` | ~ |
| Permission request | none; exec makes allow-or-deny decisions without an interactive approval step | `control_request(can_use_tool)`; answered by client on stdin | wuu∅ |
| Permission denied | `result(status:"permission_denied")`, exit 3 | recorded in `result.permission_denials[]`; **not** a terminal error subtype | ~ |
| Schema retry failure | correction `turn_completed` events (`awaiting_auto_continuation:true`) + final `result(status:"failed")`; no retry `error` events | `result(subtype:"error_max_structured_output_retries")` | ~ |

---

## 4. Flag comparison (cc headless ↔ wuu exec)

| cc flag | wuu equivalent | Rel. |
|---|---|---|
| `-p, --print` | implicit (`wuu exec` is always headless) | ~ |
| `--output-format stream-json` | **none** — wuu only has `--json` (its own JSONL) | wuu∅ (this doc's whole point) |
| `--output-format json` | none (wuu has no single-object mode) | wuu∅ |
| `--output-format text` | default `wuu exec` mode (final message on stdout) | ~ |
| `--verbose` | n/a (wuu always emits full stream in `--json`) | — |
| `--input-format stream-json` | `--input-json` reads one input object (`cmd/wuu/main.go:1858`; see **Machine Input** in `docs/en/automation/exec.md`) — **not** a streaming JSONL input channel | ~ |
| `--max-turns <n>` | `--max-turns <n>` (`cmd/wuu/main.go:1859`) | = |
| `--max-budget-usd` | none (no cost tracking) | wuu∅ |
| `--json-schema <schema>` | `--output-schema <schema.json>` (`cmd/wuu/main.go:1860`) | ~ (file vs inline; wuu injects the schema and keeps correction turns in one Run) |
| `--permission-mode <mode>` | `--permission-mode standard|read_only|unconfined` (`cmd/wuu/main.go:1844`, `1873-1879`) | ~ (different vocabulary) |
| `--allowedTools` | none | wuu∅ |
| `--disallowedTools` | none | wuu∅ |
| `--permission-prompt-tool` | none | wuu∅ |
| `--dangerously-skip-permissions` | closest: `--permission-mode unconfined` | ~ |
| `-r, --resume [id]` | `wuu exec resume <id>` / `resume --last` subcommand | ~ |
| `-c, --continue` | `wuu exec resume --last` | ~ |
| `--fork-session` | `wuu exec fork <id>` subcommand | ~ |
| `--session-id <uuid>` | none — wuu allocates its own `thread_id`, no pre-set id | wuu∅ |
| `--model` | `--model` (`cmd/wuu/main.go:1841`) | = |
| `--fallback-model` | none | wuu∅ |
| `--system-prompt` / `--append-system-prompt` | none on `wuu exec` (instruction/config-driven) | wuu∅ |
| `--add-dir` | `--workdir` (single root) (`cmd/wuu/main.go:1845`) | ~ |
| `--mcp-config` | `--config <path>` can load a full trusted Wuu config containing MCP servers; no MCP-only flag | ~ |
| `--replay-user-messages` | none | wuu∅ |
| — | `--output-last-message <file>` | cc∅ (cc has no direct equivalent) |
| — | `--effort` / `--variant` / `--profile` / `--provider` | cc∅ (wuu multi-provider knobs) |
| — | `--file` / `--image` / `--input-json` | ~ (cc uses content blocks on stdin) |

---

## 5. Compatibility-mode proposal: `wuu exec --output-format cc-stream-json`

**Not implemented — design only.** The idea: add an *additive*, opt-in translation layer
that consumes wuu's existing app-server notification stream and emits cc-shaped
`SDKMessage` NDJSON, so cc harnesses can drive wuu unchanged. This would be a new writer
alongside `emitJSON` (`runner.go:1004`), keyed off a new `--output-format` value; the native
`--json` stream stays as-is.

### 5.1 What can be synthesized cleanly

| cc message | Built from | How |
|---|---|---|
| `system/init` | `session_configured` + `thread_started` (`runner.go:113,126`) | Map `cwd`, `model`, `permissionMode`. Emit `session_id = thread_id`. Fill `tools[]`, `slash_commands[]`, `mcp_servers[]`, `agents[]` with empty arrays or best-effort (wuu doesn't list these at init — see §5.4). Constant `claude_code_version` placeholder, `apiKeySource:"none"`, `betas:[]`. |
| `user` (prompt echo) | the `--json`-invisible prompt (wuu `TurnInput.Prompt`, `runner.go:129`) | Synthesize one `user` message with `message.content` = prompt text at `turn_started`. wuu currently emits nothing here. |
| `assistant` | buffer `agent_message_delta` (`runner.go:319`) until `turn_completed`/`agent_message_final`; fold `tool_started` (`runner.go:528`) into `tool_use` content blocks | Reassemble an Anthropic-shaped `message.content[]`. Thinking blocks cannot be synthesized because wuu deliberately omits provider reasoning at the automation boundary. Hard part below (§5.5). |
| `user` (tool results) | `tool_completed`/`command_completed` (`runner.go:546-552`) | Emit a `user` message with a `tool_result` block referencing the `tool_use` id. Requires stable id mapping wuu `item_id` → cc `tool_use_id`. |
| `stream_event` | `agent_message_delta` | Only if the compat mode also opts into partial messages; wrap each text delta as a `content_block_delta` raw event. |
| `result.subtype` + `result.result` + `is_error` | `result` (`runner.go:963,967`) | Map status→subtype (§5.2); `result.result = final_message`; `is_error = status != "completed"`. |
| `result.structured_output` | `result.structured_result` (`runner.go:974`) | direct. |
| `result.num_turns` | count emitted `turn_started` | derivable. |
| `result.usage` | keep the latest cumulative `usage_updated` snapshot per turn (`runner.go:367`), with `turn_completed` (`runner.go:379`) as the final input/output fallback | derivable; sum once per turn and map the token/cache fields to cc's `usage` shape. |
| `result.permission_denials[]` | no equivalent exec event | not derivable without adding a new event contract. |
| `result.errors[]` | `result.error` string (`runner.go:971`) | wrap as single-element array. |

### 5.2 Status → subtype mapping (proposed)

| wuu `result.status` | cc `result.subtype` | Caveat |
|---|---|---|
| `completed` | `success` | clean |
| `failed` | `error_during_execution` | clean-ish |
| `permission_denied` | `error_during_execution` | cc has no permission subtype; lossy. Also record in `permission_denials[]`. |
| `timeout` | `error_during_execution` | **no cc timeout subtype** — information lost unless smuggled into `errors[]` |
| `interrupted` | `error_during_execution` | same; cc's own interrupt path exits 0, so semantics diverge |
| (max-turns cap) | `error_max_turns` | wuu doesn't emit a distinct status for the `--max-turns` cap today; would need a new signal from the runner |

### 5.3 Hard blockers (cannot be done faithfully)

1. **`total_cost_usd` (and `modelUsage[*].costUSD`).** wuu core computes **no cost**. There
   is no cost tracker; the `result` event (`runner.go:957-976`) has no cost field, and no
   `usage_updated`/`turn_completed` field carries money (`protocol.go:1732-1764`). cc's
   value comes from `getTotalCost()` (`QueryEngine.ts:628`). Options, all lossy: emit
   `total_cost_usd: 0` (misleads cost-gating harnesses), or compute a client-side estimate
   from token counts × a hardcoded price table (fragile, provider-specific, wrong for
   non-Anthropic models). **Recommend emitting `0` and documenting it as unsupported.**

2. **`session_id` UUID semantics.** wuu `thread_id` is `YYYYMMDD-HHMMSS-suffix`
   (see `thread_started` in `docs/en/automation/jsonl-events.md`), **not a UUID**. cc consumers that validate the id
   (`validateUuid`, `cli/print.ts:774`) or feed it back to `--session-id`/`--resume` (which
   require a UUID, `main.tsx:1000`) will reject or mishandle it. A synthetic UUID could be
   minted per run, but then it no longer round-trips to `wuu exec resume <thread-id>`, so
   resume-by-session-id compat breaks either way. **No lossless choice.**

3. **`stop_reason`.** Present in wuu core (`TurnCompletedNotification.StopReason`,
   `protocol.go:1760`) but **not emitted** on any JSONL event, so the compat layer can't see
   it without a runner change; it would emit `null`. Low stakes but note it.

4. **`duration_api_ms`.** wuu persists wall time on `Turn.DurationMS`
   (`internal/appserver/protocol.go:1914`) but does not expose it in the exec notification
   stream. A compat writer would need an additional app-server lookup or signal to
   populate `duration_ms`; there is no API-time source, so `duration_api_ms` would be a
   fabricated `0`.

5. **Assistant-message content fidelity for non-Anthropic providers (see §5.5).**

### 5.4 Medium-difficulty gaps

- **`system/init` tool/command inventory.** cc lists `tools[]`, `slash_commands[]`,
  `mcp_servers[]`, `agents[]`, `skills[]` at init (`systemInit.ts:62-84`). wuu's
  `session_configured` doesn't (`runner.go:486-498`). The data exists in app-server
  (`InitializeResult` has `ToolSurface`, `protocol.go:258`), but the exec runner doesn't
  emit it. Filling these requires either threading more of `InitializeResult` into the
  compat writer or accepting empty arrays (some harnesses render pickers from these).
- **Permission model translation.** cc expects the client to answer `can_use_tool`
  `control_request` on **stdin** (`structuredIO.ts:533-659`). Wuu exec has no
  interactive approval exchange. Supporting this would be a separate input and
  permission-protocol project, not an output translation detail.

### 5.5 The assistant-message reassembly problem

cc's core streaming unit is a **full Anthropic API assistant message** with a typed
`content[]` of `text` / `thinking` / `tool_use` blocks (`coreSchemas.ts:1347-1356`), and
tool results come back as `tool_result` blocks in a following `user` message. wuu instead
emits a flat sequence of token deltas plus separate `tool_started`/`tool_completed` events
keyed by `item_id`. To synthesize cc's shape the compat writer must:

- buffer text deltas into text blocks; thinking blocks cannot be reconstructed because
  wuu omits provider reasoning from its automation stream,
- convert each `tool_started` into a `tool_use` block with a generated/`item_id`-derived
  `id` and parse `arguments` (a JSON **string**, `runner.go:528`) back into an object,
- pair each `tool_completed` with a `tool_result` user message referencing that id.

The text and tool-call conversion is mechanical for Anthropic-backed runs, but thinking
content remains unavailable. For **non-Anthropic providers** (wuu is
multi-provider — `--provider openai`, `gpt-5`, etc., `cmd/wuu/main.go:1840`; see
`provider_state.provider`, `runner.go:916`) the reconstructed `message` is only *shaped
like* an Anthropic message; block types, id formats, and reasoning representations won't
match what a real cc-on-Anthropic run produces. Harnesses that inspect
`assistant.message` internals (not just `.content[].text`) may break. **Recommend:
guarantee `result.result` text + tool_use/tool_result *structure*, but explicitly document
that `message` block fidelity is best-effort and provider-dependent.**

### 5.6 Exit-code strategy

cc callers expect 0/1 (`cli/print.ts:971`). wuu's granular 0-9 (`types.go:16-27`) already
collapses safely — any nonzero reads as "error" to a cc consumer. Recommendation: **keep
wuu's granular exit codes** even in compat mode (a cc harness only checks zero/nonzero), or
add a `--cc-exit-codes` toggle to squash to 0/1 if a strict consumer needs it. Document the
choice; do not silently change wuu's codes.

### 5.7 Summary recommendation

An additive `--output-format cc-stream-json` is **feasible for the common harness path**
(system/init → user → assistant(+tool_use) → user(tool_result) → result.success with text
and usage). The unavoidable lossy points to advertise up front:

1. `total_cost_usd` / `modelUsage` cost — **not available; emit 0**.
2. `session_id` — non-UUID `thread_id` or a non-resumable synthetic UUID; can't be both
   valid-looking and round-trippable.
3. `timeout` / `interrupted` / `permission_denied` — no matching cc `result.subtype`;
   folded into `error_during_execution` (+ `errors[]`).
4. `assistant.message` block fidelity — best-effort, and weakest for non-Anthropic
   providers.
5. Incoming permission prompts over stdin (`can_use_tool` control protocol) — unsupported;
   adding them would be a separate permission-protocol project.

---

## Appendix: primary sources

- wuu emitters: `internal/exec/runner.go` (esp. `300-468`, `486-618`, `859-1010`).
- wuu options / exit codes: `internal/exec/types.go:16-131` and
  `internal/execution/exitcodes.go`.
- wuu structured output: `internal/exec/structured_output.go`,
  `internal/structuredoutput/validator.go`, and `internal/appserver/run_handlers.go`.
- wuu app-server types: `internal/appserver/protocol.go` (`InitializeResult`,
  `TurnUsageNotification`, `TurnCompletedNotification`, `Thread`, `Turn`, and `ThreadItem`).
- wuu CLI flags/usage: `cmd/wuu/main.go:1875-1906`, `2360-2508`.
- wuu contract docs: `docs/en/automation/exec.md`, `docs/en/automation/jsonl-events.md`.
- cc print/headless: `thirdparty/claude-code-sourcemap/src/cli/print.ts` (`455-974`,
  `1027-1079`).
- cc structured IO / control protocol:
  `thirdparty/claude-code-sourcemap/src/cli/structuredIO.ts`.
- cc SDK message schemas:
  `thirdparty/claude-code-sourcemap/src/entrypoints/sdk/coreSchemas.ts` (`1270-1504`,
  `1854-1889`).
- cc init builder:
  `thirdparty/claude-code-sourcemap/src/utils/messages/systemInit.ts`.
- cc result construction:
  `thirdparty/claude-code-sourcemap/src/QueryEngine.ts` (`600-638`, `818-1156`).
- cc CLI flags: `thirdparty/claude-code-sourcemap/src/main.tsx:976-1000` (and validation at
  `1279-1367`, `1825-1850`).
- cc exit: `thirdparty/claude-code-sourcemap/src/utils/gracefulShutdown.ts:336-347`.

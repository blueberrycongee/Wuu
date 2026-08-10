# App-server integration basics

The app-server is the protocol boundary between the Wuu core and the desktop, scripts,
or editor shells. When building a new client, reuse this protocol instead of
reimplementing the agent loop in the shell.

## Transport format

The current protocol transports **line-delimited JSON (JSONL)** over standard input
and output. Every request carries an `id`, a `method`, and optional `params`:

```json
{"id":"1","method":"initialize","params":{}}
```

A successful response uses the same `id`:

```json
{"id":"1","result":{}}
```

An error response contains `error`; notification messages have no `id`, for example:

```json
{"method":"turn/completed","params":{}}
```

`initialize` returns the current protocol version, `wuu-app-server/v0.1`. This is a
controlled integration protocol whose fields may still evolve; clients should handle
messages by method and event type, not by relying on natural-language error text.

## Lifecycle of a task

A client should drive the core in this order:

1. `initialize`: establish the connection and obtain capabilities, configuration, and
   the protocol version;
2. `thread/start` or `thread/resume`: create or resume a session;
3. `turn/start`: interactive clients such as the desktop start a single-turn task, or
   use `run/start` to start the automated run that `wuu exec` uses;
4. consume `turn/*` notifications and wait for `run/updated` to reach a terminal state
   (automation runs);
5. `shutdown`: request a clean shutdown when the client exits.

`thread/start` creates a persistent session by default; a session passed with
`{"ephemeral": true}` exists only in memory and cannot be resumed after the server
exits. `thread/fork` can create a new branch from an existing session, turn, or entry.

## Common methods

| Method | Purpose |
| --- | --- |
| `thread/start` | Create a session |
| `thread/resume` | Resume a session; an empty session ID means the most recent visible session |
| `thread/fork` | Create a branch from an existing session |
| `turn/start` | Start an interactive single turn, which may include attachments |
| `run/start` | Start an automation run, used by `wuu exec` |
| `turn/interrupt` | Interrupt a single-turn task |
| `run/interrupt` | Interrupt an automation run |
| `shutdown` | Ask the server to shut down |

Model and permission mode are session choices. To change them, call
`config/model/update` first rather than overriding them temporarily in a single-turn
request; a running session cannot change the model or permission mode already adopted
for this turn.

## Local debugging

The repository provides CLI debug entry points that start a local server and send a
single protocol request:

```bash
wuu debug app-server initialize --workdir /path/to/project
wuu debug app-server send thread/start '{}'
```

Production authentication, sandboxing, organization membership, secret injection, and
quotas are handled by an external control plane; they are not capabilities the
app-server itself provides. See the [app-server protocol
documentation](../integrations/app-server-protocol.md) for the complete methods and
parameter reference.

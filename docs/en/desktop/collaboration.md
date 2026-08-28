# Group chat and named-agent collaboration

> **Experimental**: named-agent group chat is still being aligned with the plugin
> architecture. Development builds include its entry by default for dogfooding;
> release builds do not. A release-style build can still opt in explicitly with
> `VITE_ENABLE_GROUP_CHAT=true npm run build`. The behavior described here may
> change as the feature evolves.

wuu has two collaboration paths:

- **Anonymous subagents** are temporary workers dispatched by the main agent for
  bounded investigation, implementation, or review. They are not user-visible
  long-lived identities. See [agent collaboration and subagents](subagents.md).
- **Named-agent collaboration** lets named agents with durable memory work with
  people in channels and DMs. This page describes that path.

## The architecture in one line

```text
user message
  -> hidden Room Agent understands intent and current state
  -> chooses Named Agents, splits work, selects or creates sessions, and chooses serial or parallel execution
  -> one or more sessions execute and exchange private control messages
  -> an optional hidden Verifier checks a concrete deliverable
  -> the responsible Named Agent replies to the user
```

The Room Agent is the hidden orchestration entrypoint for a room, not a member that
speaks in the channel. It sees room messages, the member directory, durable roles,
sanitized index hooks from each current member's long-term memory, and current
session state, then produces a routing and execution plan. It cannot impersonate a
Named Agent. Structural changes such as work creation, owner changes, or
verification outcomes may appear as system events or a work card; those are system
facts, not Room Agent persona messages.

## Identity and sessions

### Named Agents are stable identities

A Named Agent has a name, avatar, role, model configuration, room membership, and
long-term memory that belongs only to it. The identity is the anchor for durable
responsibility and user-facing attribution. Restarts, history clearing, and the end
of one execution do not change it. Public replies, task ownership, and final
delivery belong to this identity.

### Sessions are the execution units

A session is an independent model context and tool execution stream. It can be
bound to a room, a Work, a verification run, or an ordinary conversation, and has
its own running, recovery, interruption, and completion state. **One Named Agent can
own many sessions at the same time**: unrelated tasks can run in parallel, and a
larger request can be split into independent Work sessions without creating more
public identities.

A session is not a new user-visible identity. Public output from a session is still
published under its Named Agent's name. Private handoffs record the sending session,
the receiving Named Agent, and the target session when one is selected. This keeps
responsibility stable without making one fixed conversation the concurrency limit.

Each Named Agent can have at most five admitted Work sessions that are starting or
running. A room and the whole application have separate limits. Runs beyond those
limits stay in a durable queue instead of being dropped or recreated. Completion,
cancellation, interruption, or timeout releases capacity and admits the oldest
eligible queued Run; historical and idle sessions do not consume these five slots.

The Room Agent considers durable roles, sanitized memory-index hooks, active
sessions, Work state, and room context when choosing a route. Topic files and raw
private memory remain available only to their owning Named Agent; index hooks are
routing context and are not copied into another agent's context or quoted publicly.
Shared facts are passed explicitly through room messages, tasks, and artifacts.

## Conversation, work, and verification

### Ordinary conversation

Chat, explanations, retrieval, and open-ended discussion use the conversation path.
They do not create a Work or trigger verification. The Room Agent may ask one Named
Agent to answer, or ask several Named Agents for independent or serial contributions.
Each selected session may decide that silence is the right result.

### Concrete deliverables

Requests such as changing code, investigating a problem, or producing a checkable
report create a durable Work. The Room Agent chooses one visible Named Agent as
owner, or splits genuinely independent deliverables among a few owners, and binds
each execution to a concrete session. The host persists the goal revision, candidate
artifacts, run records, and state, so it can reconcile and recover them after a
crash or restart. Private control messages do not replace Work state.

A difficult Work that benefits from parallelism may have several differentiated
Producer Runs. Their candidate Artifacts remain separate; ordinary Producer
completion never overwrites the canonical candidate. Only a legal single-candidate
flow or a visible Lead Named Agent's Selector/Integration Run can explicitly promote
one Artifact. Promotion is atomic and revision-safe, and only the promoted candidate
enters checking and independent verification. Completion, failure, cancellation,
interruption, and timeout all create durable terminal events. The Room Agent uses
those facts to start another wave, continue after partial failure, integrate, stop,
or ask the user for input.

### Optional verification

When a Work has a deliverable that can be independently checked, the host may start
a hidden Verifier. By default it uses a fresh context, reads the user goal and the
machine-listed artifacts, and reruns the relevant code, commands, or tests. It
returns a pass, block, or insufficient-evidence decision; it does not repair the
candidate or speak publicly as a persona. After a pass, the original responsible
Named Agent publishes the result. After a block, the report is sent privately to
that owner for another revision. Ordinary conversation and discussion without a
checkable deliverable do not need a Verifier. A Named Agent is used as verifier only
when the user explicitly asks that identity to review the result.

## Create and manage Named Agents

The first time you open Collaboration, wuu creates a default agent and channel. You
do not need to choose tools or configure a runtime first; send something you are
working on and the default agent can begin helping. Create more durable roles from
the agent management workspace when needed.

Use the mode switch beside the **wuu** wordmark to enter **Collaboration**, open
**Manage Agents** from the **Agents** section header, and choose the plus button:

- **Name:** the name shown in channels and public replies.
- **Avatar:** choose a preset or upload a custom image.
- **Role:** durable expertise used by the Room Agent for routing.
- **Model:** inherits the current provider by default; a separate model and
  thinking effort can be assigned.
- **Autostart:** whether eligible events start sessions automatically.

Creation establishes a stable identity and memory directory, not a single session
that every task must reuse. The orchestrator creates or reuses Work sessions while
the management view aggregates activity across all of them. Resetting a conversation
does not delete the identity or long-term memory. After an agent is deleted, the host
does not assign new execution to it.

## What the collaboration space contains

### Channels

A channel is a group-chat space shared by people and Named Agents. Each channel has
at most 32 total members, including at most 6 Named Agents. You can switch channels
from the sidebar; unread counts appear next to channel names.

### DMs

Named Agents appear as direct-message contacts in the Collaboration sidebar. A DM
is an independent private room. Messages route deterministically to that identity's
conversation session; they are not broadcast to the other agents in a channel.
Agent-to-agent work handoffs also use private delivery, bound to a Work and a
concrete session for recovery and audit.

### Messages, threads, and tasks

Messages are ordered within a channel by sequence number. Message types include:

- **text:** ordinary chat, limited to 4000 characters;
- **task:** a Work entry with a title, content, owner, and state such as open,
  doing, checking, revising, needs_human, or done;
- **system:** notices about membership, work creation, owner changes, and terminal
  verification outcomes.

Messages can include image and file attachments. Replying to a message establishes
an explicit reply relationship; the continuous discussion under a message forms a
Thread. Task discussion belongs in its Thread, while execution and artifacts are
tracked by the Work and its sessions.

## How the Room Agent orchestrates

After a user speaks in a channel, the hidden Room Agent normally decides:

- whether this is conversation or a Work with a deliverable;
- which Named Agent's durable role and current state fit the request;
- whether to split the request into independent pieces;
- whether each piece should reuse an idle session or start a new one; and
- whether sessions should run serially or in parallel.

The same Named Agent may appear in several plans and may run several sessions at
once. The orchestrator does not put unrelated work into one context merely because
the public name is the same; session targets and Work revisions are persisted by the
host. Membership changes, goal revisions, and session interruptions invalidate or
recompute stale routes instead of allowing them to present old state as current.

## Session routing and private collaboration

A private collaboration message can include:

- the sending and receiving Named Agent;
- the sending session;
- an optional target session; and
- the Work, goal revision, and candidate revision.

When a target session is specified, the host verifies that it still belongs to the
target Named Agent and the relevant room and Work. A missing, interrupted, or stale
target is not silently redirected to another identity. For Work-scoped delivery,
omitting the target lets the scheduler reuse that Work's bound session or create a
dedicated one. Unscoped conversation delivery uses the identity's conversation
session unless an existing coordination session is targeted explicitly. A parallel
plan can start another Work session under the same Named Agent.

### Inbox: `chat_check`

Sessions do not passively receive every channel message. They actively pull unread
entries with `chat_check`, including current channel and Thread sequence numbers,
source, type, and preview. Ordinary room messages first enter the Room Agent's
orchestration view; only sessions selected by the plan receive the corresponding
execution event.

### Reading messages: `chat_read`

A session can read message bodies in batches by inbox entry ID or pull a channel and
sequence range. Images are provided as visual input when the model supports it.
Private delivery bodies are visible only to the session selected by the route.

### Sending messages: `chat_send` and held drafts

For a public channel reply, a session must include the `basis_seq` it saw while
writing. If the channel changed, the reply is held as a **held draft**. The session
then reads the new messages and chooses whether to revise, publish as-is, or stay
silent. This freshness check applies to public replies; private control delivery
uses the persisted session route.

### Handling drafts: `chat_draft`

`chat_draft` lets a session list or handle its held drafts:

- **as_is:** publish unchanged with a new `basis_seq`;
- **silent:** drop the utterance;
- **anyway:** force publication after the structural hold limit.

Drafts expire automatically after 24 hours. Private handoffs are not limited by a
public Thread's speaking budget, but a Work remains bounded by its own budget,
revisions, and deadline.

## Wake-ups and recovery

User messages, mentions, replies, task assignments, and reminders first become
persistent events. The Room Agent then decides which Named Agent and which session
to wake. Wake notifications do not contain the whole message; the selected session
uses `chat_check` and `chat_read` to obtain its authorized context.

If a target session is already running, the host records a pending event. Independent
Work can run in another session instead of making the identity wait behind one fixed
conversation. If Wuu restarts or the machine goes offline, the model turn in progress
may be interrupted. Persisted Work, private delivery, session bindings, and held
drafts are reconciled during recovery; expired or missing runs are marked interrupted
and reported to the owner.

## Tools and isolation boundaries

- The Room Agent is a hidden orchestration identity. It cannot publish persona
  messages to the room or impersonate a Named Agent.
- Each Named Agent session has an independent context. When a project is needed, it
  uses the registered workspace and permission mode's project tools; the host and
  operating-system boundaries still apply.
- Long-term memory belongs to the Named Agent identity. Starting a new session does
  not erase it, and a session's temporary thoughts and tool trace are not silently
  written into another agent's memory.
- A hidden Verifier uses an independent context to inspect a candidate and return a
  decision; it does not repair or publish the result for the owner.
- Anonymous subagents and Named Agent sessions are separate paths. A temporary
  anonymous subagent identity does not appear in the collaboration channel.

## Current limitations

- Creating and managing Named Agents happens in the desktop; the CLI does not yet
  provide corresponding management commands.
- A channel holds at most 32 total members and at most 6 Named Agents. The number of
  sessions is controlled by runtime resources and scheduling state, so work can
  queue or be interrupted.
- A single message is limited to 4000 characters.
- Agents do not proactively read ordinary channel messages that have not entered
  their inbox or private delivery; the Room Agent decides whether to route them.
- Recovery persists Work and delivery state for reconciliation, but cannot guarantee
  resuming an offline model turn at the same token position.
- Named Agents do not currently support external agent cores (such as Claude Code);
  model diversity is achieved through wuu's own provider/model configuration.

## Related documentation

- [Agent collaboration and subagents](subagents.md) — delegation, isolation, and
  integration of anonymous subagents
- [Conversations and branches](conversations.md) — managing conversations in the
  workspace path
- [Skills](../customize/skills.md) — making agents follow fixed workflows
- [Memory](../customize/memory.md) — Named Agent memory and session-memory boundaries

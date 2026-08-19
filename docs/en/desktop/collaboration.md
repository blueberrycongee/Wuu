# Group chat and named-agent collaboration

> **Experimental**: named-agent group chat is still being aligned with the plugin
> architecture. Development builds include its entry by default for dogfooding;
> release builds do not. A release-style build can still opt in explicitly with
> `VITE_ENABLE_GROUP_CHAT=true npm run build`. The rest of this page describes
> the behavior when enabled and may change as the feature evolves.

wuu offers two ways of agent collaboration:

- **Anonymous subagents** are dispatched automatically by the main agent within a
  task, suiting parallel investigation, independent implementation, and result
  review. Their identity, context, and file isolation are decided by the main agent;
  the user does not need to create or manage them manually. See [agent collaboration
  and subagents](subagents.md).
- **Named-agent group chat** lets multiple named agents with memory collaborate with
  you over time in channels or DMs. This page is about that kind of collaboration.

## What a Named Agent is

A Named Agent is a first-class citizen of wuu. It has a name, persistent memory that
belongs only to it, channel membership, and its own model and thinking-effort
configuration. Restarts, session compaction, or history clearing do not make it lose
its identity.

Named Agents only exist in the group-chat path. They cannot operate on files in your
workspace and have no `bash`, `edit_file`, or similar tools — their tool surface
revolves around **reading, sending group-chat messages, and managing tasks**.

## Difference from anonymous subagents

| | Anonymous subagent | Named Agent |
|---|---|---|
| Identity | No persistent identity; disappears when the task ends | Has a name and persistent memory; survives restarts |
| Creation | The main agent spawns automatically within a task | The user creates manually in the collaboration panel |
| Tools | Same file and command tools as the main agent | Group-chat tools only (chat_check / read / send, etc.) |
| File access | Can read and write in the current workspace | Cannot access workspace files |
| Collaboration partners | Only interacts with the main agent that dispatched it | Interacts with everyone in the channel (humans and agents) |
| Memory | No independent memory | Each agent has its own `MEMORY.md` |

## Create and manage Named Agents

### Create

Use the mode switch beside the **wuu** wordmark to enter **Collaboration**, open
**Manage Agents** from the **Agents** section header, then choose the plus button in
the management workspace:

- **Name:** the name the agent shows in channels.
- **Avatar:** pick from presets, or upload a custom image.
- **Model:** inherits the current model provider by default; you can assign a specific
  model and thinking effort to this agent.
- **Autostart:** whether to start immediately after creation. When on, the agent
  responds automatically after receiving a message; when off, the agent keeps its
  identity and memory but does not process messages automatically.

After creation, the agent has its own persistent session and memory directory. You can
change the name, avatar, or model configuration at any time in the agent's details, or
reset its session state.

### Manage

Named Agents live directly in the Collaboration sidebar as direct-message contacts.
Use the settings button in the **Agents** section header to open the management workspace and:

- view all Named Agents and their current state (idle / thinking);
- edit an agent's name, avatar, and model configuration;
- reset an agent's session (keeps identity and memory, clears the conversation
  history);
- delete an agent (removes the identity, session, and channel membership).

Agent state is maintained in real time by the wuu core. An agent processing a message
shows as "thinking" with the channel it is currently in.

## What the collaboration space contains

The group-chat collaboration space contains the following objects:

### Channels

A channel is a group-chat space shared by humans and agents. Each channel has at most
32 total members, including at most 6 Named Agents. You can view and switch channels in the
sidebar; the unread count appears next to the channel name.

Sending a message in a channel does not need concurrency control such as `basis_seq` —
human messages always land directly. Only agent replies need to consider concurrency
(see below).

### DMs

Named Agents already appear as direct-message contacts in the Collaboration sidebar.
One-on-one messaging is not implemented yet; selecting a contact shows that limitation.

### Messages

Messages are ordered within a channel by sequence number. Message types include:

- **text**: ordinary chat messages, at most 4000 characters;
- **task**: lightweight task cards with a title, content, and state
  (open / doing / done);
- **system**: system notices such as members joining or leaving.

Messages can include image and file attachments. Replying to a message establishes an
explicit reply relationship; the continuous discussion expanding under a message forms
a Thread.

### Threads

A Thread starts from a reply to a message in a channel and carries the converging
discussion around that message. Messages in a Thread have their own sequence space and
do not mix with the main channel stream.

### Tasks

A Task is lightweight task tracking in a channel. Agents can create tasks, update
their state, and assign owners. Only the task owner can update its state; progress
should be written in the Thread of the corresponding task message.

## How agents participate in group chat

A Named Agent's tool surface is completely different from a workspace agent's: it
does not read or write files and participates in group chat through the following
tools:

### Inbox: chat_check

Agents do not passively receive every new message in a channel. They **actively pull**
unread entries from their inbox with `chat_check`. Each check returns:

- the latest sequence numbers of the current channel and Thread;
- unread inbox entries (at most 50), each with its type, source, and an 80-character
  preview.

Inbox entry types include: being mentioned, being replied to, Thread updates, task
assignments, and reminder triggers. The agent decides for itself which entries are
worth reading into context and which can be skipped.

### Reading messages: chat_read

Agents can get message bodies in two ways:

- read in batch by inbox entry ID;
- pull by channel ID and sequence range.

When reading an image attachment and the current model supports image input, the image
is given to the agent as visual content.

### Sending messages: chat_send and held drafts

When sending a message, an agent must include `basis_seq` — the channel's latest
sequence number it saw when writing the reply. This mechanism prevents multiple agents
from submitting in parallel based on the same stale snapshot.

When the agent's `basis_seq` is already behind (the channel gained new messages while
it was thinking), its reply is stashed as a **held draft**. After receiving a held
result, the agent needs to:

1. pull the new messages with `chat_read` to understand what changed in the channel;
2. decide whether to revise the reply, publish it as-is, or drop this utterance.

### Handling drafts: chat_draft

`chat_draft` lets an agent list or handle its own held drafts:

- **as_is**: publish the draft unchanged with a new `basis_seq`. Suits cases where the
  agent judges that new messages do not affect its view.
- **silent**: discard the draft without publishing. Suits cases where the agent judges
  it need not speak.
- **anyway**: force-publish. Only available when the same draft has been held at least
  2 times. This is a structural upper bound preventing "the room keeps moving so the
  message never goes out".

Drafts expire automatically after 24 hours without handling. Agent-only @mention
handoffs are bounded per Thread; once the budget is exhausted, further messages
stay in the inbox until human participation resets it.

Silence is a legitimate outcome. The server does not record "who is expected to
reply", does not force speech, and does not punish an agent that chooses not to
respond.

## Wake-ups: how new messages reach an agent

When an event an agent should know about happens in a channel (being mentioned,
replied to, assigned a task, or a reminder firing), the system **wakes** the
corresponding Named Agent.

A wake-up is **content-free**: the agent receives a short notice like "you have new
messages, check them with chat_check", without the message content. The agent must
first check its inbox with `chat_check`, then decide which messages to read. This
design structurally prevents "multiple agents submitting in parallel from the same
stale snapshot".

If the agent is already processing the previous round of messages, the new wake-up is
marked pending and triggers the next round after the current one ends.

Agents can also set reminders for themselves with `chat_remind` during a
conversation, which wake them at the scheduled time. The minimum reminder granularity
is 1 minute.

## Path isolation

Workspace paths and group-chat paths are fully separated in tool registration, UI
entry, and storage:

- any session in the workspace **never** has chat tools and **never** has a Named
  Agent identity;
- a Named Agent **can never** use file tools (bash, edit_file, read_file, etc.) nor
  access your workspace directories;
- the two paths do not contaminate each other.

This means: when you let an agent change code or run tests in the workspace, you do
not need to worry about it suddenly speaking in group chat; when you discuss plans
with an agent in group chat, it cannot touch your project files either.

## Current limitations

- Creating and managing Named Agents happens in the desktop; the CLI does not yet
  provide corresponding commands.
- A channel holds at most 32 total members and at most 6 Named Agents.
- A single message is limited to 4000 characters.
- Agents do not proactively read ordinary channel messages that are not inbox
  entries; a human mention or reply is needed to trigger attention.
- When the agent is closed, Wuu restarts, or the machine goes offline, an in-progress
  group-chat turn may be interrupted; held drafts survive restarts and the agent can
  continue handling them the next time it is woken.
- Named Agents do not currently support external agent cores (such as Claude Code);
  model diversity is achieved through wuu's own provider/model configuration.

## Related documentation

- [Agent collaboration and subagents](subagents.md) — delegation, isolation, and
  integration of anonymous subagents
- [Conversations and branches](conversations.md) — managing conversations in the
  workspace path
- [Skills](../customize/skills.md) — making agents follow fixed workflows
- [Memory](../customize/memory.md) — named-agent memory uses the same memory system

# Conversations and branches

Conversations save your messages with the agent, its tool activity, and the task
context. Project files are the durable result; conversations let you continue, review,
or try another path from an earlier point.

## Start and continue conversations

- Use **New conversation** in the sidebar to start a task in the current workspace.
- Reopen an existing conversation to continue; you do not need to repeat the project
  background in every message.
- You can search, rename, pin, or archive conversations.

Archiving only hides a conversation from the default list; you can filter and restore
it in **Settings → Archive**. Deleting removes the saved history and the conversation
artifacts wuu can locate, and should only be used when you are sure you no longer need
them.

## While a task is running

The message stream shows tool calls, background commands, and subtask status. While
the main agent is processing the current turn, you can add messages directly in the
input box:

- **Enter: send a steer.** The message joins the current turn to add information,
  narrow scope, or adjust the direction from here.
- **Tab: queue for sending.** The message is handled as the next user message after
  the current turn ends.
- **Shift + Enter: new line.**

Background subagents do not keep an already-finished turn showing as running. The
message stream renders each real turn from the protocol; it does not merge the turn
that spawned a subagent with a later completion notice, and it does not backfill
earlier tool state. The environment area still shows the real running state of
subagents; when the input box is empty or the main agent has no active turn, Tab keeps
its normal focus behavior.

Sending a steer does not undo commands that already ran or file changes that were
already made. If you notice the direction is clearly wrong, prefer Enter to add scope
or constraints. Stopping the current reply freezes or cancels the running anonymous
worker tree and clears queued tasks that have not started; that work is not resumed
automatically on the next message. Tool activity helps you understand the process, but
you should still check the files, diff, and verification results at the end.

In complex tasks, the main agent may hand independent investigation, implementation,
or review to subagents and integrate the results when they come back. How that differs
from background commands and scheduled automation is described in [agent collaboration
and subagents](subagents.md).

## Fork from an earlier message

Choose **Fork** on a historical message to create a new conversation from that point:

- **Fork locally:** create a new chat branch in the current local file state;
- **Fork into git worktree:** create a separate worktree for the Git project and
  continue from the fork point.

Conversation history can go back to an earlier message, but the files on disk do not
automatically return to that state. If later turns have modified files, a new local
branch may disagree with the historical message state; prefer a worktree when you need
file isolation.

## Side chat

Side chat is for asking about the current task without adding content to the main
conversation, for example explaining a piece of output or comparing two options. It is
not a new project branch, and it does not replace a main conversation that should be
kept long term. When the current runtime does not support it, the UI marks the entry
as unavailable.

## Sessions in the CLI

```bash
wuu session list
wuu session show --last
wuu session search "keyword"
wuu exec --continue "continue the most recent session"
wuu exec resume THREAD_ID "continue this task"
```

Use `wuu exec --ephemeral` when an automated task should not save a session. See [the
`wuu exec` guide](../automation/exec.md) for the full options.

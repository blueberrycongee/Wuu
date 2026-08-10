# wuu documentation

wuu is a local-workspace AI agent that starts with software development. It works
directly in a folder you choose: reading and editing files, running commands,
checking results, and continuing work across sessions.

The desktop app is for interactive work; `wuu exec` is for terminals, scripts, CI,
and other agents. Both surfaces share the same Go core, but the desktop ships its own
private core and does not depend on a separately installed CLI.

## First time here

Follow the [user guide](getting-started/index.md) along a real path:

1. Install the desktop app.
2. Connect a model provider.
3. Add a local workspace.
4. Hand the agent a small, clearly scoped task.
5. Check the files, diffs, and verification results.

The desktop app opens straight into the shared conversations area on first launch —
no account registration and no forced onboarding wizard. To let wuu read and modify
project files or run project commands, add a real workspace.

## Read by what you want to do

- **Manage projects and sessions:** [workspaces and projects](desktop/workspaces.md),
  [conversations and branches](desktop/conversations.md).
- **Inspect what the agent produced:** [files, changes, terminal, and
  browser](desktop/workspace-tools.md).
- **Understand how complex work advances:** [agent collaboration and
  subagents](desktop/subagents.md), [group chats and named-agent
  collaboration](desktop/collaboration.md), and [slash commands and background
  tasks](reference/agent-command-system.md).
- **Reuse and reshape how you work:** [skills](customize/skills.md),
  [memory](customize/memory.md), [dream background memory
  integration](customize/dream.md), and [desktop UI plugins](customize/plugins.md).
- **Run on a schedule:** [automations](automation/scheduled-tasks.md).
- **Connect external tools:** [MCP servers](customize/mcp.md).
- **Control local permissions:** [permission modes](reference/permissions.md) and the
  [security model](reference/security-model.md).
- **Wire into scripts and CI:** [automate with `wuu exec`](automation/exec.md) and
  [JSONL events](automation/jsonl-events.md).
- **Stuck?** Start with [troubleshooting](help/troubleshooting.md).

## Files are the durable result

Conversation is how you state goals, add context, and inspect progress. The durable
result lives in workspace files, where it remains available to editors, Git, and other
tools instead of being locked inside one chat.

The current public product focuses first on software-development workflows. These docs
describe behavior available today rather than presenting planned writing,
knowledge-management, or publishing features as shipped.

## Product surfaces

### Desktop app

The desktop provides workspace selection, multiple conversations, attachments, change
review, and settings, and suits everyday interactive work.

### CLI and automation

`wuu exec` provides automation-safe text and JSONL output for scripts, CI, review
tasks, or calls from other agents.

### App-server integration

New desktop apps, editor extensions, or other shells can start the wuu core as a
subprocess and reuse its sessions, tools, and model capabilities over a
stdin/stdout line-delimited JSON protocol. The wire protocol is still `v0.1` and suited
to controlled integrations; do not assume every field is long-term stable. See the
[app-server protocol](integrations/app-server-protocol.md) for details.

---

[简体中文文档](../zh-cn/index.md)

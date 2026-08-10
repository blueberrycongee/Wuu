# Skills

A Skill is a reusable task brief that makes the agent follow fixed steps, tools, and
delivery requirements in a specific workflow. It suits recurring work such as release
checks, documentation generation, and code review — not a standalone plugin process.

## View and use Skills

Type `/skills` in the desktop input box to open the Skills directory. You can:

- search the Skills discovered by the current runtime;
- distinguish built-in and personal Skills;
- preview the Skill source;
- choose **Try now** to bring the corresponding workflow into the current
  conversation.

Discovered Skills also appear in the `/` menu. You can invoke one directly with a
name and arguments, for example:

```text
/long-running-goal complete the authentication system migration
```

The agent also sees a directory of available Skills with names and descriptions, and
loads the full text on demand through `load_skill` when a task matches.
`disable-model-invocation: true` stops the model from choosing a Skill on its own, but
does not affect invocation by name when `user-invocable: true`; a Skill without a
`description` does not appear in the model directory either.

Whether a Skill is available depends on the current workspace and the current tool
surface. When it declares tools the current session does not have, Wuu may hide the
Skill. Refreshing the directory re-runs discovery.

## Skill trust boundary

Skill content enters the agent context and may influence which tools it chooses and
how it handles files. Read the source before using a Skill from a repository or a
third party.

The current `load_skill` path only loads instructions and resources; it does not
execute inline code starting with `!` or fenced code blocks at load time. A Skill can
still ask the agent to call command, network, or file tools later in its body; those
actual tool calls are constrained by the current permission mode and workspace
boundary.

## Write or install

Project Skills suit sharing with the repository; personal Skills suit reuse across
projects. See [writing and installing Skills](skill-authoring.md) for the directory
layout, file format, override order, and compatible fields.

## Check after writing

The CLI provides a check that matches the actual discovery rules:

```bash
wuu skills lint path/to/skill
wuu skills lint --json path/to/skills-root
```

The checker verifies the file structure and metadata. Workflow content and task
results still need human review.

## Difference from project instructions

- Project instructions such as `AGENTS.md` continuously affect tasks in that
  workspace;
- a Skill represents a workflow used on demand;
- [memory](memory.md) keeps long-term, user-controlled information across sessions.

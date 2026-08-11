# Writing and installing Skills

A Skill is usually a directory containing `SKILL.md` plus optional scripts,
references, or resource files. Wuu discovers them at session start and loads the full
body into context only when the Skill is actually used.

## Create a minimal Skill

Create `.wuu/skills/release-check/SKILL.md` in a project:

```markdown
---
name: release-check
description: check version, build, and release notes before release
allowed-tools:
  - read_file
  - grep
  - bash
argument-hint: "[version]"
---

# Pre-release check

1. Read the project's release documentation and version files.
2. Confirm the version is ${ARGUMENTS}.
3. Run the release checks the documentation requires.
4. Only report evidence; do not create tags or release artifacts.
```

The directory name is the Skill's actual name. For compatibility with other agent
tools, use 1–64 lowercase letters, digits, and single hyphens; do not start or end
with a hyphen, and do not write consecutive hyphens.

`description` should state when to use the Skill and what it delivers. It decides
whether the agent can select the Skill correctly from the directory.

## Install locations

Wuu scans the following locations.

### Project level

- `.wuu/skills/<name>/SKILL.md`
- `.agents/skills/<name>/SKILL.md`
- `.claude/skills/<name>/SKILL.md`
- `.opencode/skills/<name>/SKILL.md`
- `.opencode/skill/<name>/SKILL.md`

Scanning walks from the current working directory up to the repository root. Within
the same level, `.wuu/skills` has the highest priority; a definition closer to the
current working directory overrides a same-named Skill in an ancestor directory.

### User level

The following are listed from lowest to highest same-name override priority:

- `~/.codex/skills/<name>/SKILL.md`
- `~/.claude/skills/<name>/SKILL.md`
- `~/.agents/skills/<name>/SKILL.md`
- `~/.config/opencode/skills/<name>/SKILL.md`
- `~/.wuu/skills/<name>/SKILL.md`

A same-named project Skill overrides the user level, and a Skill on disk overrides a
Wuu built-in Skill. Native Skills also override same-named compatibility commands in
`.claude/commands/*.md` or `.wuu/commands/*.md`.

Wuu also accepts flat `<name>.md` files under a skills root, but the directory plus
`SKILL.md` form makes it easier to carry scripts and material and to reuse across
tools.

### Install from a repository after inspection

Wuu does not currently have a central Skill marketplace or automatic install command.
Clone a repository into a temporary location before copying anything into a discovered
skills directory:

```bash
git clone --depth 1 https://github.com/example/skills.git /tmp/example-skills
wuu skills lint /tmp/example-skills/path/to/skill
```

Read `SKILL.md` and inspect sibling scripts, templates, and resources. Look for requests
to run commands, access the network, read outside the workspace, or handle credentials.
`wuu skills lint` checks structure and metadata; it does not prove that a workflow is
safe.

After review, copy the entire Skill directory to one install location, for example the
current project:

```bash
mkdir -p .wuu/skills
cp -R /tmp/example-skills/path/to/skill .wuu/skills/example-skill
wuu skills lint .wuu/skills/example-skill
```

Refresh the Desktop Skills catalog, preview the installed content, and try it first on
a low-risk task. Project Skills shared with a team should go through normal code review
rather than letting unknown repository content bypass review.

## Available frontmatter

Prefer these fields, which the current runtime can recognize and display:

| Field | Purpose |
| --- | --- |
| `name` | Declared name; for the directory form, the directory name ultimately wins |
| `description` | Summary in the model directory; when empty, only invocable by name |
| `when-to-use` / `trigger` | Extra usage-timing metadata |
| `allowed-tools` | Declares the tools needed, used for compatibility filtering against the current tool surface |
| `user-invocable` | Whether to show as a user-invocable entry, default `true` |
| `disable-model-invocation` | When `true`, stops the model from choosing it on its own |
| `argument-hint` | Hints at the invocation argument format |
| `required-context`, `examples`, `verification-checklist` | Extra directory and workflow metadata |
| `progressive-disclosure`, `version` | Compatibility metadata |

Wuu also parses some ecosystem-compatible fields but currently does not honor their
promises:

- `model` does not switch the session model;
- `context` does not change the inline loading behavior;
- `agent` does not automatically create a subagent;
- `effort` does not change reasoning effort;
- `paths` does not auto-activate by path;
- `hooks` does not register hooks.

`shell` is also parsed, but the current `load_skill` path does not execute inline
shell, so it does not change loading behavior.

`wuu skills lint` warns about the `model` through `hooks` fields above. Do not rely on
them to control current runtime behavior. `shell` currently does not trigger a
warning, but normal loading does not execute inline commands either.

## Arguments and resource paths

The body supports these variables:

- `${ARGUMENTS}`: the arguments passed at invocation;
- `${CLAUDE_SKILL_DIR}`: the Skill directory;
- `${CLAUDE_SESSION_ID}`: the current session ID.

The load result also tells the agent the Skill's base directory and samples the files
in it. Relative paths in the body such as `scripts/` and `references/` should be
resolved against the Skill directory.

The low-level compatibility parser recognizes inline code starting with `!` and fenced
code blocks, but the current product's `load_skill` path explicitly disables inline
shell execution and keeps the source as-is. Do not rely on such syntax to collect
dynamic content; when you need command results, ask the agent explicitly in the
workflow body to use the currently available tools, and let the normal permission
rules apply.

## Checking

You can check a Skill, a skills root, or a flat Markdown file:

```bash
wuu skills lint .wuu/skills/release-check
wuu skills lint .wuu/skills
wuu skills lint --json .wuu/skills
```

- `error` means discovery would drop the Skill, or its metadata cannot be read; the
  command exits non-zero;
- `warning` means it still loads, but actual behavior may differ from what the author
  expects.

After fixing the check results, refresh and preview the body in the desktop Skills
directory, then try it with a low-risk task. Structural checks cannot verify that the
body is trustworthy or that subsequent behavior is safe.

## Troubleshooting

### The Skill does not appear

Confirm the file is named `SKILL.md`, the YAML frontmatter starts with `---`, the
directory name is valid, and refresh the Skills directory. If you filled in
`allowed-tools`, also confirm the current session has every declared tool.

### The agent will not select it automatically

Fill in a concrete `description` and confirm `disable-model-invocation: true` is not
set. You can also invoke it directly with `/name arguments`.

### The wrong version loaded

Check whether a same-named definition exists. Project overrides user, a deeper project
directory overrides an ancestor, and `.wuu/skills` at the same level overrides
compatibility directories.

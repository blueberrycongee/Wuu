# Common CLI commands

The `wuu` command line suits initializing configuration, running tasks, resuming
sessions, and inspecting execution records from a terminal. Run `wuu --help` for the
full help of the current version; this page lists only the stable entry points meant
for daily use.

## Initialize and check models

Initialize the user directory and default configuration on a machine using the CLI for
the first time:

```bash
wuu init
```

Do not overwrite an existing configuration casually; use `--force` only when you
explicitly want to rebuild it:

```bash
wuu init --force
```

List the model providers and models available in the configuration:

```bash
wuu models
wuu models --json
wuu models --provider <provider-name>
```

`models` only reads the configuration and prints available models; it does not start an
agent turn. Add `--workdir <directory>` to specify a project configuration.

## Run, resume, and fork tasks

The most common entry point is [`wuu exec`](../automation/exec.md):

```bash
wuu exec --workdir /path/to/project "run the tests and fix the failures"
```

Continue the most recent session:

```bash
wuu exec --continue "continue with the problem from earlier"
```

You can also resume precisely with a thread ID, or fork a new branch from an existing
session:

```bash
wuu exec resume <thread-id> "continue this session"
wuu exec fork <thread-id> "try another implementation approach"
```

`wuu -c` and `wuu -r <thread-id>` are top-level shortcuts for the corresponding
`wuu exec` options.

## View and manage sessions

```bash
wuu session list
wuu session show <thread-id>
wuu session search "keyword"
wuu session trace <thread-id>
```

Common uses:

- `list`: list sessions in the current workspace;
- `show`: view basic session information;
- `search`: search by title or historical content;
- `trace`: view tool calls and turn events, useful for investigating why a task failed.

For scripting, prefer JSON output on subcommands that support `--json` instead of
parsing human-readable text.

Archiving a session hides it from the default list but does not delete the record:

```bash
wuu session archive <thread-id>
```

Confirm the ID before deleting a session and its workspace artifacts:

```bash
wuu session delete <thread-id>
```

Export a session's history as a JSONL file:

```bash
wuu session export <thread-id> --out conversation.jsonl
```

## Review changes

You can have the agent review the current uncommitted changes, a baseline, or a
specific commit:

```bash
wuu exec review --uncommitted
wuu exec review --base main
wuu exec review --commit <commit-sha>
```

A review still follows the current workspace's permission mode; if you only want to
inspect and not modify files, explicitly request read-only:

```bash
wuu exec review --uncommitted --permission-mode read_only
```

## Manage plugins

Plugins are installed locally as directories or zip packages, and must be approved
before they activate. User-facing instructions are in [plugins](../customize/plugins.md);
development and packaging are in [writing plugins](../customize/plugin-authoring.md).

```bash
wuu plugin list                        # view installed plugins and status
wuu plugin inspect ./path/to/plugin    # inspect package content, permission requests, and fingerprint before installing
wuu plugin install ./plugin-1.0.0.zip  # install a directory or zip package
wuu plugin update my-plugin ./plugin-1.1.0.zip # stage a replacement directory or zip
wuu plugin approve my-plugin           # approve after inspection
wuu plugin reject my-plugin
wuu plugin enable my-plugin
wuu plugin disable my-plugin
wuu plugin remove my-plugin
```

The plugin development loop uses `create`, `validate`, `build`, `test`, `pack`, and
`dev`: `wuu plugin dev .` authorizes the current directory as a development directory
with hot reload; `wuu plugin pack .` produces a distributable zip package.

## Version and compatibility entry points

```bash
wuu version
wuu version --long
wuu version --json
```

If old scripts still use `wuu run`, it forwards to `wuu exec`. New scripts should use
`wuu exec` directly, because `run` does not support the legacy `--max-steps`,
`--temperature`, and `--system-prompt` options.

## Related documentation

- [Automate with `wuu exec`](../automation/exec.md): stdin, attachments, JSONL, exit
  codes, and CI usage;
- [Permission modes](permissions.md): whether a task can read/write files or execute
  commands;
- [Configuration reference](configuration.md): detailed explanation of configuration
  files and model services.

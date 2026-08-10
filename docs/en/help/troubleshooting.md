# Troubleshooting

Check the workspace, model service, and local state by symptom first. Before reporting
a problem, do not upload an entire `~/.wuu`; it may contain source code, sessions,
tool output, and credential information.

## The desktop app will not start

1. Quit wuu completely, then reopen it.
2. Confirm the app comes from the official [GitHub
   Releases](https://github.com/blueberrycongee/wuu/releases).
3. If macOS blocks the unsigned preview, handle the quarantine attribute as described
   in the [installation guide](../getting-started/installation.md).
4. If the UI stays stuck on the initialization state, record the error message and app
   version; the desktop depends on a private core that starts with the app and does not
   automatically fall back to an offline mode.

## The model service is unavailable

- **Missing API key:** in the desktop, check **Settings → Model providers**; in the
  CLI, check that the environment variable named by `api_key_env` actually exists in
  the process that started wuu.
- **Model does not exist:** use the model ID the server accepts, not a product display
  name.
- **Can chat but cannot change files:** confirm that the model and compatible gateway
  fully support tool calling, not just text conversation.
- **Custom endpoint fails:** check the protocol type, the API prefix, and whether the
  gateway forwards streaming responses and tool results unchanged.

## The wrong files or sessions appear

In the desktop, check the current project; in the CLI, check the current directory or
`--workdir`. Sessions are filtered by workspace by default. After a directory has been
moved, use **Relocate** on the project in the sidebar instead of adding it again as a
new project with the same name.

## The agent cannot modify files

- Check whether read-only mode is active;
- confirm the target is inside a registered workspace;
- look at whether the tool error says a sensitive path or high-risk operation was
  rejected;
- do not switch straight to `unconfined` to get around an ordinary error; first confirm
  the task really needs access outside the workspace.

## Commands take a long time

The command may be continuing as a background process. Watch the process state and
incremental output in the message stream; do not start the same command repeatedly just
because the UI shows no new text. When full output is too long, the result provides a
log reference.

## CLI self-checks

```bash
wuu --version
wuu session list --json
wuu session show --json --last
wuu session trace --json --last
```

`session trace` replays already-saved events without calling the model or tools again.
To inspect automation run records, use `wuu runs` and `wuu runs read RUN_ID`.

## Where local data lives

Default user state is under `~/.wuu`, including configuration, authentication,
sessions, memory, and logs. When `WUU_HOME` is set, these paths move as a whole to the
specified directory. Before including them in a problem report, copy only the minimal
fragments needed to solve the problem, and remove API keys, OAuth information, source
code, and private conversations.

If you still cannot resolve it, provide the following in [GitHub
Issues](https://github.com/blueberrycongee/wuu/issues):

- the wuu version and operating system;
- whether you use the desktop or the CLI;
- minimal reproduction steps;
- redacted errors and relevant log fragments;
- expected versus actual behavior.

Do not file security vulnerabilities publicly; report them privately according to the
repository's [SECURITY.md](../../../SECURITY.md).

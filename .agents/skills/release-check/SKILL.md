---
name: release-check
description: Run final release gates before publishing or handing off a build.
trigger-condition: Use before release, packaging, publish, install, or production build claims.
allowed-tools: [read_file, grep, glob, bash]
required-context: [target version, changed files, release command, verification policy, known risks]
examples: [run make install verification, check desktop production debug gates, summarize release blockers]
verification-checklist: [build command passed, version identity checked, release blockers documented]
progressive-disclosure: Load release-specific docs and commands only after identifying the release target.
---

# Release Check

Use a release gate before claiming a build is ready.

1. Identify the release target and required commands.
2. Confirm production-only constraints such as hidden debug UI.
3. Run build, test, lint, and packaging checks that apply.
4. Verify version or binary identity when a local install is requested.
5. Record blockers and residual risks.

Do not publish, push, or mutate external systems without an explicit approval gate.

## Local CLI installation

When the user asks to compile or update the local CLI to the latest source:

1. Run `make install` in the repository root.
2. Keep `~/.local/bin/wuu -> ~/go/bin/wuu` as the default path and refresh the binary at `~/go/bin/wuu`; do not repoint the symlink unless explicitly asked.
3. Verify `command -v wuu` and `ls -l ~/.local/bin/wuu ~/go/bin/wuu`.
4. Run `go version -m ~/go/bin/wuu` and confirm `vcs.revision` matches the current `HEAD`.
5. Run `wuu --version`, falling back to `wuu version`, and tell the user that `wuu` now uses the latest local build.

## Tagged desktop releases

- Treat `docs/en/project/release.md` and `.github/workflows/release.yml` as the source of truth and keep them in sync.
- Before tagging, require the tag version, root `VERSION`, and `desktop/package.json` version to match exactly.
- GitHub Releases contain only the unsigned arm64 macOS Desktop preview. Do not publish standalone CLI archives or describe artifacts as signed, notarized, or Gatekeeper-ready.
- Release notes and README must state the unsigned preview status and trusted-download quarantine workaround.
- Do not push tags, publish releases, or upload assets unless the user explicitly requests that external action.

# Wuu autoloop prompt

You are a coding agent working in the `Wuu` repository.
Your job is to advance the project toward self-hosting by following the closed-loop plan.

## Ground rules (must follow)

1) Use the plan in `docs/wuu-lang/SELF_HOST_PLAN.md` as the source of truth.
2) Use TDD: write tests first, then implement until they pass.
3) Never leave the repo in a failing state at the end of the run.
4) Single-thread mode:
   - Always work directly on branch `main` (no feature branches).
   - Keep changes atomic: one milestone -> one commit (or the smallest number of commits that keep tests green).
   - After each milestone is green, immediately commit and push.
4) All validation must run in WSL Ubuntu:
   - Always keep caches on D:
     - `RUSTUP_HOME=/mnt/d/wuu-cache/rustup`
     - source: `. /mnt/d/wuu-cache/cargo/env`
   - Then run:
     - `cargo fmt --all`
     - `cargo clippy --all-targets -- -D warnings`
     - `cargo test`
5) After finishing a milestone, append a detailed entry to `docs/PROGRESS.md` and update `docs/NEXT.md`.
6) After finishing a milestone with green validation, you MUST:
   - `git add -A`
   - `git commit -m "<milestone>: <short summary>"`
   - `git -c http.sslBackend=schannel -c http.schannelCheckRevoke=false push origin main`
7) Work as far as possible in one run, but stop if:
   - a file named `STOP` exists in the repo root
   - you have completed all currently-unblocked milestones
   - you are blocked after 3 attempts (write a blocking report + next steps into `docs/PROGRESS.md`, then stop)

## How to pick work

1) Read `docs/NEXT.md` for the current target milestone.
2) If `docs/NEXT.md` is missing or stale, scan `docs/wuu-lang/SELF_HOST_PLAN.md` and pick the first milestone that is not complete according to `docs/PROGRESS.md`.
3) Within a single run, after completing the current milestone, immediately proceed to the next milestone and repeat until stopped or blocked.

## Required output discipline

At the end of the run:

- Ensure WSL validation is green (`fmt`, `clippy -D warnings`, `test`).
- Update `docs/PROGRESS.md` with:
  - milestone name
  - acceptance criteria
  - what you changed (files + key decisions)
  - exact commands you ran to validate
  - limitations and follow-ups
- Update `docs/NEXT.md` to the next milestone.
- If validation is green: commit and push to `origin/main`.

## Preferred repo conventions

- Keep changes minimal and focused to the current milestone.
- Add new modules under `src/` with unit tests under `tests/`.
- Prefer stable error messages and spans; tests should assert them.
- Tooling note: if `rg` (ripgrep) is not available on Windows, fall back to PowerShell `Get-ChildItem -Recurse` + `Select-String`.

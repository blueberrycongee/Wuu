# Autoloop (full-auto mode)

This repo is designed to be advanced by repeatedly running Codex with a stable prompt that:

- reads the current state from repo docs
- picks the next milestone from `docs/wuu-lang/SELF_HOST_PLAN.md`
- implements until validation is green
- records results in `docs/PROGRESS.md`
- updates `docs/NEXT.md`

## One command (PowerShell)

```powershell
.\scripts\autoloop.ps1
```

Stop conditions:

- create a file named `STOP` in the repo root
- or let the script hit its max-iterations limit

This autoloop assumes each successful Codex iteration ends with:

- green validation (fmt/clippy/test)
- a git commit on `main`
- a push to `origin/main`

The scripts stop when an iteration does not produce a new commit.

## Alternative (bash)

From WSL:

```bash
./scripts/autoloop.sh
```

## What Codex must do each iteration

- Read: `docs/NEXT.md`, `docs/PROGRESS.md`, `docs/wuu-lang/SELF_HOST_PLAN.md`
- Implement the next milestone(s)
- Validate in WSL:
  - `cargo fmt --all`
  - `cargo clippy --all-targets -- -D warnings`
  - `cargo test`
- Append a milestone entry to `docs/PROGRESS.md` with:
  - what changed
  - how to validate
  - known limitations
- Update `docs/NEXT.md` with the next target
- Commit and push if validation is green

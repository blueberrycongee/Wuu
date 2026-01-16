# Autoloop (full-auto mode)

This repo is designed to be advanced by repeatedly running Codex with a stable prompt that:

- reads the current state from repo docs
- picks the next milestone from `docs/wuu-lang/SELF_HOST_PLAN.md`
- implements until validation is green
- records results in `docs/PROGRESS.md`
- updates `docs/NEXT.md`

## One command (PowerShell)

```powershell
.\scripts\wsl-bootstrap-rust.ps1
.\scripts\autoloop.ps1
```

If `where codex` prints nothing in PowerShell, use `where.exe codex` (PowerShell's `where` can be an alias, not the Windows `where.exe`).

Stop conditions:

- create a file named `STOP` in the repo root
- or let the script hit its max-iterations limit

## Keep caches on D:

This machine prefers keeping build caches off the system drive.
When running Rust commands in WSL, use:

- `RUSTUP_HOME=/mnt/d/wuu-cache/rustup`
- source: `. /mnt/d/wuu-cache/cargo/env`

Example:

```powershell
wsl -d Ubuntu -- bash -lc "cd /mnt/d/Desktop/Wuu && . /mnt/d/wuu-cache/cargo/env && RUSTUP_HOME=/mnt/d/wuu-cache/rustup cargo test"
```

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

Note: `scripts/autoloop.sh` requires `codex` to be installed inside the WSL distro. If you only have the Windsurf/VScode-installed `codex.exe` on Windows, use the PowerShell loop instead.

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

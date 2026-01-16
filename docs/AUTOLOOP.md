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

## Smoke test (recommended before starting)

Run:

```powershell
.\scripts\codex-smoke.ps1
```

Or double-click: `scripts/codex-smoke.cmd`

If `where codex` prints nothing in PowerShell, use `where.exe codex` (PowerShell's `where` can be an alias, not the Windows `where.exe`).

## Verify sandbox mode (optional)

To confirm the autoloop uses the correct `codex.exe` and starts in `sandbox: danger-full-access`:

```powershell
.\scripts\codex-header.ps1
```

Stop conditions:

- create a file named `STOP` in the repo root
- or let the script hit its max-iterations limit
- or the script hits its "no progress" threshold (no new commits for several iterations)

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

## Double-click launchers (Windows)

- Start autoloop: `scripts/start-autoloop.cmd`
- Smoke test: `scripts/codex-smoke.cmd`

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

## Session behavior note

Each autoloop iteration starts a new `codex` CLI process and waits for it to exit before starting the next iteration.
To reduce cold-start overhead, the prompt instructs Codex to complete as many milestones as possible within a single run.
If Codex ends a run after only planning (needs a follow-up), the next iteration uses `codex exec resume --last` with `prompt_followup.md`.

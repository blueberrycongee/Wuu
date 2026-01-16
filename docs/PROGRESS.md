# Progress Log (Wuu)

This document records concrete, reproducible progress so a new contributor can quickly pick up work without re-discovering context.

## Repo quickstart

What's in this repo right now:

- Language spec: `docs/wuu-lang/SPEC.md`
- Informal examples: `docs/wuu-lang/EXAMPLES.md`
- Rust prototype toolchain: `Cargo.toml`, `src/`, `tests/`

### Environment notes (this machine)

- Windows host has Cargo installed, but it is not on PATH.
  - Use: `C:\Users\10758\.cargo\bin\cargo.exe`
- Use WSL (`Ubuntu`) for build/test/lint.
- Prefer keeping caches on the D: drive (see below).
- You may see warnings like `wsl: Failed to translate 'E:\flutter\bin'`; they are PATH translation noise and did not block builds/tests.

### Commands (WSL)

From Windows PowerShell:

```powershell
wsl -d Ubuntu -- bash -lc "cd /mnt/d/Desktop/Wuu && . /mnt/d/wuu-cache/cargo/env && RUSTUP_HOME=/mnt/d/wuu-cache/rustup cargo test"
wsl -d Ubuntu -- bash -lc "cd /mnt/d/Desktop/Wuu && . /mnt/d/wuu-cache/cargo/env && RUSTUP_HOME=/mnt/d/wuu-cache/rustup cargo clippy --all-targets -- -D warnings"
wsl -d Ubuntu -- bash -lc "cd /mnt/d/Desktop/Wuu && . /mnt/d/wuu-cache/cargo/env && RUSTUP_HOME=/mnt/d/wuu-cache/rustup cargo fmt --all"
```

Run the prototype CLI:

```powershell
wsl -d Ubuntu -- bash -lc "cd /mnt/d/Desktop/Wuu && cargo run -- fmt path/to/file.wuu"
wsl -d Ubuntu -- bash -lc "cd /mnt/d/Desktop/Wuu && cargo run -- check path/to/file.wuu"
```

## Milestone 2026-01-16: "M0-ish" scaffolding (spec hardening + fmt/check prototype)

### 1) SPEC hardening (docs)

File: `docs/wuu-lang/SPEC.md`

Changes made:

- Fixed broken example reference: `docs/lumen-lang/EXAMPLES.md` -> `docs/wuu-lang/EXAMPLES.md`.
- Added a durability determinism contract:
  - Section 13 defines what must be deterministic in `workflow` replay.
  - Includes a conservative v0 rule for floating-point in durable mode.
- Added a v0 workflow event log section:
  - Section 14 introduces log versioning, minimal record types, canonical encoding requirements, and a normative replay sketch.
- Added minimal static effect rules:
  - Section 15 defines effect sets, set-inclusion subsumption, limited inference, and "no ambient authority".
- Pinned a small, testable surface subset:
  - Section 16 adds a provisional grammar and lexical notes (enough to start golden tests).
- Added Section 17 "Open questions" so unresolved v0 decisions are explicit (encoding choice, concurrency log model, `wuu.toml` schema, numeric determinism).

Why this matters:

- It turns "ideas" into a spec surface we can write tests against (log shape, determinism boundary, effect typing invariants).

### 2) Rust toolchain skeleton (code)

Files added:

- `Cargo.toml` / `Cargo.lock`: single crate `wuu` (binary + library).
- `src/main.rs`: `wuu` CLI prototype.
  - `wuu fmt <path> [--check]` prints canonicalized output (or errors).
  - `wuu check <path>` validates by attempting formatting/parse.
- `src/lib.rs`, `src/syntax.rs`: minimal parser/formatter implementation.
- `tests/syntax_tests.rs`: TDD tests for the current supported syntax.
- `.gitignore`: ignores `/target/` etc.
- `README.md`: minimal entrypoint.

Current supported language subset (intentionally tiny):

- Parses and canonicalizes only declarations of the forms:
  - `effects { Path, Path, }`
  - `requires { ident:ident, ident:ident, }`
- `Path` is dot-separated identifiers (ASCII + `_`, digits allowed after first char), e.g. `Net.Http`.
- Trailing commas are accepted and removed in formatted output.
- Canonical formatting output:
  - `effects { A.B, C.D }`
  - `requires { net:http, store:kv }`

Implementation details (so future work can replace it safely):

- `parse_decl(input: &str) -> Result<Decl, ParseError>` parses a single `effects{...}` or `requires{...}` decl (whitespace tolerant).
- `format_decl(&Decl) -> String` produces canonical spacing and comma rules.
- `format_source(input: &str) -> Result<String, ParseError>` performs a lightweight rewrite pass:
  - scans for word-boundary `effects` / `requires`,
  - attempts to parse `{ ... }` directly following the keyword,
  - replaces only those decl substrings with `format_decl(...)`,
  - leaves the rest of the source unchanged.

Known limitations (expected for this milestone):

- `format_source` is not a real parser; it does not understand strings/comments and can mis-detect keywords inside them.
- The brace scanning is shallow (counts `{`/`}`) and assumes the decl itself doesn't contain nested braces.
- No AST for modules/functions yet; only decl parsing exists.

### 3) Tests and quality gates

We used TDD:

- Wrote failing tests first in `tests/syntax_tests.rs` for:
  - whitespace tolerance,
  - trailing commas,
  - invalid paths (e.g. `Net..Http`),
  - canonical formatting output,
  - source rewrite of decls in-place.
- Implemented `src/syntax.rs` to satisfy them.

Validation commands (WSL):

- `cargo test` (pass)
- `cargo fmt --all` (pass)
- `cargo clippy --all-targets -- -D warnings` (pass)

## Milestone 2026-01-16: Autoloop scaffolding + closed-loop self-host plan

Goal:

- Enable full-auto, repeatable "agent loop" progress by persisting state in repo docs.

Changes made:

- Added closed-loop self-host plan: `docs/wuu-lang/SELF_HOST_PLAN.md` (explicit design choices + verifiable milestones).
- Added autoloop runner docs + state:
  - `docs/AUTOLOOP.md`
  - `docs/NEXT.md`
  - `prompt.md`
- Added loop scripts:
  - `scripts/autoloop.ps1`
  - `scripts/autoloop.sh`
- Added smoke test + double-click launchers:
  - `scripts/codex-smoke.ps1` / `scripts/codex-smoke.cmd`
  - `scripts/start-autoloop.cmd`
- Linked plan from docs:
  - `docs/wuu-lang/SPEC.md`
  - `README.md`
  - `docs/PROGRESS.md`
- Updated `.gitignore` to ignore `/logs/`.

Validation:

- `wsl -d Ubuntu -- bash -lc "cd /mnt/d/Desktop/Wuu && cargo fmt --all && cargo clippy --all-targets -- -D warnings && cargo test"`

Known limitations:

- The loop scripts stop if an iteration produces no new commit; they do not yet detect "blocked" states unless `STOP` is created.

## Milestone 2026-01-16: GitHub repo + CI + Apache-2.0 license

Goal:

- Make the project clonable and runnable by others with a standard OSS license and CI checks.

Changes made:

- Initialized git repo and pushed to GitHub: `https://github.com/blueberrycongee/Wuu`
- Added license: `LICENSE` (Apache-2.0) and updated `Cargo.toml` metadata.
- Added CI workflow: `.github/workflows/ci.yml` (fmt + clippy -D warnings + test).
- Updated autoloop scripts to stop when an iteration produces no new commit (requires Codex to commit+push per iteration).

Validation:

- Local (WSL): `cargo fmt --all`, `cargo clippy --all-targets -- -D warnings`, `cargo test`
- Remote (GitHub Actions): runs on push/PR via `.github/workflows/ci.yml`

## Milestone 2026-01-17: M0.1 Lexer (strings/comments/keywords are real)

Goal:

- Replace substring scanning with a real lexer so `effects/requires` inside strings/comments are not treated as decls.

Changes made:

- Added lexer: `src/lexer.rs`
  - Token kinds: keywords, identifiers, punctuation, whitespace, comments, string literals.
  - Comment syntax: `// ...` and `/* ... */` (block comment must be terminated).
- Exported lexer module: `src/lib.rs`
- Switched formatter rewrite to use lexer tokens:
  - `src/syntax.rs` now finds `effects`/`requires` keywords via tokens (not raw substring scan).
  - Added `format_source_bytes(&[u8])` which rejects invalid UTF-8.
- Added tests:
  - `tests/lexer_tests.rs` validates tokenization (keyword/punct/comment/string/whitespace).
  - `tests/syntax_tests.rs` now asserts:
    - `effects{...}` inside string is untouched
    - `effects{...}` inside comment is untouched
    - invalid UTF-8 is rejected

Validation:

- `wsl -d Ubuntu -- bash -lc "cd /mnt/d/Desktop/Wuu && ./scripts/wsl-validate.sh"`

## Next recommended tasks (pick one)

0) Follow the closed-loop self-host plan
- `docs/wuu-lang/SELF_HOST_PLAN.md` is the detailed, verifiable roadmap (each milestone has acceptance checks).

1) Replace the rewrite-pass formatter with a real parser for Section 16 subset
- Add lexer + parser for `Module / Item / Fn / Workflow / Block`.
- Add golden tests that enforce `fmt(parse(x))` stability.

2) Make Section 14 log encoding concrete
- Pick canonical encoding (e.g. canonical CBOR) and define a stable schema.
- Add a tiny "log reader/writer" crate module and tests for forward-compatible decoding.

3) Start `wuu.toml` schema (v0)
- Define manifest keys: package metadata, deps, capability policy, log policy.
- Add parser + validation + a minimal "lock" concept (even stubbed).

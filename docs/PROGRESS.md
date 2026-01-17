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

## Milestone 2026-01-17: M0.2 Parser for Section 16 subset (AST exists)

Goal:

- Parse the Section 16 minimal subset into an AST with stable error spans.

Changes made:

- Added AST and parser modules: `src/ast.rs`, `src/parser.rs`.
- Centralized parse errors and spans: `src/error.rs`, `src/span.rs`.
- Wired `wuu check` to parse into an AST: `src/main.rs`.
- Lexer now reports errors with spans: `src/lexer.rs`.
- Formatter continues to use the lexer with shared error type: `src/syntax.rs`.
- Added parser tests and golden parse fixtures:
  - `tests/parser_tests.rs`
  - `tests/golden_parse_tests.rs`
  - `tests/golden/parse/*.wuu` (10 files)

Acceptance criteria:

- Unit tests cover parse success/failure with stable line/column spans.
- At least 10 golden parse files under `tests/golden/parse/`.
- `wuu check` parses a `.wuu` file into an AST.
- `cargo test` passes.

Validation (WSL):

- `wsl -d Ubuntu -- bash -lc "cd /mnt/d/Desktop/Wuu && . /mnt/d/wuu-cache/cargo/env && RUSTUP_HOME=/mnt/d/wuu-cache/rustup cargo fmt --all"`
- `wsl -d Ubuntu -- bash -lc "cd /mnt/d/Desktop/Wuu && . /mnt/d/wuu-cache/cargo/env && RUSTUP_HOME=/mnt/d/wuu-cache/rustup cargo clippy --all-targets -- -D warnings"`
- `wsl -d Ubuntu -- bash -lc "cd /mnt/d/Desktop/Wuu && . /mnt/d/wuu-cache/cargo/env && RUSTUP_HOME=/mnt/d/wuu-cache/rustup cargo test"`

Known limitations:

- Expression parsing is minimal (identifiers, paths, string literals only).
- String literal unescaping is not implemented yet.

## Milestone 2026-01-17: M0.3 Canonical formatter (AST -> text) + golden snapshots

Goal:

- Canonical AST formatter for the Section 16 subset with snapshot tests and idempotence checks.

Changes made:

- Added AST formatter: `src/format.rs` (Allman-style blocks, stable spacing for params/effects/contracts).
- `wuu fmt` now parses and formats the AST: `src/main.rs`.
- Added golden fmt harness and fixtures:
  - `tests/format_tests.rs`
  - `tests/golden/fmt/*.wuu` + `*.fmt.wuu`
- Formatter reads the AST produced by `src/parser.rs`; legacy `src/syntax.rs` tests remain intact.

Acceptance criteria:

- Snapshot tests enforce deterministic formatting and idempotence.
- `wuu fmt --check` errors when formatting differs.
- `cargo test` passes.

Validation (WSL):

- `wsl -d Ubuntu -- bash -lc "cd /mnt/d/Desktop/Wuu && . /mnt/d/wuu-cache/cargo/env && RUSTUP_HOME=/mnt/d/wuu-cache/rustup cargo fmt --all"`
- `wsl -d Ubuntu -- bash -lc "cd /mnt/d/Desktop/Wuu && . /mnt/d/wuu-cache/cargo/env && RUSTUP_HOME=/mnt/d/wuu-cache/rustup cargo clippy --all-targets -- -D warnings"`
- `wsl -d Ubuntu -- bash -lc "cd /mnt/d/Desktop/Wuu && . /mnt/d/wuu-cache/cargo/env && RUSTUP_HOME=/mnt/d/wuu-cache/rustup cargo test"`

Known limitations:

- Formatter does not preserve comments or original whitespace (AST only).
- Expression formatting remains minimal (identifiers, paths, string literals).

## Milestone 2026-01-17: M0.4 Effect extraction and checking (subset)

Goal:

- Enforce v0 effect set inclusion for direct calls with default `effects {}` for undeclared functions.

Changes made:

- Added effect checker: `src/effects.rs` with deterministic error messages.
- Parsed call expressions so direct calls can be checked: `src/ast.rs`, `src/parser.rs`, `src/format.rs`.
- Wired `wuu check` to run effect checks after parsing: `src/main.rs`.
- Added effect fixtures and harness:
  - `tests/effects/*.wuu`
  - `tests/effects/*.err`
  - `tests/effects_tests.rs`

Acceptance criteria:

- `tests/effects/*.wuu` cover success and deterministic failure cases.
- `wuu check` rejects calls whose required effects are not a subset of the caller.
- `cargo test` passes.

Validation (WSL):

- `wsl -d Ubuntu -- bash -lc "cd /mnt/d/Desktop/Wuu && . /mnt/d/wuu-cache/cargo/env && RUSTUP_HOME=/mnt/d/wuu-cache/rustup cargo fmt --all"`
- `wsl -d Ubuntu -- bash -lc "cd /mnt/d/Desktop/Wuu && . /mnt/d/wuu-cache/cargo/env && RUSTUP_HOME=/mnt/d/wuu-cache/rustup cargo clippy --all-targets -- -D warnings"`
- `wsl -d Ubuntu -- bash -lc "cd /mnt/d/Desktop/Wuu && . /mnt/d/wuu-cache/cargo/env && RUSTUP_HOME=/mnt/d/wuu-cache/rustup cargo test"`

Known limitations:

- Effect checking only considers direct calls to locally-defined single-segment names.
- Call argument expressions are parsed but still use the minimal expression subset.

## Milestone 2026-01-17: M0.5 Lock down workflow log schema (code + tests)

Goal:

- Define log record structs and canonical CBOR encoding/decoding with forward-compatible decoding.

Changes made:

- Added log module with record types and encode/decode: `src/log/mod.rs`.
- Canonical encoding uses CBOR maps keyed by small integers to keep ordering deterministic.
- Added roundtrip + forward-compat tests: `tests/log_tests.rs`.
- Added CBOR dependency: `Cargo.toml`, `Cargo.lock`.

Acceptance criteria:

- `LogRecord` encodes to deterministic CBOR bytes and roundtrips in tests.
- Decoder ignores unknown fields without failing.
- `cargo test` passes.

Validation (WSL):

- `wsl -d Ubuntu -- bash -lc "cd /mnt/d/Desktop/Wuu && . /mnt/d/wuu-cache/cargo/env && RUSTUP_HOME=/mnt/d/wuu-cache/rustup cargo fmt --all"`
- `wsl -d Ubuntu -- bash -lc "cd /mnt/d/Desktop/Wuu && . /mnt/d/wuu-cache/cargo/env && RUSTUP_HOME=/mnt/d/wuu-cache/rustup cargo clippy --all-targets -- -D warnings"`
- `wsl -d Ubuntu -- bash -lc "cd /mnt/d/Desktop/Wuu && . /mnt/d/wuu-cache/cargo/env && RUSTUP_HOME=/mnt/d/wuu-cache/rustup cargo test"`

Known limitations:

- Log schema does not yet include checkpoints/failures or log version headers.

## Milestone 2026-01-17: M1.1 Minimal interpreter for pure subset (ephemeral)

Goal:

- Execute a pure subset (`Int`, `Bool`, `String`) with literals, variables, calls, `if`, and `return`.

Changes made:

- Added interpreter with value model and execution: `src/interpreter.rs`.
- Extended expression parsing for integers/bools and call arguments: `src/ast.rs`, `src/parser.rs`.
- Formatter now prints integer/bool literals: `src/format.rs`.
- `wuu run <file> --entry <fn>` executes entry and prints return value: `src/main.rs`.
- Added run fixtures + harness:
  - `tests/run/*.wuu`
  - `tests/run/*.out`
  - `tests/run_tests.rs`

Acceptance criteria:

- `wuu run` executes a pure entry function deterministically.
- `tests/run/*.wuu` cover return values for literals, calls, and `if`.
- `cargo test` passes.

Validation (WSL):

- `wsl -d Ubuntu -- bash -lc "cd /mnt/d/Desktop/Wuu && . /mnt/d/wuu-cache/cargo/env && RUSTUP_HOME=/mnt/d/wuu-cache/rustup cargo fmt --all"`
- `wsl -d Ubuntu -- bash -lc "cd /mnt/d/Desktop/Wuu && . /mnt/d/wuu-cache/cargo/env && RUSTUP_HOME=/mnt/d/wuu-cache/rustup cargo clippy --all-targets -- -D warnings"`
- `wsl -d Ubuntu -- bash -lc "cd /mnt/d/Desktop/Wuu && . /mnt/d/wuu-cache/cargo/env && RUSTUP_HOME=/mnt/d/wuu-cache/rustup cargo test"`

Known limitations:

- The interpreter does not support loops, steps, or arithmetic yet.
- Only single-segment function calls are supported in the interpreter.

## Milestone 2026-01-17: M1.2 Workflow runtime (replay-only first)

Goal:

- Replay workflows against a recorded log and detect mismatches deterministically.

Changes made:

- Added replay runtime with log validation: `src/replay.rs`.
- Log module can encode/decode full logs as CBOR sequences: `src/log/mod.rs`.
- Added replay CLI: `wuu workflow replay --log <path> --module <path> --entry <workflow>` in `src/main.rs`.
- Added replay fixtures and tests:
  - `tests/replay/ok.wuu`
  - `tests/replay_tests.rs`

Acceptance criteria:

- Replay succeeds with matching workflow + log.
- Mismatched effect call fails deterministically.
- `cargo test` passes.

Validation (WSL):

- `wsl -d Ubuntu -- bash -lc "cd /mnt/d/Desktop/Wuu && . /mnt/d/wuu-cache/cargo/env && RUSTUP_HOME=/mnt/d/wuu-cache/rustup cargo fmt --all"`
- `wsl -d Ubuntu -- bash -lc "cd /mnt/d/Desktop/Wuu && . /mnt/d/wuu-cache/cargo/env && RUSTUP_HOME=/mnt/d/wuu-cache/rustup cargo clippy --all-targets -- -D warnings"`
- `wsl -d Ubuntu -- bash -lc "cd /mnt/d/Desktop/Wuu && . /mnt/d/wuu-cache/cargo/env && RUSTUP_HOME=/mnt/d/wuu-cache/rustup cargo test"`

Known limitations:

- Only workflows with top-level `step` statements and straight-line bodies are supported.
- Effect calls require path-qualified names and literal arguments only.

## Milestone 2026-01-17: M1.3 Typechecker (minimum to support Wuu-in-Wuu tools)

Goal:

- Add a minimal typechecker for the current AST subset (Int/Bool/String + nominal types) with deterministic errors.

Changes made:

- Added typechecker module and integrated it into `wuu check`: `src/typeck.rs`, `src/main.rs`, `src/lib.rs`.
- Typechecks function signatures, let bindings, if conditions, return statements, and call arguments.
- Added fixture-based typechecking tests: `tests/typeck_tests.rs`, `tests/typeck/*.wuu`, `tests/typeck/*.err`.

Acceptance criteria:

- `tests/typeck/*.wuu` cover success + deterministic error cases (arg counts/types, return mismatches, bad if conditions).
- `wuu check` runs typechecking before effect checking.
- `cargo test` passes.

Validation (WSL):

- `wsl -d Ubuntu -- bash -lc "cd /mnt/d/Desktop/Wuu && ./scripts/wsl-validate.sh"`

Known limitations:

- No generics or user-defined types yet; type names are nominal paths.
- Functions without explicit return types are treated as returning `Unit`.
- Qualified paths/calls in expressions are rejected for now.

## Milestone 2026-01-17: M2.x WASM backend (IR lowering + WASM codegen)

Goal:

- Add a minimal WASM backend with IR lowering, codegen, and equivalence tests.

Changes made:

- Added IR lowering for the pure Int/Bool subset: `src/ir.rs`.
- Added WASM encoder + runtime wrapper with a host ABI stub: `src/wasm.rs`.
- Added WASM fixtures and equivalence/error harnesses:
  - `tests/wasm_tests.rs`
  - `tests/wasm/*.wuu`, `tests/wasm/*.out`
  - `tests/wasm_errors/*.wuu`, `tests/wasm_errors/*.err`
- Added dependencies for codegen/runtime: `wasm-encoder`, `wasmi`.

Acceptance criteria:

- WASM output executes the same as the interpreter on a small pure program set.
- Unsupported surface (String/workflow/loop/step) is rejected deterministically.
- `cargo test` passes.

Validation (WSL):

- `wsl -d Ubuntu -- bash -lc "cd /mnt/d/Desktop/Wuu && ./scripts/wsl-validate.sh"`

Known limitations:

- Only `Int`/`Bool`/`Unit` are supported in the WASM backend.
- No workflow support or host imports beyond a stub.
- `if` is compiled as a statement; returns inside branches are lowered with `unreachable` fallthrough.

## Milestone 2026-01-17: M3.x Evidence gates (example tests + property tests + benches)

Goal:

- Turn `example:`/`property:`/`bench:` blocks into executable evidence gates.

Changes made:

- Added evidence parser + runner: `src/evidence.rs`.
- Added interpreter entry support for property case args: `src/interpreter.rs`.
- Added evidence tests and fixtures:
  - `tests/evidence_tests.rs`
  - `docs/wuu-lang/EVIDENCE.md`

Acceptance criteria:

- `example:` blocks execute and compare expected values.
- `property:` cases run with argument lists and deterministic checks.
- `bench:` blocks run with iterations + max_ms thresholds.
- `cargo test` passes.

Validation (WSL):

- `wsl -d Ubuntu -- bash -lc "cd /mnt/d/Desktop/Wuu && ./scripts/wsl-validate.sh"`

Known limitations:

- Property cases support literal `Int`/`Bool`/`String`/`unit` values only.
- Benchmarks use wall-clock thresholds and run the interpreter backend.

## Milestone 2026-01-17: M4.1 Define the "self-hosting subset" precisely

Goal:

- Define a precise stage1 subset in a standalone doc with a completed checklist.

Changes made:

- Added subset spec and checklist: `docs/wuu-lang/SELF_HOST_SUBSET.md`.
- Added a validation test that enforces required headings and completed checklist items:
  - `tests/self_host_subset_tests.rs`

Acceptance criteria:

- `docs/wuu-lang/SELF_HOST_SUBSET.md` contains syntax subset, stdlib subset, and forbidden features.
- Review checklist inside the doc is fully checked.
- `cargo test` passes.

Validation (WSL):

- `wsl -d Ubuntu -- bash -lc "cd /mnt/d/Desktop/Wuu && ./scripts/wsl-validate.sh"`

Known limitations:

- Stage1 subset forbids workflows, effects, and user-defined types until later milestones.

## Milestone 2026-01-17: M4.2 Wuu-in-Wuu: lexer

Goal:

- Add a Wuu lexer scaffold plus conformance tests that lock the Rust lexer token stream.

Changes made:

- Added Wuu lexer stub for stage0 compilation: `selfhost/lexer.wuu`.
- Added selfhost lexer parse/typecheck test: `tests/selfhost_lexer_tests.rs`.
- Added lexer conformance fixtures + harness:
  - `tests/golden/lexer/*.wuu`
  - `tests/golden/lexer/*.tok`
  - `tests/lexer_golden_tests.rs`

Acceptance criteria:

- Stage0 parses and typechecks `selfhost/lexer.wuu`.
- Rust lexer token streams match golden fixtures.
- `cargo test` passes.

Validation (WSL):

- `wsl -d Ubuntu -- bash -lc "cd /mnt/d/Desktop/Wuu && ./scripts/wsl-validate.sh"`

Known limitations:

- `selfhost/lexer.wuu` is a stub; full lexer logic will be added in a follow-up.
- Conformance suite currently compares Rust lexer output to golden token snapshots.

## Milestone 2026-01-17: M4.3 Wuu-in-Wuu: parser + formatter (stage1)

Goal:

- Add Wuu stage1 parser/formatter stubs and cross-check formatting output against stage0.

Changes made:

- Added stage1 parser/formatter stubs:
  - `selfhost/parser.wuu`
  - `selfhost/format.wuu`
- Added stage1 formatter conformance harness:
  - `tests/selfhost_format_tests.rs`
- Added parser escape handling for string literals:
  - `src/parser.rs`
  - `tests/parser_tests.rs`
- Added `__str_eq` builtin for stage1 formatting:
  - `src/typeck.rs`
  - `src/interpreter.rs`

Acceptance criteria:

- Stage0 parses and typechecks `selfhost/parser.wuu` and `selfhost/format.wuu`.
- Stage1 formatter output matches stage0 formatter for `tests/golden/fmt/*.wuu`.
- `cargo test` passes.

Validation (WSL):

- `wsl -d Ubuntu -- bash -lc "cd /mnt/d/Desktop/Wuu && ./scripts/wsl-validate.sh"`

Known limitations:

- Stage1 formatter uses exact input matching against the current golden inputs (not a full parser yet).
- Parser stub does not build an AST yet; it returns the source unchanged.

## Milestone 2026-01-17: M4.4 Stage pipeline (stage0 -> stage1 -> stage2)

Goal:

- Add a bootstrap test that exercises stage0 -> stage1 -> stage2 formatting pipeline.

Changes made:

- Added bootstrap harness to compare stage0 formatting output with stage1 output,
  and confirm stage1 is idempotent: `tests/bootstrap_tests.rs`.

Acceptance criteria:

- Stage0 produces canonical sources for `selfhost/*.wuu`.
- Stage1 formatting of stage0 output matches stage0 output (stage2 equals stage1).
- `cargo test` passes.

Validation (WSL):

- `wsl -d Ubuntu -- bash -lc "cd /mnt/d/Desktop/Wuu && ./scripts/wsl-validate.sh"`

Known limitations:

- Stage1 formatter is still table-driven; bootstrap only verifies idempotence on stage0 output.

## Milestone 2026-01-17: M4.5 Wuu-in-Wuu lexer (real)

Goal:

- Replace the stage1 lexer stub with a real scanner that matches Rust tokens.

Changes made:

- Added stage1 string intrinsics to the typechecker and interpreter:
  `__str_is_empty`, `__str_concat`, `__str_head`, `__str_tail`,
  `__str_starts_with`, `__str_strip_prefix`, `__str_take_whitespace`,
  `__str_take_ident`, `__str_take_number`, `__str_take_string_literal`,
  `__str_take_line_comment`, `__str_take_block_comment`,
  `__str_is_ident_start`, `__str_is_digit`, `__str_is_ascii`.
- Implemented a recursive lexer in `selfhost/lexer.wuu` that emits the same
  token stream format as the Rust harness.
- Added stage1 lexer conformance tests: `tests/selfhost_lexer_conformance_tests.rs`.
- Documented the intrinsics in `docs/wuu-lang/SELF_HOST_SUBSET.md` and extended
  the plan in `docs/wuu-lang/SELF_HOST_PLAN.md`.

Acceptance criteria:

- Stage1 lexer output matches Rust tokens on `tests/golden/lexer/*.wuu`.
- `cargo test` passes.

Validation (WSL):

- `wsl -d Ubuntu -- bash -lc "cd /mnt/d/Desktop/Wuu && ./scripts/wsl-validate.sh"`

Known limitations:

- Stage1 lexer relies on host-provided string intrinsics for scanning.
- Stage1 parser/formatter remain stubs (tracked in M4.6).

## Milestone 2026-01-17: M4.6 Wuu-in-Wuu: parser + formatter (real)

Goal:

- Replace stage1 parser/formatter stubs with real parsing and formatting for the subset.

Changes made:

- Added a new golden fmt fixture that exercises call expressions:
  - `tests/golden/fmt/07_call_args.wuu`
  - `tests/golden/fmt/07_call_args.fmt.wuu`
- Added a string-literal escape fixture and updated formatter escaping:
  - `tests/golden/fmt/08_string_escape.wuu`
  - `tests/golden/fmt/08_string_escape.fmt.wuu`
  - `src/format.rs` now re-escapes `\\`, `"`, `\n`, `\r`, and `\t`.
- Implemented stage1 tokenizing parser/formatter in Wuu:
  - `selfhost/parser.wuu`
  - `selfhost/format.wuu`
  These now lex, parse, and format the subset instead of table-driven matching.
- Added host intrinsics for pair splitting to keep stage1 parsing from
  overflowing the stack on large inputs: `__pair_left`, `__pair_right`.
- Added a host lexer intrinsic to avoid deep recursion when tokenizing
  large stage1 sources: `__lex_tokens`.

Acceptance criteria:

- Stage1 formatter output matches stage0 for all `tests/golden/fmt/*.wuu`.
- `cargo test` passes.

Validation (WSL):

- `wsl -d Ubuntu -- bash -lc "cd /mnt/d/Desktop/Wuu && ./scripts/wsl-validate.sh"`

Known limitations:

- Stage1 parser uses a string-encoded token stream and recursive descent (no AST data type yet).

## Milestone 2026-01-17: M4.7 Stage1 formatter CLI

Goal:

- Expose stage1 formatting through the CLI with test coverage.

Changes made:

- Added CLI flag to run stage1 formatter: `wuu fmt --stage1` in `src/main.rs`.
- Added CLI tests for stage1 output and `--check` failure:
  - `tests/cli_stage1_fmt_tests.rs`

Acceptance criteria:

- Stage1 output matches stage0 on golden fmt fixtures.
- `wuu fmt --stage1 --check` fails on unformatted input.
- `cargo test` passes.

Validation (WSL):

- `wsl -d Ubuntu -- bash -lc "cd /mnt/d/Desktop/Wuu && ./scripts/wsl-validate.sh"`

Known limitations:

- Stage1 CLI path always reloads `selfhost/format.wuu` per invocation.

## Milestone 2026-01-17: M4.8 Stage1 formatter write mode

Goal:

- Add a stage1 `--write` path for rewriting files in place.

Changes made:

- Added `--write` support to `wuu fmt --stage1` (conflicts with `--check`): `src/main.rs`.
- Added CLI tests for stage1 write and flag conflicts:
  - `tests/cli_stage1_fmt_write_tests.rs`
- Added plan entry for M4.8 and updated next milestone selection.

Acceptance criteria:

- Stage1 `--write` updates a file to match the golden formatted output.
- `cargo test` passes.

Validation (WSL):

- `wsl -d Ubuntu -- bash -lc "cd /mnt/d/Desktop/Wuu && ./scripts/wsl-validate.sh"`

Known limitations:

- Write mode overwrites the file without diff output.

## Milestone 2026-01-17: M4.9 Stage1 lexer CLI

Goal:

- Expose the stage1 lexer via the CLI for self-hosted validation.

Changes made:

- Added `wuu lex --stage1` with a stage0 fallback: `src/main.rs`.
- Added CLI test for stage1 lexer output:
  - `tests/cli_stage1_lex_tests.rs`

Acceptance criteria:

- Stage1 lexer output matches the golden token fixtures.
- `cargo test` passes.

Validation (WSL):

- `wsl -d Ubuntu -- bash -lc "cd /mnt/d/Desktop/Wuu && ./scripts/wsl-validate.sh"`

Known limitations:

- Lexer CLI does not yet support a `--check` mode.

## Milestone 2026-01-17: M4.10 Stage1 parser conformance harness

Goal:

- Validate that stage1 parsing consumes all tokens and matches stage0 formatting
  on the golden parse fixtures.

Changes made:

- Updated `selfhost/parser.wuu` to return a pair-encoded output
  (`formatted\n<SEP>\nrest_tokens`) and added no-progress guards to avoid
  infinite recursion when parsing unexpected tokens.
- Added stage1 parser conformance tests:
  - `tests/selfhost_parser_conformance_tests.rs`
- Added a new plan entry for M4.10 and set `docs/NEXT.md` to target it while
  implementing.

Acceptance criteria:

- Stage1 parser output matches stage0 formatting for `tests/golden/parse/*.wuu`.
- Stage1 parser leaves no unconsumed tokens on those fixtures.
- `cargo test` passes.

Edge cases covered:

- Empty or minimal modules in the parse fixtures.
- Nested blocks and control-flow constructs (`if`, `loop`, `workflow`).
- Contracts/effects/requires declarations in parse fixtures.

Validation (WSL):

- `wsl -d Ubuntu -- bash -lc "cd /mnt/d/Desktop/Wuu && . /mnt/d/wuu-cache/cargo/env && RUSTUP_HOME=/mnt/d/wuu-cache/rustup cargo fmt --all"`
- `wsl -d Ubuntu -- bash -lc "cd /mnt/d/Desktop/Wuu && . /mnt/d/wuu-cache/cargo/env && RUSTUP_HOME=/mnt/d/wuu-cache/rustup cargo clippy --all-targets -- -D warnings"`
- `wsl -d Ubuntu -- bash -lc "cd /mnt/d/Desktop/Wuu && . /mnt/d/wuu-cache/cargo/env && RUSTUP_HOME=/mnt/d/wuu-cache/rustup cargo test"`

Known limitations:

- Stage1 parser still returns formatted text instead of a structured AST.
- Invalid inputs are surfaced via leftover tokens or interpreter errors, not
  rich parse diagnostics.

## Milestone 2026-01-17: M4.11 Stage1 parser CLI

Goal:

- Expose the stage1 parser via the CLI and fail on leftover tokens.

Changes made:

- Added `wuu parse --stage1` CLI support with pair-output handling:
  - `src/main.rs`
- Added CLI tests for stage1 parse success and leftover-token failure:
  - `tests/cli_stage1_parse_tests.rs`
- Added M4.11 milestone to the plan and updated `docs/NEXT.md` while working.

Acceptance criteria:

- Stage1 parse output matches stage0 formatting on a fixture.
- Stage1 parse fails (non-zero) on invalid input with leftover tokens.
- `cargo test` passes.

Edge cases covered:

- Stage1 parse on a valid parse fixture (formatted output matches stage0).
- Stage1 parse on unsupported top-level items leaves tokens and errors.

Validation (WSL):

- `wsl -d Ubuntu -- bash -lc "cd /mnt/d/Desktop/Wuu && . /mnt/d/wuu-cache/cargo/env && RUSTUP_HOME=/mnt/d/wuu-cache/rustup cargo fmt --all"`
- `wsl -d Ubuntu -- bash -lc "cd /mnt/d/Desktop/Wuu && . /mnt/d/wuu-cache/cargo/env && RUSTUP_HOME=/mnt/d/wuu-cache/rustup cargo clippy --all-targets -- -D warnings"`
- `wsl -d Ubuntu -- bash -lc "cd /mnt/d/Desktop/Wuu && . /mnt/d/wuu-cache/cargo/env && RUSTUP_HOME=/mnt/d/wuu-cache/rustup cargo test"`

Known limitations:

- Stage1 parser still outputs formatted text rather than a structured AST.

## Milestone 2026-01-17: M4.12 Stage1 lexer check mode

Goal:

- Add a stage1 lexer check mode that verifies parity with stage0 tokens.

Changes made:

- Added `--check` to `wuu lex --stage1` to compare stage1 tokens against stage0:
  - `src/main.rs`
- Added CLI tests for stage1 lex check success and invalid utf-8 failure:
  - `tests/cli_stage1_lex_check_tests.rs`
- Added M4.12 milestone to the plan and updated `docs/NEXT.md` while working.

Acceptance criteria:

- Stage1 `--check` exits zero on a golden lexer fixture.
- Stage1 `--check` fails on invalid utf-8 input.
- `cargo test` passes.

Edge cases covered:

- Stage1 lex parity check on a valid lexer fixture.
- Stage1 lex check error on invalid utf-8 input.

Validation (WSL):

- `wsl -d Ubuntu -- bash -lc "cd /mnt/d/Desktop/Wuu && . /mnt/d/wuu-cache/cargo/env && RUSTUP_HOME=/mnt/d/wuu-cache/rustup cargo fmt --all"`
- `wsl -d Ubuntu -- bash -lc "cd /mnt/d/Desktop/Wuu && . /mnt/d/wuu-cache/cargo/env && RUSTUP_HOME=/mnt/d/wuu-cache/rustup cargo clippy --all-targets -- -D warnings"`
- `wsl -d Ubuntu -- bash -lc "cd /mnt/d/Desktop/Wuu && . /mnt/d/wuu-cache/cargo/env && RUSTUP_HOME=/mnt/d/wuu-cache/rustup cargo test"`

Known limitations:

- `wuu lex --check` is only supported with `--stage1`.

## Tooling 2026-01-17: GitHub HTTPS `SSL_ERROR_SYSCALL` (Windows) workaround

Issue observed:

- Git-for-Windows may fail to talk to `github.com:443` with `OpenSSL SSL_connect: SSL_ERROR_SYSCALL`.

Resolution:

- Use the Windows TLS backend (Schannel):
  - per-command: `git -c http.sslBackend=schannel -c http.schannelCheckRevoke=false push origin main`
  - permanent: `git config --global http.sslBackend schannel`
  - if Schannel still fails: `git config --global http.schannelCheckRevoke false`
- Repo tooling uses this to reduce autoloop stalls:
  - `scripts/autoloop.ps1` applies Schannel for network git commands
  - `prompt.md` uses Schannel for the required `git push`

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

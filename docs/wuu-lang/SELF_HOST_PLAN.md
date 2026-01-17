# Wuu Self-Host Plan (closed-loop)

This is a highly detailed plan from "spec + prototype" to a self-hosting subset (Wuu-in-Wuu).

Rule: every milestone must have a reproducible validation command that either passes or fails.

Primary host for validation on this machine: WSL Ubuntu.

## 0) Reality check (how we will "run until self-host")

I can keep implementing milestone-by-milestone as long as you keep this session going and keep asking to proceed.
To make progress persistent across sessions, this plan is designed to be executable by anyone:

- every milestone has acceptance criteria
- every change is gated by tests (`cargo test`) and lint (`cargo clippy -D warnings`) in WSL
- `docs/PROGRESS.md` is updated at each completed milestone

## 1) What already exists

- Roadmap outline: `docs/wuu-lang/BOOTSTRAP.md` (M0..M5, high level)
- Language spec: `docs/wuu-lang/SPEC.md` (includes v0 determinism/log/effects + minimal syntax sketch)
- Progress log: `docs/PROGRESS.md`
- Rust prototype CLI: `wuu fmt`, `wuu check` (currently only canonicalizes `effects{...}` / `requires{...}`)

## 2) Design choices (explicit tradeoffs)

These choices are made now to unblock implementation and make milestones verifiable.
If we later change them, we do so via versioned migrations and explicit tests.

### 2.1 Canonical formatting

- Formatter is normative: `wuu fmt` output is canonical.
- Formatter is defined by golden tests: for each input file, formatted output must match an expected snapshot.
- No "pretty but ambiguous" formatting; prefer minimal stable whitespace rules.

Tradeoff: we optimize for determinism and tooling over human preference. Humans can still write messy code; fmt normalizes.

### 2.2 Determinism rules (v0)

- Durable mode forbids floating-point driven decisions (already in SPEC).
- Durable mode forbids any nondeterminism outside logged effects.

Tradeoff: fewer features early, but replay is trustworthy and portable.

### 2.3 Workflow concurrency (v0 implementation choice)

Spec wants structured concurrency, but v0 toolchain will implement concurrency in phases:

- M1: allow concurrency in ephemeral mode (optional), but in durable mode steps execute sequentially and do not spawn tasks.
- Later milestone (post-M1): add durable structured concurrency with log-recorded ordering.

Tradeoff: we delay hard concurrency semantics until parser/type/effects/log pipeline is solid.

### 2.4 Workflow log encoding (v0 choice)

Choose: canonical CBOR for the workflow log payload encoding.

- Use canonical CBOR rules (deterministic map key ordering, single integer encoding).
- Avoid floats in durable logs (consistent with determinism rule).

Tradeoff: slightly more implementation work than JSON, but smaller/faster logs and a stronger long-term format.

### 2.5 Hashes (v0 choice)

- Use BLAKE3 for `program_hash` and content addressing.

Tradeoff: not a standard library primitive everywhere, but fast and easy in Rust; can be wrapped behind a stable interface.

## 3) Closed-loop workflow (how we work)

For every milestone:

1) Write/extend tests first (or golden snapshots first).
2) Make the test fail for the right reason.
3) Implement until tests pass.
4) Run in WSL:
   - `cargo fmt --all`
   - `cargo clippy --all-targets -- -D warnings`
   - `cargo test`
5) Append a short milestone entry to `docs/PROGRESS.md`:
   - what changed
   - how to validate
   - known limitations

WSL commands from Windows PowerShell:

```powershell
wsl -d Ubuntu -- bash -lc "cd /mnt/d/Desktop/Wuu && cargo fmt --all"
wsl -d Ubuntu -- bash -lc "cd /mnt/d/Desktop/Wuu && cargo clippy --all-targets -- -D warnings"
wsl -d Ubuntu -- bash -lc "cd /mnt/d/Desktop/Wuu && cargo test"
```

## 4) Milestones (all closed-loop)

Milestones are grouped by BOOTSTRAP phases (M0..M4). Each has:

- Deliverables: files/features to add
- Acceptance: commands/tests that must pass
- Done when: exact "green" criteria

### M0.1 Lexer (strings/comments/keywords are real)

Goal: stop doing substring rewrites; build a real token stream.

Deliverables:

- `src/lexer.rs` + tests
- Token types: identifiers, keywords, punctuation, string literals, comments, whitespace handling
- `wuu fmt` no longer touches keywords inside strings/comments

Acceptance:

- Add unit tests that cover:
  - `effects` inside a string is not treated as a decl
  - `effects` inside a comment is not treated as a decl
  - invalid UTF-8 is rejected early (if applicable)
- `cargo test` passes.

Done when:

- `format_source(...)` uses lexer tokens rather than raw scanning.

### M0.2 Parser for §16 subset (AST exists)

Goal: produce an AST for the minimal grammar in SPEC §16.

Deliverables:

- `src/ast.rs`, `src/parser.rs` + tests
- Parse: `Module`, `Fn`, `Workflow`, `EffectsDecl`, `Block`, minimal statements (`let`, `return`, `if`, `loop`, `step`)
- Parse errors have stable spans (line/col) so tests can assert failures.

Acceptance:

- Unit tests for parse success/failure cases.
- At least 10 "small files" in `tests/golden/parse/` that must parse.
- `cargo test` passes.

Done when:

- `wuu check` parses a `.wuu` file into an AST.

### M0.3 Canonical formatter (AST -> text) + golden snapshots

Goal: `wuu fmt` is canonical and idempotent on the supported subset.

Deliverables:

- `src/format.rs` + tests
- Golden snapshot harness (input -> expected output) under `tests/golden/fmt/`
- `wuu fmt --check <file>` returns non-zero if formatting differs

Acceptance:

- Add snapshot tests that enforce:
  - idempotence: `fmt(fmt(x)) == fmt(x)`
  - stability: whitespace changes are deterministic
- `cargo test` passes.

Done when:

- formatting rules are defined entirely by tests (no manual expectation).

### M0.4 Effect extraction and checking (subset)

Goal: implement SPEC §15 rules at least for direct calls in AST (not full typechecking yet).

Deliverables:

- Represent effect sets in AST (`effects { ... }`)
- `wuu check` verifies:
  - calling a function requiring `E_req` is only allowed under `E_decl` (set inclusion)
  - pure functions default to `effects {}`
- Tests for effect errors.

Acceptance:

- `tests/effects/*.wuu` with expected failure messages.
- `cargo test` passes.

Done when:

- we can write small multi-function examples and see effect errors deterministically.

### M0.5 Lock down workflow log schema (code + tests)

Goal: make SPEC §14 concrete in code.

Deliverables:

- `src/log/` module:
  - Rust structs for log records (WorkflowStart, StepStart, EffectCall, EffectResult, StepEnd, WorkflowEnd)
  - encode/decode using canonical CBOR (initially: encode with sorted-map types; decode tolerant of unknown fields)
- Tests:
  - roundtrip encode/decode
  - forward-compat: decoder ignores unknown fields

Acceptance:

- `cargo test` passes.

Done when:

- we can produce a deterministic byte-for-byte log record encoding in tests.

### M1.1 Minimal interpreter for pure subset (ephemeral)

Goal: run pure code for a small expression subset (enough to write tools later).

Deliverables:

- `wuu run <file> --entry <fn>` executes a chosen entry point in ephemeral mode.
- Minimal value types: Int, Bool, String; records optional.
- Minimal expressions: literals, variables, function calls, if, return.

Acceptance:

- `tests/run/*.wuu` with expected stdout (or expected return value).
- `cargo test` passes.

Done when:

- we can run a pure function from CLI deterministically.

### M1.2 Workflow runtime (replay-only first)

Goal: implement replay semantics before "real" effects.

Deliverables:

- `wuu workflow replay --log <path> --module <path> --entry <workflow>`
- Runtime checks:
  - next expected record must match the program's effect call
  - record mismatch fails with a stable error

Acceptance:

- Golden test with:
  - a workflow module
  - a pre-recorded log
  - replay succeeds and matches final outcome
- Another test: altered log fails deterministically.

Done when:

- we can replay a workflow without doing any real-world IO.

### M1.3 Typechecker (minimum to support Wuu-in-Wuu tools)

Goal: enough static checking to implement lexer/parser/formatter in Wuu later.

Deliverables:

- Types for: Int, Bool, String, Option, Result, Vec (or Array)
- Typechecking for function signatures, let bindings, if branches, calls

Acceptance:

- `tests/typeck/*.wuu` (ok + expected error cases).
- `cargo test` passes.

Done when:

- the subset is strong enough to write non-trivial string-processing code.

### M2.x WASM backend (defer until M1 stable)

We do not start WASM until M1 is stable (parser/fmt/check/run/replay are green with tests).
When we do, we add:

- IR lowering
- WASM codegen
- Host ABI stubs
- Equivalence tests: interpreter vs WASM for a small program set

### M3.x Evidence gates (tests/properties/benches)

We add:

- `example:` blocks become tests
- optional property testing
- bench harness with regression thresholds

Closed-loop: `wuu test` and `wuu bench` are both fully automated.

### M4.1 Define the "self-hosting subset" precisely

Goal: a written spec for the subset used for stage1 compiler components.

Deliverables:

- `docs/wuu-lang/SELF_HOST_SUBSET.md`:
  - syntax subset
  - standard library subset needed (strings, vec, map, io boundary)
  - forbidden features (float in durable, unsafe, etc.)

Acceptance:

- Review checklist inside the doc itself (must be fully filled).

Done when:

- there is zero ambiguity about what stage1 code is allowed to use.

### M4.2 Wuu-in-Wuu: lexer

Goal: write a lexer in Wuu that matches the Rust lexer for the subset.

Deliverables:

- `selfhost/lexer.wuu` + tests
- Rust stage0 compiler compiles it.
- A conformance test suite comparing token streams:
  - same inputs -> same token sequence as Rust lexer

Acceptance:

- `cargo test` passes and includes cross-check tests.

Done when:

- stage0 compiles the Wuu lexer, and token golden tests match.

### M4.3 Wuu-in-Wuu: parser + formatter (stage1)

Goal: stage1 (written in Wuu) can parse and format the subset.

Deliverables:

- `selfhost/parser.wuu`, `selfhost/format.wuu`
- Stage1 tool `wuu1 fmt` built by stage0 produces identical output to stage0 `wuu fmt` on golden inputs.

Acceptance:

- `tests/golden/fmt/` is run through both:
  - stage0 formatter
  - stage1 formatter
  and outputs must byte-match.

Done when:

- stage1 can replace stage0 for `fmt` on the subset (closed-loop equality).

### M4.4 Stage pipeline (stage0 -> stage1 -> stage2)

Goal: bootstrap loop is real.

Deliverables:

- stage0 builds stage1
- stage1 builds stage2 (same sources)
- stage1 and stage2 outputs match on golden tests (or match within a defined tolerance)

Acceptance:

- A scripted bootstrap test that runs end-to-end and fails on mismatch.

Done when:

- we have a repeatable bootstrap procedure that is stable across runs.

### M4.5 Wuu-in-Wuu: lexer (real)

Goal: replace the stage1 lexer stub with a real scanner that matches the Rust
lexer on the subset.

Deliverables:

- Add pure string intrinsics needed by stage1 lexer (typechecker + interpreter).
- Implement `selfhost/lexer.wuu` to scan and emit the same token stream format
  as the Rust lexer harness.
- Add a conformance test that runs stage1 lexer against the golden inputs.

Acceptance:

- `tests/golden/lexer/*.wuu` produce the same token stream in stage1 as in Rust.
- `cargo test` passes.

Done when:

- stage1 lexer output matches Rust tokenization on the golden suite.

### M4.6 Wuu-in-Wuu: parser + formatter (real)

Goal: replace stage1 parser/formatter stubs with real implementations for the
subset.

Deliverables:

- Implement `selfhost/parser.wuu` to parse the subset into a real AST.
- Implement `selfhost/format.wuu` to format that AST (no table-driven mapping).
- Add a conformance test that validates stage1 formatting against stage0.

Acceptance:

- `tests/golden/fmt/*.wuu` match stage0 output using the stage1 parser/formatter.
- `cargo test` passes.

Done when:

- stage1 parser/formatter are real and no longer table-driven.

### M4.7 Stage1 formatter CLI

Goal: expose the stage1 formatter through the CLI to validate self-hosted tooling.

Deliverables:

- `wuu fmt --stage1 <path>` runs the stage1 formatter (`selfhost/format.wuu`).
- Tests for stage1 fmt output and `--check` behavior.

Acceptance:

- Stage1 output matches stage0 on golden fmt fixtures.
- `wuu fmt --stage1 --check` fails on unformatted input.
- `cargo test` passes.

Done when:

- stage1 formatter can be exercised from the CLI with tests.

### M4.8 Stage1 formatter write mode

Goal: allow stage1 formatter to rewrite files in place (parity with common fmt workflows).

Deliverables:

- `wuu fmt --stage1 --write <path>` overwrites the file with stage1 output.
- `--write` conflicts with `--check`.
- CLI tests cover write behavior.

Acceptance:

- Stage1 `--write` updates a file to match the golden formatted output.
- `cargo test` passes.

Done when:

- stage1 formatter can rewrite files deterministically via CLI.

### M4.9 Stage1 lexer CLI

Goal: expose the stage1 lexer through the CLI for self-host validation.

Deliverables:

- `wuu lex --stage1 <path>` emits the same token stream format as the golden
  lexer fixtures.
- CLI tests for stage1 lexer output.

Acceptance:

- Stage1 lexer output matches `tests/golden/lexer/*.tok`.
- `cargo test` passes.

Done when:

- stage1 lexer can be exercised from the CLI with tests.

### M4.10 Stage1 parser conformance harness

Goal: validate that the stage1 parser consumes all tokens and matches stage0
formatting on the golden parse fixtures.

Deliverables:

- Update `selfhost/parser.wuu` to return a pair-encoded string
  (`formatted\n<SEP>\nrest_tokens`) so the host can detect leftover tokens.
- Add a no-progress guard in stage1 parsing to avoid infinite recursion when
  parsing invalid inputs.
- Add a conformance test that runs stage1 parsing on
  `tests/golden/parse/*.wuu`, compares to stage0 formatting output, and asserts
  there are no leftover tokens.

Acceptance:

- Stage1 parser output matches stage0 formatting for the golden parse fixtures.
- Stage1 parser leaves no unconsumed tokens on those fixtures.
- `cargo test` passes.

Done when:

- Stage1 parser can be exercised from Rust tests with token-consumption checks.

## 5) How far are we right now?

Current state (as of the latest entry in `docs/PROGRESS.md`):

- We have a Rust CLI and tests for a tiny decl formatter.
- We do NOT yet have lexer/parser/AST; we are pre-M0.1.

In this plan, "self-hosting subset" starts at M4.2.
So we are still early, but the next steps are clear and verifiable.

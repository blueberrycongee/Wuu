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

### M4.11 Stage1 parser CLI

Goal: expose the stage1 parser through the CLI and surface leftover tokens as a
stable error.

Deliverables:

- `wuu parse --stage1 <path>` runs `selfhost/parser.wuu` and prints the formatted
  output from the pair-encoded result.
- If stage1 parsing leaves leftover tokens, the CLI exits with a stable error.
- CLI tests cover success and leftover-token failure paths.

Acceptance:

- Stage1 parse output matches stage0 formatting on a fixture.
- Stage1 parse fails (non-zero) on an invalid input with leftover tokens.
- `cargo test` passes.

Done when:

- Stage1 parser can be exercised from the CLI with tests.

### M4.12 Stage1 lexer check mode

Goal: add a stage1 lexer check mode that verifies parity with stage0 tokens.

Deliverables:

- `wuu lex --stage1 --check <path>` runs both stage1 and stage0 lexers and
  fails if the token streams differ.
- `--check` is only supported with `--stage1`.
- CLI tests cover a passing fixture and invalid utf-8 failure.

Acceptance:

- Stage1 `--check` exits zero on a golden lexer fixture.
- Stage1 `--check` fails on invalid utf-8 input.
- `cargo test` passes.

Done when:

- Stage1 lexer parity can be verified from the CLI.

### M4.13 Stage1 formatter check parity

Goal: add a stage1 formatter check mode that verifies parity with stage0 output.

Deliverables:

- `wuu fmt --stage1 --check <path>` compares stage1 formatting to stage0 output
  and fails if they differ.
- CLI tests cover a passing fixture and a mismatch case.

Acceptance:

- Stage1 `--check` exits zero when stage1 matches stage0 for a fixture.
- Stage1 `--check` fails when stage1 output differs from stage0.
- `cargo test` passes.

Done when:

- Stage1 formatter parity can be verified from the CLI.

### M4.14 Stage1 formatter check strictness

Goal: make stage1 `--check` validate both parity and formatted input.

Deliverables:

- `wuu fmt --stage1 --check <path>` verifies:
  - stage1 output matches stage0 output
  - input is already formatted (matches stage1 output)
- CLI tests cover:
  - formatted input success
  - unformatted input failure with stable message
  - parity mismatch failure with stable message

Acceptance:

- Stage1 `--check` exits zero on a formatted fixture.
- Stage1 `--check` fails on unformatted input (even if parity matches).
- Stage1 `--check` fails on stage1/stage0 mismatch.
- `cargo test` passes.

Done when:

- Stage1 `fmt --check` is safe as a formatting gate and parity checker.

### M4.15 Stage1 formatter uses lex_tokens wrapper

Goal: route stage1 formatting through a lexing wrapper while keeping stack-safe
tokenization.

Deliverables:

- `selfhost/format.wuu` routes `format()` through `lex_tokens(...)`.
- `lex_tokens(...)` uses the host `__lex_tokens` intrinsic for stack safety.
- A test asserts the wrapper is used.

Acceptance:

- Stage1 formatter still matches stage0 output on golden fixtures.
- `selfhost/format.wuu` uses `lex_tokens` and includes a host-backed fallback.
- `cargo test` passes.

Done when:

- Stage1 formatting no longer calls `__lex_tokens` directly in `format()`.

### M4.16 Stage1 parser uses lex_tokens wrapper

Goal: route stage1 parsing through a lexing wrapper while keeping stack-safe
tokenization.

Deliverables:

- `selfhost/parser.wuu` routes `parse()` through `lex_tokens(...)`.
- `lex_tokens(...)` uses the host `__lex_tokens` intrinsic for stack safety.
- A test asserts the wrapper is used.

Acceptance:

- Stage1 parser still matches stage0 formatting on golden parse fixtures.
- `selfhost/parser.wuu` uses `lex_tokens` and includes a host-backed fallback.
- `cargo test` passes.

Done when:

- Stage1 parsing no longer calls `__lex_tokens` directly in `parse()`.

### M4.17 Stage1 lexer uses lex_tokens wrapper

Goal: route stage1 lexing through a wrapper while keeping stack-safe
tokenization.

Deliverables:

- `selfhost/lexer.wuu` routes `lex()` through `lex_tokens(...)`.
- `lex_tokens(...)` uses the host `__lex_tokens` intrinsic for stack safety.
- A test asserts the wrapper is used.

Acceptance:

- Stage1 lexer still matches Rust tokens on golden lexer fixtures.
- `selfhost/lexer.wuu` uses `lex_tokens` and includes a host-backed fallback.
- `cargo test` passes.

Done when:

- Stage1 lexing no longer calls `__lex_tokens` directly in `lex()`.

### M4.18 Stage1 lexer escapes token text

Goal: make stage1 lexer output match Rust's escaped token stream formatting.

Deliverables:

- `selfhost/lexer.wuu` post-processes `__lex_tokens` output to escape `\\`, `\n`,
  `\r`, and `\t` in token text.
- A golden lexer fixture includes escaped sequences.

Acceptance:

- Stage1 lexer matches Rust tokens on the new escape fixture.
- `cargo test` passes.

Done when:

- Stage1 lexer output is stable for backslash and whitespace escapes.

### M4.19 Stage1 lexer CLI covers escape fixtures

Goal: ensure CLI lexing covers escaped token output fixtures.

Deliverables:

- CLI tests exercise `wuu lex --stage1` and `--check` on the escape fixture.

Acceptance:

- CLI stage1 lex output matches `tests/golden/lexer/04_escapes.tok`.
- Stage1 `lex --check` succeeds on the escape fixture.
- `cargo test` passes.

Done when:

- Escape fixtures are covered in the CLI test suite.

### M4.20 Stage1 AST data model (structured parse output)

Goal: stage1 parser returns a structured AST instead of formatted text.

Deliverables:

- Define a tagged AST encoding (sum types) in `selfhost/parser.wuu` / `selfhost/format.wuu`.
- Update `selfhost/parser.wuu` to return AST values.
- Add Rust tests that parse `tests/golden/parse/*.wuu` and validate the AST tag
  plus token consumption.

Acceptance:

- Stage1 parser produces AST for all golden parse fixtures (tagged `Module`).
- Stage1 parse CLI continues to emit stage0 formatting until M4.21.
- `cargo test` passes.

Done when:

- Stage1 parse output is structured (no formatted-string pair output).

### M4.21 Stage1 formatter consumes AST end-to-end

Goal: stage1 formatting is driven by AST, not token streams.

Deliverables:

- Update `selfhost/format.wuu` to accept AST values.
- Update stage1 CLI paths to parse -> AST -> format for `fmt --stage1`.
- Add tests that validate AST->text formatting parity with stage0.

Acceptance:

- Stage1 formatter output matches stage0 on `tests/golden/fmt/*.wuu`.
- `wuu fmt --stage1 --check` still enforces parity + formatted input.
- `cargo test` passes.

Done when:

- Stage1 formatting no longer accepts token streams directly.

### M4.22 Stage1 diagnostics with spans

Goal: stage1 parser reports stable line/col spans on errors.

Deliverables:

- Add a span type and propagate spans into AST nodes in stage1.
- Update stage1 parser error reporting to include line/col ranges.
- Add fixtures that assert deterministic error messages and spans.

Acceptance:

- Stage1 parse errors are stable and include line/col info.
- CLI surface shows the stage1 error with spans.
- `cargo test` passes.

Done when:

- Stage1 diagnostics are usable without falling back to stage0 errors.

### M4.23 Stage1 lexer without host intrinsic (bounded mode)

Goal: reduce host dependency by adding a pure Wuu lexer path.

Deliverables:

- Implement a pure lexer path in `selfhost/lexer.wuu`.
- Keep `__lex_tokens` as a fallback for large inputs; add a size threshold.
- Add tests that force the pure path on golden fixtures and compare tokens.

Acceptance:

- Pure stage1 lexer matches Rust tokens on golden fixtures.
- Large-input path still uses host `__lex_tokens` for stack safety.
- `cargo test` passes.

Done when:

- Stage1 can lex without host support on bounded inputs.

### M4.24 Stage1 parser without host pair intrinsics

Goal: remove `__pair_left` / `__pair_right` from stage1 parsing.

Deliverables:

- Replace pair-encoded parsing with explicit list/stack structures.
- Update `selfhost/parser.wuu` to use iterative parsing where needed.
- Add tests that cover large parse fixtures without stack overflows.

Acceptance:

- Stage1 parser passes all golden parse fixtures without host pair helpers.
- `cargo test` passes.

Done when:

- Stage1 parsing no longer depends on host pair intrinsics.

Status:

- Done (2026-01-18).

### M4.25 Stage1 stdlib consolidation

Goal: centralize stage1 string and list helpers for reuse and testing.

Deliverables:

- Add `selfhost/stdlib.wuu` with tested string/list helpers used by lexer/parser.
- Update stage1 sources to use stdlib helpers.
- Add unit tests for stdlib helpers in Rust harness.

Acceptance:

- Stage1 lexer/parser still pass conformance tests.
- Stdlib helper tests cover edge cases (empty input, unicode rejection).
- `cargo test` passes.

Done when:

- Stage1 core logic is factored through a tested stdlib layer.

Status:

- Done (2026-01-18).

### M5.1 Stage1 bytecode VM (host) for subset

Goal: run stage1 tools on a dedicated bytecode VM instead of the interpreter.

Deliverables:

- Define a tiny bytecode/IR for the stage1 subset.
- Implement a Rust VM with deterministic execution.
- Add VM-vs-interpreter equivalence tests on `tests/run/*.wuu`.

Acceptance:

- VM produces identical outputs to the interpreter on the pure subset.
- `cargo test` passes.

Done when:

- The VM can run stage1 tools for small inputs.

Status:

- Done (2026-01-18): host VM supports intrinsics and runs stage1 lexer/parser/format on golden fixtures.

### M5.2 Stage1 compiler to bytecode (in Wuu)

Goal: compile the stage1 subset to the new bytecode.

Deliverables:

- Implement a stage1 compiler in Wuu (AST -> bytecode).
- Add tests that compile and run fixtures via the VM.

Acceptance:

- Compiled stage1 tools produce identical outputs to interpreter runs.
- `cargo test` passes.

Done when:

- Stage1 compiler can build stage1 tools into bytecode.

Status:

- In progress (2026-01-18): compiler emits jumps/labels for if/loop/step with explicit call
  arity and matches interpreter outputs for lexer/parser/format on small fixtures.
- Stage2 bootstrap test compares stage1 vs stage2 compiler output on a minimal fixture.

### M5.3 Stage2 bootstrap (stage1 -> stage2)

Goal: use the stage1 compiler to build stage2 and compare outputs.

Deliverables:

- Bootstrap test: stage1 compiler builds stage2 bytecode.
- Run stage2 tools on golden fixtures and compare to stage1 outputs.
- Check in stage2 tool bytecode artifacts and compare against golden files.

Status:

- In progress (2026-01-19): stage2 compiler parity test is in place; stage2 lexer parity
  check runs by default; parser/format parity checks require `WUU_SLOW_TESTS=1`.
- In progress (2026-01-19): stage2 tool bytecode artifacts are checked into
  `tests/golden/stage2/*.bytecode.txt` and updated via `WUU_UPDATE_GOLDENS=1`.

Acceptance:

- Stage2 outputs match stage1 outputs on golden suites.
- Stage2 tool bytecode matches golden artifacts.
- `cargo test` passes.

Done when:

- Stage2 is a reproducible, validated artifact.

### M5.4 Host intrinsic inventory and reduction

Goal: document and shrink the remaining host intrinsics.

Deliverables:

- Add a `docs/wuu-lang/HOST_INTRINSICS.md` inventory.
- Replace any remaining pure intrinsics with Wuu code where possible.
- Add tests that enforce the allowed intrinsic list.

Acceptance:

- The intrinsic list is explicit and enforced by tests.
- `cargo test` passes.

Done when:

- Remaining host dependencies are documented and minimal.

### M6.0 Full self-host bootstrap (stage2 -> stage3)

Goal: the toolchain can compile itself end-to-end with deterministic artifacts.

Deliverables:

- Complete the M6.x sub-milestones below (M6.1-M6.4).

Acceptance:

- M6.1-M6.4 acceptance criteria are all green.
- `cargo test` passes.

Done when:

- Self-hosted toolchain is reproducible and deterministic.

### M6.1 Stage2 -> Stage3 build outputs (artifact contract)

Goal: define a stable stage3 artifact format and build output contract.

Deliverables:

- A dedicated output directory and naming scheme for stage3 artifacts.
- A manifest or checksum file that captures the exact build inputs.
- Tests that assert the artifact contract is deterministic.

Acceptance:

- Stage2 produces identical stage3 artifacts across two consecutive runs.
- Artifact manifest/checksum matches the produced outputs.
- `cargo test` passes.

Done when:

- Stage3 artifact format and output paths are stable and tested.

### M6.2 Stage2 builds stage3

Goal: stage2 compiler builds stage3 using only Wuu code plus minimal runtime.

Deliverables:

- Stage2 build pipeline for stage3 (script or test harness).
- Tests that compile stage3 and run stage3 tools on golden suites.

Acceptance:

- Stage3 tools pass golden `fmt/parse/lex` fixtures.
- No fallback to stage0 in the stage3 path.
- `cargo test` passes.

Done when:

- Stage3 artifacts are produced by stage2 and validated by tests.

### M6.3 Stage3 self-compile parity

Goal: stage3 compiles stage3 with byte-for-byte identical outputs.

Deliverables:

- A bootstrap test that runs stage3 -> stage3 and compares outputs.
- Stable output hashing for comparison.

Acceptance:

- Stage3 self-compile outputs match the stage2-built stage3 artifacts.
- `cargo test` passes.

Done when:

- Stage3 is reproducible and self-compiled without differences.

### M6.4 Self-hosted CLI mode

Goal: expose a CLI mode that uses stage3 for core tooling.

Deliverables:

- `wuu fmt/check/lex/parse --selfhost` uses stage3.
- CLI tests that ensure `--selfhost` output matches stage0 on golden suites.

Acceptance:

- `--selfhost` mode passes golden fixtures and parity checks.
- `cargo test` passes.

Done when:

- The CLI can run fully self-hosted tools by default or via flag.

### M6.5 Business language core: modules and imports

Goal: support multi-module projects with explicit imports.

Deliverables:

- Module syntax and resolver rules (single-root packages).
- `wuu check` resolves imports and reports stable errors.
- Golden fixtures for multi-module programs.

Acceptance:

- Import resolution works on test fixtures.
- Stable error spans for missing or cyclic imports.
- `cargo test` passes.

Done when:

- Multi-module project structure is supported.

### M6.6 Business language core: collections and records

Goal: first-class collection literals and record manipulation.

Deliverables:

- List and map literals, indexing, and basic iteration.
- Record updates and field access with stable typing rules.
- Formatter support for the new syntax.

Acceptance:

- Typechecking and formatting pass new fixtures.
- `cargo test` passes.

Done when:

- Collections and records are usable for business logic.

### M6.7 Business language core: error handling

Goal: ergonomic error paths with `Result`/`Option` and early returns.

Deliverables:

- `Result`/`Option` syntax and typechecking rules.
- `try`/`?`-style propagation or explicit match-based patterns.
- Tests for deterministic error messages.

Acceptance:

- Error propagation works in interpreter and typechecker.
- `cargo test` passes.

Done when:

- Business code can handle failures without ad-hoc patterns.

### M6.8 Business language core: IO boundaries and effects

Goal: make file/system IO explicit and capability-gated.

Deliverables:

- Effects for file read/write and clock/time.
- Runtime stubs for IO with deterministic logging.
- Tests that enforce effect gating and replay safety.

Acceptance:

- IO requires declared effects in `wuu check`.
- IO calls are logged and replayable.
- `cargo test` passes.

Done when:

- IO is usable and auditable under capabilities.

### M6.9 Business language core: data formats (JSON/CSV)

Goal: parse and emit structured data needed for business apps.

Deliverables:

- Minimal JSON and CSV parsing/serialization in stdlib or host bridges.
- Tests covering invalid inputs and roundtrips.

Acceptance:

- Data parsing works on fixtures with stable error messages.
- `cargo test` passes.

Done when:

- The language can process real-world business data.

### M7.0 Medium project capability (real product in Wuu)

Goal: Wuu can ship a medium-sized, real-world business app with evidence gates.

Definition of "medium project":

- 3-10 modules, 2-5k LOC of Wuu code (excluding tests).
- Uses IO, structured data, and non-trivial business rules.
- Has evidence blocks that cover key requirements and edge cases.

Deliverables:

- Complete M7.x sub-milestones below (M7.1-M7.5).

Acceptance:

- M7.1-M7.5 acceptance criteria are all green.
- `cargo test` passes.

Done when:

- A real medium app is built in Wuu and validated end-to-end.

### M7.1 Project definition and evidence baseline

Goal: lock the real-world target app and define executable intent.

Deliverables:

- One-page project brief: inputs, outputs, constraints, success criteria.
- Evidence blocks that encode the top 3 requirements and 3 edge cases.
- Sample input/output fixtures under `examples/<app>/`.

Acceptance:

- Evidence blocks run and fail before implementation.
- Fixtures are checked into the repo.
- `cargo test` passes (expected failures are explicitly asserted).

Done when:

- The target app is frozen and intent is executable.

### M7.2 Data model + parsing layer

Goal: implement the data structures and parsers required by the app.

Deliverables:

- Domain types in Wuu.
- Parsing functions from JSON/CSV or flat text inputs.
- Evidence tests for valid and invalid inputs.

Acceptance:

- Parsers accept valid fixtures and reject invalid ones deterministically.
- `cargo test` passes.

Done when:

- The app can ingest its real inputs.

### M7.3 Core business rules and classification

Goal: implement the non-trivial business logic.

Deliverables:

- Rule engine or rule functions with clear ordering.
- Evidence tests for correctness and edge cases.

Acceptance:

- Rule outputs match expected fixtures.
- `cargo test` passes.

Done when:

- The app produces correct domain outputs.

### M7.4 Output generation + CLI interface

Goal: produce user-visible output and provide a runnable CLI.

Deliverables:

- Report generation (summary + detail output).
- `wuu run` or `wuu app` entrypoint with flags/config.
- Docs for running the app and sample command lines.

Acceptance:

- CLI produces the documented outputs on fixtures.
- `cargo test` passes.

Done when:

- The app is runnable end-to-end from CLI.

### M7.5 Audit log and replay verification

Goal: make the app fully auditable and replayable.

Deliverables:

- Structured log output for all effectful operations.
- Replay command that validates a prior run.
- Evidence tests that validate replay and mismatch errors.

Acceptance:

- Replay succeeds on recorded fixtures and fails on tampered logs.
- `cargo test` passes.

Done when:

- The app is auditable and replayable end-to-end.

## 5) How far are we right now?

Current state (as of the latest entry in `docs/PROGRESS.md`):

- M4.19 is complete (stage1 lexer/parser/formatter parity + CLI coverage).
- Stage1 parsing still returns formatted text and relies on host intrinsics.
- Next step is M4.20 (stage1 AST output) to move toward stage2 bootstrap.

In this plan, "self-hosting subset" starts at M4.2.
So we are still early, but the next steps are clear and verifiable.

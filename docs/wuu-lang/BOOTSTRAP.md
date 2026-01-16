# Wuu — Bootstrap Plan (self-hosting roadmap)

This document answers: “we start from which language, and how do we eventually self-host?”

---

## Starting point (chosen)

- Implement the v0 toolchain in **Rust** first.
  - Practical reasons: mature parsing ecosystem, great WASM tooling, strong safety, good performance, cross-platform.
  - Strategic reasons: we can mirror Wuu’s intended semantics (ownership, no ambient authority) in the implementation.

This is an implementation choice, not a language requirement.

---

## Milestones

### M0 — Spec + golden tests

- Lock the core grammar, canonical formatter behavior, and type/effect rules via snapshot tests.
- Define “durable workflow” event log format (versioned) and replay invariants.

### M1 — Reference interpreter (correctness-first)

- Parser + canonical formatter.
- Type checker + effect checker.
- Bytecode interpreter that runs:
  - pure code deterministically,
  - effect calls through a host shim (capabilities),
  - workflow replay by consuming a recorded log.

Deliverable: `wuu check`, `wuu fmt`, `wuu run`, `wuu workflow replay` for a small subset.

### M2 — WASM backend (deployment-first)

- Compile core IR to WASM.
- Define the host ABI for:
  - capability imports (Net/Fs/Clock/Store),
  - workflow log read/write,
  - structured concurrency primitives (if host-managed).

Deliverable: run the same workflow in interpreter and WASM and prove equivalence by replay logs.

### M3 — Standard library + evidence

- `Result/Option`, collections, text, crypto primitives (as capabilities or pure libs).
- Contracts and executable examples as tests.
- Property tests and bench harness (hosted).

Deliverable: closed-loop “evidence gates” in `wuu test` and `wuu bench`.

### M4 — “Self-hosting subset” (Wuu-in-Wuu)

- Define a restricted subset of Wuu that can implement:
  - lexer/parser,
  - formatter,
  - a small type checker.
- Re-implement parts of `wuu` in Wuu, still built by Rust `wuu` initially.

Deliverable: the language can build non-trivial tools written in itself.

### M5 — Full self-host (optional)

- Rewrite the entire compiler pipeline in Wuu.
- Keep Rust as a “trusted minimal bootstrap” (like stage0) if desired.

---

## Why “WASM first” still allows peak performance later

- The semantics stay stable: effects/capabilities/durability are defined at the language level.
- Performance-sensitive components can:
  - use `unsafe` + FFI to native modules, or
  - target LLVM later while still obeying the same effect/durability contracts.

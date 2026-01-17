# Self-Hosting Subset

This document defines the exact Wuu subset permitted for stage1 (self-hosted)
compiler components. It is a strict, closed set: anything not listed is
forbidden.

## Syntax Subset

Top-level items:

- `fn` only.
- No `workflow`, `type`, `record`, or `enum` items.

Function signatures:

- All parameters must be explicitly typed.
- Return type is required and must be a concrete type from the standard subset.

Statements:

- `let` (with optional type annotation).
- `return` (expression required).
- `if` / `else` blocks.
- Expression statement (value is discarded).

Expressions:

- Literals: `Int`, `Bool`, `String`.
- Identifiers (local variables).
- Function calls (single-segment names only).
- No paths, no field access, no indexing, no match, no loops.

Effects and contracts:

- `effects` / `requires` blocks are forbidden in this subset.
- `pre` / `post` / `invariant` contracts are forbidden.

## Standard Library Subset

All standard types are nominal and non-generic for stage1:

- `Int`, `Bool`, `String`.
- `Option`, `Result`, `Vec`, `Map` are reserved names but NOT available yet.

Allowed functions are limited to stage1-local pure functions. There is no
ambient IO. Any IO boundary is handled outside stage1 by the host toolchain.

## Forbidden Features

The following are explicitly disallowed for stage1:

- `workflow`, `step`, or any durable replay semantics.
- Floating point types or operations.
- `unsafe` blocks or any unsafe capability.
- `loop`, `match`, and pattern matching.
- Type/record/enum declarations.
- Effects, capability declarations, or host calls.
- Global state, randomness, time, or concurrency primitives.

## Review Checklist

- [x] Allowed top-level items are fully enumerated.
- [x] Allowed statements and expressions are fully enumerated.
- [x] Required typing rules for stage1 functions are explicit.
- [x] Standard library subset is explicit and minimal.
- [x] Forbidden features list is complete and unambiguous.
- [x] IO boundary and effect restrictions are explicit.

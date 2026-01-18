# Bytecode Text Format (Stage1 Compiler)

This document defines the stable text format emitted by the stage1 compiler and
consumed by the host bytecode decoder.

Format rules:

- One instruction per line.
- Lines may have leading spaces; trailing spaces are significant for strings.
- Empty/whitespace-only lines are ignored.
- Functions are delimited by `fn <name>` and `end`.
- Parameters are declared with `param <name>` lines (order is significant).

Instruction set:

- `const_int <i64>`
- `const_bool true|false`
- `const_string <escaped>` (empty string is `const_string` with no payload)
- `const_unit`
- `load <name>` (params or locals)
- `store <name>` (declares locals on first store)
- `pop`
- `call <name> <argc>`
- `call_builtin <name> <argc>`
- `label <name>`
- `jump <name>`
- `jump_if_false <name>`
- `return`

Argument counting:

- The compiler emits explicit `<argc>` for all calls.
- Legacy `arg` markers are still accepted by the decoder but should not be
  emitted by new compilers.

String escaping:

- `\\n`, `\\r`, `\\t`, `\\\\`, and `\\\"` are recognized.
- Other characters are emitted verbatim.

Determinism requirements:

- Locals are allocated by first `store` occurrence in AST traversal order.
- Labels are derived from node spans (`if_<span>_else`, `if_<span>_end`) to keep
  output stable across runs.

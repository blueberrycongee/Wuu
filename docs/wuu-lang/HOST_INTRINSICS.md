# Host Intrinsics Inventory

This document tracks the host-provided intrinsics that are still allowed for
stage1 and the self-host toolchain. It is the source of truth for M5.4.

## Allowed intrinsics (stage1)

String helpers:

- `__str_eq`
- `__str_is_empty`
- `__str_concat`
- `__str_head`
- `__str_tail`
- `__str_starts_with`
- `__str_strip_prefix`
- `__str_take_whitespace`
- `__str_take_ident`
- `__str_take_number`
- `__str_take_string_literal`
- `__str_take_line_comment`
- `__str_take_block_comment`
- `__str_is_ident_start`
- `__str_is_digit`
- `__str_is_ascii`

AST helpers:

- `__ast_escape`
- `__ast_unescape`
- `__ast_left`
- `__ast_right`

Lexer helpers:

- `__lex_tokens`
- `__lex_tokens_spanned`

Pair helpers (legacy; should be removed from stage1 paths):

- `__pair_left`
- `__pair_right`

## Notes

- Intrinsics are expected to be pure and deterministic.
- Any new intrinsic requires updating this list and the enforcement test.

# Stage3 Artifact Contract (Draft)

This document defines the deterministic artifacts that constitute the stage3
toolchain outputs and how they are compared for self-hosting validation.

## Artifact set

The stage3 artifact set is the bytecode outputs for the stage3 compiler and
stage3 tools, produced by stage2 using a fixed input set:

- `stage3/compiler.bytecode.txt`
- `stage3/lexer.bytecode.txt`
- `stage3/parser.bytecode.txt`
- `stage3/format.bytecode.txt`

These are stored under `tests/golden/stage3/` and compared against a manifest.

## Serialization format

All artifacts use the canonical text bytecode format defined in:

- `docs/wuu-lang/BYTECODE_TEXT_FORMAT.md`

Rules:

- Stable function ordering and label naming (per compiler rules).
- UTF-8 text with `\n` line endings and a trailing newline.

## Hash manifest

Artifacts are checked with SHA-256 hashes recorded in:

- `tests/golden/stage3/manifest.sha256`

Format (one per line):

```
<sha256>  <relative-path>
```

## Fixed input set

Stage3 artifacts are built from a fixed input set consisting of:

- `selfhost/stdlib.wuu`
- `selfhost/lexer.wuu`
- `selfhost/parser.wuu`
- `selfhost/format.wuu`
- `selfhost/compiler.wuu`

No other inputs are allowed for the stage3 build step.

## Comparison rules

- Stage2 builds stage3 artifacts from the fixed input set.
- Stage3 artifacts must byte-for-byte match the golden set and manifest.
- Any change to the input set or compiler output must update the manifest.

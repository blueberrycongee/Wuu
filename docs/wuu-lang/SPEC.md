# Wuu — Agent-Native General-Purpose Language Spec (v0 draft)

This document specifies a new general-purpose language designed for the “post-agent” era: long-running autonomous programs that continuously plan, execute, verify, and recover.

Status: draft.

---

Implementation + roadmap notes:

- Self-host roadmap (high level): `docs/wuu-lang/BOOTSTRAP.md`
- Self-host plan (closed-loop milestones): `docs/wuu-lang/SELF_HOST_PLAN.md`

## 0. Design Thesis

In the post-agent era, “good language design” shifts from human ergonomics to *closed-loop reliability*:

- **Semantics-first**: unambiguous, machine-checkable meaning; minimal “implementation-defined” behavior.
- **Evidence-first**: changes are accompanied by machine-checkable evidence (types/contracts/tests/bench gates).
- **Durable-first**: long-running programs survive crashes, upgrades, and partial failures via replayable execution.
- **Capability-first**: no ambient authority; all side effects require explicit, auditable capabilities.
- **Performance without heroics**: fast by default, and faster via explicit, verifiable “performance knobs”.

---

## 1. Non-Goals (v0)

- “One language rules all” (GPU kernels, UI, infra) in v0.
- Dependent types / full formal proofs in v0 (contracts + properties instead).
- Implicit, global reflection-heavy metaprogramming.
- “Magic” concurrency primitives with unclear cancellation/ordering semantics.

---

## 2. Core Architecture (chosen defaults)

### 2.1 Execution targets

- **Primary target (v0)**: **WASM** modules executed inside a **Wuu Host Runtime**.
  - Rationale: sandboxing, portability, deterministic behavior surface, capability injection, and auditability.
- **Secondary target (future)**: **LLVM native backend** for high-performance deployments.
  - Rationale: peak CPU performance and deeper system integration, while keeping core semantics identical.

### 2.2 Memory model (v0)

Goal: memory safety + predictable latency + replay-friendly behavior.

- **No tracing GC** in v0.
- **Owned values + lexical borrows (Rust-lite)**:
  - Each value has a single owner; moves transfer ownership.
  - Temporary references (borrows) are lexical and cannot outlive their scope.
  - Mutable aliasing is forbidden in safe code.
- **Scoped arenas** for short-lived allocations:
  - Every `step` (see durability) has an implicit arena; allocations can be bulk-freed at step end.
  - Durable state is stored separately (serialization boundary).
- `unsafe` exists, but is isolated and requires explicit proof obligations (see §10).

This gives predictable behavior for long-running agents (bounded leaks, clear lifetimes) while remaining performant.

---

## 3. Language Surface (syntax philosophy)

### 3.1 “Controlled English”, not natural language

Wuu is **not** natural-language programming.

Instead it uses:

- conventional programming syntax for computation;
- *controlled, unambiguous keywords* for intent: `requires`, `pre`, `post`, `invariant`, `policy`.

Rule: every program has a single parse and a single meaning (no synonymy, no context-dependent grammar).

### 3.2 Deterministic formatting

`wuu fmt` produces a canonical representation. Canonical formatting is part of the spec (tooling is language).

---

## 4. Type System (v0)

- Algebraic data types (sum/product), pattern matching with exhaustiveness.
- Parametric polymorphism (generics).
- Traits/interfaces (minimal) for ad-hoc polymorphism.
- `Result<T, E>` and `Option<T>` are standard.
- No `null`.

### 4.1 Errors

- No exceptions in v0.
- Error propagation uses `Result` and the `?` operator.

---

## 5. Effects + Capabilities (v0 cornerstone)

### 5.1 Capability model (no ambient authority)

Side effects require explicit capability values provided by the host runtime, e.g.:

- `Net.Http`
- `Fs.Read`
- `Fs.Write`
- `Clock`
- `Process.Spawn`
- `Store.Kv`

Code cannot do IO “just because” it imported a module.

### 5.2 Effect typing

Functions declare their effect requirements. Two equivalent surface forms are allowed:

- `requires { net:http, store:kv }`
- `effects { Net.Http, Store.Kv }`

The compiler checks that:

- pure functions have no effects;
- a caller has (or can receive) the required capabilities.

v0 prefers explicit effect annotations; limited inference is allowed if it is sound and deterministic.

---

## 6. Durability + Replay (v0 cornerstone)

### 6.1 Two execution modes

- **Ephemeral mode**: ordinary computation; termination is expected; no replay guarantees.
- **Durable mode**: `workflow` functions run under event-sourced execution with replay.

### 6.2 Workflow model

`workflow` defines a deterministic state machine:

- All non-determinism (time/random/IO) must go through effects/capabilities.
- The runtime records an **event log** for each workflow execution:
  - effect calls (inputs/outputs),
  - step boundaries,
  - failures and retries,
  - explicit durable state snapshots/checkpoints (optional).

On restart, the runtime **replays** the workflow by re-feeding recorded effect results to reach the same state.

### 6.3 Step boundaries

Durable code is segmented into `step`s:

- A `step` is an atomic unit for retry, idempotency, and audit.
- A `step` may allocate freely in its arena; the arena is freed on step exit.
- A `step` may commit durable state changes.

Rule: durable state updates are only allowed at step boundaries (or via transactional APIs that are log-recorded).

---

## 7. Concurrency (structured, replay-aware)

- **Structured concurrency**: tasks live within a lexical `scope`.
- **Cancellation** is explicit and propagates down.
- **Timeouts/retries** are part of the durable semantics (recorded in the log).

Replay constraints:

- scheduling-sensitive nondeterminism must be controlled (either deterministic scheduling or log-recorded ordering).
v0 chooses: *log-recorded ordering for workflow-visible results*.

---

## 8. Contracts + Evidence (v0 cornerstone)

Wuu programs can embed machine-checkable “evidence” alongside code:

- `pre:` and `post:` contracts for functions
- `invariant:` for types/modules/workflows
- `example:` executable examples (become tests)
- `property:` randomized properties (property-based testing)
- `bench:` microbenchmarks with regression thresholds

Policy option: in production durable workflows, host may require that certain effects (e.g. `Fs.Write`, `Process.Spawn`) are allowed only when evidence gates pass.

---

## 9. Module/Package System (v0)

- Package manifest: `wuu.toml` (name, version, dependencies, capabilities policy).
- Semantic versioning with machine-readable API changes.
- Reproducible builds: lockfile + content hashes.

---

## 10. Unsafe + FFI (escape hatch, but auditable)

`unsafe` exists for:

- calling native modules,
- zero-copy buffers,
- specialized kernels.

Constraints:

- `unsafe` blocks are lexically scoped and explicit.
- Each `unsafe` block must declare an obligation set (checked by tooling) such as:
  - bounds, alignment, aliasing, thread-safety, panic boundary.
- Durable workflows may restrict `unsafe` by policy.

Interop v0:

- WASI for baseline syscalls (as allowed).
- Host-provided capability imports for privileged operations.

---

## 11. Toolchain Contract (part of the language)

`wuu` CLI is normative for:

- `wuu fmt` canonical formatting
- `wuu check` type/effect checking
- `wuu test` (examples + properties)
- `wuu bench` (with regression gates)
- `wuu run` (ephemeral)
- `wuu workflow run/replay` (durable)

Compiler services (machine API):

- parse to stable AST,
- semantic rename/refactor primitives,
- semantic diff output,
- automated migration scaffolds between versions.

---

## 12. Minimal Example (informal)

See `docs/wuu-lang/EXAMPLES.md` for a sketch of a durable agent program that:

- reads a repository snapshot (capability),
- proposes a patch (pure computation),
- runs tests (capability),
- applies patch and records evidence (capability),
- is replayable and crash-safe.

---

## 13. Determinism (normative for durability)

This section defines the minimum determinism contract required for `workflow` replay.

### 13.1 Determinism boundary

In **durable mode**, program execution must be a deterministic function of:

- the workflow input arguments,
- the program/module bytes,
- the workflow event log for that execution,
- the host runtime versioned semantics for log interpretation (see §14.1).

All sources of nondeterminism **must** be expressed as effect calls and therefore appear in the log, including:

- time (`Clock`),
- randomness (`Rand`),
- IO (`Fs.*`, `Net.*`),
- process execution (`Process.Spawn`),
- external state (`Store.*`).

### 13.2 Disallowed sources of nondeterminism (v0)

In durable mode, the following are forbidden in safe code unless their outcomes are fully determined by logged inputs:

- reading uninitialized memory,
- depending on pointer/address identity,
- relying on hash iteration order unless the hash algorithm/seed is specified by the language (v0: not specified; therefore order is unspecified and must not be observed),
- data races (safe code must be race-free),
- wall-clock time, entropy, and ambient OS state outside capabilities.

### 13.3 Floating-point (v0 conservative rule)

To keep replay portable, v0 treats floating-point results as potentially platform-dependent.

- In durable mode, any computation whose externally-visible decisions depend on floating-point behavior must be either:
  - performed via a host capability whose results are logged, or
  - performed using a specified deterministic numeric mode (future work).

Ephemeral mode places no such restriction.

---

## 14. Workflow Event Log (v0)

This section defines the on-disk (or remote) log representation for durable workflows.

### 14.1 Versioning

Each workflow execution has a single append-only log with:

- `log_version`: a semver-like tuple `{ major, minor }`.
  - `major` changes may break replay compatibility.
  - `minor` changes must be backward compatible: older runtimes must be able to ignore unknown record kinds/fields without changing the meaning of known records.
- `runtime_id`: an identifier for the host runtime build that produced the log.
- `program_hash`: a cryptographic hash of the workflow module bytes (or package lock graph) used to produce the log.

Replay requires that the host runtime either:

- implements the recorded `log_version`, or
- provides a verified migration from recorded version to a supported version.

### 14.2 Record ordering

The log is a linear sequence of records. The runtime must append records in program order such that replay can reconstitute the same control-flow decisions.

### 14.3 Record kinds (minimum set)

The following record kinds are required in v0.

`WorkflowStart`

- fields: `workflow_name`, `args` (canonical encoding), `run_id`

`StepStart`

- fields: `step_id`, `step_name` (string), `attempt` (u32)

`EffectCall`

- fields: `call_id`, `capability` (e.g. `Fs.Read`), `op` (string), `input` (canonical encoding)

`EffectResult`

- fields: `call_id`, `outcome` (`Ok`/`Err`), `output` (canonical encoding)

`StepEnd`

- fields: `step_id`, `outcome` (`Ok`/`Err`)

`WorkflowEnd`

- fields: `outcome` (`Ok`/`Err`)

Optional but recommended:

`Checkpoint`

- fields: `checkpoint_id`, `state_digest`, `state_blob` (canonical encoding or content-addressed reference)

`Failure`

- fields: `kind` (string), `message` (string), `data` (optional canonical encoding)

### 14.4 Canonical encoding (v0)

The log must use a canonical, deterministic encoding for all structured values.

v0 requirement:

- the encoding must be stable across platforms and runtime builds,
- integers must have a single representation,
- maps/records must have a deterministic field ordering.

The specific encoding is not fixed in this draft (e.g., canonical CBOR, canonical JSON, or a custom binary format are acceptable), but once chosen it becomes normative for v0 logs.

### 14.5 Replay algorithm (normative sketch)

Replay executes the workflow code while consuming the recorded log:

- on `EffectCall`, the runtime verifies the call matches the recorded `capability/op/input`,
- the runtime supplies the recorded `EffectResult` instead of performing the real-world effect,
- step retries are driven by the log (not by re-evaluating nondeterministic policy at replay time),
- if the running program attempts an effect call that does not match the next expected record, replay fails.

---

## 15. Effects and capabilities (type rules)

This section defines the minimum static rules for effect checking.

### 15.1 Terminology

- A **capability** is a value that authorizes a class of operations (e.g. `Fs.Read`).
- An **effect set** is a finite set of capability types.
- A **pure** expression has an empty effect set.

### 15.2 Function effect declarations

Each function must have one of:

- an explicit effect set declaration, or
- no declaration, which is equivalent to `effects {}` (pure).

In surface syntax:

- `effects { Net.Http, Store.Kv }` declares an effect set.
- `requires { net:http, store:kv }` is an equivalent alias in v0 surface form.

### 15.3 Subsumption rule

A function with declared effects `E_decl` may call another function requiring `E_req` iff:

- `E_req ⊆ E_decl`.

This is the only effect subtyping rule in v0 (set inclusion).

### 15.4 Effect inference (v0 limited)

Inference is permitted only when it is deterministic and does not change public API meaning:

- within a function body, the compiler may compute the minimal required effect set from callees and direct capability use,
- the result must be checked against any explicit `effects { ... }` declaration.

Exported functions in a package should prefer explicit effect annotations to keep API stable.

### 15.5 Capability passing (no ambient authority)

In v0, there is no implicit global capability.

- The only way to perform an effect is to use a capability value provided by parameters, locals, or host-injected bindings.
- Importing a module does not grant effects.

### 15.6 Durable workflows

`workflow` functions are ordinary functions with additional runtime semantics (logging/replay). The type/effect rules above still apply.

In addition:

- durable workflows must not use capabilities that the host cannot log/replay under the selected policy,
- any capability used in a workflow step must have its call/return fully representable in the log encoding (see §14.4).

---

## 16. Minimal surface syntax (provisional but testable)

This section pins a small, stable subset of the surface syntax so tooling can be built and tested.

### 16.1 Lexical conventions (v0)

- `Ident`: ASCII letters/digits/underscore, not starting with a digit.
- `Path`: dot-separated `Ident` segments (e.g. `Net.Http`, `Store.Kv`).
- Keywords include: `fn`, `workflow`, `type`, `record`, `enum`, `let`, `if`, `else`, `match`, `loop`, `return`, `step`, `effects`, `requires`, `pre`, `post`, `invariant`, `unsafe`.
- Strings are UTF-8.

### 16.2 Grammar sketch (EBNF-like)

This is not the full grammar; it is the minimal subset intended for M0 golden tests.

```
Module      := { Item } ;
Item        := Fn | Workflow | TypeDecl ;

Fn          := "fn" Ident "(" [ Params ] ")" [ "->" Type ]
               [ EffectsDecl ] { Contract } Block ;

Workflow    := "workflow" Ident "(" [ Params ] ")" [ "->" Type ]
               [ EffectsDecl ] { Contract } Block ;

EffectsDecl := ("effects" "{" EffectList "}") | ("requires" "{" RequireList "}") ;
EffectList  := Path { "," Path } [ "," ] ;
RequireList := Ident ":" Ident { "," Ident ":" Ident } [ "," ] ;

Contract    := ("pre" ":" Expr) | ("post" ":" Expr) | ("invariant" ":" Expr) ;

TypeDecl    := ("type" Ident "=" Type ";")
             | ("record" Ident "{" { Field "," } "}")
             | ("enum" Ident "{" { Variant "," } "}") ;

Block       := "{" { Stmt } "}" ;
Stmt        := Let | If | Loop | Step | Return | Expr ";" ;

Step        := "step" String Block ;
Let         := "let" Ident [ ":" Type ] "=" Expr ";" ;
If          := "if" Expr Block [ "else" Block ] ;
Loop        := "loop" Block ;
Return      := "return" [ Expr ] ";" ;

Expr        := ... ;
```

### 16.3 `step` semantics (v0)

- A `step` is only valid inside a `workflow`.
- Entering a step begins a new log segment (`StepStart`).
- Exiting a step appends `StepEnd` with the outcome.
- Durable state commits (host-provided) are only permitted within a step and become visible to the next step.

---

## 17. Open questions (tracked gaps for v0)

- Exact canonical encoding choice for logs and hashes (see §14.4).
- Deterministic numeric mode (or a pure-integer numeric standard library) for durable mode (see §13.3).
- Structured concurrency record model: whether task scheduling must be deterministic or only workflow-visible results are logged (see §7).
- `wuu.toml` schema and how capability policy is declared and enforced.

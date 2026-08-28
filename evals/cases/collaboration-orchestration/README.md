# Collaboration orchestration comparison

This public case suite evaluates the durable orchestration contract rather than
rewarding a system merely for launching more sessions. Run every scenario in
[`matrix.json`](matrix.json) with the four configurations below, using the same
model, fixture revision, permissions, timeout, and deterministic grader.

- **A — single:** one Named Agent and one Session.
- **B — independent:** independently acceptable deliverables become separate
  Work items with one Session each.
- **C — fan-in:** a difficult Work may use two or three differentiated Producer
  Runs followed by one visible Lead's Selector or Integration Run.
- **D — verified fan-in:** configuration C followed by one fresh independent
  Verifier over only the promoted canonical candidate.

The exact user turns are in [`prompts.md`](prompts.md). The runner must preserve
all Work, Run, Artifact, promotion, Collaboration envelope, verification, token,
and timing records. It must not include hidden reasoning. Repeat each
case/configuration pair at least five times before making performance or cost
claims.

## Required measurements

Record goal completion, requirement coverage, factual/code correctness, unique
risk findings, evidence lost during integration, verifier true/false blocks,
human nudges, time to first useful result, wall time, input/output tokens, model
turns, tool calls, peak admitted Sessions, queue time, cost per Work/Run/success,
duplicate investigation, useless candidates, and recovery re-execution cost.

The deterministic orchestration invariants are covered by
`internal/channels/collaboration_orchestration_test.go`; this suite measures the
model and product outcomes that unit tests cannot establish.

## Success rules

Configuration B must reduce wall time for the three independent tasks. C or D
must improve quality, coverage, or reliability on suitable difficult tasks,
while the serial case remains single-route. Added tokens must be reported and
justified by value. The public answer must retain one visible owner and must not
dump internal topology or rejected drafts into the room.

Fixtures are intentionally specified by behavior instead of vendoring a
third-party repository. A public result must add a redistributable fixture and
its immutable source/license before execution, then conform to
`evals/result.schema.json`.

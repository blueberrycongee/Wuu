package coordinator

// Worker prompt presets.
//
// These are verbatim text blocks the coordinator pastes into a worker
// prompt to inject a specific role posture without needing a separate
// worker type for every variation. They live as Go constants (rather
// than being baked directly into SystemPromptPreamble) so other code
// — tests, possible future tooling — can reference them by name.
//
// The contract with the model is: COPY THESE BLOCKS VERBATIM. Any
// paraphrase weakens them, because each line is tuned to a specific
// failure mode (e.g. "never write PASS without evidence" defends
// against the rubber-stamp failure mode that LLMs gravitate toward
// when asked to verify their own work).

// VerificationPreset turns a generic worker into an adversarial
// tester. The frame inversion at the top — "NOT to confirm... TRY
// TO BREAK IT" — is the load-bearing line; without it the model
// defaults to a "helpful confirmer" posture and starts looking for
// reasons to say PASS instead of reasons to say FAIL.
//
// Use this when the coordinator's intent is "judge whether this
// change is actually safe to ship": code review, post-fix
// regression check, PR verification, release readiness gate.
const VerificationPreset = `You are operating in VERIFICATION mode. Your job is NOT to confirm
that the code works — it is to TRY TO BREAK IT.

Hard rules:
- Never write "PASS" without showing the actual command output that
  justifies it. A claim without evidence is worse than no claim.
- Run the project's real test suite, build, lint, and type checker.
  Use the commands documented in CLAUDE.md / AGENTS.md / README.
- Investigate every failure. Do NOT dismiss errors as "unrelated"
  or "pre-existing" without evidence — chase them until you can prove
  they are unrelated, or report them.
- Look for the last 20%: edge cases, error paths, empty inputs,
  malformed inputs, race conditions, resource exhaustion, the case
  the implementer forgot to test.
- Be skeptical of UI / surface signals. A page rendering correctly
  does not mean the backend handles bad input. A test passing does
  not mean the feature is enabled.
- Do NOT modify project files. You may write throwaway scripts to
  /tmp if you need to probe.

End your final message with EXACTLY one of these lines, on its own:
  VERDICT: PASS
  VERDICT: FAIL
  VERDICT: PARTIAL

Followed by a short justification (under 200 words) listing the
strongest piece of evidence for your verdict.`

// ResearchPreset turns a generic worker into a focused, read-only
// investigator. Three failure modes this defends against:
//
//  1. Scope creep — the model wanders off the original question to
//     "fix things it noticed". The Notes section gives that impulse
//     a legal home so it gets logged but not pursued.
//  2. Side effects — without a hard "do not mutate state" rule the
//     model will sometimes run installs, migrations, or commits as
//     "part of the investigation".
//  3. Vague citations — claims like "there's a check in validate.ts"
//     that the coordinator can't act on. The file:line mandate
//     forces the worker to give the coordinator something it can
//     immediately turn into a follow-up spec.
//
// Use this when the coordinator needs to understand the codebase
// before deciding what to do: analyze a module, locate a bug
// origin, find all callers of an API, study how a third-party
// library is used.
const ResearchPreset = `You are operating in READ-ONLY RESEARCH mode. Your job is to
investigate the codebase and answer a specific question, then
report findings — nothing else.

Hard rules:
- Do NOT modify, create, delete, or move any project files. Do not
  run commands that mutate state (no migrations, no installs, no
  builds that write artifacts into the tree, no git commits).
- Stay focused on the question you were asked. Do NOT refactor,
  do NOT suggest improvements, do NOT explore tangents. If you
  notice something interesting that's outside the question, save
  it for the "Notes" section at the end — don't pursue it.
- Be efficient. Use parallel tool calls when reading multiple files.
  Don't read more than you need to answer the question. Stop as
  soon as you have enough.
- Be specific. Every claim about the code must be backed by a
  file:line reference. "There's a null check in validate.ts" is
  not acceptable; "validate.ts:42 has ` + "`if (user == null) return`" + `"
  is.

End your final message with a plain-text summary (under 250 words)
in this shape:

  ## Answer
  Direct answer to the question, in 1-3 sentences.

  ## Evidence
  - path/to/file.ext:NN — what it shows
  - path/to/other.ext:NN — what it shows

  ## Notes (optional)
  Anything you noticed that's outside the question but the
  orchestrator might want to know. One line each.

Do not include preamble, do not restate the question, do not pad.`

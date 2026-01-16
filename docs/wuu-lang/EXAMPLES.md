# Wuu — Examples (informal sketches)

These are **illustrative** and not final syntax. The goal is to make the intended semantics concrete.

---

## 1) Durable agent loop (replayable, capability-gated)

```aegis
type TaskId = String

record TaskSpec {
  id: TaskId,
  repo_path: String,
  goal: String,
}

record AgentState {
  attempt: Int,
  last_plan: Option<String>,
  last_patch_hash: Option<String>,
}

workflow run_task(spec: TaskSpec) -> Result<Unit, String>
requires { fs:read, fs:write, process:spawn, store:kv, clock }
invariant: state.attempt <= 50
{
  state: AgentState = store.kv.get_or_init(spec.id, AgentState { attempt: 0, last_plan: None, last_patch_hash: None })

  loop {
    step "plan" {
      let repo = fs.read_tree(spec.repo_path)
      let plan = propose_plan(repo, spec.goal)              // pure
      state.last_plan = Some(plan.render())
      store.kv.put(spec.id, state)
    }

    step "implement" {
      let patch = synthesize_patch(spec.repo_path, state.last_plan?) // pure + file diff construction
      let patch_hash = patch.hash()
      fs.apply_patch(spec.repo_path, patch)                           // effect, logged
      state.last_patch_hash = Some(patch_hash)
      store.kv.put(spec.id, state)
    }

    step "verify" retry { max: 3, backoff_ms: 500 } {
      let result = process.spawn({
        cmd: "pnpm",
        args: ["test"],
        cwd: spec.repo_path,
        timeout_ms: 20 * 60 * 1000,
      })

      if result.exit_code == 0 {
        return Ok(())
      }
    }

    step "decide" {
      state.attempt = state.attempt + 1
      store.kv.put(spec.id, state)
      if state.attempt >= 50 { return Err("budget exceeded") }
    }
  }
}
```

Intended properties:

- Crash-safe: after restart, `run_task` replays from the workflow log and resumes.
- No ambient authority: file writes and process spawn require capabilities.
- Idempotent steps: each step is log-recorded; retries are structured and auditable.
- Evidence gates: `verify` produces evidence for changes; policy can require it before allowing certain effects.

---

## 2) Pure core + effectful shell

```aegis
fn propose_plan(repo: RepoSnapshot, goal: String) -> Plan
pre: goal.len() > 0
post: result.steps.len() > 0
{
  // Pure logic only: deterministic, easy to test and replay.
}
```

The “agent-ness” lives in durable workflows and capabilities; the core logic stays pure and testable.

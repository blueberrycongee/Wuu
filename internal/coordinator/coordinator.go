// Package coordinator wires the orchestration tools (spawn_agent,
// send_message, stop_agent, list_agents) to the underlying subagent
// and worktree subsystems.
//
// The coordinator is the brain that the main agent talks to in
// coordinator mode. It owns the SubAgent Manager and Worktree Manager,
// and exposes a small API the toolkit uses to implement the
// orchestration tools.
package coordinator

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/blueberrycongee/wuu/internal/agent"
	"github.com/blueberrycongee/wuu/internal/providers"
	"github.com/blueberrycongee/wuu/internal/subagent"
	"github.com/blueberrycongee/wuu/internal/worktree"
)

// WorkerToolkitFactory builds a fresh ToolExecutor for a worker that
// will run inside the given root directory (typically a worktree path).
// The factory should configure skills, memory, and any per-worker
// restrictions but MUST NOT include orchestration tools — workers
// don't spawn sub-sub-agents. The worker type is provided so the
// factory can apply tool whitelisting via FilterToolsForWorker.
type WorkerToolkitFactory func(rootDir string, wt WorkerType) (agent.ToolExecutor, error)

// Coordinator owns the orchestration runtime for one wuu session.
type Coordinator struct {
	manager     *subagent.Manager
	worktrees   *worktree.Manager
	sessionID   string
	historyDir  string
	workerFact  WorkerToolkitFactory
	defaultSys  string // base system prompt prefix added to every worker
	maxParallel int
}

// Config holds the dependencies needed to build a Coordinator.
type Config struct {
	// Client is the streaming LLM client every worker spawned by this
	// coordinator will share. It must be a StreamClient (not just a
	// Client) so workers run through the same streaming transport as
	// the interactive main agent.
	Client       providers.StreamClient
	DefaultModel string
	ParentRepo   string // absolute path to the user's workspace (must be a git repo)
	WorktreeRoot string // .wuu/worktrees/
	HistoryDir   string // .wuu/sessions/{session-id}/workers/
	SessionID    string
	WorkerSysPrompt string
	WorkerFactory WorkerToolkitFactory
	MaxParallel  int
}

// New constructs a Coordinator. Returns an error if the parent repo
// is not a git repository.
func New(cfg Config) (*Coordinator, error) {
	if cfg.Client == nil {
		return nil, errors.New("Client required")
	}
	if cfg.WorkerFactory == nil {
		return nil, errors.New("WorkerFactory required")
	}
	wt, err := worktree.NewManager(cfg.ParentRepo, cfg.WorktreeRoot)
	if err != nil {
		return nil, fmt.Errorf("worktree manager: %w", err)
	}
	mgr := subagent.NewManager(cfg.Client, cfg.DefaultModel)

	maxP := cfg.MaxParallel
	if maxP <= 0 {
		maxP = 5
	}
	return &Coordinator{
		manager:     mgr,
		worktrees:   wt,
		sessionID:   cfg.SessionID,
		historyDir:  cfg.HistoryDir,
		workerFact:  cfg.WorkerFactory,
		defaultSys:  cfg.WorkerSysPrompt,
		maxParallel: maxP,
	}, nil
}

// Manager exposes the underlying subagent.Manager for advanced use
// (Subscribe, etc.).
func (c *Coordinator) Manager() *subagent.Manager {
	return c.manager
}

// SetSessionInfo updates the coordinator's session ID and history dir
// after the TUI has generated them. Safe to call once at startup.
func (c *Coordinator) SetSessionInfo(sessionID, historyDir string) {
	c.sessionID = sessionID
	c.historyDir = historyDir
}

// SessionID returns the bound session ID, or "session-pending" if
// SetSessionInfo hasn't been called yet.
func (c *Coordinator) SessionID() string {
	return c.sessionID
}

// SpawnRequest is the internal shape of a spawn_agent tool invocation
// after argument validation.
type SpawnRequest struct {
	Type        string
	Description string
	Prompt      string
	BaseRepo    string // optional: chain off another worktree (worktree mode only)
	Synchronous bool
	Timeout     time.Duration
	// Isolation overrides the worker type's DefaultIsolation when set.
	// Empty string means "use the type default". Use this from
	// spawn_agent to opt a normally-inplace worker into a worktree
	// (e.g. an explorer that needs to run a destructive script).
	Isolation string
}

// SpawnResult is what the spawn_agent tool returns to the model.
type SpawnResult struct {
	AgentID      string `json:"agent_id"`
	Status       string `json:"status"`
	Isolation    string `json:"isolation"`              // "inplace" or "worktree"
	WorktreePath string `json:"worktree_path,omitempty"` // empty for inplace spawns
	Result       string `json:"result,omitempty"`
	Error        string `json:"error,omitempty"`
	DurationMS   int64  `json:"duration_ms,omitempty"`
}

// Spawn launches a sub-agent. In synchronous mode it blocks until
// the sub-agent finishes; in async mode it returns immediately with
// status "running" and the agent_id the orchestrator can poll.
func (c *Coordinator) Spawn(ctx context.Context, req SpawnRequest) (*SpawnResult, error) {
	// Concurrency cap.
	if c.manager.CountRunning() >= c.maxParallel {
		return nil, fmt.Errorf("max parallel sub-agents reached (%d). Wait for one to complete or stop one with stop_agent.", c.maxParallel)
	}

	if strings.TrimSpace(req.Prompt) == "" {
		return nil, errors.New("prompt is required")
	}

	// Resolve worker type (validates the name).
	wt, err := LookupWorkerType(req.Type)
	if err != nil {
		return nil, err
	}
	wtype := wt.Name

	workerID := newCoordinatorWorkerID(wtype)

	// Resolve effective isolation: caller override > type default.
	isolation, err := NormalizeIsolation(req.Isolation, wt)
	if err != nil {
		return nil, err
	}
	// BaseRepo only makes sense for chained worktree spawns.
	if isolation == IsolationInplace && strings.TrimSpace(req.BaseRepo) != "" {
		return nil, errors.New("base_repo is only supported with isolation=worktree")
	}

	// 1. Determine the worker's working directory.
	//    - inplace: share the parent repo (no checkout cost)
	//    - worktree: `git worktree add --detach` based on parent HEAD
	var (
		workerRoot  string
		worktreeRef *worktree.Worktree
	)
	if isolation == IsolationWorktree {
		worktreeRef, err = c.worktrees.Create(c.sessionID, workerID, req.BaseRepo)
		if err != nil {
			return nil, fmt.Errorf("worktree create: %w", err)
		}
		workerRoot = worktreeRef.Path
	} else {
		workerRoot = c.worktrees.ParentRepo()
	}

	// 2. Build worker's toolkit rooted at the chosen working directory.
	workerKit, err := c.workerFact(workerRoot, wt)
	if err != nil {
		if worktreeRef != nil {
			_ = c.worktrees.Cleanup(worktreeRef)
		}
		return nil, fmt.Errorf("worker toolkit: %w", err)
	}

	// 3. Compose system prompt: type-specific role + working dir + base prompt.
	sys := composeWorkerSystemPrompt(c.defaultSys, wt, workerRoot, isolation)

	// 4. History path.
	historyPath := ""
	if c.historyDir != "" {
		historyPath = filepath.Join(c.historyDir, workerID+".json")
	}

	// 5. Spawn via manager. We pass the worker ID we already created
	// the worktree under so they line up. Manager will pick its own
	// internal ID; we surface BOTH.
	sa, err := c.manager.Spawn(ctx, subagent.SpawnOptions{
		Type:         wtype,
		Description:  req.Description,
		Prompt:       req.Prompt,
		SystemPrompt: sys,
		Toolkit:      workerKit,
		HistoryPath:  historyPath,
	})
	if err != nil {
		if worktreeRef != nil {
			_ = c.worktrees.Cleanup(worktreeRef)
		}
		return nil, fmt.Errorf("spawn: %w", err)
	}

	result := &SpawnResult{
		AgentID:   sa.ID,
		Status:    string(sa.Status),
		Isolation: string(isolation),
	}
	if worktreeRef != nil {
		result.WorktreePath = worktreeRef.Path
	}

	if !req.Synchronous {
		// Async path: schedule background recycle once the worker
		// finishes. Detached context so it survives a cancelled
		// parent (the worker itself runs detached too).
		if worktreeRef != nil {
			go c.recycleWorktreeWhenDone(sa.ID, worktreeRef)
		}
		return result, nil
	}

	// Synchronous mode: wait for completion.
	timeout := req.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Minute
	}
	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	snap, err := c.manager.Wait(waitCtx, sa.ID)
	if err != nil {
		return nil, fmt.Errorf("wait: %w", err)
	}
	result.Status = string(snap.Status)
	result.Result = snap.Result
	if snap.Error != nil {
		result.Error = snap.Error.Error()
	}
	if !snap.CompletedAt.IsZero() && !snap.StartedAt.IsZero() {
		result.DurationMS = snap.CompletedAt.Sub(snap.StartedAt).Milliseconds()
	}

	// Sync recycle: drop the worktree if the worker left it pristine.
	// Anything dirty stays on disk so the orchestrator (or user) can
	// inspect / merge it.
	if worktreeRef != nil {
		if kept, cerr := c.worktrees.CleanupIfClean(worktreeRef); cerr == nil && !kept {
			result.WorktreePath = "" // recycled — no path to surface
		}
	}
	return result, nil
}

// recycleWorktreeWhenDone is the async-spawn cleanup tail. It blocks
// on the worker's completion, then attempts to drop the worktree if
// nothing was modified. Errors are intentionally swallowed: cleanup is
// best-effort and the user can always run `git worktree prune` later.
func (c *Coordinator) recycleWorktreeWhenDone(agentID string, wtRef *worktree.Worktree) {
	if wtRef == nil {
		return
	}
	// Long ceiling — workers can legitimately run for a while. The
	// real cap comes from the worker's own context, not this wait.
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Hour)
	defer cancel()
	if _, err := c.manager.Wait(ctx, agentID); err != nil {
		return
	}
	_, _ = c.worktrees.CleanupIfClean(wtRef)
}

// StopAll cancels every running worker. Used for Ctrl+C handling.
func (c *Coordinator) StopAll() {
	c.manager.StopAll()
}

// Stop cancels a specific worker by ID. Returns false if not found.
func (c *Coordinator) Stop(id string) bool {
	return c.manager.Stop(id)
}

// List returns snapshots of all sub-agents in this session.
func (c *Coordinator) List() []subagent.SubAgentSnapshot {
	return c.manager.List()
}

// SendMessage delivers a follow-up message to a specific sub-agent.
// In Phase 3 this is a stub that will be filled in once the manager
// supports message injection — for now it returns an error explaining
// the feature is not yet implemented.
func (c *Coordinator) SendMessage(agentID, message string) error {
	if c.manager.Get(agentID) == nil {
		return fmt.Errorf("agent %q not found", agentID)
	}
	return errors.New("send_message: follow-up messaging is not yet implemented in this build (worker is one-shot)")
}

// Subscribe forwards to the underlying manager so the TUI can receive
// status notifications and inject worker-result messages.
func (c *Coordinator) Subscribe(ch chan<- subagent.Notification) {
	c.manager.Subscribe(ch)
}

// SystemPromptPreamble returns the orchestration instructions
// prepended to the main agent's system prompt. It teaches:
//
//   - the three communication planes (filesystem for data,
//     send_message for control, trajectories for history),
//   - when to act directly vs delegate,
//   - the spawn-vs-fork judgment heuristic,
//   - honesty rules around worker results.
//
// There is NO "coordinator role" here. The agent has the full tool
// set; orchestration tools are simply additional capabilities. The
// preamble is guidance for how to *use* those capabilities well, not
// a restriction on what the agent can do.
func SystemPromptPreamble() string {
	return `# How To Work

You are a coding agent with the full file / shell / web tool set, plus four orchestration primitives for working with sub-agents: ` + "`spawn_agent`" + `, ` + "`fork_agent`" + `, ` + "`send_message_to_agent`" + `, ` + "`stop_agent`" + `. You can do tasks directly, or delegate to sub-agents, or both. The right choice depends on the task — read the rest of this section for the judgment.

## Direct vs delegate

**Do it yourself when:**
- The task is small (minutes of work, a handful of files).
- The task needs tight iteration — form a hypothesis, check, revise.
- The task is exploratory: you don't know what you're looking for until you see it.
- A single read or grep is all you need.

**Delegate to a sub-agent when:**
- The task has N independent subtasks that can run in parallel.
- The task will take long enough that you shouldn't block the user conversation (async).
- The task needs adversarial verification — spawn a fresh worker so its judgment isn't anchored to your beliefs.
- You want the subtask's intermediate context to stay out of your own (keep your context strategic, not bloated).

When in doubt for a small task, do it yourself. Delegation has a fixed setup cost; small jobs almost never repay it.

## spawn vs fork

- **` + "`spawn_agent`" + `** creates a child with **zero inherited context**. It only sees its system prompt and the prompt you give it. Use spawn when the task is independent of your conversation, when you specifically want fresh framing (e.g. an adversarial verifier that should not inherit your beliefs), or when you have N near-independent subtasks to parallelize.
- **` + "`fork_agent`" + `** creates a child that **inherits your full conversation history** — every tool call, every observation, every piece of reasoning you've done so far. Use fork when you've already built up understanding the child needs and would otherwise have to recap in prose.

**The 100-word rule:** if you can describe the task in under 100 words without recapping your own context, use ` + "`spawn`" + `. If you would need to paraphrase a lot of what you've already learned to make the task legible, use ` + "`fork`" + `.

Spawn is the common case. Fork is the right answer when state fidelity matters more than a clean room.

## The three communication planes

Sub-agents can't read your conversation. To work with them well, separate **data** from **control**:

### 1. Data goes through the filesystem

If you have findings, plans, intermediate results, or anything more than a sentence that another agent should see, **write it to a file** and reference the path. Use the project's working tree for code, and use ` + "`.wuu/shared/`" + ` for cross-agent artifacts that aren't part of the project itself:

- ` + "`.wuu/shared/findings/<topic>.md`" + ` — investigation reports
- ` + "`.wuu/shared/plans/<topic>.md`" + ` — plans, designs, todos
- ` + "`.wuu/shared/status/<topic>.md`" + ` — progress tracking
- ` + "`.wuu/shared/reports/<topic>.md`" + ` — final summaries / verdicts

These paths are conventions, not requirements. Pick a reasonable path and use it. You can always ` + "`read_file`" + ` what another agent wrote — they're just files.

### 2. Control goes through send_message

` + "`send_message_to_agent`" + ` is for **short signals**: "I finished, results at ` + "`.wuu/shared/findings/X.md`" + `", "stop, the situation changed", "new instruction", "I failed, class=auth". If your message is more than a sentence, you're using the wrong channel — write a file and send the path.

**Never duplicate file content inside a message.** A 500-word "summary of findings" sent via ` + "`send_message`" + ` is information that should have been written to ` + "`.wuu/shared/findings/`" + `, with the message saying only "see findings/X.md".

### 3. Trajectories are auto-recorded

Every tool call you make is automatically logged. You don't need to do anything to record — just work normally, and another agent (or the user) can review what you did later by reading your trace. If you want a downstream agent to understand your reasoning, you can point it at your trace path; you don't need to recap.

## Parallelism

When tasks are independent, spawn multiple workers in the same response — they run concurrently. The cost of three parallel ` + "`spawn_agent`" + ` calls is roughly the same as one. Don't artificially serialize.

When tasks have a dependency (B needs A's results), do A first, **then** spawn B with a reference to A's output file in ` + "`.wuu/shared/`" + `. Don't ask B to "act on A's findings" without telling B where to find them.

## Working with worker results

When a sub-agent finishes, a notification arrives in your next turn with its agent_id, status, final message, and the path to its trajectory. Read the relevant artifact files (if any), then decide the next step yourself. Don't ask a follow-up worker to "synthesize the previous worker's findings" — synthesize them yourself, write the synthesis to ` + "`.wuu/shared/`" + ` if needed, then delegate the next concrete step.

## Honesty rules

These are non-negotiable. Violating any of them is worse than admitting you can't help.

- **Never fabricate or predict worker results.** Do not describe what a worker "found" or "did" before its result arrives in your context. After spawning a worker, briefly tell the user what you launched, then stop and wait.
- **Never paper over a stuck state with a fake plan.** If you genuinely can't accomplish a step, say so and ask the user. Do NOT propose a follow-up action you don't expect to work just to keep moving.
- **Trust artifacts that already exist.** If a worker wrote a file, the file is where the worker put it — don't spawn a second worker to "redo" the work unless you have a concrete reason to believe the new spawn will land somewhere different.
- **Synthesize before delegating again.** When a worker returns, read its result yourself before writing the next prompt.

## Failure handling

When a sub-agent fails, the notification includes an error class. React based on the class:

- ` + "`retryable`" + ` — transient (rate limit, network). Re-spawning the same prompt may succeed; wait briefly, try again.
- ` + "`auth`" + ` — credentials rejected. Don't retry. Tell the user.
- ` + "`context_overflow`" + ` — the worker's context was too large. Split the task and re-spawn with smaller pieces.
- ` + "`cancelled`" + ` — stopped intentionally. Don't auto-retry.
- ` + "`resource_exhausted`" + ` — the worker hit its token / time / tool-call budget. Consider splitting or raising the budget for a retry.
- ` + "`fatal`" + ` — unknown. Report and stop.

There is no automatic restart. You decide what to do.

## Verification mindset

When you need a worker to judge whether something is actually safe — code review, post-fix regression check, PR verification — spawn a fresh worker (not fork) and tell it to TRY TO BREAK the work, not confirm it works. The frame inversion is the load-bearing instruction: a confirmer-by-default worker will rubber-stamp; a breaker-by-default worker will find real problems. The ` + "`VerificationPreset`" + ` and ` + "`ResearchPreset`" + ` constants in this codebase contain reference text you may quote (or paraphrase) when writing such prompts; they are starting points, not required boilerplate.

`
}

// FormatWorkerResult turns a sub-agent snapshot into the XML message
// that the orchestrator sees when a worker completes. The format
// mirrors Claude Code's <task-notification>:
//
//	<worker-result agent_id="..." type="..." status="completed">
//	<summary>...</summary>
//	<duration_ms>1234</duration_ms>
//	<result>
//	... worker's final assistant message ...
//	</result>
//	</worker-result>
func FormatWorkerResult(snap subagent.SubAgentSnapshot) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<worker-result agent_id=%q type=%q status=%q>\n",
		snap.ID, snap.Type, snap.Status)
	if snap.Description != "" {
		fmt.Fprintf(&b, "<summary>%s</summary>\n", snap.Description)
	}
	if !snap.CompletedAt.IsZero() && !snap.StartedAt.IsZero() {
		ms := snap.CompletedAt.Sub(snap.StartedAt).Milliseconds()
		fmt.Fprintf(&b, "<duration_ms>%d</duration_ms>\n", ms)
	}
	if snap.Error != nil {
		class := ClassifyError(snap.Error)
		fmt.Fprintf(&b, "<error class=%q>%s</error>\n", class, snap.Error.Error())
	}
	if snap.Result != "" {
		b.WriteString("<result>\n")
		b.WriteString(snap.Result)
		b.WriteString("\n</result>\n")
	}
	b.WriteString("</worker-result>")
	return b.String()
}

// CleanupSession removes all worktrees belonging to this session.
func (c *Coordinator) CleanupSession() error {
	return c.worktrees.CleanupSession(c.sessionID)
}

// composeWorkerSystemPrompt builds the system prompt for a worker.
// It prepends the worker type's role-specific prompt + a description
// of the working directory and isolation mode, then appends the base
// prompt (typically the main agent's project memory and skills, NOT
// the coordinator instructions).
func composeWorkerSystemPrompt(base string, wt WorkerType, workerRoot string, isolation IsolationMode) string {
	var b strings.Builder
	b.WriteString(wt.SystemPrompt)
	b.WriteString("\n\n")
	switch isolation {
	case IsolationWorktree:
		fmt.Fprintf(&b, "Your working directory is %s — a git worktree isolated from other workers. ", workerRoot)
		b.WriteString("Edits you make stay sandboxed; the orchestrator will inspect the worktree after you finish. ")
	default: // inplace
		fmt.Fprintf(&b, "Your working directory is %s — the SHARED parent repository. ", workerRoot)
		b.WriteString("You are running inplace (no worktree isolation), so be especially careful: ")
		b.WriteString("read-only operations are safe, but any file you modify is visible to the orchestrator and other workers immediately. ")
	}
	b.WriteString("All file paths in your tools resolve relative to this directory. ")
	b.WriteString("You CANNOT spawn further sub-agents.\n")
	if base != "" {
		b.WriteString("\n---\n\n")
		b.WriteString(base)
	}
	return b.String()
}

// newCoordinatorWorkerID generates a worker ID. Mirrors subagent's
// scheme but is generated by the coordinator since worktree creation
// happens before subagent.Manager.Spawn.
func newCoordinatorWorkerID(typ string) string {
	if typ == "" {
		typ = "agent"
	}
	return fmt.Sprintf("%s-%d", typ, time.Now().UnixNano()%1_000_000_000)
}

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

	"crypto/rand"
	"encoding/hex"

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
	manager      *subagent.Manager
	worktrees    *worktree.Manager // nil when workspace is not a git repo
	parentRepo   string            // absolute path to workspace root
	worktreeRoot string            // .wuu/worktrees/ directory
	sessionID    string
	historyDir   string
	workerFact   WorkerToolkitFactory
	defaultSys   string // base system prompt prefix added to every worker
	maxParallel  int
}

// Config holds the dependencies needed to build a Coordinator.
type Config struct {
	// Client is the streaming LLM client every worker spawned by this
	// coordinator will share. It must be a StreamClient (not just a
	// Client) so workers run through the same streaming transport as
	// the interactive main agent.
	Client          providers.StreamClient
	DefaultModel    string
	ParentRepo      string // absolute path to the user's workspace
	WorktreeRoot    string // .wuu/worktrees/ (only used when workspace is a git repo)
	HistoryDir      string // .wuu/sessions/{session-id}/workers/
	SessionID       string
	WorkerSysPrompt string
	WorkerFactory   WorkerToolkitFactory
	MaxParallel     int
}

// New constructs a Coordinator. Worktree isolation is only available
// when the workspace is a git repository; inplace spawns and forks
// work regardless.
func New(cfg Config) (*Coordinator, error) {
	if cfg.Client == nil {
		return nil, errors.New("Client required")
	}
	if cfg.WorkerFactory == nil {
		return nil, errors.New("WorkerFactory required")
	}

	// Worktree manager is optional — only created when the workspace
	// is a git repo. Non-git workspaces can still spawn inplace
	// workers and fork agents; only isolation=worktree is unavailable.
	var wt *worktree.Manager
	if worktree.IsGitRepo(cfg.ParentRepo) {
		var err error
		wt, err = worktree.NewManager(cfg.ParentRepo, cfg.WorktreeRoot)
		if err != nil {
			return nil, fmt.Errorf("worktree manager: %w", err)
		}
	}

	mgr := subagent.NewManager(cfg.Client, cfg.DefaultModel)

	maxP := cfg.MaxParallel
	if maxP <= 0 {
		maxP = 5
	}
	return &Coordinator{
		manager:      mgr,
		worktrees:    wt,
		parentRepo:   cfg.ParentRepo,
		worktreeRoot: cfg.WorktreeRoot,
		sessionID:    cfg.SessionID,
		historyDir:   cfg.HistoryDir,
		workerFact:   cfg.WorkerFactory,
		defaultSys:   cfg.WorkerSysPrompt,
		maxParallel:  maxP,
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
	Isolation    string `json:"isolation"`               // "inplace" or "worktree"
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
		if c.worktrees == nil {
			return nil, errors.New("isolation=worktree requires a git repository (this workspace is not a git repo)")
		}
		worktreeRef, err = c.worktrees.Create(c.sessionID, workerID, req.BaseRepo)
		if err != nil {
			return nil, fmt.Errorf("worktree create: %w", err)
		}
		workerRoot = worktreeRef.Path
	} else {
		workerRoot = c.parentRepo
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
	workerCtx := ctx
	if !req.Synchronous {
		workerCtx = context.WithoutCancel(ctx)
	}

	sa, err := c.manager.Spawn(workerCtx, subagent.SpawnOptions{
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

// ForkRequest is the internal shape of a fork_agent tool invocation
// after argument validation. Unlike SpawnRequest there is no Type or
// Isolation choice — fork is always inplace (it inherits the parent's
// conversation continuation, so a worktree sandbox would defeat the
// continuation semantics) and always uses the default worker type.
type ForkRequest struct {
	Description string
	// Prompt is what the worker sees as its FINAL user message,
	// appended to the inherited history. Callers should wrap any
	// role-override instructions in <system-reminder> tags so the
	// model treats them as authoritative over anything in the
	// inherited parent system prompt.
	Prompt      string
	Synchronous bool
	Timeout     time.Duration
}

// Fork launches a sub-agent that inherits a snapshot of the parent
// agent's conversation history. The worker's first request to the
// LLM provider replays the parent's history verbatim and adds the
// fork prompt as the final user message — preserving prompt-cache
// hits across the fork boundary.
//
// `parentHistory` MUST be a complete history with no dangling
// tool_use blocks: the caller (the fork_agent tool handler) is
// expected to have already stripped the in-flight fork_agent
// assistant turn before passing it through.
func (c *Coordinator) Fork(ctx context.Context, req ForkRequest, parentHistory []providers.ChatMessage) (*SpawnResult, error) {
	if c.manager.CountRunning() >= c.maxParallel {
		return nil, fmt.Errorf("max parallel sub-agents reached (%d). Wait for one to complete or stop one with stop_agent.", c.maxParallel)
	}
	if strings.TrimSpace(req.Prompt) == "" {
		return nil, errors.New("prompt is required")
	}
	if len(parentHistory) == 0 {
		return nil, errors.New("fork_agent: no parent history (only the main agent in an interactive session can fork)")
	}

	// Resolve the default worker type so the worker has the full
	// tool set (minus orchestration / ask_user, which are blocked
	// by the no-bridge / no-coordinator pattern in the WorkerFactory).
	wt, err := LookupWorkerType("worker")
	if err != nil {
		return nil, err
	}

	workerID := newCoordinatorWorkerID("fork")
	workerRoot := c.parentRepo

	workerKit, err := c.workerFact(workerRoot, wt)
	if err != nil {
		return nil, fmt.Errorf("worker toolkit: %w", err)
	}

	historyPath := ""
	if c.historyDir != "" {
		historyPath = filepath.Join(c.historyDir, workerID+".json")
	}

	// Note: we deliberately do NOT set SystemPrompt — when
	// InitialHistory is non-nil, the subagent runner uses
	// history[0] as the system message and ignores the option.
	workerCtx := ctx
	if !req.Synchronous {
		workerCtx = context.WithoutCancel(ctx)
	}

	sa, err := c.manager.Spawn(workerCtx, subagent.SpawnOptions{
		Type:           "fork",
		Description:    req.Description,
		Prompt:         req.Prompt,
		Toolkit:        workerKit,
		HistoryPath:    historyPath,
		InitialHistory: parentHistory,
	})
	if err != nil {
		return nil, fmt.Errorf("spawn: %w", err)
	}

	result := &SpawnResult{
		AgentID:   sa.ID,
		Status:    string(sa.Status),
		Isolation: string(IsolationInplace),
	}

	if !req.Synchronous {
		return result, nil
	}

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
	return result, nil
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
// Messages are queued while the worker is running and injected as
// user-role turns before the next model round.
func (c *Coordinator) SendMessage(agentID, message string) error {
	id := strings.TrimSpace(agentID)
	if id == "" {
		return errors.New("agent_id is required")
	}
	msg := strings.TrimSpace(message)
	if msg == "" {
		return errors.New("message is required")
	}
	sa := c.manager.Get(id)
	if sa == nil {
		return fmt.Errorf("agent %q not found", id)
	}
	snap := sa.Snapshot()
	switch snap.Status {
	case subagent.StatusCompleted, subagent.StatusFailed, subagent.StatusCancelled:
		return fmt.Errorf("agent %q is %s and cannot receive follow-up messages", id, snap.Status)
	}
	if ok := c.manager.QueueMessage(id, msg); !ok {
		return fmt.Errorf("agent %q not found", id)
	}
	return nil
}

// Subscribe forwards to the underlying manager so the TUI can receive
// status notifications and inject worker-result messages.
func (c *Coordinator) Subscribe(ch chan<- subagent.Notification) {
	c.manager.Subscribe(ch)
}

// SystemPromptPreamble returns the instructions prepended to the
// main agent's system prompt. It teaches, in order:
//
//   - Step 0: classify every task before acting (Path A / B / C and
//     the "referenced artifact" override).
//   - Path A: when the user has a specific answer in their head,
//     extract it via the ask_user tool instead of guessing.
//   - Path B: when the user hands the decision to the agent, gather
//     context, form a recommendation, and declare it before acting.
//   - The phantom-read rule: if the user references an existing
//     artifact, read_file it in full before planning.
//   - The interview loop: the default iterative rhythm for
//     non-trivial tasks.
//   - Delegation rules (spawn vs fork, communication planes,
//     honesty rules, failure handling) — but only AFTER alignment.
//
// There is NO separate "coordinator role" persona here. The main
// agent is read-oriented and orchestration-capable: it should inspect,
// align, and delegate mutations to workers. The preamble teaches how
// to use that split well, not just that tools exist.
func SystemPromptPreamble() string {
	return `You are a coordinator. Your job is to help the user achieve their goal by directing workers to research, implement, and verify code changes.

## Your Tools

- spawn_agent — start a new worker with a clean slate. Best for context-independent tasks or when you want fresh framing.
- fork_agent — start a worker that inherits your full conversation history. Best for context-sensitive tasks that depend on what you have read and discussed.
- send_message_to_agent — continue an existing worker with new instructions.
- stop_agent — stop a running worker that is stuck or off-track.
- list_agents — see active workers and their status.

## Workers

Workers have the full tool set including read_file, write_file, edit_file, run_shell, grep, glob, and git. They execute tasks autonomously.

## Task Workflow

| Phase | Who | Purpose |
|-------|-----|---------|
| Research | Workers (parallel) | Investigate codebase, find files, understand problem |
| Synthesis | You | Read findings, understand the problem, craft implementation specs |
| Implementation | Workers | Make targeted changes per spec |
| Verification | Workers | Test changes work |

## Delegation Discipline

Do not spawn workers for trivial tasks you can handle yourself — reading a specific file, running a quick grep, or reporting a command output. Spawn agents for higher-level work: multi-file refactors, parallel research across different areas, verification that requires running the full test suite, or tasks that benefit from isolated context.

Do not delegate work that blocks your immediate next step. If the very next action depends on that result, do it locally to keep the critical path moving.

## Concurrency

Launch independent workers in parallel whenever possible. Research tasks can run freely in parallel. Write-heavy tasks should run one at a time per file set to avoid conflicts.

## Working with Worker Results

When a worker finishes, its result arrives as a notification in your next turn. Workers cannot see your conversation history or other workers' results.

Before launching follow-up work, read the returned content yourself and do your own synthesis. Never chain workers by implication with phrases like "based on your findings" or "based on the research".

Good worker prompts are self-contained: specific file paths, line numbers, exactly what to change, and what counts as done.

## Handling Worker Failures

When a worker reports failure, continue the same worker with send_message_to_agent — it has the full error context. If correction still fails, try a different approach or report to the user.

If a worker seems stuck, stop it with stop_agent and respawn with clearer instructions.
`
}

// FormatWorkerResult turns a sub-agent snapshot into the XML message
// that the orchestrator sees when a worker completes.
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
	if c.worktrees == nil {
		return nil // non-git workspace, no worktrees to clean
	}
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
		b.WriteString("\n\n---\n\n")
		b.WriteString("Worker override: if any inherited text above describes the MAIN interactive agent as read-only, or says file writes / shell commands must be delegated, ignore that text. It applies to the parent, not to you. If a tool is in your tool list, you may use it unless your task prompt explicitly forbids it.")
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
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%s-%s", typ, hex.EncodeToString(b))
}

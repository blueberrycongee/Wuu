package claudeengine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/blueberrycongee/wuu/internal/agent"
	"github.com/blueberrycongee/wuu/internal/agentengine"
	"github.com/blueberrycongee/wuu/internal/providers"
)

// ResolveBinary locates the claude executable. The WUU_CLAUDE_BINARY
// environment variable wins; otherwise PATH lookup of "claude".
func ResolveBinary() (string, error) {
	if path := strings.TrimSpace(envClaudeBinary()); path != "" {
		return path, nil
	}
	path, err := exec.LookPath("claude")
	if err != nil {
		return "", errors.New("claude binary not found: set WUU_CLAUDE_BINARY or install the claude CLI on PATH")
	}
	return path, nil
}

func envClaudeBinary() string {
	return strings.TrimSpace(os.Getenv("WUU_CLAUDE_BINARY"))
}

// Version reports the CLI version via `claude --version`. Empty on failure.
func Version(binaryPath string) string {
	out, err := exec.Command(binaryPath, "--version").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// Engine is the claude agent engine: the Claude Code CLI hosted as an
// external process in headless stream-json mode. It implements
// agentengine.Factory and agentengine.ThreadBoundFactory.
type Engine struct {
	binaryPath string
	rootDir    string
}

// NewEngine builds the claude engine around a resolved binary path.
func NewEngine(binaryPath, rootDir string) *Engine {
	return &Engine{binaryPath: binaryPath, rootDir: rootDir}
}

// Descriptor describes the claude engine.
func (e *Engine) Descriptor(context.Context) (agentengine.Descriptor, error) {
	version := Version(e.binaryPath)
	return agentengine.Descriptor{
		ID:      agentengine.EngineID("claude"),
		Version: version,
		Capabilities: []string{
			"native-tool-loop",
			"native-session-resume",
		},
	}, nil
}

// Open starts a fresh claude session for a new conversation.
func (e *Engine) Open(ctx context.Context, req agentengine.OpenRequest) (agentengine.Session, error) {
	return e.newSession(ctx, sessionOptions{
		threadID: req.ThreadID,
		rootDir:  firstNonEmpty(req.RootDir, e.rootDir),
	})
}

// Resume reopens a claude session by its native session id.
func (e *Engine) Resume(ctx context.Context, req agentengine.ResumeRequest) (agentengine.Session, error) {
	return e.newSession(ctx, sessionOptions{
		threadID:    req.ThreadID,
		rootDir:     firstNonEmpty(req.RootDir, e.rootDir),
		externalRef: req.ExternalSessionRef,
	})
}

// SessionForThread binds the claude engine to an existing thread runtime.
func (e *Engine) SessionForThread(ctx context.Context, binding agentengine.ThreadBinding) (agentengine.Session, error) {
	return e.newSession(ctx, sessionOptions{
		threadID:       binding.ThreadID,
		rootDir:        firstNonEmpty(binding.RootDir, e.rootDir),
		model:          binding.Model,
		effort:         binding.Effort,
		permissionMode: binding.PermissionMode,
		externalRef:    binding.ExternalRef,
		persistRef:     binding.PersistRef,
	})
}

type sessionOptions struct {
	threadID       string
	rootDir        string
	model          string
	effort         string
	permissionMode string
	externalRef    string
	persistRef     func(string) error
}

func (e *Engine) newSession(ctx context.Context, opts sessionOptions) (agentengine.Session, error) {
	if e == nil {
		return nil, errors.New("claude engine is not configured")
	}
	return &Session{
		engine:         e,
		rootDir:        opts.rootDir,
		model:          opts.model,
		effort:         opts.effort,
		permissionMode: opts.permissionMode,
		ref:            opts.externalRef,
		persist:        opts.persistRef,
	}, nil
}

// Session is one wuu thread's handle on a claude CLI session. The claude
// child is spawned lazily on the first turn and resumed on later turns via
// --resume <session-id>.
type Session struct {
	engine         *Engine
	rootDir        string
	model          string
	effort         string
	permissionMode string

	mu      sync.Mutex
	ref     string
	persist func(string) error

	// writeMu serializes all stdin writes (user turns, tool results).
	writeMu sync.Mutex
}

// RunTurn sends one user prompt and translates the claude stream into Wuu
// events. It blocks until the result line or the context is canceled.
func (s *Session) RunTurn(ctx context.Context, input agentengine.TurnInput, sink agentengine.EventSink) (agentengine.TurnResult, error) {
	if s == nil || s.engine == nil {
		return agentengine.TurnResult{}, errors.New("claude session is not configured")
	}
	if err := ctx.Err(); err != nil {
		return agentengine.TurnResult{}, err
	}
	prompt := turnPrompt(input.History)
	if strings.TrimSpace(prompt) == "" {
		return agentengine.TurnResult{}, errors.New("claude turn requires a user message")
	}

	done := make(chan turnOutcome, 1)
	sub := newTurnSubscription(sink, done)
	transport, err := s.spawn(ctx, sub)
	if err != nil {
		sub.close()
		return agentengine.TurnResult{}, err
	}
	defer func() {
		sub.close()
		_ = transport.Close()
	}()

	s.writeMu.Lock()
	err = transport.WriteLine(ctx, marshalLine(userPromptEnvelope(prompt)))
	s.writeMu.Unlock()
	if err != nil {
		return agentengine.TurnResult{}, fmt.Errorf("send claude user input: %w", err)
	}

	select {
	case out := <-done:
		s.persistSessionID(sub)
		return out.result, out.err
	case <-ctx.Done():
		_ = transport.Close()
		select {
		case out := <-done:
			s.persistSessionID(sub)
			return out.result, out.err
		default:
			return agentengine.TurnResult{}, ctx.Err()
		}
	}
}

// persistSessionID records the claude session id from system/init into the
// thread's engine_ref so later turns resume the same session.
func (s *Session) persistSessionID(sub *turnSubscription) {
	sub.mu.Lock()
	sid := sub.sessionID
	sub.mu.Unlock()
	if sid == "" {
		return
	}
	s.mu.Lock()
	if s.ref == sid {
		s.mu.Unlock()
		return
	}
	s.ref = sid
	persist := s.persist
	s.mu.Unlock()
	if persist != nil {
		_ = persist(sid)
	}
}

// Interrupt cancels the in-flight turn. The child is closed; the next turn
// respawns it (resumed by session id when available).
func (s *Session) Interrupt(context.Context, string) error {
	return errors.New("claude interrupt is handled via turn context cancellation")
}

// Close releases the session. The child is closed by RunTurn's deferred
// cleanup; nothing to do here.
func (s *Session) Close(context.Context) error {
	return nil
}

// spawn starts (or resumes) the claude child for one turn. The subscription
// must be registered before spawn so no early line is missed.
func (s *Session) spawn(ctx context.Context, sub *turnSubscription) (*Transport, error) {
	s.mu.Lock()
	ref := s.ref
	s.mu.Unlock()

	args := []string{
		"-p",
		"--input-format", "stream-json",
		"--output-format", "stream-json",
		"--verbose",
		"--include-partial-messages",
		"--permission-mode", claudePermissionMode(s.permissionMode),
	}
	if model := strings.TrimSpace(s.model); model != "" {
		args = append(args, "--model", model)
	}
	if effort := strings.TrimSpace(s.effort); effort != "" {
		args = append(args, "--effort", effort)
	}
	if ref != "" {
		args = append(args, "--resume", ref)
	}
	transport, err := NewTransport(TransportOptions{
		BinaryPath: s.engine.binaryPath,
		Args:       args,
		CWD:        s.rootDir,
	})
	if err != nil {
		return nil, err
	}
	transport.OnLine(sub.handleLine)
	sub.attachTransport(transport)
	return transport, nil
}

func claudePermissionMode(mode string) string {
	switch strings.TrimSpace(mode) {
	case "read_only":
		return "plan"
	case "unconfined":
		return "bypassPermissions"
	default:
		// The headless stream-json transport has no Claude permission-prompt
		// bridge yet, so standard mode denies requests instead of hanging.
		return "dontAsk"
	}
}

// turnPrompt extracts the user's latest message as the turn input.
func turnPrompt(history []providers.ChatMessage) string {
	for i := len(history) - 1; i >= 0; i-- {
		msg := history[i]
		if msg.Role == "user" && strings.TrimSpace(msg.Content) != "" {
			return msg.Content
		}
	}
	return ""
}

func marshalLine(v any) string {
	data, _ := json.Marshal(v)
	return string(data)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// turnOutcome carries the converted loop result and terminal error.
type turnOutcome struct {
	result agentengine.TurnResult
	err    error
}

// turnSubscription translates claude stdout lines into Wuu stream events.
type turnSubscription struct {
	sink agentengine.EventSink
	done chan turnOutcome

	mu        sync.Mutex
	transport *Transport
	sessionID string
	closed    bool

	text          strings.Builder
	reasoning     strings.Builder
	usage         providers.TokenUsage
	tool          *pendingTool
	toolBuffer    strings.Builder
	agentTasks    map[string]observedAgentTask
	lastRawResult json.RawMessage
	finishOnce    sync.Once
}

type pendingTool struct {
	id            string
	name          string
	agentActivity bool
}

type observedAgentTask struct {
	activityID string
	label      string
}

func newTurnSubscription(sink agentengine.EventSink, done chan turnOutcome) *turnSubscription {
	return &turnSubscription{sink: sink, done: done, agentTasks: make(map[string]observedAgentTask)}
}

func (sub *turnSubscription) attachTransport(t *Transport) {
	sub.mu.Lock()
	sub.transport = t
	sub.mu.Unlock()
}

func (sub *turnSubscription) close() {
	sub.mu.Lock()
	sub.closed = true
	sub.mu.Unlock()
}

// claudeLine is the parsed top-level shape of one stdout line.
type claudeLine struct {
	Type         string          `json:"type"`
	Subtype      string          `json:"subtype"`
	Message      json.RawMessage `json:"message"`
	Event        string          `json:"event"`
	Delta        json.RawMessage `json:"delta"`
	ContentBlock json.RawMessage `json:"content_block"`
	IsError      bool            `json:"is_error"`
	StopReason   string          `json:"stop_reason"`
	Usage        json.RawMessage `json:"usage"`
	SessionID    string          `json:"session_id"`
	TaskID       string          `json:"task_id"`
	ToolUseID    string          `json:"tool_use_id"`
	TaskType     string          `json:"task_type"`
	Description  string          `json:"description"`
	Status       string          `json:"status"`
}

// handleLine dispatches one stdout line. Unknown top-level types are
// ignored (the raw JSON stays in diagnostics via stderr handlers).
func (sub *turnSubscription) handleLine(line string) {
	var envelope claudeLine
	if err := json.Unmarshal([]byte(line), &envelope); err != nil {
		sub.emit(providers.StreamEvent{Type: providers.EventError, Error: fmt.Errorf("claude sent invalid JSON: %w", err)})
		sub.finish(agent.LoopResult{}, fmt.Errorf("claude sent invalid JSON: %w", err))
		return
	}
	if envelope.Type == "result" {
		sub.mu.Lock()
		sub.lastRawResult = json.RawMessage(line)
		sub.mu.Unlock()
	}
	switch envelope.Type {
	case "system":
		// Real CLI 2.1.x emits the session id on the first system message
		// (init or hook_started), whichever arrives first. Track either.
		if envelope.SessionID != "" {
			sub.mu.Lock()
			sub.sessionID = envelope.SessionID
			sub.mu.Unlock()
		}
		if envelope.Subtype == "init" {
			var init initMessage
			if err := json.Unmarshal(envelope.Message, &init); err == nil && init.SessionID != "" {
				sub.mu.Lock()
				sub.sessionID = init.SessionID
				sub.mu.Unlock()
			}
		}
		sub.handleTaskEvent(envelope)
	case "assistant":
		sub.handleAssistant(envelope.Message)
	case "stream_event":
		sub.handleStreamEvent(envelope)
	case "user":
		// tool_result echo; nothing to surface.
	case "result":
		sub.handleResult(envelope)
	default:
		// Unknown type: keep going, the result line still terminates.
	}
}

func (sub *turnSubscription) handleAssistant(raw json.RawMessage) {
	var msg assistantMessage
	if err := json.Unmarshal(raw, &msg); err != nil {
		return
	}
	for _, block := range msg.Content {
		switch block.Type {
		case "text":
			if block.Text != "" {
				sub.text.WriteString(block.Text)
				sub.emit(providers.StreamEvent{
					Type:    providers.EventContentDelta,
					Content: block.Text,
				})
			}
		case "thinking":
			if block.Text != "" {
				sub.reasoning.WriteString(block.Text)
				sub.emit(providers.StreamEvent{
					Type:    providers.EventThinkingDelta,
					Content: block.Text,
				})
			}
		case "tool_use":
			sub.startTool(block)
		}
	}
}

func (sub *turnSubscription) handleStreamEvent(envelope claudeLine) {
	switch envelope.Event {
	case "content_block_start":
		var block assistantContentBlock
		if err := json.Unmarshal(envelope.ContentBlock, &block); err == nil && block.Type == "tool_use" {
			sub.startTool(block)
		}
	case "content_block_delta":
		var delta streamEventDelta
		if err := json.Unmarshal(envelope.Delta, &delta); err != nil {
			return
		}
		switch delta.Type {
		case "text_delta":
			if delta.Text != "" {
				sub.text.WriteString(delta.Text)
				sub.emit(providers.StreamEvent{Type: providers.EventContentDelta, Content: delta.Text})
			}
		case "thinking_delta":
			if delta.Thinking != "" {
				sub.reasoning.WriteString(delta.Thinking)
				sub.emit(providers.StreamEvent{Type: providers.EventThinkingDelta, Content: delta.Thinking})
			}
		case "input_json_delta":
			sub.toolBuffer.WriteString(delta.PartialJSON)
		}
	case "message_delta":
		var usage tokenUsage
		if len(envelope.Usage) > 0 && json.Unmarshal(envelope.Usage, &usage) == nil {
			sub.accumulateUsage(usage)
		}
	}
}

func (sub *turnSubscription) startTool(block assistantContentBlock) {
	if sub.tool != nil {
		// A previous tool never completed; close it without arguments.
		sub.finishTool(providers.AgentActivityCompleted)
	}
	agentActivity := isClaudeAgentTool(block.Name)
	sub.tool = &pendingTool{id: block.ID, name: block.Name, agentActivity: agentActivity}
	sub.toolBuffer.Reset()
	sub.emit(providers.StreamEvent{
		Type:     providers.EventToolUseStart,
		ToolCall: &providers.ToolCall{ID: block.ID, Name: block.Name},
	})
	if block.Input != nil {
		if args, err := json.Marshal(block.Input); err == nil {
			sub.toolBuffer.Write(args)
		}
	}
	if agentActivity {
		sub.emitAgentActivity(block.ID, claudeAgentToolLabel(block.Input), providers.AgentActivityRunning)
	}
}

func (sub *turnSubscription) handleTaskEvent(envelope claudeLine) {
	taskID := strings.TrimSpace(envelope.TaskID)
	if taskID == "" {
		return
	}
	switch envelope.Subtype {
	case "task_started":
		if taskType := strings.TrimSpace(envelope.TaskType); taskType != "" && !strings.Contains(strings.ToLower(taskType), "agent") {
			return
		}
		activityID := firstNonEmpty(envelope.ToolUseID, taskID)
		label := strings.TrimSpace(envelope.Description)
		if label == "" {
			label = "Claude agent"
		}
		sub.agentTasks[taskID] = observedAgentTask{activityID: activityID, label: label}
		sub.emitAgentActivity(activityID, label, providers.AgentActivityRunning)
	case "task_progress":
		task, ok := sub.agentTasks[taskID]
		if !ok {
			return
		}
		if label := strings.TrimSpace(envelope.Description); label != "" {
			task.label = label
			sub.agentTasks[taskID] = task
		}
		sub.emitAgentActivity(task.activityID, task.label, providers.AgentActivityRunning)
	case "task_notification":
		task, ok := sub.agentTasks[taskID]
		if !ok {
			return
		}
		state := claudeTaskTerminalState(envelope.Status)
		sub.emitAgentActivity(task.activityID, task.label, state)
		if state == providers.AgentActivityCompleted || state == providers.AgentActivityFailed {
			delete(sub.agentTasks, taskID)
		}
	}
}

func (sub *turnSubscription) finishTool(agentState providers.AgentActivityState) {
	if sub.tool == nil {
		return
	}
	tool := sub.tool
	sub.emit(providers.StreamEvent{
		Type:     providers.EventToolUseEnd,
		ToolCall: &providers.ToolCall{ID: tool.id, Name: tool.name, Arguments: sub.toolBuffer.String()},
	})
	if tool.agentActivity && !sub.tracksAgentActivity(tool.id) {
		sub.emitAgentActivity(tool.id, "Claude agent", agentState)
	}
	sub.tool = nil
	sub.toolBuffer.Reset()
}

func (sub *turnSubscription) tracksAgentActivity(activityID string) bool {
	for _, task := range sub.agentTasks {
		if task.activityID == activityID {
			return true
		}
	}
	return false
}

func (sub *turnSubscription) emitAgentActivity(id, label string, state providers.AgentActivityState) {
	id = strings.TrimSpace(id)
	if id == "" {
		return
	}
	sub.emit(providers.StreamEvent{
		Type: providers.EventAgentActivity,
		AgentActivity: &providers.AgentActivity{
			ID:     id,
			Engine: "claude",
			Label:  firstNonEmpty(strings.TrimSpace(label), "Claude agent"),
			State:  state,
		},
	})
}

func claudeTaskTerminalState(status string) providers.AgentActivityState {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "running", "in_progress":
		return providers.AgentActivityRunning
	case "failed", "error", "errored":
		return providers.AgentActivityFailed
	default:
		return providers.AgentActivityCompleted
	}
}

func isClaudeAgentTool(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "agent", "task":
		return true
	default:
		return false
	}
}

func claudeAgentToolLabel(input any) string {
	fields, ok := input.(map[string]any)
	if !ok {
		return "Claude agent"
	}
	for _, key := range []string{"name", "description", "subagent_type"} {
		if value, ok := fields[key].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return "Claude agent"
}

func (sub *turnSubscription) accumulateUsage(u tokenUsage) {
	sub.usage = providers.TokenUsage{
		InputTokens:         u.InputTokens,
		OutputTokens:        u.OutputTokens,
		CacheCreationTokens: u.CacheCreationInputTokens,
		CacheReadTokens:     u.CacheReadInputTokens,
	}
	sub.emit(providers.StreamEvent{Type: providers.EventUsage, Usage: &sub.usage})
}

func (sub *turnSubscription) handleResult(envelope claudeLine) {
	// Close any in-flight tool rendering.
	if sub.tool != nil {
		state := providers.AgentActivityCompleted
		if envelope.IsError {
			state = providers.AgentActivityFailed
		}
		sub.finishTool(state)
	}
	if envelope.IsError {
		message := "claude turn failed"
		var res resultMessage
		if err := json.Unmarshal(sub.lastRawResult, &res); err == nil && res.Error != nil && res.Error.Message != "" {
			message = res.Error.Message
		}
		err := errors.New(message)
		sub.emit(providers.StreamEvent{Type: providers.EventError, Error: err})
		sub.finish(agent.LoopResult{}, err)
		return
	}
	var usage tokenUsage
	if len(envelope.Usage) > 0 {
		_ = json.Unmarshal(envelope.Usage, &usage)
		sub.accumulateUsage(usage)
	}
	sub.emit(providers.StreamEvent{
		Type:         providers.EventDone,
		FinishReason: providers.FinishReasonStop,
		StopReason:   envelope.StopReason,
	})
	content := sub.text.String()
	result := agent.LoopResult{
		Content:         strings.TrimSpace(content),
		FinishReason:    providers.FinishReasonStop,
		StopReason:      envelope.StopReason,
		InputTokens:     sub.usage.InputTokens,
		OutputTokens:    sub.usage.OutputTokens,
		CacheReadTokens: sub.usage.CacheReadTokens,
		NewMessages: []providers.ChatMessage{{
			Role:             "assistant",
			Content:          content,
			ReasoningContent: sub.reasoning.String(),
		}},
	}
	sub.finish(result, nil)
}

func (sub *turnSubscription) emit(ev providers.StreamEvent) {
	if sub.sink != nil {
		sub.sink(ev)
	}
}

func (sub *turnSubscription) finish(result agent.LoopResult, err error) {
	sub.finishOnce.Do(func() {
		select {
		case sub.done <- turnOutcome{result: agentengine.TurnResult{Result: result}, err: err}:
		default:
		}
	})
}

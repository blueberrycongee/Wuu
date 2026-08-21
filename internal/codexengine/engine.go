package codexengine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/blueberrycongee/wuu/internal/agent"
	"github.com/blueberrycongee/wuu/internal/agentengine"
	"github.com/blueberrycongee/wuu/internal/providers"
)

// Engine is the codex agent engine: the Codex CLI app-server hosted as an
// external process. It implements agentengine.Factory and
// agentengine.ThreadBoundFactory so the app-server can route threads bound
// to engine_id "codex" through it.
type Engine struct {
	host *Host

	// ApproveCommand decides shell-command approval requests. Nil denies
	// everything (the design's default: no mapping, no approval).
	ApproveCommand func(CommandExecutionApprovalParams) string
	// ApproveFileChange decides file-change approval requests. Nil denies.
	ApproveFileChange func(FileChangeApprovalParams) string
}

// NewEngine builds the codex engine around a host (binary discovery +
// app-server lifecycle).
func NewEngine(host *Host) *Engine {
	return &Engine{host: host}
}

// Descriptor describes the codex engine.
func (e *Engine) Descriptor(context.Context) (agentengine.Descriptor, error) {
	return agentengine.Descriptor{
		ID:      agentengine.EngineID("codex"),
		Version: "1",
		Capabilities: []string{
			"native-tool-loop",
			"native-session-resume",
			"approval-requests",
		},
	}, nil
}

// Open starts a codex thread for a new conversation.
func (e *Engine) Open(ctx context.Context, req agentengine.OpenRequest) (agentengine.Session, error) {
	return e.newSession(ctx, sessionOptions{
		threadID: req.ThreadID,
		rootDir:  req.RootDir,
	})
}

// Resume reopens an existing codex thread by its native reference.
func (e *Engine) Resume(ctx context.Context, req agentengine.ResumeRequest) (agentengine.Session, error) {
	return e.newSession(ctx, sessionOptions{
		threadID:    req.ThreadID,
		rootDir:     req.RootDir,
		externalRef: req.ExternalSessionRef,
	})
}

// SessionForThread binds the codex engine to an existing thread runtime (the
// live app-server path). The binding carries the persisted codex thread id;
// an empty ref means the first turn creates the codex thread lazily.
func (e *Engine) SessionForThread(ctx context.Context, binding agentengine.ThreadBinding) (agentengine.Session, error) {
	return e.newSession(ctx, sessionOptions{
		threadID:    binding.ThreadID,
		rootDir:     binding.RootDir,
		model:       binding.Model,
		externalRef: binding.ExternalRef,
		persistRef:  binding.PersistRef,
	})
}

type sessionOptions struct {
	threadID    string
	rootDir     string
	model       string
	externalRef string
	persistRef  func(string) error
}

func (e *Engine) newSession(ctx context.Context, opts sessionOptions) (agentengine.Session, error) {
	if e == nil || e.host == nil {
		return nil, errors.New("codex engine is not configured")
	}
	client, err := e.host.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	return &Session{
		engine:   e,
		client:   client,
		threadID: opts.threadID,
		rootDir:  opts.rootDir,
		model:    opts.model,
		ref:      opts.externalRef,
		persist:  opts.persistRef,
	}, nil
}

// Session is one wuu thread's handle on a codex thread. The codex thread is
// created lazily on the first turn (thread/start) and its id is persisted via
// the binding's PersistRef; later turns run turn/start directly on the shared
// app-server, or thread/resume first if the app-server restarted.
type Session struct {
	engine *Engine
	client *Client

	threadID string
	rootDir  string
	model    string

	mu      sync.Mutex
	ref     string
	persist func(string) error
}

// RunTurn starts a codex turn and translates its notifications into Wuu
// stream events. It blocks until turn/completed or the context is canceled.
func (s *Session) RunTurn(ctx context.Context, input agentengine.TurnInput, sink agentengine.EventSink) (agentengine.TurnResult, error) {
	if s == nil || s.client == nil {
		return agentengine.TurnResult{}, errors.New("codex session has no app-server client")
	}
	if err := ctx.Err(); err != nil {
		return agentengine.TurnResult{}, err
	}
	// First turn: create the codex thread and persist its id so later turns
	// (and later app-server processes) resume the same native session.
	if err := s.ensureThread(ctx); err != nil {
		return agentengine.TurnResult{}, err
	}
	prompt := turnPrompt(input.History)
	// Subscribe before turn/start: the app-server streams notifications
	// immediately after responding, so a late subscription would drop them.
	done := make(chan turnOutcome, 1)
	sub := newTurnSubscription(s.client, sink, done)
	defer sub.close()
	turnResp, err := s.startTurn(ctx, prompt)
	if err != nil {
		return agentengine.TurnResult{}, err
	}
	turnID := turnResp.Turn.ID
	sub.setTurnID(turnID)

	select {
	case out := <-done:
		return out.result, out.err
	case <-ctx.Done():
		_ = s.interrupt(context.Background(), turnID)
		select {
		case out := <-done:
			return out.result, out.err
		default:
			return agentengine.TurnResult{}, ctx.Err()
		}
	}
}

// ensureThread runs thread/start once and persists the native thread id.
func (s *Session) ensureThread(ctx context.Context) error {
	s.mu.Lock()
	ref := s.ref
	s.mu.Unlock()
	if ref != "" {
		return nil
	}
	var resp ThreadStartResponse
	err := s.client.Request(ctx, MethodThreadStart, ThreadStartParams{
		Model: s.model,
		CWD:   s.rootDir,
		// Interactive approvals surface through the server request handlers;
		// the engine's default decision policy applies when no handler is
		// wired (deny).
		ApprovalPolicy: ApprovalOnRequest,
	}, &resp)
	if err != nil {
		return fmt.Errorf("codex thread/start: %w", err)
	}
	ref = resp.Thread.ID
	if strings.TrimSpace(ref) == "" {
		return errors.New("codex thread/start returned an empty thread id")
	}
	if s.persist != nil {
		if err := s.persist(ref); err != nil {
			// The native thread exists but we could not record it; fail the
			// turn rather than orphan it silently.
			return fmt.Errorf("persist codex thread reference: %w", err)
		}
	}
	s.mu.Lock()
	s.ref = ref
	s.mu.Unlock()
	return nil
}

func (s *Session) startTurn(ctx context.Context, prompt string) (TurnStartResponse, error) {
	var resp TurnStartResponse
	err := s.client.Request(ctx, MethodTurnStart, TurnStartParams{
		ThreadID: s.ref,
		Input:    []UserInput{{Type: "text", Text: prompt}},
		Model:    s.model,
	}, &resp)
	return resp, err
}

func (s *Session) interrupt(ctx context.Context, turnID string) error {
	if s.ref == "" {
		return agentengine.ErrNoActiveTurn
	}
	var out map[string]any
	return s.client.Request(ctx, MethodTurnInterrupt, TurnInterruptParams{
		ThreadID: s.ref,
		TurnID:   turnID,
	}, &out)
}

// Interrupt cancels the in-flight turn via turn/interrupt.
func (s *Session) Interrupt(ctx context.Context, reason string) error {
	if s == nil {
		return errors.New("codex session is nil")
	}
	s.mu.Lock()
	ref := s.ref
	s.mu.Unlock()
	if ref == "" {
		return agentengine.ErrNoActiveTurn
	}
	return s.client.Request(ctx, MethodTurnInterrupt, TurnInterruptParams{
		ThreadID: ref,
		TurnID:   "",
	}, &map[string]any{})
}

// Close releases the client reference held by this session. The shared
// app-server stays alive while other threads use it.
func (s *Session) Close(context.Context) error {
	if s == nil || s.engine == nil || s.engine.host == nil {
		return nil
	}
	s.engine.host.Release()
	return nil
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

// turnOutcome carries the converted loop result and terminal error.
type turnOutcome struct {
	result agentengine.TurnResult
	err    error
}

// turnNotification is one queued notification with its method name (the
// client dispatches by method; the subscription needs it to translate).
type turnNotification struct {
	method string
	params json.RawMessage
}

// turnSubscription translates the notifications of one codex turn into Wuu
// stream events and signals completion.
type turnSubscription struct {
	client *Client
	turnID string
	sink   agentengine.EventSink
	done   chan turnOutcome

	events chan turnNotification
	mu     sync.Mutex
	closed bool
	unsubs []func()
}

func newTurnSubscription(client *Client, sink agentengine.EventSink, done chan turnOutcome) *turnSubscription {
	sub := &turnSubscription{
		client: client,
		sink:   sink,
		done:   done,
		events: make(chan turnNotification, 512),
	}
	handlers := map[string]struct{}{
		NotifyAgentMessageDelta:     {},
		NotifyReasoningTextDelta:    {},
		NotifyReasoningSummaryDelta: {},
		NotifyTokenUsageUpdated:     {},
		NotifyTurnCompleted:         {},
		NotifyError:                 {},
	}
	for method := range handlers {
		sub.unsubs = append(sub.unsubs, client.OnNotification(method, func(params json.RawMessage) {
			sub.enqueue(turnNotification{method: method, params: params})
		}))
	}
	return sub
}

// setTurnID fixes the turn the subscription filters for and starts the
// consume loop. Notifications that arrived before the id was known (between
// subscription and turn/start response) stay buffered and are filtered once
// the id is set.
func (sub *turnSubscription) setTurnID(turnID string) {
	sub.turnID = turnID
	go sub.run()
}

func (sub *turnSubscription) enqueue(notification turnNotification) {
	select {
	case sub.events <- notification:
	default:
		// A slow sink must not wedge the app-server; drop the notification
		// and let turn/completed reconcile the final state.
	}
}

func (sub *turnSubscription) close() {
	sub.mu.Lock()
	if sub.closed {
		sub.mu.Unlock()
		return
	}
	sub.closed = true
	unsubs := sub.unsubs
	sub.unsubs = nil
	sub.mu.Unlock()
	for _, unsub := range unsubs {
		unsub()
	}
}

// run consumes notifications until turn/completed or errorNotification, then
// reports the outcome. Events from other turns on the shared app-server are
// filtered by turn id.
func (sub *turnSubscription) run() {
	var (
		text      strings.Builder
		reasoning strings.Builder
		usage     providers.TokenUsage
	)
	for notification := range sub.events {
		if sub.turnID == "" {
			// No turn id (should not happen); drop everything.
			continue
		}
		var envelope struct {
			TurnID string `json:"turnId"`
			ItemID string `json:"itemId"`
			Delta  string `json:"delta"`
			Turn   struct {
				ID     string `json:"id"`
				Status string `json:"status"`
			} `json:"turn"`
			Message string `json:"message"`
			Error   *struct {
				Message string `json:"message"`
			} `json:"error"`
			TokenUsage *struct {
				Total TokenUsageBreakdown `json:"total"`
				Last  TokenUsageBreakdown `json:"last"`
			} `json:"tokenUsage"`
		}
		if err := json.Unmarshal(notification.params, &envelope); err != nil {
			continue
		}
		// turn/completed carries the turn id nested; all other notifications
		// carry it top-level.
		turnID := envelope.TurnID
		if turnID == "" && envelope.Turn.ID != "" {
			turnID = envelope.Turn.ID
		}
		if turnID != sub.turnID {
			continue
		}
		switch notification.method {
		case NotifyAgentMessageDelta:
			if envelope.Delta != "" {
				text.WriteString(envelope.Delta)
				sub.emit(providers.StreamEvent{
					Type:           providers.EventContentDelta,
					Content:        envelope.Delta,
					ProviderItemID: envelope.ItemID,
				})
			}
		case NotifyReasoningTextDelta, NotifyReasoningSummaryDelta:
			if envelope.Delta != "" {
				reasoning.WriteString(envelope.Delta)
				sub.emit(providers.StreamEvent{
					Type:           providers.EventThinkingDelta,
					Content:        envelope.Delta,
					ProviderItemID: envelope.ItemID,
				})
			}
		case NotifyTokenUsageUpdated:
			if envelope.TokenUsage != nil {
				// "last" is this turn's increment; "total" is the thread
				// lifetime cumulative. Convert to wuu's TokenUsage shape:
				// uncached input, output incl. reasoning, cached read.
				last := envelope.TokenUsage.Last
				usage.InputTokens += last.InputTokens - last.CachedInputTokens
				usage.OutputTokens += last.OutputTokens + last.ReasoningOutputTokens
				usage.CacheReadTokens += last.CachedInputTokens
				sub.emit(providers.StreamEvent{Type: providers.EventUsage, Usage: &usage})
			}
		case NotifyError:
			lastErr := errors.New("codex app-server reported an error")
			if envelope.Error != nil && strings.TrimSpace(envelope.Error.Message) != "" {
				lastErr = errors.New(strings.TrimSpace(envelope.Error.Message))
			}
			sub.emit(providers.StreamEvent{Type: providers.EventError, Error: lastErr})
			sub.finish(agent.LoopResult{}, lastErr)
			return
		case NotifyTurnCompleted:
			status := envelope.Turn.Status
			finishReason := providers.FinishReasonStop
			if status != "" && status != "completed" {
				finishReason = providers.FinishReasonError
			}
			sub.emit(providers.StreamEvent{
				Type:         providers.EventDone,
				FinishReason: finishReason,
				StopReason:   status,
			})
			content := text.String()
			result := agent.LoopResult{
				Content:         strings.TrimSpace(content),
				FinishReason:    finishReason,
				StopReason:      status,
				InputTokens:     usage.InputTokens,
				OutputTokens:    usage.OutputTokens,
				CacheReadTokens: usage.CacheReadTokens,
				NewMessages: []providers.ChatMessage{{
					Role:             "assistant",
					Content:          content,
					ReasoningContent: reasoning.String(),
				}},
			}
			sub.finish(result, nil)
			return
		}
	}
	// Stream closed without a terminal notification.
	sub.finish(agent.LoopResult{}, errors.New("codex turn stream closed before completion"))
}

func (sub *turnSubscription) emit(ev providers.StreamEvent) {
	if sub.sink != nil {
		sub.sink(ev)
	}
}

func (sub *turnSubscription) finish(result agent.LoopResult, err error) {
	select {
	case sub.done <- turnOutcome{result: agentengine.TurnResult{Result: result}, err: err}:
	default:
	}
}

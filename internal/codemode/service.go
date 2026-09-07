package codemode

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/blueberrycongee/wuu/internal/providers"
	"github.com/blueberrycongee/wuu/internal/toolctx"
)

// DefaultToolNamespace is the host's default namespace for unnamespaced tools.
const DefaultToolNamespace = "functions"

// ToolNameString decodes the host's ToolName into the provider tool name Wuu
// dispatches. It mirrors the code-mode host's encoding: the default namespace
// is the plain name; a namespace ending in "_" or a name starting with "_"
// concatenates directly; anything else joins with "__".
func ToolNameString(tool ToolName) string {
	namespace := ""
	if tool.Namespace != nil {
		namespace = *tool.Namespace
	}
	if namespace == "" || namespace == DefaultToolNamespace {
		return tool.Name
	}
	if strings.HasSuffix(namespace, "_") || strings.HasPrefix(tool.Name, "_") {
		return namespace + tool.Name
	}
	return namespace + "__" + tool.Name
}

// NotifyFunc delivers one host notification to the session's user-visible
// channel. Arguments are the delegate call ID, the owning cell ID, and the
// notification text.
type NotifyFunc func(ctx context.Context, callID, cellID, text string) error

// ServiceConfig pins how one Wuu session talks to its code-mode host.
type ServiceConfig struct {
	// Executable is the absolute path of the wuu-code-mode-host binary. The
	// launcher rejects relative paths; it never searches or downloads one.
	Executable string
	// SessionID names the Wuu session. One client owns one host connection;
	// connections are never shared between Wuu sessions.
	SessionID string
	// Limits are negotiated at session/open. The host must support them or the
	// handshake fails.
	Limits CellLimits
	// DefaultYieldMS is applied when an execute request leaves yield_time_ms
	// unset. Zero leaves the host default.
	DefaultYieldMS uint64
	// Stderr receives host process diagnostics. May be nil.
	Stderr io.Writer
	// Notify delivers notification/send delegate requests.
	Notify NotifyFunc
	// Life bounds the host connection's lifetime. When it ends, the process is
	// killed even if an isolate is stuck. Nil means a background lifetime.
	Life context.Context
}

// Service is the session-scoped owner of one host connection and the delegate
// bridge between host cells and Wuu's nested tool execution. Tool invocations
// are routed through the cell's owning execution scope — policy, scheduling,
// the shared gate, and durable recording all apply. Transport failure is
// terminal: the client never reconnects and never replays an execute.
type Service struct {
	config ServiceConfig

	mu          sync.Mutex
	client      *Client
	cells       map[string]toolctx.NestedExecutor
	pendingExec int
	started     bool
	bound       chan struct{}
}

// NewService builds the session service. Executable must be an absolute path.
func NewService(config ServiceConfig) (*Service, error) {
	if config.Executable == "" {
		return nil, errors.New("code-mode host executable is not configured")
	}
	if config.SessionID == "" {
		return nil, errors.New("code-mode session ID is required")
	}
	return &Service{config: config, cells: make(map[string]toolctx.NestedExecutor), bound: make(chan struct{})}, nil
}

// Available reports why code mode cannot run in this session, or nil.
func (s *Service) Available() error {
	if s == nil || s.config.Executable == "" {
		return errors.New("code-mode host is not configured")
	}
	return nil
}

func (s *Service) clientLocked(ctx context.Context) (*Client, error) {
	if s.client != nil {
		return s.client, nil
	}
	life := s.config.Life
	if life == nil {
		life = context.Background()
	}
	client, err := Start(life, s.config.Executable, s.config.SessionID, s.config.Limits, s, s.config.Stderr)
	if err != nil {
		return nil, fmt.Errorf("connect code-mode host: %w", err)
	}
	s.client = client
	s.started = true
	return client, nil
}

// Execute starts one cell. EnabledTools is the effective model tool surface of
// this turn, including the current edit mode. Callers that can supply the
// owning nested scope should use ExecuteBound so the cell is bound before its
// first delegate frame can be dispatched.
func (s *Service) Execute(ctx context.Context, request ExecuteRequest) (Response, error) {
	if request.Source == "" {
		return Response{}, errors.New("code-mode execute requires source")
	}
	if request.YieldTimeMS == nil && s.config.DefaultYieldMS != 0 {
		yield := s.config.DefaultYieldMS
		request.YieldTimeMS = &yield
	}
	s.mu.Lock()
	client, err := s.clientLocked(ctx)
	s.mu.Unlock()
	if err != nil {
		return Response{}, err
	}
	response, err := client.Execute(ctx, request)
	if err != nil {
		return Response{}, err
	}
	if response.State == "Result" || response.State == "Terminated" {
		s.unbind(response.CellID)
	}
	return response, nil
}

// ExecuteBound starts a cell and atomically binds its owning nested scope
// before any delegate frame for the cell can be dispatched. The host starts
// executing a cell before the acknowledgement arrives, so binding after
// Execute returns would race the cell's first tool call. A yielded cell keeps
// its binding until it terminates; an immediately finished cell releases it.
func (s *Service) ExecuteBound(ctx context.Context, request ExecuteRequest, executor toolctx.NestedExecutor) (Response, error) {
	if executor == nil {
		return Response{}, errors.New("code-mode execute requires an owning nested scope")
	}
	s.mu.Lock()
	s.pendingExec++
	s.mu.Unlock()
	response, err := s.Execute(ctx, request)
	s.mu.Lock()
	s.pendingExec--
	if err == nil && response.State == "Yielded" {
		if bound, ok := s.cells[response.CellID]; !ok || bound != executor {
			s.cells[response.CellID] = executor
			s.signalBindingLocked()
			s.watchScopeLifecycle(response.CellID, executor)
		}
	}
	s.signalBindingLocked()
	s.mu.Unlock()
	if err != nil {
		return Response{}, err
	}
	return response, nil
}

// watchScopeLifecycle terminates a yielded cell as soon as its owning turn
// scope ends, whether through normal completion or cancellation. A cell
// without an owning turn must not keep executing or making nested calls.
func (s *Service) watchScopeLifecycle(cellID string, executor toolctx.NestedExecutor) {
	scoped, ok := executor.(toolctx.ScopedExecutor)
	if !ok {
		return
	}
	life := s.config.Life
	if life == nil {
		life = context.Background()
	}
	go func() {
		select {
		case <-scoped.Done():
			terminateCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			_, _ = s.Terminate(terminateCtx, cellID)
			cancel()
		case <-life.Done():
		}
	}()
}

// BindCell attaches the owning execution scope for a yielded cell. Nested tool
// calls from the cell route through this executor until the cell terminates.
// Binding the same cell to the same executor is idempotent; rebinding a live
// cell to a different executor is rejected.
func (s *Service) BindCell(cellID string, executor toolctx.NestedExecutor) error {
	if cellID == "" || executor == nil {
		return errors.New("code-mode cell binding requires a cell ID and executor")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if bound, ok := s.cells[cellID]; ok && bound != executor {
		return errors.New("code-mode cell is already bound to another execution scope")
	}
	s.cells[cellID] = executor
	s.signalBindingLocked()
	return nil
}

func (s *Service) unbind(cellID string) {
	s.mu.Lock()
	delete(s.cells, cellID)
	s.signalBindingLocked()
	s.mu.Unlock()
}

func (s *Service) signalBindingLocked() {
	close(s.bound)
	s.bound = make(chan struct{})
}

// waitForBinding pauses until the cell's owning scope appears, ctx ends, or no
// execute is in flight that could still produce the binding. A cell starts
// running before Execute returns its acknowledgement, so its first delegate
// frames can arrive while the exec tool handler is still binding the
// acknowledgement's cell ID to the scope. Delegate delivery is per-request on
// a separate goroutine, so this wait never blocks the wire reader, and the
// in-flight guard keeps a stale frame for a finished cell from hanging.
func (s *Service) waitForBinding(ctx context.Context, cellID string) error {
	for {
		s.mu.Lock()
		if _, ok := s.cells[cellID]; ok {
			s.mu.Unlock()
			return nil
		}
		if s.pendingExec == 0 {
			s.mu.Unlock()
			return errNoCellScope
		}
		wait := s.bound
		s.mu.Unlock()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-wait:
		}
	}
}

var errNoCellScope = errors.New("code-mode cell has no owning execution scope")

func (s *Service) cellScope(cellID string) toolctx.NestedExecutor {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cells[cellID]
}

// Wait observes one cell's incremental output. A Terminated, Result, or
// Missing response releases the cell binding.
func (s *Service) Wait(ctx context.Context, cellID string, yieldTimeMS uint64) (Response, error) {
	s.mu.Lock()
	client := s.client
	s.mu.Unlock()
	if client == nil {
		return Response{}, errors.New("code-mode session is not running")
	}
	response, err := client.Wait(ctx, cellID, yieldTimeMS)
	if err != nil {
		return Response{}, err
	}
	if response.State != "Yielded" || response.Missing {
		s.unbind(cellID)
	}
	return response, nil
}

// Terminate stops one cell and releases its binding.
func (s *Service) Terminate(ctx context.Context, cellID string) (Response, error) {
	s.mu.Lock()
	client := s.client
	s.mu.Unlock()
	if client == nil {
		s.unbind(cellID)
		return Response{}, nil
	}
	response, err := client.Terminate(ctx, cellID)
	s.unbind(cellID)
	return response, err
}

// Interrupt terminates every live cell. It is the turn-cancellation hook:
// closing the owning turn must stop cell execution and its nested tools.
func (s *Service) Interrupt(ctx context.Context) {
	s.mu.Lock()
	client := s.client
	cellIDs := make([]string, 0, len(s.cells))
	for cellID := range s.cells {
		cellIDs = append(cellIDs, cellID)
	}
	s.mu.Unlock()
	if client == nil {
		return
	}
	for _, cellID := range cellIDs {
		if _, err := client.Terminate(ctx, cellID); err != nil {
			continue
		}
		s.unbind(cellID)
	}
}

// Close tears down the host connection and all live cells.
func (s *Service) Close() error {
	s.mu.Lock()
	client := s.client
	s.client = nil
	s.cells = make(map[string]toolctx.NestedExecutor)
	s.mu.Unlock()
	if client == nil {
		return nil
	}
	return client.Close()
}

// Invoke implements Delegate. It resolves the owning scope of the requesting
// cell and dispatches through Wuu's nested execution pipeline. It never calls
// a tool registry directly, and it retains structured JSON without flattening.
func (s *Service) Invoke(ctx context.Context, invocation Invocation) (json.RawMessage, error) {
	if invocation.CellID == "" || invocation.RuntimeToolCallID == "" {
		return nil, errors.New("code-mode tool invocation requires a cell and call ID")
	}
	executor := s.cellScope(invocation.CellID)
	if executor == nil {
		if err := s.waitForBinding(ctx, invocation.CellID); err != nil {
			return nil, err
		}
		executor = s.cellScope(invocation.CellID)
		if executor == nil {
			return nil, fmt.Errorf("code-mode cell %q has no owning execution scope", invocation.CellID)
		}
	}
	call := providers.ToolCall{
		ID:        invocation.RuntimeToolCallID,
		Name:      ToolNameString(invocation.ToolName),
		Arguments: string(invocation.Input),
		Kind:      providers.ToolCallKindFunction,
	}
	result, err := executor.Invoke(ctx, call)
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("encode code-mode tool result: %w", err)
	}
	return data, nil
}

// Notify implements Delegate. It forwards host notifications to the session
// channel configured on the service.
func (s *Service) Notify(ctx context.Context, callID, cellID, text string) error {
	if s.config.Notify == nil {
		return errors.New("code-mode notifications are not connected")
	}
	return s.config.Notify(ctx, callID, cellID, text)
}

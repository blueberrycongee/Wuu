package dream

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	pluginapi "github.com/blueberrycongee/wuu/packages/plugin-go"
)

const (
	capabilityTurnCompleted = "agent.turn.completed"
	capabilityLifecycle     = "agent.turn.lifecycle"
	capabilityClient        = "plugin.client.request"
	// sessionMemoryService is the registry name dream resolves to read the
	// current project memory before each run. The name is the contract; no
	// provider-specific wiring exists on either side.
	sessionMemoryService  = "memory.session"
	stateStorageKey       = "dream.state"
	defaultIntervalDays   = 7
	defaultMinSessions    = 5
	failureBackoff        = time.Hour
)

type settings struct {
	Enabled      bool   `json:"enabled"`
	IntervalDays int    `json:"interval_days"`
	MinSessions  int    `json:"min_sessions"`
	ModelAlias   string `json:"model_alias,omitempty"`
}

type persistedState struct {
	Settings        settings          `json:"settings"`
	Candidates      map[string]string `json:"candidates,omitempty"`
	LastRunAt       time.Time         `json:"last_run_at,omitempty"`
	LastStartedAt   time.Time         `json:"last_started_at,omitempty"`
	LastFinishedAt  time.Time         `json:"last_finished_at,omitempty"`
	LastStatus      string            `json:"last_status,omitempty"`
	LastError       string            `json:"last_error,omitempty"`
	ActiveRequestID string            `json:"active_request_id,omitempty"`
	ActiveSessionID string            `json:"active_session_id,omitempty"`
}

type turnCompletedInput struct {
	ThreadID    string    `json:"thread_id"`
	CompletedAt time.Time `json:"completed_at"`
	Succeeded   bool      `json:"succeeded"`
}

type controller struct {
	mu       sync.Mutex
	host     pluginapi.Host
	state    persistedState
	running  bool
	stop     chan struct{}
	done     chan struct{}
	stopOnce sync.Once
	now      func() time.Time
	tick     time.Duration
}

func Handler() pluginapi.Handler {
	c := &controller{now: time.Now, tick: 15 * time.Second}
	return pluginapi.Handler{
		Definition: pluginapi.Definition{
			Capabilities: []pluginapi.Capability{
				{ID: capabilityTurnCompleted, Kind: "observe", Version: 1, ErrorPolicy: "isolate"},
				{ID: capabilityLifecycle, Kind: "observe", Version: 1, ErrorPolicy: "isolate"},
				{ID: capabilityClient, Kind: "decision", Version: 1},
			},
			RequiredHostServices: []pluginapi.HostService{
				{ID: pluginapi.HostServiceSessionCreate, Required: true},
				{ID: pluginapi.HostServiceSessionSend, Required: true},
				{ID: pluginapi.HostServiceStorageGet, Required: true},
				{ID: pluginapi.HostServiceStorageSet, Required: true},
			},
			RequiredServices: []pluginapi.ServiceRequirement{
				{Name: sessionMemoryService, MajorVersion: 1},
			},
		},
		Initialize: func(ctx context.Context, host pluginapi.Host, _ pluginapi.InitializeParams) error {
			return c.prepare(ctx, host)
		},
		Activate: c.activate,
		Shutdown: c.shutdown,
		InvokeCapability: func(ctx context.Context, _ pluginapi.Host, call pluginapi.CapabilityCall) (json.RawMessage, error) {
			return c.invokeCapability(ctx, call)
		},
	}
}

func (c *controller) prepare(ctx context.Context, host pluginapi.Host) error {
	if host == nil {
		return errors.New("dream host is required")
	}
	c.mu.Lock()
	c.host = host
	c.stop = make(chan struct{})
	c.done = make(chan struct{})
	c.state = persistedState{Settings: defaultSettings(), Candidates: make(map[string]string)}
	c.mu.Unlock()
	if err := c.load(ctx); err != nil {
		return err
	}
	return nil
}

func (c *controller) activate(ctx context.Context) error {
	if err := c.save(ctx); err != nil {
		return err
	}
	go c.loop()
	return nil
}

func (c *controller) shutdown(context.Context) error {
	c.stopOnce.Do(func() { close(c.stop) })
	<-c.done
	return nil
}

func (c *controller) loop() {
	defer close(c.done)
	ticker := time.NewTicker(c.tick)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			c.startIfDue(context.Background())
		case <-c.stop:
			return
		}
	}
}

func defaultSettings() settings {
	return settings{IntervalDays: defaultIntervalDays, MinSessions: defaultMinSessions}
}

func (c *controller) load(ctx context.Context) error {
	var result struct {
		Value *string `json:"value"`
	}
	if err := c.host.CallHost(ctx, pluginapi.HostServiceStorageGet, map[string]any{"scope": "workspace", "key": stateStorageKey}, &result); err != nil {
		return err
	}
	if result.Value == nil {
		return nil
	}
	var state persistedState
	if err := json.Unmarshal([]byte(*result.Value), &state); err != nil {
		return fmt.Errorf("decode dream plugin state: %w", err)
	}
	if state.Settings.IntervalDays <= 0 {
		state.Settings.IntervalDays = defaultIntervalDays
	}
	if state.Settings.MinSessions <= 0 {
		state.Settings.MinSessions = defaultMinSessions
	}
	if state.Candidates == nil {
		state.Candidates = make(map[string]string)
	}
	if state.LastStatus == "running" {
		state.LastStatus = "failed"
		state.LastError = "interrupted: plugin generation stopped while Dream was running"
		state.LastFinishedAt = c.now().UTC()
		state.ActiveRequestID = ""
		state.ActiveSessionID = ""
	}
	c.mu.Lock()
	c.state = state
	c.mu.Unlock()
	return nil
}

func (c *controller) save(ctx context.Context) error {
	c.mu.Lock()
	encoded, err := json.Marshal(c.state)
	c.mu.Unlock()
	if err != nil {
		return err
	}
	return c.host.CallHost(ctx, pluginapi.HostServiceStorageSet, map[string]any{"scope": "workspace", "key": stateStorageKey, "value": string(encoded)}, &struct{}{})
}

func (c *controller) invokeCapability(ctx context.Context, call pluginapi.CapabilityCall) (json.RawMessage, error) {
	switch call.Capability {
	case capabilityTurnCompleted:
		var input turnCompletedInput
		if err := json.Unmarshal(call.Input, &input); err != nil {
			return nil, err
		}
		if input.Succeeded && strings.TrimSpace(input.ThreadID) != "" {
			when := input.CompletedAt
			if when.IsZero() {
				when = c.now().UTC()
			}
			c.mu.Lock()
			c.state.Candidates[input.ThreadID] = when.UTC().Format(time.RFC3339Nano)
			c.mu.Unlock()
			if err := c.save(ctx); err != nil {
				return nil, err
			}
			c.startIfDue(context.Background())
		}
		return json.RawMessage(`{}`), nil
	case capabilityLifecycle:
		var input pluginapi.TurnLifecycleInput
		if err := json.Unmarshal(call.Input, &input); err != nil {
			return nil, err
		}
		return json.RawMessage(`{}`), c.settle(ctx, input)
	case capabilityClient:
		var request struct {
			Method string          `json:"method"`
			Input  json.RawMessage `json:"input"`
		}
		if err := json.Unmarshal(call.Input, &request); err != nil {
			return nil, err
		}
		value, err := c.clientRequest(ctx, request.Method, request.Input)
		if err != nil {
			return nil, err
		}
		return json.Marshal(map[string]any{"result": value})
	default:
		return nil, fmt.Errorf("unknown dream capability %q", call.Capability)
	}
}

func (c *controller) clientRequest(ctx context.Context, method string, raw json.RawMessage) (any, error) {
	switch strings.TrimSpace(method) {
	case "dream.get":
		return c.snapshot(), nil
	case "dream.update":
		var next settings
		if err := json.Unmarshal(raw, &next); err != nil {
			return nil, err
		}
		if next.IntervalDays < 1 || next.IntervalDays > 365 || next.MinSessions < 1 || next.MinSessions > 100 {
			return nil, errors.New("Dream interval must be 1-365 days and minimum sessions must be 1-100")
		}
		next.ModelAlias = strings.TrimSpace(next.ModelAlias)
		c.mu.Lock()
		c.state.Settings = next
		c.mu.Unlock()
		if err := c.save(ctx); err != nil {
			return nil, err
		}
		c.startIfDue(context.Background())
		return c.snapshot(), nil
	case "dream.run":
		c.startIfDueForce(context.Background())
		return c.snapshot(), nil
	default:
		return nil, fmt.Errorf("unknown dream client method %q", method)
	}
}

func (c *controller) snapshot() persistedState {
	c.mu.Lock()
	defer c.mu.Unlock()
	copy := c.state
	copy.Candidates = make(map[string]string, len(c.state.Candidates))
	for id, at := range c.state.Candidates {
		copy.Candidates[id] = at
	}
	return copy
}

func (c *controller) startIfDue(ctx context.Context)      { c.startDream(ctx, false) }
func (c *controller) startIfDueForce(ctx context.Context) { c.startDream(ctx, true) }

func (c *controller) startDream(ctx context.Context, force bool) {
	now := c.now().UTC()
	c.mu.Lock()
	state := c.state
	if c.running || !state.Settings.Enabled || len(state.Candidates) == 0 || (!force && len(state.Candidates) < state.Settings.MinSessions) {
		c.mu.Unlock()
		return
	}
	if !force && !state.LastRunAt.IsZero() && now.Sub(state.LastRunAt) < time.Duration(state.Settings.IntervalDays)*24*time.Hour {
		c.mu.Unlock()
		return
	}
	if !force && state.LastStatus == "failed" && !state.LastFinishedAt.IsZero() && now.Sub(state.LastFinishedAt) < failureBackoff {
		c.mu.Unlock()
		return
	}
	// Claim the run before touching the registry so a concurrent starter
	// cannot double-launch while the memory read is in flight.
	c.running = true
	parentID := latestCandidate(state.Candidates)
	modelAlias := state.Settings.ModelAlias
	c.mu.Unlock()

	projectMemory, err := c.readProjectMemory(ctx)
	if err != nil {
		c.skip(context.Background(), err)
		return
	}

	c.mu.Lock()
	c.state.LastStatus = "running"
	c.state.LastStartedAt = now
	c.state.LastFinishedAt = time.Time{}
	c.state.LastError = ""
	c.mu.Unlock()
	go c.launch(ctx, parentID, modelAlias, projectMemory)
}

// readProjectMemory resolves the session-memory service fresh for every run,
// so a provider upgrade in a new plugin generation is picked up without any
// dream-side bookkeeping. A resolution or provider failure skips the run
// rather than consolidating against stale or missing memory.
func (c *controller) readProjectMemory(ctx context.Context) (string, error) {
	var result struct {
		Exists  bool   `json:"exists"`
		Content string `json:"content"`
	}
	err := pluginapi.CallService(ctx, c.host, sessionMemoryService, "read", map[string]any{"target": "project_memory"}, &result)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(result.Content), nil
}

// skip releases a claimed run without launching it; the next trigger may
// retry immediately since LastRunAt is untouched.
func (c *controller) skip(ctx context.Context, err error) {
	c.mu.Lock()
	c.running = false
	c.state.LastStatus = "skipped"
	c.state.LastFinishedAt = c.now().UTC()
	c.state.LastError = err.Error()
	c.mu.Unlock()
	_ = c.save(ctx)
}

func (c *controller) launch(ctx context.Context, parentID, modelAlias, projectMemory string) {
	id, err := randomID()
	if err != nil {
		c.fail(context.Background(), err)
		return
	}
	requestID := id + ":turn"
	var created pluginapi.SessionCreateResult
	err = c.host.CallHost(ctx, pluginapi.HostServiceSessionCreate, pluginapi.SessionCreateParams{
		RequestID: id + ":session", Name: "Dream", Visibility: "plugin", ParentSessionID: parentID,
		ContextSource: "fork", Workspace: "shared", ModelAlias: modelAlias,
	}, &created)
	if err != nil {
		c.fail(context.Background(), err)
		return
	}
	var sent pluginapi.SessionSendResult
	err = c.host.CallHost(ctx, pluginapi.HostServiceSessionSend, pluginapi.SessionSendParams{
		RequestID: requestID, SessionID: created.SessionID,
		Input: pluginapi.SessionInput{Prompt: dreamPrompt(projectMemory)}, Cause: "dream.consolidate",
	}, &sent)
	if err != nil {
		c.fail(context.Background(), err)
		return
	}
	c.mu.Lock()
	c.state.ActiveRequestID = requestID
	c.state.ActiveSessionID = created.SessionID
	c.mu.Unlock()
	if err := c.save(context.Background()); err != nil {
		c.fail(context.Background(), err)
	}
}

func (c *controller) settle(ctx context.Context, input pluginapi.TurnLifecycleInput) error {
	if input.State != "completed" && input.State != "failed" && input.State != "interrupted" && input.State != "discarded" {
		return nil
	}
	c.mu.Lock()
	if input.RequestID != c.state.ActiveRequestID {
		c.mu.Unlock()
		return nil
	}
	now := c.now().UTC()
	c.running = false
	c.state.LastFinishedAt = now
	c.state.ActiveRequestID = ""
	c.state.ActiveSessionID = ""
	if input.State == "completed" {
		c.state.LastStatus = "completed"
		c.state.LastRunAt = now
		c.state.LastError = ""
		c.state.Candidates = make(map[string]string)
	} else {
		c.state.LastStatus = "failed"
		c.state.LastError = strings.TrimSpace(input.Error)
		if c.state.LastError == "" {
			c.state.LastError = input.State
		}
	}
	c.mu.Unlock()
	return c.save(ctx)
}

func (c *controller) fail(ctx context.Context, err error) {
	c.mu.Lock()
	c.running = false
	c.state.LastStatus = "failed"
	c.state.LastFinishedAt = c.now().UTC()
	c.state.LastError = err.Error()
	c.state.ActiveRequestID = ""
	c.state.ActiveSessionID = ""
	c.mu.Unlock()
	_ = c.save(ctx)
}

func latestCandidate(candidates map[string]string) string {
	var latestID, latestAt string
	for id, at := range candidates {
		if at > latestAt || (at == latestAt && id > latestID) {
			latestID, latestAt = id, at
		}
	}
	return latestID
}

func dreamPrompt(projectMemory string) string {
	prompt := `Review the forked completed conversation and consolidate only durable workspace knowledge.`
	if projectMemory = strings.TrimSpace(projectMemory); projectMemory != "" {
		prompt += "\n\nCurrent project_memory, read when this run started; consolidate against it:\n\n" + projectMemory
	} else {
		prompt += "\n\nUse session_memory to read project_memory before editing it."
	}
	return prompt + ` Append or replace project_memory via the session_memory tool only for stable architecture decisions, conventions, tool quirks, or recurring workflow lessons that should survive future sessions. Never store secrets, raw transcripts, temporary progress, PR numbers, commit SHAs, or facts likely to go stale within a week. Do not modify source files. If nothing deserves durable memory, reply exactly: Nothing to dream.`
}

func randomID() (string, error) {
	var raw [10]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return "dream-" + hex.EncodeToString(raw[:]), nil
}

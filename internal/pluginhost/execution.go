package pluginhost

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Execution scope vocabulary. One execution is exactly one tool.execute or
// capability.invoke dispatch: open rides the invoke frame itself (the
// execution_id param), close rides its response, and cancel is the only
// mid-flight frame. Cancel is fire-and-forget on the host side — the core's
// terminal state never waits for a plugin acknowledgement, and any response
// the plugin writes for it is discarded.
const (
	// ExecutionCancelMethod is the Host → Plugin signal translating Go
	// context cancellation of the dispatching call. Plugins translate it
	// into whatever local cancellation primitive they own; the core builds
	// no task tree on top of it.
	ExecutionCancelMethod = "execution.cancel"

	// ExecutionUpdateService is the kernel service a plugin calls to report
	// progress for an execution it owns.
	ExecutionUpdateService = "execution.update"
)

// ExecutionUpdateParams is the pluginhost-side view of one execution.update
// call. Detail carries arbitrary plugin-owned progress payload.
type ExecutionUpdateParams struct {
	ExecutionID string          `json:"execution_id"`
	Message     string          `json:"message,omitempty"`
	Detail      json.RawMessage `json:"detail,omitempty"`
}

// ExecutionCancelParams is the wire body of an execution.cancel frame.
type ExecutionCancelParams struct {
	ExecutionID string `json:"execution_id"`
}

// ExecutionSnapshot is the host's recorded view of one live execution.
type ExecutionSnapshot struct {
	ID        string          `json:"id"`
	PluginID  string          `json:"plugin_id"`
	Message   string          `json:"message,omitempty"`
	Detail    json.RawMessage `json:"detail,omitempty"`
	UpdatedAt time.Time       `json:"updated_at,omitempty"`
}

// ExecutionTracker owns the live execution table for one Host. IDs are
// monotonic per host process and precise to a single dispatch, so a cancel
// can never hit a later execution in the same session. Records are removed
// when the invoke returns; a late update for a removed ID fails with
// execution_not_found and can never reopen it.
type ExecutionTracker struct {
	seq   atomic.Uint64
	mu    sync.Mutex
	live  map[string]*ExecutionSnapshot
	owner map[string]string
}

func NewExecutionTracker() *ExecutionTracker {
	return &ExecutionTracker{live: make(map[string]*ExecutionSnapshot), owner: make(map[string]string)}
}

// Begin registers one dispatch and returns its execution ID.
func (t *ExecutionTracker) Begin(pluginID string) string {
	id := fmt.Sprintf("exec-%d", t.seq.Add(1))
	t.mu.Lock()
	t.live[id] = &ExecutionSnapshot{ID: id, PluginID: pluginID}
	t.owner[id] = pluginID
	t.mu.Unlock()
	return id
}

// End closes the execution. It is idempotent and never blocks: the core's
// terminal state is decided by the invoke returning, not by plugin
// acknowledgement.
func (t *ExecutionTracker) End(id string) {
	t.mu.Lock()
	delete(t.live, id)
	delete(t.owner, id)
	t.mu.Unlock()
}

// RecordUpdate validates that caller owns the live execution and stores its
// latest progress.
func (t *ExecutionTracker) RecordUpdate(callerPluginID string, params ExecutionUpdateParams) *HostServiceError {
	id := strings.TrimSpace(params.ExecutionID)
	if id == "" {
		return &HostServiceError{Code: "invalid_request", Message: "execution.update requires execution_id"}
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	owner, ok := t.owner[id]
	if !ok {
		return &HostServiceError{Code: "execution_not_found", Message: fmt.Sprintf("execution %s is not live", id)}
	}
	if owner != callerPluginID {
		return &HostServiceError{Code: "service_not_authorized", Message: fmt.Sprintf("execution %s belongs to plugin %q", id, owner)}
	}
	snapshot := t.live[id]
	snapshot.Message = params.Message
	if len(params.Detail) != 0 {
		snapshot.Detail = append(json.RawMessage(nil), params.Detail...)
	}
	snapshot.UpdatedAt = time.Now().UTC()
	return nil
}

// Snapshot returns the live executions, ordered by ID, for diagnostics.
func (t *ExecutionTracker) Snapshot() []ExecutionSnapshot {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]ExecutionSnapshot, 0, len(t.live))
	for _, snapshot := range t.live {
		out = append(out, *snapshot)
	}
	return out
}

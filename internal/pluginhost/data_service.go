package pluginhost

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const (
	// KernelDataQueryService is the kernel's read-only host data service. It
	// exposes the first-party session data base — turn/step boundaries, model
	// calls with usage, and tool calls/results — as a stable query snapshot.
	KernelDataQueryService = "host.data.query"

	// DataQueryServiceVersion is the wire version of the query contract.
	DataQueryServiceVersion = "1.0.0"
)

// First-party data fact types. These are the stable discriminants the current
// host may emit. The set is open-ended at the query layer: unknown types are
// forward-compatible (they simply match no events) rather than rejected, so the
// host can add facts without breaking older consumers.
const (
	DataEventTypeTurn                     = "turn"
	DataEventTypeStep                     = "step"
	DataEventTypeModelCall                = "model_call"
	DataEventTypeToolCall                 = "tool_call"
	DataEventTypeToolResult               = "tool_result"
	DataEventTypeStreamingChunk           = "streaming_chunk"
	DataEventTypeContextRequests          = "context_requests"
	DataEventTypeProviderStates           = "provider_states"
	DataEventTypeCompactAttempts          = "compact_attempts"
	DataEventTypeBarrierToolBatchRejected = "barrier_tool_batch_rejected"
	DataEventTypeToolInventory            = "tool_inventory"
	DataEventTypeToolRecords              = "tool_records"
	DataEventTypeFinal                    = "final"
)

// DataEvent is the stable envelope for one first-party data fact. Payload
// shapes are versioned per Type; the envelope never carries configuration
// values, credentials, or another plugin's private data.
type DataEvent struct {
	Type      string          `json:"type"`
	ThreadID  string          `json:"thread_id,omitempty"`
	TurnID    string          `json:"turn_id,omitempty"`
	CreatedAt time.Time       `json:"created_at,omitempty"`
	Data      json.RawMessage `json:"data,omitempty"`
}

// DataQueryParams requests one read-only snapshot for a thread.
type DataQueryParams struct {
	ThreadID string `json:"thread_id"`
	// Types restricts the result to these event discriminants. Empty means all
	// types. Unknown values are valid and simply match no events.
	Types []string `json:"types,omitempty"`
	// TurnID restricts the result to one turn. Empty means all turns.
	TurnID string `json:"turn_id,omitempty"`
	// Limit bounds the returned event count. Zero or negative means no explicit
	// bound; callers should keep payloads within their own read budget.
	Limit int `json:"limit,omitempty"`
}

// DataQueryResult is the read-only query response.
type DataQueryResult struct {
	Version  string      `json:"version"`
	ThreadID string      `json:"thread_id"`
	Events   []DataEvent `json:"events"`
}

// KernelDataQueryDescriptor is the descriptor the kernel registers for the
// read-only host data service. It rides the same registry contract as the
// other kernel services.
func KernelDataQueryDescriptor() ServiceDescriptor {
	return ServiceDescriptor{
		Name: KernelDataQueryService, Version: DataQueryServiceVersion,
		Methods: []ServiceMethodDescriptor{{
			Name:         KernelServiceMethod,
			InputSchema:  "host.data.query.input.v1",
			OutputSchema: "host.data.query.output.v1",
		}},
	}
}

// ValidateDataQueryParams validates a plugin-supplied query before the runtime
// resolves any filesystem path from it.
func ValidateDataQueryParams(params DataQueryParams) error {
	if err := validateDataScopeID("thread_id", params.ThreadID); err != nil {
		return err
	}
	if params.Limit < 0 {
		return &HostServiceError{Code: "invalid_params", Message: "data query limit cannot be negative"}
	}
	if err := validateDataEventTypes(params.Types); err != nil {
		return err
	}
	if params.TurnID != "" {
		if err := validateDataScopeID("turn_id", params.TurnID); err != nil {
			return err
		}
	}
	return nil
}

// KnownDataEventType reports whether value is one of the stable discriminants
// documented by the current host. The query surface is intentionally forward
// compatible: an unknown type is valid and simply matches no events, so a newer
// host can emit new fact types without breaking older plugin queries.
func KnownDataEventType(value string) bool {
	_, ok := knownDataEventTypes[value]
	return ok
}

func validateDataEventTypes(types []string) error {
	seen := make(map[string]struct{}, len(types))
	for _, value := range types {
		value = strings.TrimSpace(value)
		if value == "" {
			return &HostServiceError{Code: "invalid_params", Message: "data query types cannot contain an empty value"}
		}
		if _, duplicate := seen[value]; duplicate {
			return &HostServiceError{Code: "invalid_params", Message: fmt.Sprintf("data query type %q is duplicated", value)}
		}
		seen[value] = struct{}{}
	}
	return nil
}

func validateDataScopeID(field, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return &HostServiceError{Code: "invalid_params", Message: fmt.Sprintf("data query %s is required", field)}
	}
	if len(id) > 256 {
		return &HostServiceError{Code: "invalid_params", Message: fmt.Sprintf("data query %s exceeds 256 bytes", field)}
	}
	for _, r := range id {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '-', r == '_', r == '.':
		default:
			return &HostServiceError{Code: "invalid_params", Message: fmt.Sprintf("data query %s contains invalid characters", field)}
		}
	}
	return nil
}

func validateDataThreadID(id string) error {
	return validateDataScopeID("thread_id", id)
}

var knownDataEventTypes = map[string]struct{}{
	DataEventTypeTurn:                     {},
	DataEventTypeStep:                     {},
	DataEventTypeModelCall:                {},
	DataEventTypeToolCall:                 {},
	DataEventTypeToolResult:               {},
	DataEventTypeStreamingChunk:           {},
	DataEventTypeContextRequests:          {},
	DataEventTypeProviderStates:           {},
	DataEventTypeCompactAttempts:          {},
	DataEventTypeBarrierToolBatchRejected: {},
	DataEventTypeToolInventory:            {},
	DataEventTypeToolRecords:              {},
	DataEventTypeFinal:                    {},
}

// FilterDataEvents applies the read-only query contract to a snapshot: type
// filtering first, then turn filtering, then limit. Order is preserved. The
// caller is responsible for validating params first.
func FilterDataEvents(events []DataEvent, params DataQueryParams) []DataEvent {
	if len(params.Types) == 0 && params.TurnID == "" && params.Limit <= 0 {
		return events
	}
	allowed := make(map[string]struct{}, len(params.Types))
	for _, value := range params.Types {
		allowed[value] = struct{}{}
	}
	out := make([]DataEvent, 0, len(events))
	for _, event := range events {
		if len(allowed) > 0 {
			if _, ok := allowed[event.Type]; !ok {
				continue
			}
		}
		if params.TurnID != "" && event.TurnID != params.TurnID {
			continue
		}
		out = append(out, event)
		if params.Limit > 0 && len(out) >= params.Limit {
			break
		}
	}
	return out
}

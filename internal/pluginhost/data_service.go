package pluginhost

import (
	"encoding/json"
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
	if err := validateDataThreadID(params.ThreadID); err != nil {
		return err
	}
	if params.Limit < 0 {
		return &HostServiceError{Code: "invalid_params", Message: "data query limit cannot be negative"}
	}
	return nil
}

func validateDataThreadID(id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return &HostServiceError{Code: "invalid_params", Message: "data query thread_id is required"}
	}
	if len(id) > 256 {
		return &HostServiceError{Code: "invalid_params", Message: "data query thread_id exceeds 256 bytes"}
	}
	for _, r := range id {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '-', r == '_', r == '.':
		default:
			return &HostServiceError{Code: "invalid_params", Message: "data query thread_id contains invalid characters"}
		}
	}
	return nil
}

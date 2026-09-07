package codemode

import (
	"encoding/json"
	"errors"
	"fmt"
)

const resourceLimitsCapability = "session-cell-execution-resource-limits"

// ToolDefinition names both the JavaScript property and the underlying tool.
// Callers supply their effective tool surface, including the current edit mode.
type ToolDefinition struct {
	Name         string          `json:"name"`
	ToolName     ToolName        `json:"tool_name"`
	Description  string          `json:"description"`
	Kind         string          `json:"kind"`
	InputSchema  json.RawMessage `json:"input_schema"`
	OutputSchema json.RawMessage `json:"output_schema"`
}

type ToolName struct {
	Name      string  `json:"name"`
	Namespace *string `json:"namespace"`
}

type ExecuteRequest struct {
	ToolCallID      string           `json:"tool_call_id"`
	EnabledTools    []ToolDefinition `json:"enabled_tools"`
	Source          string           `json:"source"`
	YieldTimeMS     *uint64          `json:"yield_time_ms"`
	MaxOutputTokens *int32           `json:"max_output_tokens"`
}

type CellLimits struct {
	MaxYieldTimeMS   *uint64 `json:"maxYieldTimeMs,omitempty"`
	MaxHeapSizeBytes *uint64 `json:"maxHeapSizeBytes,omitempty"`
}

// Invocation must be dispatched through the owning Wuu execution scope, never
// directly to a tool registry. Input retains structured JSON without flattening.
type Invocation struct {
	CellID            string          `json:"cell_id"`
	RuntimeToolCallID string          `json:"runtime_tool_call_id"`
	ToolName          ToolName        `json:"tool_name"`
	ToolKind          string          `json:"tool_kind"`
	Input             json.RawMessage `json:"input"`
}

type ContentItem struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	ImageURL string `json:"image_url,omitempty"`
	Detail   string `json:"detail,omitempty"`
	AudioURL string `json:"audio_url,omitempty"`
}

// Response contains only output since the preceding observation. A yielded cell
// remains in memory until waited on or terminated; it is not a resumable record
// after a host crash. Never retry an execute automatically on transport failure.
type Response struct {
	State          string
	CellID         string
	Content        []ContentItem
	ErrorText      *string
	HostDurationNS uint64
	Missing        bool
}

func decodeResponse(data json.RawMessage) (Response, error) {
	var variants map[string]json.RawMessage
	if err := json.Unmarshal(data, &variants); err != nil {
		return Response{}, err
	}
	if len(variants) != 1 {
		return Response{}, errors.New("invalid code-mode runtime response")
	}
	for state, body := range variants {
		if state != "Yielded" && state != "Terminated" && state != "Result" {
			return Response{}, fmt.Errorf("unknown code-mode response state %q", state)
		}
		var value struct {
			CellID   string        `json:"cell_id"`
			Content  []ContentItem `json:"content_items"`
			Error    *string       `json:"error_text"`
			Duration *uint64       `json:"code_mode_host_duration_ns"`
		}
		if err := json.Unmarshal(body, &value); err != nil {
			return Response{}, err
		}
		if value.CellID == "" || value.Duration == nil || value.Content == nil {
			return Response{}, errors.New("incomplete code-mode runtime response")
		}
		for _, item := range value.Content {
			if item.Type != "input_text" && item.Type != "input_image" && item.Type != "input_audio" {
				return Response{}, fmt.Errorf("unknown code-mode content type %q", item.Type)
			}
		}
		return Response{State: state, CellID: value.CellID, Content: value.Content,
			ErrorText: value.Error, HostDurationNS: *value.Duration}, nil
	}
	panic("unreachable")
}

type wireResult struct {
	Status  string          `json:"status"`
	Value   json.RawMessage `json:"value"`
	Message string          `json:"message"`
}

func (r wireResult) unwrap() (json.RawMessage, error) {
	switch r.Status {
	case "ok":
		if len(r.Value) == 0 || string(r.Value) == "null" {
			return nil, errors.New("missing code-mode result value")
		}
		return r.Value, nil
	case "error":
		return nil, fmt.Errorf("code-mode host: %s", r.Message)
	default:
		return nil, fmt.Errorf("invalid code-mode result status %q", r.Status)
	}
}

type hostMessage struct {
	Type            string          `json:"type"`
	ID              int64           `json:"id"`
	SessionID       string          `json:"sessionId"`
	CellID          string          `json:"cellId"`
	SelectedVersion int             `json:"selectedVersion"`
	Capabilities    []string        `json:"capabilities"`
	Reason          json.RawMessage `json:"reason"`
	Result          wireResult      `json:"result"`
	Request         delegateRequest `json:"request"`
}

type delegateRequest struct {
	Type       string     `json:"type"`
	Invocation Invocation `json:"invocation"`
	CallID     string     `json:"callId"`
	CellID     string     `json:"cellId"`
	Text       string     `json:"text"`
}

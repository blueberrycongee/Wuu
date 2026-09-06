package pluginhost

import (
	"fmt"
	"strings"
)

const (
	SessionHistoryReadLimit     = 50
	SessionHistorySearchLimit   = 25
	SessionHistoryDefaultLimit  = 20
	SessionHistorySearchDefault = 10
)

type SessionHistoryReadParams struct {
	SessionID   string `json:"session_id"`
	StartSeq    int    `json:"start_seq,omitempty"`
	EndSeq      int    `json:"end_seq,omitempty"`
	SnapshotSeq int    `json:"snapshot_seq,omitempty"`
	Limit       int    `json:"limit,omitempty"`
	Cursor      string `json:"cursor,omitempty"`
}

type SessionHistorySearchParams struct {
	SessionID   string `json:"session_id"`
	Query       string `json:"query"`
	StartSeq    int    `json:"start_seq,omitempty"`
	EndSeq      int    `json:"end_seq,omitempty"`
	SnapshotSeq int    `json:"snapshot_seq,omitempty"`
	BeforeSeq   int    `json:"before_seq,omitempty"`
	Limit       int    `json:"limit,omitempty"`
	Cursor      string `json:"cursor,omitempty"`
}

type SessionHistoryRecord struct {
	Seq              int    `json:"seq"`
	Role             string `json:"role"`
	Name             string `json:"name,omitempty"`
	Content          string `json:"content,omitempty"`
	DisplayContent   string `json:"display_content,omitempty"`
	ReasoningContent string `json:"reasoning_content,omitempty"`
	ToolCalls        string `json:"tool_calls,omitempty"`
	ToolCallID       string `json:"tool_call_id,omitempty"`
	ToolResult       string `json:"tool_result,omitempty"`
	FinishReason     string `json:"finish_reason,omitempty"`
	Truncated        bool   `json:"truncated,omitempty"`
}

type SessionHistoryMatch struct {
	Seq     int    `json:"seq"`
	Role    string `json:"role"`
	Name    string `json:"name,omitempty"`
	Excerpt string `json:"excerpt,omitempty"`
}

type SessionHistoryPage struct {
	SessionID       string                 `json:"session_id"`
	HeadSeq         int                    `json:"head_seq"`
	SnapshotSeq     int                    `json:"snapshot_seq"`
	Records         []SessionHistoryRecord `json:"records,omitempty"`
	Matches         []SessionHistoryMatch  `json:"matches,omitempty"`
	HasMore         bool                   `json:"has_more,omitempty"`
	BudgetExhausted bool                   `json:"budget_exhausted,omitempty"`
	NextCursor      string                 `json:"next_cursor,omitempty"`
}

func ValidateSessionHistoryReadParams(params SessionHistoryReadParams) error {
	if err := validateDataScopeID("session_id", params.SessionID); err != nil {
		return err
	}
	if params.StartSeq < 0 || params.EndSeq < 0 || params.SnapshotSeq < 0 {
		return &HostServiceError{Code: "invalid_params", Message: "history snapshot bounds must be non-negative"}
	}
	if params.StartSeq < 1 && strings.TrimSpace(params.Cursor) == "" {
		return &HostServiceError{Code: "invalid_params", Message: "history read requires start_seq or cursor"}
	}
	if params.Limit < 0 || params.Limit > SessionHistoryReadLimit {
		return &HostServiceError{Code: "invalid_params", Message: fmt.Sprintf("history read limit must be between 0 and %d", SessionHistoryReadLimit)}
	}
	return nil
}

func ValidateSessionHistorySearchParams(params SessionHistorySearchParams) error {
	if err := validateDataScopeID("session_id", params.SessionID); err != nil {
		return err
	}
	if strings.TrimSpace(params.Query) == "" {
		return &HostServiceError{Code: "invalid_params", Message: "history search query is required"}
	}
	if params.StartSeq < 0 || params.EndSeq < 0 || params.SnapshotSeq < 0 || params.BeforeSeq < 0 {
		return &HostServiceError{Code: "invalid_params", Message: "history snapshot bounds must be non-negative"}
	}
	if params.Limit < 0 || params.Limit > SessionHistorySearchLimit {
		return &HostServiceError{Code: "invalid_params", Message: fmt.Sprintf("history search limit must be between 0 and %d", SessionHistorySearchLimit)}
	}
	return nil
}

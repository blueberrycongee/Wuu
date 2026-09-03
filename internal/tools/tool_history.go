package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/blueberrycongee/wuu/internal/providers"
	"github.com/blueberrycongee/wuu/internal/session"
	"github.com/blueberrycongee/wuu/internal/statepath"
)

const (
	historyReadToolName       = "history_read"
	historySearchToolName     = "history_search"
	defaultHistoryReadLimit   = 20
	maximumHistoryReadLimit   = 50
	defaultHistoryReadChars   = 12_000
	maximumHistoryReadChars   = 40_000
	defaultHistorySearchLimit = 10
	maximumHistorySearchLimit = 25
	historySearchExcerptChars = 800
)

type HistoryReadTool struct{ env *Env }

func NewHistoryReadTool(env *Env) *HistoryReadTool { return &HistoryReadTool{env: env} }
func (*HistoryReadTool) Name() string              { return historyReadToolName }
func (*HistoryReadTool) IsReadOnly() bool          { return true }
func (*HistoryReadTool) IsConcurrencySafe() bool   { return true }

func (*HistoryReadTool) Definition() providers.ToolDefinition {
	return providers.ToolDefinition{
		Name: historyReadToolName,
		Description: "Read a bounded range from this session's append-only original conversation and tool history by stable Seq address. " +
			"Use this after a context-window transition when a continuation note names a Seq or an omitted range. Results never read another session.",
		InputSchema: map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"required":             []string{"start_seq"},
			"properties": map[string]any{
				"start_seq": map[string]any{"type": "integer", "minimum": 1, "description": "Inclusive stable Seq at which to start."},
				"limit":     map[string]any{"type": "integer", "minimum": 1, "maximum": maximumHistoryReadLimit, "description": "Maximum records to return (default 20)."},
				"max_chars": map[string]any{"type": "integer", "minimum": 1, "maximum": maximumHistoryReadChars, "description": "Maximum model-visible payload characters (default 12000)."},
			},
		},
	}
}

func (t *HistoryReadTool) Execute(ctx context.Context, argsJSON string) (string, error) {
	var args struct {
		StartSeq int `json:"start_seq"`
		Limit    int `json:"limit"`
		MaxChars int `json:"max_chars"`
	}
	if err := decodeArgs(argsJSON, &args); err != nil {
		return "", err
	}
	if args.StartSeq < 1 {
		return "", errors.New("history_read requires a positive start_seq")
	}
	limit, err := boundedPositive(args.Limit, defaultHistoryReadLimit, maximumHistoryReadLimit, "history_read limit")
	if err != nil {
		return "", err
	}
	maxChars, err := boundedPositive(args.MaxChars, defaultHistoryReadChars, maximumHistoryReadChars, "history_read max_chars")
	if err != nil {
		return "", err
	}
	sessDir, sessionID, err := currentHistorySession(t.env, historyReadToolName)
	if err != nil {
		return "", err
	}
	page, err := session.ReadHistoryPage(ctx, sessDir, sessionID, args.StartSeq, limit)
	if err != nil {
		return "", fmt.Errorf("history_read: %w", err)
	}
	views, payloadTruncated := boundedHistoryViews(page.Records, maxChars)
	nextSeq := 0
	if len(page.Records) > 0 && (page.HasMore || payloadTruncated) {
		nextSeq = page.Records[len(views)-1].Seq + 1
	}
	result := map[string]any{
		"action":            historyReadToolName,
		"session_id":        sessionID,
		"head_seq":          page.HeadSeq,
		"records":           views,
		"payload_truncated": payloadTruncated,
	}
	if nextSeq > 0 {
		result["next"] = map[string]int{"start_seq": nextSeq}
	}
	return mustJSON(result)
}

type HistorySearchTool struct{ env *Env }

func NewHistorySearchTool(env *Env) *HistorySearchTool { return &HistorySearchTool{env: env} }
func (*HistorySearchTool) Name() string                { return historySearchToolName }
func (*HistorySearchTool) IsReadOnly() bool            { return true }
func (*HistorySearchTool) IsConcurrencySafe() bool     { return true }

func (*HistorySearchTool) Definition() providers.ToolDefinition {
	return providers.ToolDefinition{
		Name:        historySearchToolName,
		Description: "Search this session's append-only original conversation and tool history. Returns bounded newest-first matches with stable Seq addresses; use history_read for surrounding exact records.",
		InputSchema: map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"required":             []string{"query"},
			"properties": map[string]any{
				"query":      map[string]any{"type": "string", "description": "Case-insensitive literal text to find in messages, reasoning, tool calls, or tool results."},
				"before_seq": map[string]any{"type": "integer", "minimum": 0, "description": "Exclusive older-page cursor. Use next.before_seq from the previous result."},
				"limit":      map[string]any{"type": "integer", "minimum": 1, "maximum": maximumHistorySearchLimit, "description": "Maximum matches to return (default 10)."},
			},
		},
	}
}

func (t *HistorySearchTool) Execute(ctx context.Context, argsJSON string) (string, error) {
	var args struct {
		Query     string `json:"query"`
		BeforeSeq int    `json:"before_seq"`
		Limit     int    `json:"limit"`
	}
	if err := decodeArgs(argsJSON, &args); err != nil {
		return "", err
	}
	args.Query = strings.TrimSpace(args.Query)
	if args.Query == "" {
		return "", errors.New("history_search requires query")
	}
	if args.BeforeSeq < 0 {
		return "", errors.New("history_search before_seq must be non-negative")
	}
	limit, err := boundedPositive(args.Limit, defaultHistorySearchLimit, maximumHistorySearchLimit, "history_search limit")
	if err != nil {
		return "", err
	}
	sessDir, sessionID, err := currentHistorySession(t.env, historySearchToolName)
	if err != nil {
		return "", err
	}
	page, err := session.SearchHistoryPage(ctx, sessDir, sessionID, args.Query, args.BeforeSeq, limit)
	if err != nil {
		return "", fmt.Errorf("history_search: %w", err)
	}
	matches := make([]map[string]any, 0, len(page.Records))
	for _, record := range page.Records {
		matches = append(matches, map[string]any{
			"seq":     record.Seq,
			"role":    record.Role,
			"name":    record.Name,
			"excerpt": historySearchExcerpt(historyRecordSearchText(record), args.Query, historySearchExcerptChars),
		})
	}
	result := map[string]any{
		"action":     historySearchToolName,
		"session_id": sessionID,
		"head_seq":   page.HeadSeq,
		"query":      args.Query,
		"matches":    matches,
	}
	if page.HasMore && len(page.Records) > 0 {
		result["next"] = map[string]int{"before_seq": page.Records[len(page.Records)-1].Seq}
	}
	return mustJSON(result)
}

type historyRecordToolView struct {
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
	PayloadTruncated bool   `json:"payload_truncated,omitempty"`
}

func boundedHistoryViews(records []session.HistoryRecord, maxChars int) ([]historyRecordToolView, bool) {
	views := make([]historyRecordToolView, 0, len(records))
	remaining := maxChars
	for _, record := range records {
		if remaining <= 0 {
			return views, true
		}
		view := historyRecordToolView{
			Seq: record.Seq, Role: record.Role, Name: record.Name, ToolCallID: record.ToolCallID,
			FinishReason: record.FinishReason, Truncated: record.Truncated,
		}
		fields := []*string{&view.Content, &view.DisplayContent, &view.ReasoningContent}
		sources := []string{record.Content, record.DisplayContent, record.ReasoningContent}
		for index, source := range sources {
			part, clipped := takeHistoryChars(source, remaining)
			*fields[index] = part
			remaining -= len([]rune(part))
			view.PayloadTruncated = view.PayloadTruncated || clipped
		}
		for _, raw := range []struct {
			source json.RawMessage
			target *string
		}{{record.ToolCalls, &view.ToolCalls}, {record.ToolResult, &view.ToolResult}} {
			part, clipped := takeHistoryChars(string(raw.source), remaining)
			*raw.target = part
			remaining -= len([]rune(part))
			view.PayloadTruncated = view.PayloadTruncated || clipped
		}
		views = append(views, view)
		if view.PayloadTruncated {
			return views, true
		}
	}
	return views, false
}

func currentHistorySession(env *Env, toolName string) (string, string, error) {
	if env == nil || strings.TrimSpace(env.SessionID) == "" {
		return "", "", fmt.Errorf("%s: current session is unavailable", toolName)
	}
	sessDir := strings.TrimSpace(env.SessionsDir)
	if sessDir == "" {
		home, err := statepath.Home("")
		if err != nil {
			return "", "", fmt.Errorf("%s: resolve wuu home: %w", toolName, err)
		}
		sessDir = statepath.SessionsDir(home)
	}
	if sessDir == "" {
		return "", "", fmt.Errorf("%s: sessions dir is empty", toolName)
	}
	return sessDir, strings.TrimSpace(env.SessionID), nil
}

func boundedPositive(value, defaultValue, maximum int, name string) (int, error) {
	if value == 0 {
		return defaultValue, nil
	}
	if value < 1 || value > maximum {
		return 0, fmt.Errorf("%s must be between 1 and %d", name, maximum)
	}
	return value, nil
}

func takeHistoryChars(value string, limit int) (string, bool) {
	runes := []rune(value)
	if len(runes) <= limit {
		return value, false
	}
	if limit <= 0 {
		return "", value != ""
	}
	return string(runes[:limit]), true
}

func historyRecordSearchText(record session.HistoryRecord) string {
	return strings.Join([]string{
		record.Content, record.DisplayContent, record.ReasoningContent, string(record.ToolCalls), string(record.ToolResult), record.Name,
	}, "\n")
}

func historySearchExcerpt(text, query string, limit int) string {
	textRunes := []rune(strings.TrimSpace(text))
	if len(textRunes) <= limit {
		return string(textRunes)
	}
	lowerText := []rune(strings.ToLower(string(textRunes)))
	lowerQuery := []rune(strings.ToLower(query))
	match := 0
	for index := 0; index+len(lowerQuery) <= len(lowerText); index++ {
		if string(lowerText[index:index+len(lowerQuery)]) == string(lowerQuery) {
			match = index
			break
		}
	}
	start := match - limit/3
	if start < 0 {
		start = 0
	}
	end := start + limit
	if end > len(textRunes) {
		end = len(textRunes)
		start = max(0, end-limit)
	}
	prefix := ""
	suffix := ""
	if start > 0 {
		prefix = "…"
	}
	if end < len(textRunes) {
		suffix = "…"
	}
	return prefix + string(textRunes[start:end]) + suffix
}

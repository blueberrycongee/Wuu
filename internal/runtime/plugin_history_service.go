package runtime

import (
	"context"
	"errors"
	"strings"

	"github.com/blueberrycongee/wuu/internal/pluginhost"
	"github.com/blueberrycongee/wuu/internal/session"
	"github.com/blueberrycongee/wuu/internal/statepath"
)

func readPluginSessionHistory(ctx context.Context, wuuHome, pluginID string, params pluginhost.SessionHistoryReadParams) (pluginhost.SessionHistoryPage, error) {
	if err := pluginhost.ValidateSessionHistoryReadParams(params); err != nil {
		return pluginhost.SessionHistoryPage{}, err
	}
	sessDir, meta, err := loadVisibleHistorySession(wuuHome, pluginID, params.SessionID)
	if err != nil {
		return pluginhost.SessionHistoryPage{}, err
	}
	limit := params.Limit
	if limit == 0 {
		limit = pluginhost.SessionHistoryDefaultLimit
	}
	var cursor *session.HistoryCursor
	if strings.TrimSpace(params.Cursor) != "" {
		decoded, decodeErr := session.DecodeHistoryCursor(params.Cursor)
		if decodeErr != nil {
			return pluginhost.SessionHistoryPage{}, serviceError("invalid_params", decodeErr.Error())
		}
		cursor = &decoded
	}
	page, err := session.ReadHistoryQuery(ctx, sessDir, session.HistoryReadQuery{
		SessionID: meta.ID, StartSeq: params.StartSeq, EndSeq: params.EndSeq, SnapshotSeq: params.SnapshotSeq, Limit: limit, Cursor: cursor,
	})
	if err != nil {
		return pluginhost.SessionHistoryPage{}, historyServiceError(err)
	}
	result := pluginhost.SessionHistoryPage{
		SessionID: meta.ID, HeadSeq: page.HeadSeq, SnapshotSeq: page.SnapshotSeq, HasMore: page.HasMore, BudgetExhausted: page.BudgetExhausted,
		Records: make([]pluginhost.SessionHistoryRecord, 0, len(page.Records)),
	}
	for _, record := range page.Records {
		result.Records = append(result.Records, pluginhost.SessionHistoryRecord{
			Seq: record.Seq, Role: record.Role, Name: record.Name, Content: record.Content, DisplayContent: record.DisplayContent,
			ReasoningContent: record.ReasoningContent, ToolCalls: string(record.ToolCalls), ToolCallID: record.ToolCallID,
			ToolResult: string(record.ToolResult), FinishReason: record.FinishReason, Truncated: record.Truncated,
		})
	}
	if page.Next != nil {
		result.NextCursor = session.EncodeHistoryCursor(*page.Next)
	}
	return result, nil
}

func searchPluginSessionHistory(ctx context.Context, wuuHome, pluginID string, params pluginhost.SessionHistorySearchParams) (pluginhost.SessionHistoryPage, error) {
	if err := pluginhost.ValidateSessionHistorySearchParams(params); err != nil {
		return pluginhost.SessionHistoryPage{}, err
	}
	sessDir, meta, err := loadVisibleHistorySession(wuuHome, pluginID, params.SessionID)
	if err != nil {
		return pluginhost.SessionHistoryPage{}, err
	}
	limit := params.Limit
	if limit == 0 {
		limit = pluginhost.SessionHistorySearchDefault
	}
	var cursor *session.HistoryCursor
	if strings.TrimSpace(params.Cursor) != "" {
		decoded, decodeErr := session.DecodeHistoryCursor(params.Cursor)
		if decodeErr != nil {
			return pluginhost.SessionHistoryPage{}, serviceError("invalid_params", decodeErr.Error())
		}
		cursor = &decoded
	}
	page, err := session.SearchHistoryQuery(ctx, sessDir, session.HistorySearchQuery{
		SessionID: meta.ID, Query: params.Query, StartSeq: params.StartSeq, EndSeq: params.EndSeq,
		SnapshotSeq: params.SnapshotSeq, BeforeSeq: params.BeforeSeq, Limit: limit, Cursor: cursor,
	})
	if err != nil {
		return pluginhost.SessionHistoryPage{}, historyServiceError(err)
	}
	result := pluginhost.SessionHistoryPage{
		SessionID: meta.ID, HeadSeq: page.HeadSeq, SnapshotSeq: page.SnapshotSeq, HasMore: page.HasMore, BudgetExhausted: page.BudgetExhausted,
		Matches: make([]pluginhost.SessionHistoryMatch, 0, len(page.Records)),
	}
	for _, record := range page.Records {
		result.Matches = append(result.Matches, pluginhost.SessionHistoryMatch{
			Seq: record.Seq, Role: record.Role, Name: record.Name, Excerpt: record.Content,
		})
	}
	if page.Next != nil {
		result.NextCursor = session.EncodeHistoryCursor(*page.Next)
	}
	return result, nil
}

func loadVisibleHistorySession(wuuHome, pluginID, sessionID string) (string, session.Session, error) {
	home := strings.TrimSpace(wuuHome)
	if home == "" {
		return "", session.Session{}, serviceError("service_unavailable", "session history is unavailable")
	}
	sessDir := statepath.SessionsDir(home)
	meta, ok, err := session.Find(sessDir, sessionID)
	if err != nil {
		return "", session.Session{}, historyServiceError(err)
	}
	if !ok {
		return "", session.Session{}, serviceError("not_found", "session history is unavailable")
	}
	owner := "plugin:" + strings.TrimSpace(pluginID)
	if meta.Visibility == pluginhost.SessionVisibilityPlugin && meta.Owner != owner {
		return "", session.Session{}, serviceError("not_found", "session history is unavailable")
	}
	return sessDir, meta, nil
}

func historyServiceError(err error) error {
	switch {
	case errors.Is(err, session.ErrSessionNotFound), errors.Is(err, session.ErrHistoryUnavailable):
		return serviceError("not_found", "session history is unavailable")
	case errors.Is(err, session.ErrHistorySnapshotGone):
		return serviceError("invalid_params", err.Error())
	default:
		return err
	}
}

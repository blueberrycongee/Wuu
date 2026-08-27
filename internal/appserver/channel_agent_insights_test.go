package appserver

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/blueberrycongee/wuu/internal/channels"
	"github.com/blueberrycongee/wuu/internal/session"
	"github.com/blueberrycongee/wuu/internal/toolresult"
)

func TestCollectChannelAgentInsightAggregatesRecentAttributableWork(t *testing.T) {
	t.Parallel()
	sessDir := t.TempDir()
	now := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	agent := channels.NamedAgent{ID: "agent-1", Name: "Galileo", CreatedAt: now.Add(-24 * time.Hour)}
	sessionID := namedAgentSessionID(agent)
	if _, err := session.CreateWithMetadata(sessDir, sessionID, "/work/wuu"); err != nil {
		t.Fatalf("create session: %v", err)
	}
	detail := map[string]any{"files": []any{
		map[string]any{"path": "desktop/src/Card.tsx", "action": "update", "diff": map[string]any{"hunks": []any{
			map[string]any{"lines": []any{
				map[string]any{"op": "equal"},
				map[string]any{"op": "insert"},
				map[string]any{"op": "insert"},
				map[string]any{"op": "delete"},
			}},
		}}},
		map[string]any{"path": "internal/card.go", "action": "add", "diff": map[string]any{"new_file": true, "lines": 5}},
	}}
	structured, err := json.Marshal(detail)
	if err != nil {
		t.Fatal(err)
	}
	envelope, err := json.Marshal(toolresult.Result{StructuredContent: structured})
	if err != nil {
		t.Fatal(err)
	}
	records := []session.HistoryRecord{
		{Role: "tool", Name: "apply_patch", At: now.Add(-time.Hour), ToolResult: envelope},
		{Role: "meta", Content: "token_usage", At: now.Add(-time.Hour), InputTokens: 100, OutputTokens: 25, CacheReadTokens: 40},
		{Role: "tool", Name: "apply_patch", At: now.AddDate(0, 0, -8), ToolResult: envelope},
	}
	if err := session.RewriteHistoryRecords(sessDir, sessionID, records); err != nil {
		t.Fatalf("rewrite history: %v", err)
	}
	workSessionID := namedAgentWorkSessionID(agentRuntimeFromNamed(agent), "work-2")
	if _, err := session.CreateWithMetadata(sessDir, workSessionID, "/work/wuu"); err != nil {
		t.Fatalf("create work session: %v", err)
	}
	if _, err := session.SetSource(sessDir, workSessionID, namedAgentSessionSource+agent.ID); err != nil {
		t.Fatalf("set work session source: %v", err)
	}
	workDetail, err := json.Marshal(map[string]any{"path": "scripts/check.py", "diff": map[string]any{"new_file": true, "lines": 3}})
	if err != nil {
		t.Fatal(err)
	}
	workEnvelope, err := json.Marshal(toolresult.Result{StructuredContent: workDetail})
	if err != nil {
		t.Fatal(err)
	}
	if err := session.RewriteHistoryRecords(sessDir, workSessionID, []session.HistoryRecord{
		{Role: "tool", Name: "apply_patch", At: now.Add(-30 * time.Minute), ToolResult: workEnvelope},
		{Role: "meta", Content: "token_usage", At: now.Add(-30 * time.Minute), InputTokens: 10, OutputTokens: 5},
	}); err != nil {
		t.Fatalf("rewrite work history: %v", err)
	}

	got := collectChannelAgentInsight(sessDir, agent, now)
	if got.AgentID != agent.ID || got.WindowDays != 7 || got.FilesChanged != 3 {
		t.Fatalf("unexpected identity/counts: %+v", got)
	}
	if got.Additions != 10 || got.Deletions != 1 {
		t.Fatalf("changes = +%d/-%d, want +10/-1", got.Additions, got.Deletions)
	}
	if got.InputTokens != 150 || got.OutputTokens != 30 {
		t.Fatalf("tokens = %d/%d, want 150/30", got.InputTokens, got.OutputTokens)
	}
	if got.Workspace != "wuu" {
		t.Fatalf("workspace = %q, want wuu", got.Workspace)
	}
	if len(got.Languages) != 3 || got.Languages[0].Name != "Go" || got.Languages[0].Lines != 5 || got.Languages[1].Name != "Python" || got.Languages[2].Name != "TypeScript" {
		t.Fatalf("languages = %+v", got.Languages)
	}
}

func TestAttributableEditsReadsLegacyTextProjection(t *testing.T) {
	t.Parallel()
	detail := `{"path":"src/index.py","diff":{"new_file":true,"lines":3}}`
	raw, err := json.Marshal(toolresult.Result{Content: []toolresult.ContentPart{{Type: toolresult.ContentTypeText, Text: detail}}})
	if err != nil {
		t.Fatal(err)
	}
	edits := attributableEdits(raw)
	if len(edits) != 1 || edits[0].Path != "src/index.py" {
		t.Fatalf("edits = %+v", edits)
	}
	if additions, deletions := summarizeInsightDiff(edits[0].Diff); additions != 3 || deletions != 0 {
		t.Fatalf("summary = +%d/-%d, want +3/-0", additions, deletions)
	}
}

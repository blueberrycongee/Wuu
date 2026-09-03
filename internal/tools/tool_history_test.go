package tools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/blueberrycongee/wuu/internal/modelprofile"
	"github.com/blueberrycongee/wuu/internal/session"
)

func TestHistoryToolsAreBoundToCurrentSession(t *testing.T) {
	dir := t.TempDir()
	for _, id := range []string{"current", "other"} {
		if _, err := session.CreateWithMetadata(dir, id, t.TempDir()); err != nil {
			t.Fatalf("create %s: %v", id, err)
		}
	}
	if err := session.AppendHistoryRecords(dir, "current", []session.HistoryRecord{
		{Role: "user", Content: "find the regression"},
		{Role: "assistant", Content: "fixed current session"},
	}); err != nil {
		t.Fatalf("append current: %v", err)
	}
	if err := session.AppendHistoryRecord(dir, "other", session.HistoryRecord{Role: "user", Content: "private other session"}); err != nil {
		t.Fatalf("append other: %v", err)
	}
	env := &Env{SessionsDir: dir, SessionID: "current"}

	read, err := NewHistoryReadTool(env).Execute(context.Background(), `{"start_seq":1}`)
	if err != nil {
		t.Fatalf("history_read: %v", err)
	}
	var readResult struct {
		SessionID string                  `json:"session_id"`
		Records   []historyRecordToolView `json:"records"`
	}
	if err := json.Unmarshal([]byte(read), &readResult); err != nil {
		t.Fatalf("decode read result: %v", err)
	}
	if readResult.SessionID != "current" || len(readResult.Records) != 2 {
		t.Fatalf("read result = %+v", readResult)
	}

	search, err := NewHistorySearchTool(env).Execute(context.Background(), `{"query":"fixed current"}`)
	if err != nil {
		t.Fatalf("history_search: %v", err)
	}
	var searchResult struct {
		Matches []struct {
			Seq int `json:"seq"`
		} `json:"matches"`
	}
	if err := json.Unmarshal([]byte(search), &searchResult); err != nil {
		t.Fatalf("decode search result: %v", err)
	}
	if len(searchResult.Matches) != 1 || searchResult.Matches[0].Seq != 2 {
		t.Fatalf("search result = %+v", searchResult)
	}

	search, err = NewHistorySearchTool(env).Execute(context.Background(), `{"query":"private other session"}`)
	if err != nil {
		t.Fatalf("cross-session search: %v", err)
	}
	if err := json.Unmarshal([]byte(search), &searchResult); err != nil {
		t.Fatalf("decode cross-session search: %v", err)
	}
	if len(searchResult.Matches) != 0 {
		t.Fatalf("history search escaped current session: %+v", searchResult)
	}
}

func TestHistoryReadReportsPayloadTruncationAndContinuation(t *testing.T) {
	dir := t.TempDir()
	if _, err := session.CreateWithMetadata(dir, "current", t.TempDir()); err != nil {
		t.Fatalf("create session: %v", err)
	}
	if err := session.AppendHistoryRecords(dir, "current", []session.HistoryRecord{
		{Role: "user", Content: "1234567890"},
		{Role: "assistant", Content: "later"},
	}); err != nil {
		t.Fatalf("append history: %v", err)
	}
	result, err := NewHistoryReadTool(&Env{SessionsDir: dir, SessionID: "current"}).Execute(context.Background(), `{"start_seq":1,"max_chars":5}`)
	if err != nil {
		t.Fatalf("history_read: %v", err)
	}
	var decoded struct {
		PayloadTruncated bool                    `json:"payload_truncated"`
		Records          []historyRecordToolView `json:"records"`
		Next             map[string]int          `json:"next"`
	}
	if err := json.Unmarshal([]byte(result), &decoded); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if !decoded.PayloadTruncated || len(decoded.Records) != 1 || !decoded.Records[0].PayloadTruncated || decoded.Next["start_seq"] != 2 {
		t.Fatalf("result = %+v", decoded)
	}
}

func TestContextWindowToolsRequireRuntimeEnablement(t *testing.T) {
	kit, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("new toolkit: %v", err)
	}
	kit.SetActiveProfile(modelprofile.Resolve("openai", "gpt-5-codex"), true)
	assertDefined := func(want bool) {
		t.Helper()
		got := map[string]bool{}
		for _, definition := range kit.Definitions() {
			got[definition.Name] = true
		}
		for _, name := range contextWindowToolNames {
			if got[name] != want {
				t.Fatalf("tool %q visible = %v, want %v", name, got[name], want)
			}
		}
	}
	assertDefined(false)
	kit.SetContextWindowToolsEnabled(true)
	assertDefined(true)
	kit.SetContextWindowToolsEnabled(false)
	assertDefined(false)
}

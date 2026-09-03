package session

import (
	"context"
	"testing"
)

func TestHistoryRecoveryReadsAndSearchesPhysicalTranscript(t *testing.T) {
	dir := t.TempDir()
	if _, err := CreateWithMetadata(dir, "thread-history-recovery", t.TempDir()); err != nil {
		t.Fatalf("create session: %v", err)
	}
	records := []HistoryRecord{
		{Role: "user", Content: "first request"},
		{Role: "assistant", Content: "checking source", ToolCalls: []byte(`[{"name":"grep","arguments":"{\"pattern\":\"needle\"}"}]`)},
		{Role: "tool", Content: "", ToolResult: []byte(`{"content":[{"type":"text","text":"exact tool fact"}]}`)},
		{Role: "meta", Content: "token_usage"},
		{Role: "assistant", Content: "finished"},
	}
	start, end, err := AppendHistoryRecordsReturningRange(dir, "thread-history-recovery", records)
	if err != nil {
		t.Fatalf("append records: %v", err)
	}
	if start != 1 || end != 5 {
		t.Fatalf("range = %d..%d, want 1..5", start, end)
	}

	page, err := ReadHistoryPage(context.Background(), dir, "thread-history-recovery", 2, 2)
	if err != nil {
		t.Fatalf("read history: %v", err)
	}
	if page.HeadSeq != 5 || !page.HasMore || len(page.Records) != 2 {
		t.Fatalf("page = %+v", page)
	}
	if page.Records[0].Seq != 2 || page.Records[1].Seq != 3 {
		t.Fatalf("read seqs = %d, %d", page.Records[0].Seq, page.Records[1].Seq)
	}

	search, err := SearchHistoryPage(context.Background(), dir, "thread-history-recovery", "exact tool fact", 0, 10)
	if err != nil {
		t.Fatalf("search tool result: %v", err)
	}
	if len(search.Records) != 1 || search.Records[0].Seq != 3 {
		t.Fatalf("search = %+v", search)
	}
	search, err = SearchHistoryPage(context.Background(), dir, "thread-history-recovery", "needle", 0, 10)
	if err != nil {
		t.Fatalf("search tool arguments: %v", err)
	}
	if len(search.Records) != 1 || search.Records[0].Seq != 2 {
		t.Fatalf("tool argument search = %+v", search)
	}
}

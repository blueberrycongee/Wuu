package appserver

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/blueberrycongee/wuu/internal/providers"
	"github.com/blueberrycongee/wuu/internal/runtime"
	"github.com/blueberrycongee/wuu/internal/session"
)

func TestThreadSearchLazyHistoryPreservesResults(t *testing.T) {
	rt := newTestRuntime(t, &fakeClient{})
	if _, err := session.CreateWithMetadata(rt.SessionDir, "disk", rt.RootDir); err != nil {
		t.Fatal(err)
	}
	if err := rewriteChatHistory(rt.SessionDir, "disk", []providers.ChatMessage{{Role: "user", Content: "disk-only needle"}}); err != nil {
		t.Fatal(err)
	}
	out := &lockedBuffer{}
	srv := New(rt, out)
	now := time.Now()
	live := &threadState{ID: "disk", Title: "live title", Model: "model", UpdatedAt: now, History: []providers.ChatMessage{
		{Role: "user", Content: "unpersisted needle"},
		{Role: "assistant", Content: "hidden secret", Hidden: true},
	}}
	srv.threads[live.ID] = live
	srv.threads["pinned"] = &threadState{ID: "pinned", Title: "pinned title", PinnedAt: &now, UpdatedAt: now.Add(-time.Hour)}
	search := func(id, query string, limit int) []ThreadSearchResultItem {
		t.Helper()
		params, err := json.Marshal(ThreadSearchParams{Query: query, Limit: limit})
		if err != nil {
			t.Fatal(err)
		}
		rawID, _ := json.Marshal(id)
		if err := srv.handleThreadSearch(Request{ID: rawID, Params: params}); err != nil {
			t.Fatal(err)
		}
		response := responseByID(t, parseOutput(t, out.String()), id)
		if response["error"] != nil {
			t.Fatalf("search failed: %+v", response)
		}
		return remarshal[ThreadSearchResult](t, response["result"]).Results
	}
	if got := search("1", "", 1); len(got) != 1 || got[0].Thread.ID != "pinned" || got[0].Snippet != "pinned title" {
		t.Fatalf("opening search must preserve pin order and limit: %+v", got)
	}
	if got := search("2", "needle", 40); len(got) != 1 || got[0].Snippet != "unpersisted needle" {
		t.Fatalf("live history must override disk: %+v", got)
	}
	for i, query := range []string{"disk-only", "hidden secret"} {
		if got := search(fmt.Sprint(i+3), query, 40); len(got) != 0 {
			t.Fatalf("unexpected matches for %q: %+v", query, got)
		}
	}
	live.mu.Lock()
	live.History = append(live.History, providers.ChatMessage{Role: "assistant", Content: "fresh update"})
	live.mu.Unlock()
	if got := search("5", "fresh update", 40); len(got) != 1 {
		t.Fatalf("search must see new messages: %+v", got)
	}
}

// Opening the dialog should scale with conversation metadata, not transcript
// size. Keep a large-history workload to make eager transcript loads visible.
func BenchmarkThreadSearchOpen(b *testing.B) {
	root := b.TempDir()
	rt := &runtime.Session{RootDir: root, SessionDir: root + "/sessions", Model: "model", ProviderName: "provider"}
	for i := 0; i < 100; i++ {
		id := fmt.Sprintf("thread-%03d", i)
		if _, err := session.CreateWithMetadata(rt.SessionDir, id, root); err != nil {
			b.Fatal(err)
		}
		if err := session.UpdateIndex(rt.SessionDir, id, 1, "conversation summary"); err != nil {
			b.Fatal(err)
		}
		if err := rewriteChatHistory(rt.SessionDir, id, []providers.ChatMessage{{Role: "user", Content: strings.Repeat("history text ", 10000)}}); err != nil {
			b.Fatal(err)
		}
	}
	srv := New(rt, io.Discard)
	params, _ := json.Marshal(ThreadSearchParams{Limit: 40})
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := srv.handleThreadSearch(Request{ID: json.RawMessage(`"bench"`), Params: params}); err != nil {
			b.Fatal(err)
		}
	}
}

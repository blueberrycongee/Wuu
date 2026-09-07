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

func TestThreadSearchRanksBeforeLimiting(t *testing.T) {
	rt := newTestRuntime(t, &fakeClient{})
	out := &lockedBuffer{}
	srv := New(rt, out)
	now := time.Now()
	srv.threads["tool"] = &threadState{ID: "tool", Title: "Recent logs", UpdatedAt: now, PinnedAt: &now,
		History: []providers.ChatMessage{{Role: "tool", Content: `{"output":"onboarding\\nlog"}`}}}
	srv.threads["body"] = &threadState{ID: "body", Title: "Design discussion", UpdatedAt: now.Add(-time.Hour),
		History: []providers.ChatMessage{
			{Role: "tool", Content: "onboarding build output"},
			{Role: "user", Content: "Improve onboarding for new users"},
		}}
	srv.threads["title"] = &threadState{ID: "title", Title: "Onboarding layout", UpdatedAt: now.Add(-2 * time.Hour)}
	for _, limit := range []int{1, 2, 3} {
		id := fmt.Sprint(limit)
		params, _ := json.Marshal(ThreadSearchParams{Query: "ONBOARDING", Limit: limit})
		rawID, _ := json.Marshal(id)
		if err := srv.handleThreadSearch(Request{ID: rawID, Params: params}); err != nil {
			t.Fatal(err)
		}
		response := responseByID(t, parseOutput(t, out.String()), id)
		if response["error"] != nil {
			t.Fatalf("search failed: %+v", response)
		}
		got := remarshal[ThreadSearchResult](t, response["result"]).Results
		if len(got) != limit {
			t.Fatalf("limit %d: %+v", limit, got)
		}
		for i, want := range []string{"title", "body", "tool"}[:limit] {
			if got[i].Thread.ID != want {
				t.Fatalf("limit %d: result %d should be %s: %+v", limit, i, want, got)
			}
		}
		if limit >= 2 && got[1].Snippet != "Improve onboarding for new users" {
			t.Fatalf("conversation snippet should outrank earlier tool output: %+v", got[1])
		}
	}
}

func TestThreadSearchLimitedPageMatchesFullRanking(t *testing.T) {
	rt := newTestRuntime(t, &fakeClient{})
	out := &lockedBuffer{}
	srv := New(rt, out)
	now := time.Now()
	for i := 0; i < 16; i++ {
		id := fmt.Sprintf("thread-%02d", i)
		th := &threadState{ID: id, Title: "unrelated", UpdatedAt: now.Add(time.Duration(i/2) * time.Hour)}
		switch i % 4 {
		case 0:
			th.Title = "needle title"
		case 1:
			th.History = []providers.ChatMessage{{Role: "tool", Content: "needle log"}, {Role: "user", Content: "needle discussion"}}
		case 2:
			th.History = []providers.ChatMessage{{Role: "tool", Content: "needle log"}}
		case 3:
			th.History = []providers.ChatMessage{{Role: "user", Content: "needle hidden", Hidden: true}}
		}
		if i%3 == 0 {
			th.PinnedAt = &now
		}
		srv.threads[id] = th
	}
	search := func(limit int) []ThreadSearchResultItem {
		t.Helper()
		id := fmt.Sprint(limit)
		params, _ := json.Marshal(ThreadSearchParams{Query: "needle", Limit: limit})
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
	full := search(100)
	if len(full) != 12 {
		t.Fatalf("expected 12 visible matches, got %d", len(full))
	}
	for limit := 1; limit <= len(full); limit++ {
		page := search(limit)
		if len(page) != limit {
			t.Fatalf("limit %d: got %d results", limit, len(page))
		}
		for i := range page {
			if page[i].Thread.ID != full[i].Thread.ID || page[i].Snippet != full[i].Snippet {
				t.Fatalf("limit %d: result %d differs from full ranking", limit, i)
			}
		}
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
		preview := "unrelated conversation"
		if i < 40 {
			preview = "conversation summary"
		}
		if err := session.UpdateIndex(rt.SessionDir, id, 1, preview); err != nil {
			b.Fatal(err)
		}
		if err := rewriteChatHistory(rt.SessionDir, id, []providers.ChatMessage{{Role: "user", Content: strings.Repeat("history text ", 10000)}}); err != nil {
			b.Fatal(err)
		}
	}
	srv := New(rt, io.Discard)
	for _, query := range []string{"", "summary", "history", "missing"} {
		b.Run("query="+query, func(b *testing.B) {
			params, _ := json.Marshal(ThreadSearchParams{Query: query, Limit: 40})
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if err := srv.handleThreadSearch(Request{ID: json.RawMessage(`"bench"`), Params: params}); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func TestThreadSearchPersistedHistoryRefreshesAcrossQueries(t *testing.T) {
	rt := newTestRuntime(t, &fakeClient{})
	for _, id := range []string{"first", "second"} {
		if _, err := session.CreateWithMetadata(rt.SessionDir, id, rt.RootDir); err != nil {
			t.Fatal(err)
		}
		if err := rewriteChatHistory(rt.SessionDir, id, []providers.ChatMessage{{Role: "user", Content: "onboarding discussion"}}); err != nil {
			t.Fatal(err)
		}
	}
	out := &lockedBuffer{}
	srv := New(rt, out)
	search := func(id, query string, want int) {
		t.Helper()
		params, _ := json.Marshal(ThreadSearchParams{Query: query, Limit: 40})
		rawID, _ := json.Marshal(id)
		if err := srv.handleThreadSearch(Request{ID: rawID, Params: params}); err != nil {
			t.Fatal(err)
		}
		response := responseByID(t, parseOutput(t, out.String()), id)
		if response["error"] != nil {
			t.Fatalf("search failed: %+v", response)
		}
		got := remarshal[ThreadSearchResult](t, response["result"]).Results
		if len(got) != want {
			t.Fatalf("query %q: got %d results, want %d", query, len(got), want)
		}
	}
	search("1", "onboarding", 2)
	if err := rewriteChatHistory(rt.SessionDir, "first", []providers.ChatMessage{{Role: "user", Content: "replacement discussion"}}); err != nil {
		t.Fatal(err)
	}
	search("2", "onboarding", 1)
	search("3", "replacement", 1)
	if err := session.AppendHistoryRecord(rt.SessionDir, "second", session.HistoryRecord{Role: "user", Content: "fresh append"}); err != nil {
		t.Fatal(err)
	}
	search("4", "fresh append", 1)
}

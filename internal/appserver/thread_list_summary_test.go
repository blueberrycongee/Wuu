package appserver

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestThreadListSummaryPreservesMetadataAndFullHistory(t *testing.T) {
	for _, method := range []string{"thread/list", "thread/listAll", "thread/listArchived"} {
		t.Run(method, func(t *testing.T) {
			rt := newTestRuntime(t, &fakeClient{})
			out := &lockedBuffer{}
			srv := New(rt, out)
			th := newThreadState("summary-thread", nil, rt.ProviderName, rt.Model, rt.RootDir, false, time.Now())
			th.Turns = []Turn{{ID: "retained-turn"}}
			th.Title = "Conversation title"
			th.running = true
			if method == "thread/listArchived" {
				now := time.Now()
				th.ArchivedAt = &now
			}
			srv.threads[th.ID] = th
			for i, summary := range []bool{true, false} {
				id := string(rune('a' + i))
				raw, _ := json.Marshal(map[string]any{"id": id, "method": method, "params": ThreadListParams{SummaryOnly: summary}})
				if err := srv.handleLine(context.Background(), raw); err != nil {
					t.Fatal(err)
				}
				result := remarshal[ThreadListResult](t, responseByID(t, parseOutput(t, out.String()), id)["result"])
				if len(result.Threads) != 1 {
					t.Fatalf("missing thread: %+v", result)
				}
				got := result.Threads[0]
				if got.ID != th.ID || got.Title != th.Title || got.Status != ThreadStatusInProgress || got.CWD != rt.RootDir {
					t.Fatal("summary lost metadata", got)
				}
				if summary {
					if got.Turns == nil || len(got.Turns) != 0 {
						t.Fatal("summary must contain an empty turns array")
					}
				} else if len(got.Turns) != 1 || got.Turns[0].ID != "retained-turn" {
					t.Fatal("summary changed retained history", got)
				}
			}
		})
	}
}

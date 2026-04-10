package subagent

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/blueberrycongee/wuu/internal/providers"
)

// fakeClient is a tiny providers.StreamClient stub for tests. It
// returns the canned response on every Chat / StreamChat call.
type fakeClient struct {
	response providers.ChatResponse
	err      error
	calls    atomic.Int32
	delay    time.Duration
}

func (f *fakeClient) Chat(ctx context.Context, _ providers.ChatRequest) (providers.ChatResponse, error) {
	f.calls.Add(1)
	if f.delay > 0 {
		select {
		case <-time.After(f.delay):
		case <-ctx.Done():
			return providers.ChatResponse{}, ctx.Err()
		}
	}
	if f.err != nil {
		return providers.ChatResponse{}, f.err
	}
	return f.response, nil
}

// StreamChat replays the same canned response as a single content
// delta followed by a terminal Done event. Errors and the delay knob
// behave the same way they do for Chat so existing tests don't need
// to grow a stream-specific code path.
func (f *fakeClient) StreamChat(ctx context.Context, _ providers.ChatRequest) (<-chan providers.StreamEvent, error) {
	f.calls.Add(1)
	if f.err != nil {
		return nil, f.err
	}
	ch := make(chan providers.StreamEvent, 4)
	go func() {
		defer close(ch)
		if f.delay > 0 {
			select {
			case <-time.After(f.delay):
			case <-ctx.Done():
				ch <- providers.StreamEvent{Type: providers.EventError, Error: ctx.Err()}
				return
			}
		}
		if f.response.Content != "" {
			ch <- providers.StreamEvent{Type: providers.EventContentDelta, Content: f.response.Content}
		}
		ch <- providers.StreamEvent{
			Type:       providers.EventDone,
			StopReason: f.response.StopReason,
			Truncated:  f.response.Truncated,
		}
	}()
	return ch, nil
}

// fakeToolkit is a no-op ToolExecutor that satisfies the runner contract.
type fakeToolkit struct{}

func (fakeToolkit) Definitions() []providers.ToolDefinition { return nil }
func (fakeToolkit) Execute(_ context.Context, _ providers.ToolCall) (string, error) {
	return "", nil
}

func TestSpawn_HappyPath(t *testing.T) {
	client := &fakeClient{response: providers.ChatResponse{Content: "all done"}}
	mgr := NewManager(client, "fake-model")

	sa, err := mgr.Spawn(context.Background(), SpawnOptions{
		Type:    "explorer",
		Prompt:  "find foo",
		Toolkit: fakeToolkit{},
	})
	if err != nil {
		t.Fatalf("Spawn: %v", err)
	}
	if sa.ID == "" || sa.Type != "explorer" {
		t.Fatalf("unexpected sub-agent: %+v", sa)
	}

	snap, err := mgr.Wait(context.Background(), sa.ID)
	if err != nil {
		t.Fatalf("Wait: %v", err)
	}
	if snap.Status != StatusCompleted {
		t.Fatalf("expected completed, got %s", snap.Status)
	}
	if snap.Result != "all done" {
		t.Fatalf("got result %q", snap.Result)
	}
	if client.calls.Load() != 1 {
		t.Fatalf("expected 1 LLM call, got %d", client.calls.Load())
	}
}

func TestSpawn_LLMError(t *testing.T) {
	client := &fakeClient{err: errors.New("boom")}
	mgr := NewManager(client, "fake-model")

	sa, err := mgr.Spawn(context.Background(), SpawnOptions{
		Type:    "worker",
		Prompt:  "do thing",
		Toolkit: fakeToolkit{},
	})
	if err != nil {
		t.Fatal(err)
	}
	snap, _ := mgr.Wait(context.Background(), sa.ID)
	if snap.Status != StatusFailed {
		t.Fatalf("expected failed, got %s", snap.Status)
	}
	if snap.Error == nil {
		t.Fatal("expected error to be set")
	}
}

func TestSpawn_Cancel(t *testing.T) {
	client := &fakeClient{
		response: providers.ChatResponse{Content: "ok"},
		delay:    2 * time.Second,
	}
	mgr := NewManager(client, "fake-model")

	sa, err := mgr.Spawn(context.Background(), SpawnOptions{
		Type:    "worker",
		Prompt:  "slow",
		Toolkit: fakeToolkit{},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Give the goroutine a moment to start, then cancel.
	time.Sleep(50 * time.Millisecond)
	if !mgr.Stop(sa.ID) {
		t.Fatal("Stop returned false")
	}

	snap, _ := mgr.Wait(context.Background(), sa.ID)
	if snap.Status != StatusCancelled {
		t.Fatalf("expected cancelled, got %s", snap.Status)
	}
}

func TestStopAll(t *testing.T) {
	client := &fakeClient{
		response: providers.ChatResponse{Content: "ok"},
		delay:    2 * time.Second,
	}
	mgr := NewManager(client, "fake-model")

	for i := 0; i < 3; i++ {
		_, err := mgr.Spawn(context.Background(), SpawnOptions{
			Type:    "worker",
			Prompt:  "slow",
			Toolkit: fakeToolkit{},
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	time.Sleep(50 * time.Millisecond)
	if mgr.CountRunning() != 3 {
		t.Fatalf("expected 3 running, got %d", mgr.CountRunning())
	}

	mgr.StopAll()
	deadline := time.Now().Add(1 * time.Second)
	for time.Now().Before(deadline) {
		if mgr.CountRunning() == 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if mgr.CountRunning() != 0 {
		t.Fatalf("expected 0 running after StopAll, got %d", mgr.CountRunning())
	}
}

func TestNotifications(t *testing.T) {
	client := &fakeClient{response: providers.ChatResponse{Content: "ok"}}
	mgr := NewManager(client, "fake-model")

	ch := make(chan Notification, 16)
	mgr.Subscribe(ch)

	sa, _ := mgr.Spawn(context.Background(), SpawnOptions{
		Type:    "explorer",
		Prompt:  "p",
		Toolkit: fakeToolkit{},
	})
	mgr.Wait(context.Background(), sa.ID)

	// Should have received: running + completed.
	statuses := []Status{}
	timeout := time.After(500 * time.Millisecond)
loop:
	for {
		select {
		case n := <-ch:
			statuses = append(statuses, n.Status)
			if n.Status == StatusCompleted {
				break loop
			}
		case <-timeout:
			t.Fatalf("did not receive completed notification, got %v", statuses)
		}
	}
	if len(statuses) < 2 {
		t.Fatalf("expected at least 2 notifications, got %d: %v", len(statuses), statuses)
	}
}

func TestPersistHistory(t *testing.T) {
	dir := t.TempDir()
	historyPath := filepath.Join(dir, "subagents", "worker.json")

	client := &fakeClient{response: providers.ChatResponse{Content: "ok"}}
	mgr := NewManager(client, "fake-model")

	sa, _ := mgr.Spawn(context.Background(), SpawnOptions{
		Type:        "worker",
		Description: "test task",
		Prompt:      "do it",
		Toolkit:     fakeToolkit{},
		HistoryPath: historyPath,
	})
	mgr.Wait(context.Background(), sa.ID)

	if _, err := os.Stat(historyPath); err != nil {
		t.Fatalf("history file not written: %v", err)
	}
	data, _ := os.ReadFile(historyPath)
	if len(data) < 10 || !contains(string(data), "ok") {
		t.Fatalf("history file content unexpected: %s", data)
	}
}

func TestList(t *testing.T) {
	client := &fakeClient{response: providers.ChatResponse{Content: "ok"}}
	mgr := NewManager(client, "fake-model")

	for i := 0; i < 3; i++ {
		_, _ = mgr.Spawn(context.Background(), SpawnOptions{
			Type:    "worker",
			Prompt:  "p",
			Toolkit: fakeToolkit{},
		})
	}
	if got := len(mgr.List()); got != 3 {
		t.Fatalf("expected 3 sub-agents in list, got %d", got)
	}
}

func TestSpawn_RequiresToolkitAndPrompt(t *testing.T) {
	mgr := NewManager(&fakeClient{}, "m")

	_, err := mgr.Spawn(context.Background(), SpawnOptions{Prompt: "x"})
	if err == nil {
		t.Error("expected error for missing toolkit")
	}
	_, err = mgr.Spawn(context.Background(), SpawnOptions{Toolkit: fakeToolkit{}})
	if err == nil {
		t.Error("expected error for missing prompt")
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

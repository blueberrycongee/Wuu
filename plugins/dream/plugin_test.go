package dream

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	pluginapi "github.com/blueberrycongee/wuu/packages/plugin-go"
)

type fakeHost struct {
	mu      sync.Mutex
	state   string
	creates []pluginapi.SessionCreateParams
	sends   []pluginapi.SessionSendParams
}

func (h *fakeHost) InitializeParams() pluginapi.InitializeParams { return pluginapi.InitializeParams{} }
func (h *fakeHost) CallHost(_ context.Context, method string, params, result any) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	raw, _ := json.Marshal(params)
	switch method {
	case hostStorageGet:
		if h.state != "" {
			_ = json.Unmarshal([]byte(`{"value":`+quoted(h.state)+`}`), result)
		}
	case hostStorageSet:
		var input struct {
			Value string `json:"value"`
		}
		_ = json.Unmarshal(raw, &input)
		h.state = input.Value
	case pluginapi.HostServiceSessionCreate:
		var input pluginapi.SessionCreateParams
		_ = json.Unmarshal(raw, &input)
		h.creates = append(h.creates, input)
		_ = json.Unmarshal([]byte(`{"session_id":"dream-session","created":true}`), result)
	case pluginapi.HostServiceSessionSend:
		var input pluginapi.SessionSendParams
		_ = json.Unmarshal(raw, &input)
		h.sends = append(h.sends, input)
		_ = json.Unmarshal([]byte(`{"state":"queued","session_id":"dream-session","queue_id":"q"}`), result)
	}
	return nil
}

func TestDreamUsesForkedPrivateSession(t *testing.T) {
	host := &fakeHost{}
	c := &controller{now: func() time.Time { return time.Date(2026, 8, 8, 0, 0, 0, 0, time.UTC) }, tick: time.Hour}
	if err := c.start(context.Background(), host); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.shutdown(context.Background()) })
	c.mu.Lock()
	c.state.Settings = settings{Enabled: true, IntervalDays: 7, MinSessions: 1}
	c.mu.Unlock()
	raw, _ := json.Marshal(turnCompletedInput{ThreadID: "parent", Succeeded: true, CompletedAt: c.now()})
	if _, err := c.invokeCapability(context.Background(), pluginapi.CapabilityCall{Capability: capabilityTurnCompleted, Input: raw}); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		host.mu.Lock()
		done := len(host.sends) == 1
		host.mu.Unlock()
		if done {
			break
		}
		time.Sleep(time.Millisecond)
	}
	host.mu.Lock()
	defer host.mu.Unlock()
	if len(host.creates) != 1 || host.creates[0].ParentSessionID != "parent" || host.creates[0].ContextSource != "fork" || host.creates[0].Visibility != "plugin" {
		t.Fatalf("creates=%+v", host.creates)
	}
	if len(host.sends) != 1 || host.sends[0].Cause != "dream.consolidate" {
		t.Fatalf("sends=%+v", host.sends)
	}
}

func quoted(value string) string { raw, _ := json.Marshal(value); return string(raw) }

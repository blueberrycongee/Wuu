package appserver

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/blueberrycongee/wuu/internal/providers"
)

type answerReadyBlockingStreamClient struct {
	started chan struct{}
	release chan struct{}
	content string
	once    sync.Once
}

func newAnswerReadyBlockingStreamClient(content string) *answerReadyBlockingStreamClient {
	return &answerReadyBlockingStreamClient{
		started: make(chan struct{}),
		release: make(chan struct{}),
		content: content,
	}
}

func (c *answerReadyBlockingStreamClient) Chat(ctx context.Context, _ providers.ChatRequest) (providers.ChatResponse, error) {
	<-c.started
	select {
	case <-c.release:
	case <-ctx.Done():
		return providers.ChatResponse{}, ctx.Err()
	}
	return providers.ChatResponse{Content: c.content}, nil
}

func (c *answerReadyBlockingStreamClient) StreamChat(ctx context.Context, _ providers.ChatRequest) (<-chan providers.StreamEvent, error) {
	ch := make(chan providers.StreamEvent, 4)
	go func() {
		defer close(ch)
		ch <- providers.StreamEvent{
			Type: providers.EventMessage,
			Message: &providers.ChatMessage{
				Role:    "assistant",
				Content: c.content,
				Phase:   providers.MessagePhaseFinalAnswer,
			},
		}
		c.once.Do(func() { close(c.started) })
		select {
		case <-c.release:
			ch <- providers.StreamEvent{Type: providers.EventDone}
		case <-ctx.Done():
			ch <- providers.StreamEvent{Type: providers.EventError, Error: ctx.Err()}
		}
	}()
	return ch, nil
}

func TestServerStartsFollowUpAfterFinalAnswerWithoutBusyReject(t *testing.T) {
	client := newAnswerReadyBlockingStreamClient("done")
	rt := newTestRuntime(t, &fakeClient{})
	rt.StreamRunner.Client = client
	out := &lockedBuffer{}
	srv := New(rt, out)
	t.Cleanup(func() {
		select {
		case <-client.release:
		default:
			close(client.release)
		}
		srv.Close()
	})

	if err := srv.handleLine(context.Background(), []byte(`{"id":"1","method":"thread/start"}`)); err != nil {
		t.Fatalf("thread/start: %v", err)
	}
	threadID := remarshal[ThreadStartResult](t, responseByID(t, parseOutput(t, out.String()), "1")["result"]).Thread.ID

	startReq := fmt.Sprintf(`{"id":"2","method":"turn/start","params":{"thread_id":%q,"prompt":"first"}}`, threadID)
	if err := srv.handleLine(context.Background(), []byte(startReq)); err != nil {
		t.Fatalf("turn/start: %v", err)
	}
	select {
	case <-client.started:
	case <-time.After(2 * time.Second):
		t.Fatal("first turn did not become answer-ready")
	}
	waitUntil(t, 2*time.Second, func() bool {
		return threadCurrentTurnIsAnswerReady(srv.thread(threadID))
	})

	followUp := fmt.Sprintf(`{"id":"3","method":"turn/start","params":{"thread_id":%q,"prompt":"next question"}}`, threadID)
	if err := srv.handleLine(context.Background(), []byte(followUp)); err != nil {
		t.Fatalf("follow-up turn/start: %v", err)
	}
	resp := waitForResponseByID(t, out, "3")
	if resp["error"] != nil {
		t.Fatalf("follow-up turn/start was rejected after the answer was ready: %+v", resp["error"])
	}
	started := remarshal[TurnStartResult](t, resp["result"])
	if started.Turn.ID == "" || started.Turn.Status != TurnStatusInProgress {
		t.Fatalf("follow-up turn = %+v", started.Turn)
	}

	th := srv.thread(threadID)
	th.mu.Lock()
	defer th.mu.Unlock()
	if len(th.Turns) < 2 {
		t.Fatalf("turns = %+v, want previous answer plus follow-up", th.Turns)
	}
	previous := th.Turns[0]
	if previous.Status != TurnStatusCompleted {
		t.Fatalf("previous turn status = %q, want completed", previous.Status)
	}
	if previous.Error != nil {
		t.Fatalf("previous turn should not carry a stream error after answer-ready handoff: %+v", previous.Error)
	}
	follow := th.Turns[len(th.Turns)-1]
	if follow.ID != started.Turn.ID {
		t.Fatalf("latest turn %q != started %q", follow.ID, started.Turn.ID)
	}
	foundFollowUp := false
	for _, item := range follow.Items {
		if item.Type == ThreadItemUserMessage && strings.Contains(item.Text, "next question") {
			foundFollowUp = true
			break
		}
	}
	if !foundFollowUp {
		t.Fatalf("follow-up user message missing: %+v", follow.Items)
	}
}

func TestServerStillRejectsTurnStartBeforeAnswerIsReady(t *testing.T) {
	client := newBlockingStreamClient("still thinking")
	rt := newTestRuntime(t, &fakeClient{})
	rt.StreamRunner.Client = client
	out := &lockedBuffer{}
	srv := New(rt, out)
	t.Cleanup(func() {
		select {
		case <-client.release:
		default:
			close(client.release)
		}
		srv.Close()
	})

	if err := srv.handleLine(context.Background(), []byte(`{"id":"1","method":"thread/start"}`)); err != nil {
		t.Fatalf("thread/start: %v", err)
	}
	threadID := remarshal[ThreadStartResult](t, responseByID(t, parseOutput(t, out.String()), "1")["result"]).Thread.ID

	startReq := fmt.Sprintf(`{"id":"2","method":"turn/start","params":{"thread_id":%q,"prompt":"first"}}`, threadID)
	if err := srv.handleLine(context.Background(), []byte(startReq)); err != nil {
		t.Fatalf("turn/start: %v", err)
	}
	select {
	case <-client.started:
	case <-time.After(2 * time.Second):
		t.Fatal("first turn did not start")
	}

	followUp := fmt.Sprintf(`{"id":"3","method":"turn/start","params":{"thread_id":%q,"prompt":"too early"}}`, threadID)
	if err := srv.handleLine(context.Background(), []byte(followUp)); err != nil {
		t.Fatalf("early turn/start: %v", err)
	}
	resp := responseByID(t, parseOutput(t, out.String()), "3")
	if resp["error"] == nil {
		t.Fatal("turn/start before the answer is ready should fail busy")
	}
	message, _ := resp["error"].(map[string]any)["message"].(string)
	if !strings.Contains(message, "already has a running turn") {
		t.Fatalf("error = %q, want running-turn busy", message)
	}
}

func waitUntil(t *testing.T, timeout time.Duration, ready func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if ready() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timed out waiting for condition")
}

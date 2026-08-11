package pluginhost

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestUserQuestionBrokerAskRespondRoundTrip(t *testing.T) {
	broker := NewUserQuestionBroker()
	events, unsubscribe := broker.Subscribe(4)
	defer unsubscribe()
	result := make(chan UserQuestionAnswer, 1)
	errCh := make(chan error, 1)
	go func() {
		answer, err := broker.Ask(context.Background(), "ask-user", "exec-1", UserQuestionAskParams{
			ThreadID: "thread-1", TurnID: "turn-1",
			Questions: []UserQuestion{{
				ID: "color", Question: "Pick a color", MultiSelect: true, AllowCustom: true,
				Options: []UserQuestionOption{{Label: "Blue"}, {Label: "Green"}},
			}},
		})
		if err != nil {
			errCh <- err
			return
		}
		result <- answer
	}()

	requested := waitUserQuestionEvent(t, events)
	if requested.Type != UserQuestionRequested || requested.Request == nil || requested.Request.ExecutionID != "exec-1" {
		t.Fatalf("requested event = %+v", requested)
	}
	if err := broker.Respond(requested.Request.RequestID, UserQuestionAnswer{Answers: []UserQuestionAnswerItem{{
		ID: "color", Selected: []string{"Blue"}, Custom: "with contrast",
	}}}); err != nil {
		t.Fatalf("Respond() = %v", err)
	}
	select {
	case err := <-errCh:
		t.Fatal(err)
	case answer := <-result:
		if len(answer.Answers) != 1 || answer.Answers[0].Custom != "with contrast" {
			t.Fatalf("answer = %+v", answer)
		}
	case <-time.After(time.Second):
		t.Fatal("Ask did not resume")
	}
	resolved := waitUserQuestionEvent(t, events)
	if resolved.Type != UserQuestionResolved || resolved.Outcome != "answered" {
		t.Fatalf("resolved event = %+v", resolved)
	}
	if got := broker.List("thread-1"); len(got) != 0 {
		t.Fatalf("pending = %+v", got)
	}
}

func TestUserQuestionBrokerCancellationRemovesPendingRequest(t *testing.T) {
	broker := NewUserQuestionBroker()
	events, unsubscribe := broker.Subscribe(4)
	defer unsubscribe()
	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		_, err := broker.Ask(ctx, "ask-user", "exec-2", UserQuestionAskParams{
			ThreadID: "thread-2", TurnID: "turn-2",
			Questions: []UserQuestion{{ID: "name", Question: "Name?", AllowCustom: true}},
		})
		errCh <- err
	}()
	requested := waitUserQuestionEvent(t, events)
	cancel()
	select {
	case err := <-errCh:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Ask() error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Ask did not cancel")
	}
	resolved := waitUserQuestionEvent(t, events)
	if resolved.RequestID != requested.Request.RequestID || resolved.Outcome != "cancelled" {
		t.Fatalf("resolved event = %+v", resolved)
	}
	if err := broker.Respond(requested.Request.RequestID, UserQuestionAnswer{}); !IsUserQuestionErrorCode(err, "question_not_pending") {
		t.Fatalf("late Respond() error = %v", err)
	}
}

func TestUserQuestionBrokerRejectsMalformedAnswerWithoutClaiming(t *testing.T) {
	broker := NewUserQuestionBroker()
	events, unsubscribe := broker.Subscribe(2)
	defer unsubscribe()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		_, _ = broker.Ask(ctx, "ask-user", "exec-3", UserQuestionAskParams{
			ThreadID: "thread-3", TurnID: "turn-3",
			Questions: []UserQuestion{{ID: "choice", Question: "Choose", Options: []UserQuestionOption{{Label: "A"}}}},
		})
	}()
	requested := waitUserQuestionEvent(t, events)
	err := broker.Respond(requested.Request.RequestID, UserQuestionAnswer{Answers: []UserQuestionAnswerItem{{ID: "choice"}}})
	if !IsUserQuestionErrorCode(err, "invalid_answer") {
		t.Fatalf("Respond(blank) error = %v", err)
	}
	err = broker.Respond(requested.Request.RequestID, UserQuestionAnswer{Answers: []UserQuestionAnswerItem{{ID: "choice", Selected: []string{"B"}}}})
	if !IsUserQuestionErrorCode(err, "invalid_answer") {
		t.Fatalf("Respond() error = %v", err)
	}
	if got := broker.List("thread-3"); len(got) != 1 {
		t.Fatalf("pending = %+v", got)
	}
}

func waitUserQuestionEvent(t *testing.T, events <-chan UserQuestionEvent) UserQuestionEvent {
	t.Helper()
	select {
	case event := <-events:
		return event
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for user question event")
		return UserQuestionEvent{}
	}
}

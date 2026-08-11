package appserver

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/blueberrycongee/wuu/internal/pluginhost"
	pluginapi "github.com/blueberrycongee/wuu/packages/plugin-go"
	askuser "github.com/blueberrycongee/wuu/plugins/ask-user"
)

func TestAppServerAskUserPluginProcessHelper(t *testing.T) {
	if os.Getenv("WUU_APP_SERVER_ASK_USER_HELPER") != "1" {
		return
	}
	if err := pluginapi.Serve(context.Background(), askuser.Handler()); err != nil {
		os.Exit(2)
	}
	os.Exit(0)
}

func TestRealAskUserPluginResumesThroughPublicAppServerProtocol(t *testing.T) {
	rt := newTestRuntime(t, &fakeClient{})
	rt.UserQuestions = pluginhost.NewUserQuestionBroker()
	client, err := pluginhost.Start(context.Background(), pluginhost.ProcessConfig{
		ID: "ask-user", Command: os.Args[0], Args: []string{"-test.run=^TestAppServerAskUserPluginProcessHelper$"},
		Env: map[string]string{"WUU_APP_SERVER_ASK_USER_HELPER": "1"}, Timeout: 5 * time.Second,
		ServiceRouter: userQuestionBrokerRouter{broker: rt.UserQuestions},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close(context.Background()) })
	var out synchronizedBuffer
	srv := New(rt, &out)
	defer srv.Close()

	toolDone := make(chan pluginhost.ToolExecuteResult, 1)
	errCh := make(chan error, 1)
	go func() {
		result, executeErr := client.ExecuteTool(context.Background(), pluginhost.ToolExecuteParams{
			ToolID: "ask_user",
			ToolExecuteInput: pluginhost.ToolExecuteInput{
				ThreadID: "thread-real", TurnID: "turn-real", ExecutionID: "execution-real", CallID: "call-real",
				Arguments: json.RawMessage(`{"questions":[{"id":"path","question":"Choose a path","options":[{"label":"Safe"},{"label":"Fast"}]}]}`),
			},
		})
		if executeErr != nil {
			errCh <- executeErr
			return
		}
		toolDone <- result
	}()

	var pending pluginhost.UserQuestionRequest
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		questions := rt.UserQuestions.List("thread-real")
		if len(questions) == 1 {
			pending = questions[0]
			break
		}
		time.Sleep(time.Millisecond)
	}
	if pending.RequestID == "" {
		t.Fatal("real plugin did not publish a pending question")
	}
	callPluginPackageRPC(t, srv, "respond-real", MethodUserQuestionRespond, UserQuestionRespondParams{
		RequestID: pending.RequestID,
		Answer: pluginhost.UserQuestionAnswer{Answers: []pluginhost.UserQuestionAnswerItem{{
			ID: "path", Selected: []string{"Safe"},
		}}},
	})
	select {
	case executeErr := <-errCh:
		t.Fatal(executeErr)
	case result := <-toolDone:
		if len(result.Result.Content) != 1 || result.Result.Content[0].Text == "" {
			t.Fatalf("tool result = %+v", result)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("real plugin Tool execution did not resume")
	}
}

type userQuestionBrokerRouter struct {
	broker *pluginhost.UserQuestionBroker
}

func (r userQuestionBrokerRouter) RouteServiceCall(ctx context.Context, pluginID string, call pluginhost.ServiceCallParams) (json.RawMessage, *pluginhost.HostServiceError) {
	if call.Service != pluginhost.KernelUserQuestionAskService {
		return nil, &pluginhost.HostServiceError{Code: "service_not_found", Message: "unexpected service"}
	}
	var params pluginhost.UserQuestionAskParams
	if err := json.Unmarshal(call.Params, &params); err != nil {
		return nil, &pluginhost.HostServiceError{Code: "invalid_params", Message: err.Error()}
	}
	answer, err := r.broker.Ask(ctx, pluginhost.UserQuestionOwner{
		PluginID: pluginID, ExecutionID: call.ExecutionID,
		ThreadID: "thread-real", TurnID: "turn-real", CallID: "call-real",
	}, params)
	if err != nil {
		return nil, &pluginhost.HostServiceError{Code: "question_failed", Message: err.Error()}
	}
	encoded, err := json.Marshal(answer)
	if err != nil {
		return nil, &pluginhost.HostServiceError{Code: "service_failed", Message: err.Error()}
	}
	return encoded, nil
}

func TestUserQuestionPublicProtocolResumesWaitingExecution(t *testing.T) {
	rt := newTestRuntime(t, &fakeClient{})
	rt.UserQuestions = pluginhost.NewUserQuestionBroker()
	var out synchronizedBuffer
	srv := New(rt, &out)
	defer srv.Close()
	events, unsubscribe := rt.UserQuestions.Subscribe(4)
	defer unsubscribe()
	answerCh := make(chan pluginhost.UserQuestionAnswer, 1)
	errCh := make(chan error, 1)
	go func() {
		answer, err := rt.UserQuestions.Ask(context.Background(), appServerQuestionOwner("1"), pluginhost.UserQuestionAskParams{
			Questions: []pluginhost.UserQuestion{{
				ID: "color", Question: "Which color?", AllowCustom: true,
				Options: []pluginhost.UserQuestionOption{{Label: "Blue"}, {Label: "Green"}},
			}},
		})
		if err != nil {
			errCh <- err
			return
		}
		answerCh <- answer
	}()
	requested := waitAppServerUserQuestionEvent(t, events)

	callPluginPackageRPC(t, srv, "list", MethodUserQuestionList, UserQuestionListParams{ThreadID: "thread-1"})
	listResponse := responseByID(t, parseOutput(t, out.String()), "list")
	list := remarshal[UserQuestionListResult](t, listResponse["result"])
	if len(list.Questions) != 1 || list.Questions[0].RequestID != requested.Request.RequestID {
		t.Fatalf("questions = %+v", list.Questions)
	}

	callPluginPackageRPC(t, srv, "respond", MethodUserQuestionRespond, UserQuestionRespondParams{
		RequestID: requested.Request.RequestID,
		Answer: pluginhost.UserQuestionAnswer{Answers: []pluginhost.UserQuestionAnswerItem{{
			ID: "color", Selected: []string{"Blue"}, Custom: "high contrast",
		}}},
	})
	response := responseByID(t, parseOutput(t, out.String()), "respond")
	if response["error"] != nil {
		t.Fatalf("respond response = %+v", response)
	}
	select {
	case err := <-errCh:
		t.Fatal(err)
	case answer := <-answerCh:
		if len(answer.Answers) != 1 || answer.Answers[0].Custom != "high contrast" {
			t.Fatalf("answer = %+v", answer)
		}
	case <-time.After(time.Second):
		t.Fatal("waiting execution did not resume")
	}

	deadline := time.Now().Add(time.Second)
	for {
		records := parseOutput(t, out.String())
		requestedSeen, resolvedSeen := false, false
		for _, record := range records {
			switch record["method"] {
			case NotificationUserQuestionRequested:
				requestedSeen = true
			case NotificationUserQuestionResolved:
				resolvedSeen = true
			}
		}
		if requestedSeen && resolvedSeen {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("missing question notifications: %s", out.String())
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestUserQuestionPublicProtocolCancelsWaitingExecution(t *testing.T) {
	rt := newTestRuntime(t, &fakeClient{})
	rt.UserQuestions = pluginhost.NewUserQuestionBroker()
	var out synchronizedBuffer
	srv := New(rt, &out)
	defer srv.Close()
	events, unsubscribe := rt.UserQuestions.Subscribe(2)
	defer unsubscribe()
	errCh := make(chan error, 1)
	go func() {
		_, err := rt.UserQuestions.Ask(context.Background(), appServerQuestionOwner("cancel"), pluginhost.UserQuestionAskParams{
			Questions: []pluginhost.UserQuestion{{ID: "confirm", Question: "Continue?", Options: []pluginhost.UserQuestionOption{{Label: "Yes"}}}},
		})
		errCh <- err
	}()
	requested := waitAppServerUserQuestionEvent(t, events)
	callPluginPackageRPC(t, srv, "cancel", MethodUserQuestionCancel, UserQuestionCancelParams{RequestID: requested.Request.RequestID})
	select {
	case err := <-errCh:
		if !pluginhost.IsUserQuestionErrorCode(err, "question_cancelled") {
			t.Fatalf("Ask() error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("cancel did not release waiting execution")
	}
}

func appServerQuestionOwner(suffix string) pluginhost.UserQuestionOwner {
	return pluginhost.UserQuestionOwner{
		PluginID: "ask-user", ExecutionID: "exec-" + suffix,
		ThreadID: "thread-" + suffix, TurnID: "turn-" + suffix, CallID: "call-" + suffix,
	}
}

func waitAppServerUserQuestionEvent(t *testing.T, events <-chan pluginhost.UserQuestionEvent) pluginhost.UserQuestionEvent {
	t.Helper()
	select {
	case event := <-events:
		return event
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for user question event")
		return pluginhost.UserQuestionEvent{}
	}
}

type synchronizedBuffer struct {
	mu     sync.Mutex
	buffer bytes.Buffer
}

func (b *synchronizedBuffer) Write(data []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.Write(data)
}

func (b *synchronizedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.String()
}

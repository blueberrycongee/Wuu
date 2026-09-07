package askuser_test

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/blueberrycongee/wuu/internal/pluginhost"
	pluginapi "github.com/blueberrycongee/wuu/packages/plugin-go"
	askuser "github.com/blueberrycongee/wuu/plugins/ask-user"
)

func TestAskUserPluginProcessHelper(t *testing.T) {
	if os.Getenv("WUU_ASK_USER_PLUGIN_TEST_HELPER") != "1" {
		return
	}
	if err := pluginapi.Serve(context.Background(), askuser.Handler()); err != nil {
		os.Exit(2)
	}
	os.Exit(0)
}

func TestAskUserPluginWaitsForKernelAnswerAcrossRealProcess(t *testing.T) {
	router := &askUserTestRouter{called: make(chan pluginhost.ServiceCallParams, 1), answer: make(chan json.RawMessage, 1)}
	client, err := pluginhost.Start(context.Background(), pluginhost.ProcessConfig{
		ID: "ask-user", Command: os.Args[0], Args: []string{"-test.run=^TestAskUserPluginProcessHelper$"},
		Env: map[string]string{"WUU_ASK_USER_PLUGIN_TEST_HELPER": "1"}, Timeout: 5 * time.Second, ServiceRouter: router,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close(context.Background()) })

	type result struct {
		output pluginhost.ToolExecuteResult
		err    error
	}
	done := make(chan result, 1)
	go func() {
		output, executeErr := client.ExecuteTool(context.Background(), pluginhost.ToolExecuteParams{
			ToolID: "ask_user",
			ToolExecuteInput: pluginhost.ToolExecuteInput{
				ThreadID: "thread-1", TurnID: "turn-1", ExecutionID: "execution-1", CallID: "call-1",
				Arguments: json.RawMessage(`{"questions":[{"id":"choice","question":"Which path?","options":[{"label":"A"},{"label":"B"}]}]}`),
			},
		})
		done <- result{output: output, err: executeErr}
	}()

	var serviceCall pluginhost.ServiceCallParams
	select {
	case serviceCall = <-router.called:
	case <-time.After(2 * time.Second):
		t.Fatal("plugin did not ask the kernel")
	}
	if serviceCall.Service != pluginhost.KernelUserQuestionOfferService || serviceCall.ExecutionID != "execution-1" {
		t.Fatalf("service call = %+v", serviceCall)
	}
	router.answer <- json.RawMessage(`{"request_id":"question-1","expires_at":"2026-01-01T00:00:20Z"}`)
	select {
	case completed := <-done:
		if completed.err != nil {
			t.Fatal(completed.err)
		}
		if len(completed.output.Result.Content) != 1 || !strings.Contains(completed.output.Result.Content[0].Text, `"question-1"`) {
			t.Fatalf("tool output = %+v", completed.output)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("tool did not return after offering the question")
	}
}

type askUserTestRouter struct {
	called chan pluginhost.ServiceCallParams
	answer chan json.RawMessage
}

func (r *askUserTestRouter) RouteServiceCall(ctx context.Context, _ string, params pluginhost.ServiceCallParams) (json.RawMessage, *pluginhost.HostServiceError) {
	r.called <- params
	select {
	case answer := <-r.answer:
		return answer, nil
	case <-ctx.Done():
		return nil, &pluginhost.HostServiceError{Code: "cancelled", Message: ctx.Err().Error()}
	}
}

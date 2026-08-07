package subagent_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/blueberrycongee/wuu/internal/pluginhost"
	pluginapi "github.com/blueberrycongee/wuu/packages/plugin-go"
	"github.com/blueberrycongee/wuu/plugins/subagent"
)

func TestSubagentPluginProcessHelper(t *testing.T) {
	if os.Getenv("WUU_SUBAGENT_PLUGIN_TEST_HELPER") != "1" {
		return
	}
	if err := pluginapi.Serve(context.Background(), subagent.Handler()); err != nil {
		os.Exit(2)
	}
	os.Exit(0)
}

func TestSubagentPluginCallsNeutralHostAcrossProcess(t *testing.T) {
	services := &childSessionTestServices{}
	client, err := pluginhost.Start(context.Background(), pluginhost.ProcessConfig{
		ID: "subagent", Command: os.Args[0], Args: []string{"-test.run=^TestSubagentPluginProcessHelper$"},
		Env: map[string]string{"WUU_SUBAGENT_PLUGIN_TEST_HELPER": "1"}, Timeout: 5 * time.Second,
		HostServiceHandler: services, SupportedHostServices: services.SupportedHostServices(),
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close(context.Background()) })
	result, err := client.ExecuteTool(context.Background(), pluginhost.ToolExecuteParams{
		ToolID: "send_message", ToolExecuteInput: pluginhost.ToolExecuteInput{
			ActorID: "parent", ActorPath: "/root", Arguments: json.RawMessage(`{"target":"child","message":"continue"}`),
		},
	})
	if err != nil || services.got.Action != "send" || services.got.ActorID != "parent" || len(result.Result.Content) != 1 {
		t.Fatalf("result=%+v request=%+v err=%v", result, services.got, err)
	}
}

type childSessionTestServices struct {
	got pluginhost.ChildSessionRequestParams
}

func (s *childSessionTestServices) SupportedHostServices() []pluginhost.HostServiceMethod {
	return []pluginhost.HostServiceMethod{pluginhost.HostServiceChildSessionRequest}
}
func (s *childSessionTestServices) HandleHostService(_ context.Context, method pluginhost.HostServiceMethod, raw json.RawMessage) (json.RawMessage, error) {
	if err := json.Unmarshal(raw, &s.got); err != nil {
		return nil, err
	}
	return json.RawMessage(`{"status":"sent"}`), nil
}

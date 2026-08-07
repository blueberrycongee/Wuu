package subagent_test

import (
	"context"
	"encoding/json"
	"os"
	"sync"
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

func TestSubagentPluginComposesSessionServicesAcrossProcess(t *testing.T) {
	services := &sessionTestServices{}
	client, err := pluginhost.Start(context.Background(), pluginhost.ProcessConfig{ID: "subagent", Command: os.Args[0], Args: []string{"-test.run=^TestSubagentPluginProcessHelper$"}, Env: map[string]string{"WUU_SUBAGENT_PLUGIN_TEST_HELPER": "1"}, Timeout: 5 * time.Second, HostServiceHandler: services, SupportedHostServices: services.SupportedHostServices()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close(context.Background()) })
	result, err := client.ExecuteTool(context.Background(), pluginhost.ToolExecuteParams{ToolID: "spawn_agent", ToolExecuteInput: pluginhost.ToolExecuteInput{SessionID: "parent", Arguments: json.RawMessage(`{"description":"review parser","prompt":"inspect it","run_in_background":true}`)}})
	if err != nil || len(result.Result.Content) != 1 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	services.mu.Lock()
	calls := append([]pluginhost.HostServiceMethod(nil), services.calls...)
	services.mu.Unlock()
	if len(calls) != 6 || calls[0] != pluginhost.HostServiceSessionCreate || calls[3] != pluginhost.HostServiceSessionSend {
		t.Fatalf("calls=%v", calls)
	}
}

type sessionTestServices struct {
	mu    sync.Mutex
	calls []pluginhost.HostServiceMethod
}

func (s *sessionTestServices) SupportedHostServices() []pluginhost.HostServiceMethod {
	return []pluginhost.HostServiceMethod{pluginhost.HostServiceSessionCreate, pluginhost.HostServiceSessionSend, pluginhost.HostServiceSessionList, pluginhost.HostServiceSessionCancel, pluginhost.HostServiceStorageGet, pluginhost.HostServiceStorageSet}
}
func (s *sessionTestServices) HandleHostService(_ context.Context, method pluginhost.HostServiceMethod, _ json.RawMessage) (json.RawMessage, error) {
	s.mu.Lock()
	s.calls = append(s.calls, method)
	s.mu.Unlock()
	switch method {
	case pluginhost.HostServiceSessionCreate:
		return json.RawMessage(`{"session_id":"child-1","created":true}`), nil
	case pluginhost.HostServiceSessionSend:
		return json.RawMessage(`{"state":"running","session_id":"child-1","turn_id":"turn-1"}`), nil
	default:
		return json.RawMessage(`{}`), nil
	}
}

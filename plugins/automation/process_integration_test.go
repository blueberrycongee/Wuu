package automation_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/blueberrycongee/wuu/internal/pluginhost"
	pluginapi "github.com/blueberrycongee/wuu/packages/plugin-go"
	"github.com/blueberrycongee/wuu/plugins/automation"
)

func TestAutomationPluginProcessHelper(t *testing.T) {
	if os.Getenv("WUU_AUTOMATION_PLUGIN_TEST_HELPER") != "1" {
		return
	}
	if err := pluginapi.Serve(context.Background(), automation.Handler()); err != nil {
		os.Exit(2)
	}
	os.Exit(0)
}

func TestAutomationPluginNegotiatesAcrossRealProcessProtocol(t *testing.T) {
	services := &automationTestHostServices{}
	client, err := pluginhost.Start(context.Background(), pluginhost.ProcessConfig{
		ID:                    "automation",
		Command:               os.Args[0],
		Args:                  []string{"-test.run=^TestAutomationPluginProcessHelper$"},
		Env:                   map[string]string{"WUU_AUTOMATION_PLUGIN_TEST_HELPER": "1"},
		Timeout:               5 * time.Second,
		HostServiceHandler:    services,
		SupportedHostServices: services.SupportedHostServices(),
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close(context.Background()) })

	created, err := client.ExecuteTool(context.Background(), pluginhost.ToolExecuteParams{
		ToolID: "cron",
		ToolExecuteInput: pluginhost.ToolExecuteInput{
			ThreadID:  "thread-1",
			Arguments: json.RawMessage(`{"action":"add","title":"Daily review","prompt":"Review open work","cron":"0 9 * * *","timezone":"UTC","mode":"thread_heartbeat","recurring":true,"durable":true}`),
		},
	})
	if err != nil || len(created.Result.Content) != 1 {
		t.Fatalf("create = %+v, err = %v", created, err)
	}
	services.mu.Lock()
	state := services.state
	services.mu.Unlock()
	if state == "" {
		t.Fatal("automation process did not persist its task through host storage")
	}
}

type automationTestHostServices struct {
	mu    sync.Mutex
	state string
}

func (s *automationTestHostServices) SupportedHostServices() []pluginhost.HostServiceMethod {
	return []pluginhost.HostServiceMethod{
		pluginhost.HostServiceStorageGet,
		pluginhost.HostServiceStorageSet,
		pluginhost.HostServiceSessionCreate,
		pluginhost.HostServiceSessionSend,
	}
}

func (s *automationTestHostServices) HandleHostService(_ context.Context, method pluginhost.HostServiceMethod, raw json.RawMessage) (json.RawMessage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch method {
	case pluginhost.HostServiceStorageGet:
		if s.state == "" {
			return json.Marshal(pluginhost.StorageGetResult{})
		}
		return json.Marshal(pluginhost.StorageGetResult{Value: &s.state})
	case pluginhost.HostServiceStorageSet:
		var params pluginhost.StorageSetParams
		if err := json.Unmarshal(raw, &params); err != nil {
			return nil, err
		}
		s.state = params.Value
		return json.RawMessage(`{}`), nil
	case pluginhost.HostServiceSessionCreate:
		return json.Marshal(pluginhost.SessionCreateResult{SessionID: "session-1", Created: true})
	case pluginhost.HostServiceSessionSend:
		return json.Marshal(pluginhost.SessionSendResult{State: pluginhost.TurnLifecycleQueued, SessionID: "session-1", QueueID: "queue-1"})
	default:
		return nil, errors.New("unsupported host service")
	}
}

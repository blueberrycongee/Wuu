package goal_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/blueberrycongee/wuu/internal/pluginhost"
	pluginapi "github.com/blueberrycongee/wuu/packages/plugin-go"
	"github.com/blueberrycongee/wuu/plugins/goal"
)

func TestGoalPluginProcessHelper(t *testing.T) {
	if os.Getenv("WUU_GOAL_PLUGIN_TEST_HELPER") != "1" {
		return
	}
	if err := pluginapi.Serve(context.Background(), goal.Handler()); err != nil {
		os.Exit(2)
	}
	os.Exit(0)
}

func TestGoalPluginNegotiatesAcrossRealProcessProtocol(t *testing.T) {
	services := &goalTestHostServices{values: map[string]string{}}
	client, err := pluginhost.Start(context.Background(), pluginhost.ProcessConfig{
		ID:                    "goal",
		Command:               os.Args[0],
		Args:                  []string{"-test.run=^TestGoalPluginProcessHelper$"},
		Env:                   map[string]string{"WUU_GOAL_PLUGIN_TEST_HELPER": "1"},
		Timeout:               5 * time.Second,
		HostServiceHandler:    services,
		SupportedHostServices: services.SupportedHostServices(),
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close(context.Background()) })

	created, err := client.ExecuteTool(context.Background(), pluginhost.ToolExecuteParams{
		ToolID:           "create_goal",
		ToolExecuteInput: pluginhost.ToolExecuteInput{ThreadID: "thread-1", Arguments: json.RawMessage(`{"objective":"prove process integration"}`)},
	})
	if err != nil || len(created.Result.Content) != 1 {
		t.Fatalf("create = %+v, err = %v", created, err)
	}
	capability := findGoalCapability(t, client, pluginhost.CapabilityAgentContinuation)
	var prepared pluginhost.AgentContinuationOutput
	if err := pluginhost.New(client).InvokeCapability(context.Background(), capability, pluginhost.AgentContinuationInput{
		ThreadID: "thread-1", Phase: pluginhost.ContinuationPhasePrepare,
	}, &prepared); err != nil {
		t.Fatal(err)
	}
	if !prepared.Continue || len(prepared.Blocks) != 1 || !strings.Contains(prepared.Blocks[0].Content, "prove process integration") {
		t.Fatalf("prepared = %+v", prepared)
	}
}

func findGoalCapability(t *testing.T, client *pluginhost.ProcessClient, id string) pluginhost.RegisteredCapability {
	t.Helper()
	registered := pluginhost.New(client).Capabilities(id)
	if len(registered) != 1 {
		t.Fatalf("capabilities %q = %+v", id, registered)
	}
	return registered[0]
}

type goalTestHostServices struct {
	mu     sync.Mutex
	values map[string]string
}

func (s *goalTestHostServices) SupportedHostServices() []pluginhost.HostServiceMethod {
	return []pluginhost.HostServiceMethod{pluginhost.HostServiceStorageGet, pluginhost.HostServiceStorageSet, pluginhost.HostServiceStorageDelete}
}

func (s *goalTestHostServices) HandleHostService(_ context.Context, method pluginhost.HostServiceMethod, raw json.RawMessage) (json.RawMessage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch method {
	case pluginhost.HostServiceStorageGet:
		var params pluginhost.StorageGetParams
		if err := json.Unmarshal(raw, &params); err != nil {
			return nil, err
		}
		value, ok := s.values[params.Key]
		if !ok {
			return json.Marshal(pluginhost.StorageGetResult{})
		}
		return json.Marshal(pluginhost.StorageGetResult{Value: &value})
	case pluginhost.HostServiceStorageSet:
		var params pluginhost.StorageSetParams
		if err := json.Unmarshal(raw, &params); err != nil {
			return nil, err
		}
		s.values[params.Key] = params.Value
		return json.RawMessage(`{}`), nil
	case pluginhost.HostServiceStorageDelete:
		var params pluginhost.StorageDeleteParams
		if err := json.Unmarshal(raw, &params); err != nil {
			return nil, err
		}
		delete(s.values, params.Key)
		return json.RawMessage(`{}`), nil
	default:
		return nil, errors.New("unsupported host service")
	}
}

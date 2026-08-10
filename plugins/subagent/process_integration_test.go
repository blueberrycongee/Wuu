package subagent_test

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

func TestProactiveDelegationNegotiatesAcrossRealProcessProtocol(t *testing.T) {
	services := &subagentTestHostServices{values: map[string]string{}}
	client, err := pluginhost.Start(context.Background(), pluginhost.ProcessConfig{
		ID:                    "subagent",
		Command:               os.Args[0],
		Args:                  []string{"-test.run=^TestSubagentPluginProcessHelper$"},
		Env:                   map[string]string{"WUU_SUBAGENT_PLUGIN_TEST_HELPER": "1"},
		Timeout:               5 * time.Second,
		HostServiceHandler:    services,
		SupportedHostServices: services.SupportedHostServices(),
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close(context.Background()) })

	host := pluginhost.New(client)
	clientCapability := onlySubagentCapability(t, host, pluginhost.CapabilityPluginClientRequest)
	var update struct {
		Result struct {
			Enabled bool `json:"enabled"`
		} `json:"result"`
	}
	if err := host.InvokeCapability(context.Background(), clientCapability, map[string]any{
		"method": "ultra.update",
		"input":  map[string]bool{"enabled": true},
	}, &update); err != nil {
		t.Fatal(err)
	}
	if !update.Result.Enabled {
		t.Fatal("proactive delegation setting was not enabled")
	}

	preStepCapability := onlySubagentCapability(t, host, pluginhost.CapabilityAgentPreStep)
	output := pluginhost.AgentPreStepOutput{}
	if err := host.InvokeCapability(context.Background(), preStepCapability, pluginhost.AgentPreStepInput{
		SessionID: "thread-1",
		ThreadID:  "thread-1",
		Model:     "test-model",
		Messages:  []pluginhost.ModelMessageViewV1{{Role: "user", Content: "work"}},
	}, &output); err != nil {
		t.Fatal(err)
	}
	if len(output.AppendMessages) != 1 || !strings.Contains(output.AppendMessages[0].Content, "Proactive delegation is enabled") {
		t.Fatalf("pre-step contribution = %+v", output)
	}
}

func onlySubagentCapability(t *testing.T, host *pluginhost.Host, id string) pluginhost.RegisteredCapability {
	t.Helper()
	capabilities := host.Capabilities(id)
	if len(capabilities) != 1 {
		t.Fatalf("capabilities %q = %+v", id, capabilities)
	}
	return capabilities[0]
}

type subagentTestHostServices struct {
	mu     sync.Mutex
	values map[string]string
}

func (s *subagentTestHostServices) SupportedHostServices() []pluginhost.HostServiceMethod {
	return []pluginhost.HostServiceMethod{
		pluginhost.HostServiceSessionCreate,
		pluginhost.HostServiceSessionSend,
		pluginhost.HostServiceSessionList,
		pluginhost.HostServiceSessionCancel,
		pluginhost.HostServiceStorageGet,
		pluginhost.HostServiceStorageSet,
	}
}

func (s *subagentTestHostServices) HandleHostService(_ context.Context, method pluginhost.HostServiceMethod, raw json.RawMessage) (json.RawMessage, error) {
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
	default:
		return nil, errors.New("unexpected host service invocation")
	}
}

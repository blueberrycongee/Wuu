package dream_test

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
	"github.com/blueberrycongee/wuu/plugins/dream"
)

func TestDreamPluginProcessHelper(t *testing.T) {
	if os.Getenv("WUU_DREAM_PLUGIN_TEST_HELPER") != "1" {
		return
	}
	if err := pluginapi.Serve(context.Background(), dream.Handler()); err != nil {
		os.Exit(2)
	}
	os.Exit(0)
}

func TestDreamPluginNegotiatesAcrossRealProcessProtocol(t *testing.T) {
	services := &dreamHostServices{}
	client, err := pluginhost.Start(context.Background(), pluginhost.ProcessConfig{ID: "dream", Command: os.Args[0], Args: []string{"-test.run=^TestDreamPluginProcessHelper$"}, Env: map[string]string{"WUU_DREAM_PLUGIN_TEST_HELPER": "1"}, Timeout: 5 * time.Second, HostServiceHandler: services, SupportedHostServices: services.SupportedHostServices()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close(context.Background()) })
	capabilities := pluginhost.New(client).Capabilities(pluginhost.CapabilityPluginClientRequest)
	if len(capabilities) != 1 {
		t.Fatalf("client capabilities=%+v", capabilities)
	}
	var output struct {
		Result json.RawMessage `json:"result"`
	}
	input := map[string]any{"method": "dream.update", "input": map[string]any{"enabled": true, "interval_days": 7, "min_sessions": 1}}
	if err := pluginhost.New(client).InvokeCapability(context.Background(), capabilities[0], input, &output); err != nil {
		t.Fatal(err)
	}
	turnCapabilities := pluginhost.New(client).Capabilities(pluginhost.CapabilityAgentTurnCompleted)
	if len(turnCapabilities) != 1 {
		t.Fatalf("turn capabilities=%+v", turnCapabilities)
	}
	if err := pluginhost.New(client).InvokeCapability(context.Background(), turnCapabilities[0], pluginhost.AgentTurnCompletedInput{ThreadID: "parent", TurnID: "turn", CompletedAt: time.Now().UTC(), Succeeded: true}, &pluginhost.AgentTurnCompletedOutput{}); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		services.mu.Lock()
		done := len(services.sends) > 0
		services.mu.Unlock()
		if done {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("dream process did not send a private session turn")
}

type dreamHostServices struct {
	mu    sync.Mutex
	state string
	sends []pluginhost.SessionSendParams
}

func (s *dreamHostServices) SupportedHostServices() []pluginhost.HostServiceMethod {
	return []pluginhost.HostServiceMethod{pluginhost.HostServiceStorageGet, pluginhost.HostServiceStorageSet, pluginhost.HostServiceSessionCreate, pluginhost.HostServiceSessionSend}
}
func (s *dreamHostServices) HandleHostService(_ context.Context, method pluginhost.HostServiceMethod, raw json.RawMessage) (json.RawMessage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch method {
	case pluginhost.HostServiceStorageGet:
		if s.state == "" {
			return json.Marshal(pluginhost.StorageGetResult{})
		}
		return json.Marshal(pluginhost.StorageGetResult{Value: &s.state})
	case pluginhost.HostServiceStorageSet:
		var p pluginhost.StorageSetParams
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, err
		}
		s.state = p.Value
		return json.RawMessage(`{}`), nil
	case pluginhost.HostServiceSessionCreate:
		return json.Marshal(pluginhost.SessionCreateResult{SessionID: "dream-session", Created: true})
	case pluginhost.HostServiceSessionSend:
		var p pluginhost.SessionSendParams
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, err
		}
		s.sends = append(s.sends, p)
		return json.Marshal(pluginhost.SessionSendResult{State: pluginhost.TurnLifecycleQueued, SessionID: p.SessionID, QueueID: "q"})
	default:
		return nil, errors.New("unsupported host service")
	}
}

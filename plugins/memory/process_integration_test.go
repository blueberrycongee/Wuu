package memory_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/blueberrycongee/wuu/internal/pluginhost"
	pluginapi "github.com/blueberrycongee/wuu/packages/plugin-go"
	"github.com/blueberrycongee/wuu/plugins/memory"
)

func TestMemoryPluginProcessHelper(t *testing.T) {
	if os.Getenv("WUU_MEMORY_PLUGIN_TEST_HELPER") != "1" {
		return
	}
	if err := pluginapi.Serve(context.Background(), memory.Handler()); err != nil {
		os.Exit(2)
	}
	os.Exit(0)
}

func TestMemoryPluginNegotiatesAcrossRealProcessProtocol(t *testing.T) {
	home := t.TempDir()
	workspaceStateDir := t.TempDir()
	services := &memoryTestHostServices{}
	client, err := pluginhost.Start(context.Background(), pluginhost.ProcessConfig{
		ID: "memory", Command: os.Args[0], Args: []string{"-test.run=^TestMemoryPluginProcessHelper$"},
		Env: map[string]string{"WUU_MEMORY_PLUGIN_TEST_HELPER": "1"}, WuuHome: home, WorkspaceStateDir: workspaceStateDir, Timeout: 5 * time.Second,
		ServiceRouter: services,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close(context.Background()) })

	written, err := client.ExecuteTool(context.Background(), pluginhost.ToolExecuteParams{
		ToolID: "memory_write", ToolExecuteInput: pluginhost.ToolExecuteInput{Arguments: json.RawMessage(`{"file":"MEMORY.md","content":"- [Testing](feedback_testing.md) — prefer real tests"}`)},
	})
	if err != nil || len(written.Result.Content) != 1 {
		t.Fatalf("write = %+v, err = %v", written, err)
	}
	if _, err := os.Stat(filepath.Join(home, "memory", "MEMORY.md")); err != nil {
		t.Fatal(err)
	}

	sessionWritten, err := client.ExecuteTool(context.Background(), pluginhost.ToolExecuteParams{
		ToolID: "session_memory", ToolExecuteInput: pluginhost.ToolExecuteInput{SessionID: "thread-1", Arguments: json.RawMessage(`{"action":"replace","target":"summary","content":"process-owned summary"}`)},
	})
	if err != nil || len(sessionWritten.Result.Content) != 1 {
		t.Fatalf("session write = %+v, err = %v", sessionWritten, err)
	}
	if content, err := os.ReadFile(filepath.Join(workspaceStateDir, "sessions", "thread-1", "session-memory", "summary.md")); err != nil || string(content) != "process-owned summary\n" {
		t.Fatalf("session content = %q, err = %v", content, err)
	}

	promptCapability := onlyMemoryCapability(t, client, pluginhost.CapabilityAgentSystemPromptSection)
	var prompt pluginhost.SystemPromptSectionOutput
	if err := pluginhost.New(client).InvokeCapability(context.Background(), promptCapability, pluginhost.SystemPromptSectionInput{}, &prompt); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(prompt.Text, "prefer real tests") {
		t.Fatalf("prompt = %q", prompt.Text)
	}

	requestCapability := onlyMemoryCapability(t, client, pluginhost.CapabilityPluginClientRequest)
	var response struct {
		Result json.RawMessage `json:"result"`
	}
	input := map[string]any{"method": "memory.overview.start", "input": map[string]any{}}
	if err := pluginhost.New(client).InvokeCapability(context.Background(), requestCapability, input, &response); err != nil {
		t.Fatal(err)
	}
	services.mu.Lock()
	defer services.mu.Unlock()
	if len(services.sends) != 1 || services.sends[0].SessionID != "memory-session" || services.sends[0].Cause != "memory.overview" {
		t.Fatalf("sends = %+v", services.sends)
	}
}

func onlyMemoryCapability(t *testing.T, client *pluginhost.ProcessClient, id string) pluginhost.RegisteredCapability {
	t.Helper()
	capabilities := pluginhost.New(client).Capabilities(id)
	if len(capabilities) != 1 {
		t.Fatalf("capabilities %q = %+v", id, capabilities)
	}
	return capabilities[0]
}

type memoryTestHostServices struct {
	mu    sync.Mutex
	sends []pluginhost.SessionSendParams
}

func (s *memoryTestHostServices) SupportedHostServices() []pluginhost.HostServiceMethod {
	return []pluginhost.HostServiceMethod{pluginhost.HostServiceSessionCreate, pluginhost.HostServiceSessionSend}
}

func (s *memoryTestHostServices) HandleHostService(_ context.Context, method pluginhost.HostServiceMethod, raw json.RawMessage) (json.RawMessage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch method {
	case pluginhost.HostServiceSessionCreate:
		return json.Marshal(pluginhost.SessionCreateResult{SessionID: "memory-session", Created: true})
	case pluginhost.HostServiceSessionSend:
		var params pluginhost.SessionSendParams
		if err := json.Unmarshal(raw, &params); err != nil {
			return nil, err
		}
		s.sends = append(s.sends, params)
		return json.Marshal(pluginhost.SessionSendResult{State: pluginhost.TurnLifecycleQueued, SessionID: params.SessionID, QueueID: "memory-queue"})
	default:
		return nil, errors.New("unsupported host service")
	}
}

func (s *memoryTestHostServices) RouteServiceCall(ctx context.Context, _ string, params pluginhost.ServiceCallParams) (json.RawMessage, *pluginhost.HostServiceError) {
	result, err := s.HandleHostService(ctx, pluginhost.HostServiceMethod(params.Service), params.Params)
	if err != nil {
		return nil, &pluginhost.HostServiceError{Code: "service_unavailable", Message: err.Error()}
	}
	return result, nil
}

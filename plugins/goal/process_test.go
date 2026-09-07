package goal

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/blueberrycongee/wuu/internal/pluginhost"
	pluginapi "github.com/blueberrycongee/wuu/packages/plugin-go"
)

type processRouter struct {
	host  *testHost
	ready bool
}

func (r *processRouter) RouteServiceCall(ctx context.Context, _ string, params pluginhost.ServiceCallParams) (json.RawMessage, *pluginhost.HostServiceError) {
	if !r.ready {
		return nil, &pluginhost.HostServiceError{Code: "service_unavailable", Message: "services are not registered until initialize completes"}
	}
	var result json.RawMessage
	if err := r.host.CallHost(ctx, pluginapi.HostServiceCallMethod, params, &result); err != nil {
		return nil, &pluginhost.HostServiceError{Code: "test_error", Message: err.Error()}
	}
	return result, nil
}

func TestGoalProcessNegotiationAndShutdown(t *testing.T) {
	if os.Getenv("WUU_GOAL_TEST_PROCESS") == "1" {
		if err := pluginapi.Serve(context.Background(), Handler()); err != nil {
			os.Exit(2)
		}
		os.Exit(0)
	}
	h := &testHost{}
	router := &processRouter{host: h}
	client, err := pluginhost.Start(context.Background(), pluginhost.ProcessConfig{ID: "goal", Command: os.Args[0], Args: []string{"-test.run=^TestGoalProcessNegotiationAndShutdown$"}, Env: map[string]string{"WUU_GOAL_TEST_PROCESS": "1"}, PluginRoot: t.TempDir(), ProjectRoot: t.TempDir(), WuuHome: t.TempDir(), Timeout: 3 * time.Second, ServiceRouter: router, PrepareOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close(context.Background())
	router.ready = true
	if err := client.Activate(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := client.ExecuteTool(context.Background(), pluginhost.ToolExecuteParams{ToolID: "create_goal", ToolExecuteInput: pluginhost.ToolExecuteInput{SessionID: "thread", TurnID: "initial", Arguments: json.RawMessage(`{"objective":"work"}`)}}); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	raw, _ := json.Marshal(completedTurn{ThreadID: "thread", TurnID: "initial", StartedAt: now, CompletedAt: now.Add(time.Second), Succeeded: true, InputTokens: 10})
	if _, err := client.InvokeCapability(context.Background(), pluginhost.CapabilityInvokeParams{Capability: completedCapability, Input: raw}); err != nil {
		t.Fatal(err)
	}
	if err := client.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.sends) != 1 || len(h.cancels) != 1 {
		t.Fatalf("process lifecycle sends=%d cancels=%d", len(h.sends), len(h.cancels))
	}
	var state map[string]Goal
	if h.stored == nil {
		t.Fatal("state not persisted")
	}
	if err := json.Unmarshal([]byte(*h.stored), &state); err != nil {
		t.Fatal(err)
	}
	if state["thread"].Status != "paused" {
		t.Fatalf("shutdown state = %+v", state["thread"])
	}
}

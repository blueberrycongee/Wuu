package runtime

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/blueberrycongee/wuu/internal/pluginhost"
)

func TestPluginTurnRouterBindsLatestHandlerAndProtectsNewBinding(t *testing.T) {
	router := NewPluginTurnRouter()
	if _, err := router.Submit(context.Background(), "demo", pluginhost.TurnSubmitParams{}); err == nil {
		t.Fatal("unbound router accepted a turn")
	}
	firstUnbind := router.Bind(func(_ context.Context, pluginID string, _ pluginhost.TurnSubmitParams) (pluginhost.TurnSubmitResult, error) {
		return pluginhost.TurnSubmitResult{ThreadID: "first:" + pluginID}, nil
	})
	secondUnbind := router.Bind(func(_ context.Context, pluginID string, _ pluginhost.TurnSubmitParams) (pluginhost.TurnSubmitResult, error) {
		return pluginhost.TurnSubmitResult{ThreadID: "second:" + pluginID}, nil
	})
	firstUnbind()
	result, err := router.Submit(context.Background(), "owner", pluginhost.TurnSubmitParams{})
	if err != nil || result.ThreadID != "second:owner" {
		t.Fatalf("latest binding = %+v, %v", result, err)
	}
	secondUnbind()
	if _, err := router.Submit(context.Background(), "owner", pluginhost.TurnSubmitParams{}); err == nil {
		t.Fatal("unbound latest handler remained callable")
	}
}

func TestPluginHostTurnSubmitUsesGenerationBoundOwner(t *testing.T) {
	router := NewPluginTurnRouter()
	var owner string
	router.Bind(func(_ context.Context, pluginID string, params pluginhost.TurnSubmitParams) (pluginhost.TurnSubmitResult, error) {
		owner = pluginID
		return pluginhost.TurnSubmitResult{State: pluginhost.TurnLifecycleRunning, ThreadID: "thread-1", TurnID: params.RequestID}, nil
	})
	handler := newPluginHostServices(serviceTestPlugin("alpha", "plugin:user:alpha", "one"), t.TempDir(), t.TempDir(), router)
	raw, err := handler.HandleHostService(context.Background(), pluginhost.HostServiceTurnSubmit, json.RawMessage(`{"request_id":"opaque","prompt":"work"}`))
	if err != nil {
		t.Fatal(err)
	}
	var result pluginhost.TurnSubmitResult
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatal(err)
	}
	if owner != "alpha" || result.TurnID != "opaque" {
		t.Fatalf("owner = %q, result = %+v", owner, result)
	}
	if _, err := handler.HandleHostService(context.Background(), pluginhost.HostServiceTurnSubmit, json.RawMessage(`{"plugin_id":"beta","request_id":"opaque","prompt":"work"}`)); err == nil {
		t.Fatal("caller-supplied plugin identity was accepted")
	}
}

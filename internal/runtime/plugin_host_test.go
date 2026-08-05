package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	pluginpkg "github.com/blueberrycongee/wuu/internal/plugin"
	"github.com/blueberrycongee/wuu/internal/pluginhost"
	"github.com/blueberrycongee/wuu/internal/providers"
)

type runtimePluginClient struct {
	input pluginhost.ChatRequestInput
}

func (c *runtimePluginClient) ID() string { return "runtime-test" }
func (c *runtimePluginClient) Hooks() []pluginhost.Hook {
	return []pluginhost.Hook{pluginhost.HookChatRequest}
}
func (c *runtimePluginClient) Status() pluginhost.Status {
	return pluginhost.Status{ID: c.ID(), State: pluginhost.StateActive}
}
func (c *runtimePluginClient) Close(context.Context) error { return nil }
func (c *runtimePluginClient) Invoke(_ context.Context, params pluginhost.InvokeParams) (pluginhost.InvokeResult, error) {
	if err := json.Unmarshal(params.Input, &c.input); err != nil {
		return pluginhost.InvokeResult{}, err
	}
	var output pluginhost.ChatRequestOutput
	if err := json.Unmarshal(params.Output, &output); err != nil {
		return pluginhost.InvokeResult{}, err
	}
	output.Model = "plugin-model"
	output.ProviderOptions = map[string]any{"plugin": true}
	data, err := json.Marshal(output)
	return pluginhost.InvokeResult{Output: data}, err
}

func TestPluginRequestInterceptorCarriesThreadContextAndTransformsRequest(t *testing.T) {
	client := &runtimePluginClient{}
	host := pluginhost.New(client)
	intercept := pluginRequestInterceptor(host, "openai", "thread-1", "/workspace")
	request := providers.ChatRequest{
		Model:           "original",
		Messages:        []providers.ChatMessage{{Role: "user", Content: "hello"}},
		StepIndex:       2,
		ProviderOptions: map[string]any{"original": true},
	}
	if err := intercept(context.Background(), &request); err != nil {
		t.Fatal(err)
	}
	if request.Model != "plugin-model" || request.ProviderOptions["plugin"] != true {
		t.Fatalf("request = %+v", request)
	}
	if client.input.ThreadID != "thread-1" || client.input.SessionID != "thread-1" || client.input.CWD != "/workspace" || client.input.Provider != "openai" || client.input.StepIndex != 2 {
		t.Fatalf("input = %+v", client.input)
	}
}

func TestPluginRequestInterceptorSkipsHostsWithoutChatRequestHook(t *testing.T) {
	if intercept := pluginRequestInterceptor(pluginhost.New(&messagePluginClient{}), "openai", "thread-1", "/workspace"); intercept != nil {
		t.Fatal("expected nil interceptor for host without chat request hook")
	}
	if intercept := pluginRequestInterceptor(pluginhost.New(pluginhost.Failed("broken", errors.New("boom"))), "openai", "thread-1", "/workspace"); intercept != nil {
		t.Fatal("expected nil interceptor for failed plugin host")
	}
}

func TestStartPluginHostPreservesRuntimeFailure(t *testing.T) {
	host := startPluginHost([]pluginpkg.Plugin{{
		Manifest: pluginpkg.Manifest{ID: "broken", Runtime: &pluginpkg.RuntimeSpec{
			Protocol: pluginhost.ProtocolName,
			Command:  "/definitely/not/a/wuu-plugin",
		}},
		Root: t.TempDir(),
	}}, t.TempDir(), t.TempDir(), nil)
	statuses := host.Statuses()
	if len(statuses) != 1 || statuses[0].State != pluginhost.StateFailed || !strings.Contains(statuses[0].Error, "start plugin") {
		t.Fatalf("statuses = %+v", statuses)
	}
}

type closeTrackingPluginClient struct {
	id     string
	state  pluginhost.State
	closed bool
}

func (c *closeTrackingPluginClient) ID() string               { return c.id }
func (c *closeTrackingPluginClient) Hooks() []pluginhost.Hook { return nil }
func (c *closeTrackingPluginClient) Status() pluginhost.Status {
	state := c.state
	if state == "" {
		state = pluginhost.StateActive
	}
	return pluginhost.Status{ID: c.id, State: state}
}
func (c *closeTrackingPluginClient) Close(context.Context) error { c.closed = true; return nil }
func (c *closeTrackingPluginClient) Invoke(context.Context, pluginhost.InvokeParams) (pluginhost.InvokeResult, error) {
	return pluginhost.InvokeResult{}, nil
}

func TestReconcilePluginHostKeepsPreviousClientsWhenCandidateFails(t *testing.T) {
	stable := pluginpkg.Plugin{
		Manifest: pluginpkg.Manifest{ID: "stable", Runtime: &pluginpkg.RuntimeSpec{
			Protocol: pluginhost.ProtocolName,
			Command:  "/stable/runtime",
		}},
		Fingerprint: "stable-fingerprint",
		Root:        t.TempDir(),
	}
	broken := pluginpkg.Plugin{
		Manifest: pluginpkg.Manifest{ID: "broken", Runtime: &pluginpkg.RuntimeSpec{
			Protocol: pluginhost.ProtocolName,
			Command:  "/definitely/not/a/wuu-plugin",
		}},
		Fingerprint: "broken-fingerprint",
		Root:        t.TempDir(),
	}
	stableClient := &closeTrackingPluginClient{id: stable.ID}
	host := pluginhost.New(stableClient)

	err := reconcilePluginHost(host, []pluginpkg.Plugin{stable}, []pluginpkg.Plugin{stable, broken}, t.TempDir(), t.TempDir(), nil)
	if err == nil || !strings.Contains(err.Error(), `prepare plugin "broken"`) {
		t.Fatalf("reconcile error = %v", err)
	}
	clients := host.Clients()
	if len(clients) != 1 || clients[0] != stableClient {
		t.Fatalf("host clients changed after failed candidate: %+v", clients)
	}
	if stableClient.closed {
		t.Fatal("stable client was closed after failed candidate")
	}
}

func TestReconcilePluginHostRetriesFailedUnchangedClient(t *testing.T) {
	item := pluginpkg.Plugin{
		Manifest: pluginpkg.Manifest{ID: "recovering", Runtime: &pluginpkg.RuntimeSpec{
			Protocol: pluginhost.ProtocolName,
			Command:  "/runtime",
		}},
		Fingerprint: "same-fingerprint",
	}
	failed := &closeTrackingPluginClient{id: item.ID, state: pluginhost.StateFailed}
	replacement := &closeTrackingPluginClient{id: item.ID}
	host := pluginhost.New(failed)
	starts := 0

	err := reconcilePluginHostWithStarter(host, []pluginpkg.Plugin{item}, []pluginpkg.Plugin{item}, "", "", func(got pluginpkg.Plugin, _, _ string) (pluginhost.Client, error) {
		starts++
		if got.ID != item.ID {
			t.Fatalf("started plugin = %q, want %q", got.ID, item.ID)
		}
		return replacement, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if starts != 1 {
		t.Fatalf("start attempts = %d, want 1", starts)
	}
	clients := host.Clients()
	if len(clients) != 1 || clients[0] != replacement {
		t.Fatalf("host clients = %+v, want recovered client", clients)
	}
	if !failed.closed {
		t.Fatal("replaced failed client was not closed")
	}
}

func TestReconcilePluginHostKeepsFailedClientWhenRetryStillFails(t *testing.T) {
	item := pluginpkg.Plugin{
		Manifest:    pluginpkg.Manifest{ID: "still-broken", Runtime: &pluginpkg.RuntimeSpec{Protocol: pluginhost.ProtocolName, Command: "/runtime"}},
		Fingerprint: "same-fingerprint",
	}
	failed := &closeTrackingPluginClient{id: item.ID, state: pluginhost.StateFailed}
	host := pluginhost.New(failed)

	err := reconcilePluginHostWithStarter(host, []pluginpkg.Plugin{item}, []pluginpkg.Plugin{item}, "", "", func(pluginpkg.Plugin, string, string) (pluginhost.Client, error) {
		return nil, errors.New("still unavailable")
	})
	if err != nil {
		t.Fatalf("opportunistic retry blocked policy update: %v", err)
	}
	clients := host.Clients()
	if len(clients) != 1 || clients[0] != failed || failed.closed {
		t.Fatalf("failed diagnostic client was not preserved: %+v", clients)
	}
}

type messagePluginClient struct{ input pluginhost.ChatMessageInput }

func (c *messagePluginClient) ID() string { return "message-plugin" }
func (c *messagePluginClient) Hooks() []pluginhost.Hook {
	return []pluginhost.Hook{pluginhost.HookChatMessage}
}
func (c *messagePluginClient) Status() pluginhost.Status {
	return pluginhost.Status{ID: c.ID(), State: pluginhost.StateActive, Hooks: c.Hooks()}
}
func (c *messagePluginClient) Close(context.Context) error { return nil }
func (c *messagePluginClient) Invoke(_ context.Context, params pluginhost.InvokeParams) (pluginhost.InvokeResult, error) {
	if err := json.Unmarshal(params.Input, &c.input); err != nil {
		return pluginhost.InvokeResult{}, err
	}
	var output pluginhost.ChatMessageOutput
	if err := json.Unmarshal(params.Output, &output); err != nil {
		return pluginhost.InvokeResult{}, err
	}
	output.Content = "[plugin] " + output.Content
	data, err := json.Marshal(output)
	return pluginhost.InvokeResult{Output: data}, err
}

func TestTransformUserMessageUsesPluginOutput(t *testing.T) {
	client := &messagePluginClient{}
	session := &Session{RootDir: "/root", PluginHost: pluginhost.New(client)}
	message, err := session.TransformUserMessage(context.Background(), "thread-1", "/thread", providers.ChatMessage{Role: "user", Content: "hello"})
	if err != nil {
		t.Fatal(err)
	}
	if message.Content != "[plugin] hello" {
		t.Fatalf("message = %+v", message)
	}
	if client.input.ThreadID != "thread-1" || client.input.CWD != "/thread" {
		t.Fatalf("input = %+v", client.input)
	}
}

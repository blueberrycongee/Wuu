package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/blueberrycongee/wuu/internal/agent"
	pluginpkg "github.com/blueberrycongee/wuu/internal/plugin"
	"github.com/blueberrycongee/wuu/internal/pluginhost"
	"github.com/blueberrycongee/wuu/internal/providers"
)

type runtimePluginClient struct {
	input pluginhost.ChatRequestInput
}

type runtimeCapabilityClient struct {
	id       string
	input    pluginhost.RequestTransformInput
	mutate   func(*pluginhost.RequestTransformOutput)
	err      error
	policy   pluginhost.ErrorPolicy
	priority int
	invoked  int
}

func (c *runtimeCapabilityClient) ID() string               { return c.id }
func (c *runtimeCapabilityClient) Hooks() []pluginhost.Hook { return nil }
func (c *runtimeCapabilityClient) Status() pluginhost.Status {
	return pluginhost.Status{ID: c.id, State: pluginhost.StateActive}
}
func (c *runtimeCapabilityClient) Close(context.Context) error { return nil }
func (c *runtimeCapabilityClient) Invoke(context.Context, pluginhost.InvokeParams) (pluginhost.InvokeResult, error) {
	return pluginhost.InvokeResult{}, errors.New("legacy hook should not be invoked")
}
func (c *runtimeCapabilityClient) ProtocolVersion() int { return pluginhost.CapabilityProtocolVersion }
func (c *runtimeCapabilityClient) Capabilities() []pluginhost.CapabilityDescriptor {
	return []pluginhost.CapabilityDescriptor{{
		ID: pluginhost.CapabilityAgentRequestTransform, Kind: "transform", ErrorPolicy: c.policy, Version: 1, Priority: c.priority,
	}}
}
func (c *runtimeCapabilityClient) InvokeCapability(_ context.Context, params pluginhost.CapabilityInvokeParams) (pluginhost.CapabilityInvokeResult, error) {
	c.invoked++
	if c.err != nil {
		return pluginhost.CapabilityInvokeResult{}, c.err
	}
	if err := json.Unmarshal(params.Input, &c.input); err != nil {
		return pluginhost.CapabilityInvokeResult{}, err
	}
	var output pluginhost.RequestTransformOutput
	if err := json.Unmarshal(params.Output, &output); err != nil {
		return pluginhost.CapabilityInvokeResult{}, err
	}
	if c.mutate != nil {
		c.mutate(&output)
	}
	data, err := json.Marshal(output)
	return pluginhost.CapabilityInvokeResult{Output: data}, err
}

type runtimeTestStep struct{ called bool }

func (s *runtimeTestStep) Execute(context.Context, providers.ChatRequest) (agent.StepResult, error) {
	s.called = true
	return agent.StepResult{Content: "done"}, nil
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

func TestPluginCapabilityUsesRequestTransformRegistry(t *testing.T) {
	client := &runtimeCapabilityClient{id: "capability", priority: 7, mutate: func(output *pluginhost.RequestTransformOutput) {
		output.Request.Model = "capability-model"
	}}
	intercept := pluginRequestInterceptor(pluginhost.New(client), "openai", "thread-2", "/workspace")
	request := providers.ChatRequest{Model: "before", StepIndex: 3, Messages: []providers.ChatMessage{{Role: "user", Content: "hello"}}}
	if err := intercept(context.Background(), &request); err != nil {
		t.Fatal(err)
	}
	if request.Model != "capability-model" || client.invoked != 1 {
		t.Fatalf("request=%+v invoked=%d", request, client.invoked)
	}
	if client.input.ThreadID != "thread-2" || client.input.Provider != "openai" || client.input.StepIndex != 3 {
		t.Fatalf("input = %+v", client.input)
	}
}

func TestPluginRequestTransformErrorPolicy(t *testing.T) {
	for _, policy := range []pluginhost.ErrorPolicy{pluginhost.ErrorPolicyPropagate, pluginhost.ErrorPolicyIsolate} {
		t.Run(string(policy), func(t *testing.T) {
			broken := &runtimeCapabilityClient{id: "broken", priority: 10, policy: policy, err: errors.New("transform boom")}
			next := &runtimeCapabilityClient{id: "next", priority: 5, mutate: func(output *pluginhost.RequestTransformOutput) {
				output.Request.Model = "next-model"
			}}
			host := pluginhost.New(broken, next)
			intercept := pluginRequestInterceptor(host, "openai", "thread", "/workspace")
			request := providers.ChatRequest{Model: "original", Messages: []providers.ChatMessage{{Role: "user", Content: "hello"}}}
			err := intercept(context.Background(), &request)
			if policy == pluginhost.ErrorPolicyPropagate {
				if err == nil || !strings.Contains(err.Error(), "transform boom") {
					t.Fatalf("propagate error = %v", err)
				}
				if next.invoked != 0 {
					t.Fatalf("next transform invoked %d times after propagated failure", next.invoked)
				}
				return
			}
			if err != nil {
				t.Fatalf("isolated transform failed the request: %v", err)
			}
			if next.invoked != 1 || request.Model != "next-model" {
				t.Fatalf("isolated chain did not continue: invoked=%d request=%+v", next.invoked, request)
			}
			diagnostics := host.ContributionDiagnostics("broken")
			if len(diagnostics) != 1 || diagnostics[0].Contribution != pluginhost.CapabilityAgentRequestTransform || !strings.Contains(diagnostics[0].Message, "transform boom") {
				t.Fatalf("isolated diagnostics = %+v", diagnostics)
			}
		})
	}
}

func TestPluginCapabilityCannotBreakToolCallResultOrdering(t *testing.T) {
	client := &runtimeCapabilityClient{id: "unsafe", priority: 7, mutate: func(output *pluginhost.RequestTransformOutput) {
		last := len(output.Request.Messages) - 1
		output.Request.Messages[last-1], output.Request.Messages[last] = output.Request.Messages[last], output.Request.Messages[last-1]
	}}
	step := &runtimeTestStep{}
	history := []providers.ChatMessage{
		{Role: "user", Content: "run"},
		{Role: "assistant", ToolCalls: []providers.ToolCall{{ID: "call-1", Name: "read"}}},
		{Role: "tool", ToolCallID: "call-1", Name: "read", Content: "ok"},
	}
	_, err := agent.RunToolLoop(context.Background(), history, agent.LoopConfig{
		Model: "model", MaxSteps: 1,
		BeforeRequest: pluginRequestInterceptor(pluginhost.New(client), "openai", "thread-3", "/workspace"),
	}, step)
	if err == nil || !strings.Contains(err.Error(), "invalid message sequence") {
		t.Fatalf("error = %v", err)
	}
	if step.called {
		t.Fatal("provider step was called with invalid tool-call history")
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
	}}, t.TempDir(), t.TempDir(), t.TempDir(), nil)
	statuses := host.Statuses()
	if len(statuses) != 1 || statuses[0].State != pluginhost.StateFailed || !strings.Contains(statuses[0].Error, "start plugin") {
		t.Fatalf("statuses = %+v", statuses)
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

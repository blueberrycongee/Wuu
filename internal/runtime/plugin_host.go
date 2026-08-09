package runtime

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/blueberrycongee/wuu/internal/agent"
	pluginpkg "github.com/blueberrycongee/wuu/internal/plugin"
	"github.com/blueberrycongee/wuu/internal/pluginhost"
	"github.com/blueberrycongee/wuu/internal/providers"
)

type pluginClientStarter func(context.Context, pluginhost.ProcessConfig) (pluginhost.Client, error)

func startPluginClient(ctx context.Context, cfg pluginhost.ProcessConfig) (pluginhost.Client, error) {
	return pluginhost.Start(ctx, cfg)
}

func startPluginHost(plugins []pluginpkg.Plugin, projectRoot, wuuHome, workspaceStateDir string, turnRouter *PluginSessionRouter) *pluginhost.Host {
	host, err := buildPluginHost(plugins, projectRoot, wuuHome, workspaceStateDir, nil, startPluginClient, turnRouter)
	if err != nil {
		return pluginhost.New(pluginhost.Failed("capability-negotiation", err))
	}
	if err := host.Activate(context.Background()); err != nil {
		providers.DebugLogf("activate initial plugin generation: %v", err)
	}
	return host
}

func buildPluginHost(plugins []pluginpkg.Plugin, projectRoot, wuuHome, workspaceStateDir string, required map[string]bool, start pluginClientStarter, turnRouter *PluginSessionRouter) (*pluginhost.Host, error) {
	host := pluginhost.New()
	for _, item := range plugins {
		if item.Runtime == nil {
			continue
		}
		timeout := time.Duration(item.Runtime.Timeout) * time.Second
		client, err := start(context.Background(), pluginhost.ProcessConfig{
			ID:                 item.ID,
			Command:            item.Runtime.Command,
			Args:               item.Runtime.Args,
			Env:                item.Runtime.Env,
			PluginRoot:         item.Root,
			ProjectRoot:        projectRoot,
			WuuHome:            wuuHome,
			WorkspaceStateDir:  workspaceStateDir,
			Timeout:            timeout,
			HostServiceHandler: newPluginHostServices(item, projectRoot, wuuHome, turnRouter),
			PrepareOnly:        true,
		})
		if err != nil {
			host.Add(pluginhost.Failed(item.ID, err))
			if required[item.ID] || pluginhost.IsCapabilityNegotiationError(err) {
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				closeErr := host.Close(ctx)
				cancel()
				return nil, pluginActivationError(item.ID, err, closeErr)
			}
			continue
		}
		host.Add(client)
	}
	if err := host.ValidateCapabilities(); err != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		closeErr := host.Close(ctx)
		cancel()
		return nil, errors.Join(err, closeErr)
	}
	return host, nil
}

func pluginActivationError(id string, startErr, closeErr error) error {
	err := fmt.Errorf("activate plugin %q: %w", id, startErr)
	if closeErr != nil {
		return fmt.Errorf("%w (close candidate generation: %v)", err, closeErr)
	}
	return err
}

func pluginRequestInterceptor(host *pluginhost.Host, provider, threadID, cwd string) func(context.Context, *providers.ChatRequest) error {
	return pluginRequestInterceptorWithTransforms(host, buildPluginRequestTransforms(host, provider, threadID, cwd), provider, threadID, cwd)
}

func pluginRequestInterceptorWithTransforms(host *pluginhost.Host, transforms *agent.RequestTransformChain, provider, threadID, cwd string) func(context.Context, *providers.ChatRequest) error {
	if transforms == nil || transforms.Count() == 0 {
		return nil
	}
	return func(ctx context.Context, request *providers.ChatRequest) error {
		if request == nil {
			return nil
		}
		return transforms.Apply(ctx, request, nil)
	}
}

func buildPluginRequestTransforms(host *pluginhost.Host, provider, threadID, cwd string) *agent.RequestTransformChain {
	chain := agent.NewRequestTransformChain()
	if host == nil {
		return chain
	}
	for _, registered := range host.Capabilities(pluginhost.CapabilityAgentRequestTransform) {
		capability := registered
		key := capability.PluginID + ":" + capability.Descriptor.ID
		chain.AddWithOwner(agent.NewRequestTransform(key, func(ctx context.Context, request *providers.ChatRequest) error {
			output := pluginhost.RequestTransformOutput{}
			if err := host.InvokeCapability(ctx, capability, pluginhost.RequestTransformInput{
				SessionID: threadID,
				ThreadID:  threadID,
				CWD:       cwd,
				Provider:  provider,
				StepIndex: request.StepIndex,
				Request:   modelRequestViewV1(request),
			}, &output); err != nil {
				return host.HandleCapabilityError(capability, err)
			}
			if err := applyRequestTransformPatch(request, output); err != nil {
				return host.HandleCapabilityError(capability, fmt.Errorf("plugin %q request transform patch: %w", capability.PluginID, err))
			}
			return nil
		}, capability.Descriptor.Priority), capability.PluginID)
	}
	return chain
}

func modelRequestViewV1(request *providers.ChatRequest) pluginhost.ModelRequestViewV1 {
	view := pluginhost.ModelRequestViewV1{Version: 1}
	if request == nil {
		return view
	}
	view.Model = request.Model
	view.Temperature = request.Temperature
	view.MaxTokens = request.MaxTokens
	view.Effort = request.Effort
	view.NativeDeferredToolDiscovery = request.NativeDeferredToolDiscovery
	view.ForceToolName = request.ForceToolName
	view.Messages = make([]pluginhost.ModelMessageViewV1, 0, len(request.Messages))
	for _, message := range request.Messages {
		item := pluginhost.ModelMessageViewV1{
			Role: message.Role, Name: message.Name, Content: message.Content, Hidden: message.Hidden,
			HasImages: len(message.Images) != 0, HasFiles: len(message.Files) != 0,
			ToolCallID: message.ToolCallID, HasToolResult: message.ToolResult != nil,
		}
		for _, call := range message.ToolCalls {
			item.ToolCalls = append(item.ToolCalls, pluginhost.ModelToolCallViewV1{
				ID: call.ID, Name: call.Name, Arguments: call.Arguments, Kind: string(call.Kind),
			})
		}
		for _, tool := range message.DiscoveredTools {
			item.DiscoveredTools = append(item.DiscoveredTools, tool.Name)
		}
		view.Messages = append(view.Messages, item)
	}
	view.Tools = make([]pluginhost.ModelToolViewV1, 0, len(request.Tools))
	for _, tool := range request.Tools {
		view.Tools = append(view.Tools, pluginhost.ModelToolViewV1{
			Name: tool.Name, Description: tool.Description, InputSchema: tool.InputSchema, DeferLoading: tool.DeferLoading,
		})
	}
	return view
}

func applyRequestTransformPatch(request *providers.ChatRequest, patch pluginhost.RequestTransformOutput) error {
	if request == nil || len(patch.PrependSystemMessages) == 0 {
		return nil
	}
	if len(patch.PrependSystemMessages) > 16 {
		return errors.New("prepend_system_messages exceeds 16 entries")
	}
	prefix := make([]providers.ChatMessage, 0, len(patch.PrependSystemMessages))
	totalBytes := 0
	for _, content := range patch.PrependSystemMessages {
		content = strings.TrimSpace(content)
		if content == "" {
			return errors.New("prepend_system_messages contains an empty message")
		}
		totalBytes += len([]byte(content))
		if totalBytes > 64*1024 {
			return errors.New("prepend_system_messages exceeds 65536 bytes")
		}
		prefix = append(prefix, providers.ChatMessage{Role: "system", Content: content, Hidden: true})
	}
	request.Messages = append(prefix, request.Messages...)
	return nil
}

func buildPluginAgentCapabilities(ctx context.Context, host *pluginhost.Host, provider, model, cwd string) (*agent.SystemPromptAssembler, *agent.CompactionRegistry, error) {
	prompts := agent.NewSystemPromptAssembler()
	compactions := agent.NewCompactionRegistry()
	if host == nil {
		return prompts, compactions, nil
	}
	for _, registered := range host.Capabilities(pluginhost.CapabilityAgentSystemPromptSection) {
		output := pluginhost.SystemPromptSectionOutput{}
		if err := host.InvokeCapability(ctx, registered, pluginhost.SystemPromptSectionInput{
			CWD: cwd, Provider: provider, Model: model,
		}, &output); err != nil {
			if policyErr := host.HandleCapabilityError(registered, err); policyErr != nil {
				return nil, nil, policyErr
			}
			continue
		}
		key := registered.PluginID + ":" + registered.Descriptor.ID
		prompts.AddWithOwner(agent.NewStaticPromptSection(key, output.Text, registered.Descriptor.Priority), registered.PluginID)
	}
	for _, registered := range host.Capabilities(pluginhost.CapabilityAgentCompaction) {
		capability := registered
		key := capability.PluginID + ":" + capability.Descriptor.ID
		compactions.RegisterWithOwner(&pluginCompactionProvider{
			key: key, priority: capability.Descriptor.Priority, host: host, capability: capability,
		}, capability.PluginID)
	}
	return prompts, compactions, nil
}

type pluginCompactionProvider struct {
	key        string
	priority   int
	host       *pluginhost.Host
	capability pluginhost.RegisteredCapability
}

func (p *pluginCompactionProvider) CompactionKey() string   { return p.key }
func (p *pluginCompactionProvider) CompactionPriority() int { return p.priority }
func (p *pluginCompactionProvider) Compact(ctx context.Context, model string, messages []providers.ChatMessage) ([]providers.ChatMessage, error) {
	output := pluginhost.CompactionOutput{}
	if err := p.host.InvokeCapability(ctx, p.capability, pluginhost.CompactionInput{
		Model: model, Messages: providers.CloneChatMessages(messages),
	}, &output); err != nil {
		if policyErr := p.host.HandleCapabilityError(p.capability, err); policyErr != nil {
			return nil, policyErr
		}
		return providers.CloneChatMessages(messages), nil
	}
	compacted := providers.CloneChatMessages(output.Messages)
	if err := providers.ValidateToolCallHistory(compacted); err != nil {
		return nil, fmt.Errorf("plugin compaction returned invalid tool-call history: %w", err)
	}
	return compacted, nil
}

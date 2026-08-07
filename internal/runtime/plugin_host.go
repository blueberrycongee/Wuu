package runtime

import (
	"context"
	"errors"
	"fmt"
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

func startPluginHost(plugins []pluginpkg.Plugin, projectRoot, wuuHome string, childSession ...childSessionRequestHandler) *pluginhost.Host {
	host, err := buildPluginHost(plugins, projectRoot, wuuHome, nil, startPluginClient, childSession...)
	if err != nil {
		return pluginhost.New(pluginhost.Failed("capability-negotiation", err))
	}
	return host
}

func buildPluginHost(plugins []pluginpkg.Plugin, projectRoot, wuuHome string, required map[string]bool, start pluginClientStarter, childSession ...childSessionRequestHandler) (*pluginhost.Host, error) {
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
			Timeout:            timeout,
			HostServiceHandler: newPluginHostServices(item, projectRoot, wuuHome, childSession...),
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
	hasLegacyHook := host != nil && host.HasHook(pluginhost.HookChatRequest)
	if !hasLegacyHook && (transforms == nil || transforms.Count() == 0) {
		return nil
	}
	return func(ctx context.Context, request *providers.ChatRequest) error {
		if request == nil {
			return nil
		}
		if hasLegacyHook {
			output := pluginhost.ChatRequestOutput{
				Model:                       request.Model,
				Messages:                    append([]providers.ChatMessage(nil), request.Messages...),
				Tools:                       append([]providers.ToolDefinition(nil), request.Tools...),
				Temperature:                 request.Temperature,
				MaxTokens:                   request.MaxTokens,
				Effort:                      request.Effort,
				NativeDeferredToolDiscovery: request.NativeDeferredToolDiscovery,
				ForceToolName:               request.ForceToolName,
			}
			if request.ProviderOptions != nil {
				output.ProviderOptions = make(map[string]any, len(request.ProviderOptions))
				for key, value := range request.ProviderOptions {
					output.ProviderOptions[key] = value
				}
			}
			if err := host.Run(ctx, pluginhost.HookChatRequest, pluginhost.ChatRequestInput{
				SessionID: threadID,
				ThreadID:  threadID,
				CWD:       cwd,
				Provider:  provider,
				StepIndex: request.StepIndex,
			}, &output); err != nil {
				return err
			}
			request.Model = output.Model
			request.Messages = output.Messages
			request.Tools = output.Tools
			request.Temperature = output.Temperature
			request.MaxTokens = output.MaxTokens
			request.Effort = output.Effort
			request.ProviderOptions = output.ProviderOptions
			request.NativeDeferredToolDiscovery = output.NativeDeferredToolDiscovery
			request.ForceToolName = output.ForceToolName
		}
		if transforms != nil {
			return transforms.Apply(ctx, request, nil)
		}
		return nil
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
			output := pluginhost.RequestTransformOutput{Request: *request}
			if err := host.InvokeCapability(ctx, capability, pluginhost.RequestTransformInput{
				SessionID: threadID,
				ThreadID:  threadID,
				CWD:       cwd,
				Provider:  provider,
				StepIndex: request.StepIndex,
			}, &output); err != nil {
				return host.HandleCapabilityError(capability, err)
			}
			*request = output.Request
			return nil
		}, capability.Descriptor.Priority), capability.PluginID)
	}
	return chain
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

// TransformUserMessage runs before app-server or CLI persistence so the user,
// UI history, durable history, and model all observe the same plugin output.
func (s *Session) TransformUserMessage(ctx context.Context, threadID, cwd string, message providers.ChatMessage) (providers.ChatMessage, error) {
	if s == nil || s.PluginHost == nil {
		return message, nil
	}
	output := pluginhost.ChatMessageOutput{
		Content:        message.Content,
		DisplayContent: message.DisplayContent,
		Images:         append([]providers.InputImage(nil), message.Images...),
		Files:          append([]providers.InputFile(nil), message.Files...),
	}
	if err := s.PluginHost.Run(ctx, pluginhost.HookChatMessage, pluginhost.ChatMessageInput{
		SessionID: threadID,
		ThreadID:  threadID,
		CWD:       firstNonEmptyString(cwd, s.RootDir),
	}, &output); err != nil {
		return providers.ChatMessage{}, err
	}
	message.Content = output.Content
	message.DisplayContent = output.DisplayContent
	message.Images = output.Images
	message.Files = output.Files
	return message, nil
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

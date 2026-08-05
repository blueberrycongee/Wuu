package runtime

import (
	"context"
	"fmt"
	"time"

	pluginpkg "github.com/blueberrycongee/wuu/internal/plugin"
	"github.com/blueberrycongee/wuu/internal/pluginhost"
	"github.com/blueberrycongee/wuu/internal/providers"
)

type pluginClientStarter func(context.Context, pluginhost.ProcessConfig) (pluginhost.Client, error)

func startPluginClient(ctx context.Context, cfg pluginhost.ProcessConfig) (pluginhost.Client, error) {
	return pluginhost.Start(ctx, cfg)
}

func startPluginHost(plugins []pluginpkg.Plugin, projectRoot, wuuHome string) *pluginhost.Host {
	host, _ := buildPluginHost(plugins, projectRoot, wuuHome, nil, startPluginClient)
	return host
}

func buildPluginHost(plugins []pluginpkg.Plugin, projectRoot, wuuHome string, required map[string]bool, start pluginClientStarter) (*pluginhost.Host, error) {
	host := pluginhost.New()
	for _, item := range plugins {
		if item.Runtime == nil {
			continue
		}
		timeout := time.Duration(item.Runtime.Timeout) * time.Second
		client, err := start(context.Background(), pluginhost.ProcessConfig{
			ID:          item.ID,
			Command:     item.Runtime.Command,
			Args:        item.Runtime.Args,
			Env:         item.Runtime.Env,
			PluginRoot:  item.Root,
			ProjectRoot: projectRoot,
			WuuHome:     wuuHome,
			Timeout:     timeout,
		})
		if err != nil {
			host.Add(pluginhost.Failed(item.ID, err))
			if required[item.ID] {
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				closeErr := host.Close(ctx)
				cancel()
				return nil, pluginActivationError(item.ID, err, closeErr)
			}
			continue
		}
		host.Add(client)
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
	if host == nil || !host.HasHook(pluginhost.HookChatRequest) {
		return nil
	}
	return func(ctx context.Context, request *providers.ChatRequest) error {
		if request == nil {
			return nil
		}
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
		return nil
	}
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

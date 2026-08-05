package runtime

import (
	"context"
	"errors"
	"fmt"
	"time"

	pluginpkg "github.com/blueberrycongee/wuu/internal/plugin"
	"github.com/blueberrycongee/wuu/internal/pluginhost"
	"github.com/blueberrycongee/wuu/internal/providers"
)

// GrantedPermissionsResolver returns the exact permission set the user has
// granted for one plugin. A nil result means nothing was granted (fail closed:
// every permission-gated protocol hook will be stripped at initialize).
type GrantedPermissionsResolver func(pluginpkg.Plugin) []string

func startPluginHost(plugins []pluginpkg.Plugin, projectRoot, wuuHome string, granted GrantedPermissionsResolver) *pluginhost.Host {
	host := pluginhost.New()
	for _, item := range plugins {
		if item.Runtime == nil {
			continue
		}
		client, err := startPluginClient(item, projectRoot, wuuHome, resolveGrantedPermissions(granted, item))
		if err != nil {
			host.Add(pluginhost.Failed(item.ID, err))
			continue
		}
		host.Add(client)
	}
	return host
}

func resolveGrantedPermissions(resolver GrantedPermissionsResolver, item pluginpkg.Plugin) []string {
	if resolver == nil {
		return nil
	}
	return resolver(item)
}

func startPluginClient(item pluginpkg.Plugin, projectRoot, wuuHome string, granted []string) (pluginhost.Client, error) {
	if item.Runtime == nil {
		return nil, nil
	}
	timeout := time.Duration(item.Runtime.Timeout) * time.Second
	return pluginhost.Start(context.Background(), pluginhost.ProcessConfig{
		ID:                 item.ID,
		Command:            item.Runtime.Command,
		Args:               item.Runtime.Args,
		Env:                item.Runtime.Env,
		PluginRoot:         item.Root,
		GrantedPermissions: granted,
		ProjectRoot:        projectRoot,
		WuuHome:            wuuHome,
		Timeout:            timeout,
	})
}

// reconcilePluginHost prepares every added or changed runtime before
// publishing the replacement client order. Unchanged clients are reused, so an
// unrelated policy mutation does not restart long-lived plugin processes. If a
// candidate fails to initialize, newly started candidates are closed and the
// previous host remains untouched.
func reconcilePluginHost(host *pluginhost.Host, previous, next []pluginpkg.Plugin, projectRoot, wuuHome string, granted GrantedPermissionsResolver) error {
	starter := func(item pluginpkg.Plugin, root, home string) (pluginhost.Client, error) {
		return startPluginClient(item, root, home, resolveGrantedPermissions(granted, item))
	}
	return reconcilePluginHostWithStarter(host, previous, next, projectRoot, wuuHome, starter)
}

func reconcilePluginHostWithStarter(
	host *pluginhost.Host,
	previous, next []pluginpkg.Plugin,
	projectRoot, wuuHome string,
	start func(pluginpkg.Plugin, string, string) (pluginhost.Client, error),
) error {
	if host == nil {
		return errors.New("plugin host is not initialized")
	}
	previousFingerprints := make(map[string]string, len(previous))
	for _, item := range previous {
		if item.Runtime != nil {
			previousFingerprints[item.ID] = item.Fingerprint
		}
	}
	currentByID := make(map[string]pluginhost.Client)
	for _, client := range host.Clients() {
		currentByID[client.ID()] = client
	}

	nextClients := make([]pluginhost.Client, 0, len(next))
	reused := make(map[string]struct{})
	var started []pluginhost.Client
	for _, item := range next {
		if item.Runtime == nil {
			continue
		}
		if previousFingerprints[item.ID] == item.Fingerprint {
			if client := currentByID[item.ID]; client != nil {
				state := client.Status().State
				if state != pluginhost.StateFailed && state != pluginhost.StateStopped {
					nextClients = append(nextClients, client)
					reused[item.ID] = struct{}{}
					continue
				}
				// Retry a dead unchanged runtime opportunistically. If it still cannot
				// start, retain its diagnostic client rather than making an unrelated
				// policy update fail.
				candidate, err := start(item, projectRoot, wuuHome)
				if err != nil {
					nextClients = append(nextClients, client)
					reused[item.ID] = struct{}{}
					continue
				}
				nextClients = append(nextClients, candidate)
				started = append(started, candidate)
				continue
			}
		}
		client, err := start(item, projectRoot, wuuHome)
		if err != nil {
			closePluginClients(started)
			return fmt.Errorf("prepare plugin %q: %w", item.ID, err)
		}
		nextClients = append(nextClients, client)
		started = append(started, client)
	}

	previousClients := host.Replace(nextClients)
	removed := make([]pluginhost.Client, 0, len(previousClients))
	for _, client := range previousClients {
		if _, ok := reused[client.ID()]; !ok {
			removed = append(removed, client)
		}
	}
	closePluginClients(removed)
	return nil
}

func closePluginClients(clients []pluginhost.Client) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	for index := len(clients) - 1; index >= 0; index-- {
		_ = clients[index].Close(ctx)
	}
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

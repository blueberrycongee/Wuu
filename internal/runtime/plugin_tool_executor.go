package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/blueberrycongee/wuu/internal/agent"
	"github.com/blueberrycongee/wuu/internal/agentthread"
	"github.com/blueberrycongee/wuu/internal/pluginhost"
	"github.com/blueberrycongee/wuu/internal/providers"
	"github.com/blueberrycongee/wuu/internal/toolctx"
	"github.com/blueberrycongee/wuu/internal/toolerrors"
	"github.com/blueberrycongee/wuu/internal/toolresult"
)

type pluginToolExecutor struct {
	inner    agent.ToolExecutor
	host     *pluginhost.Host
	threadID string
	cwd      string
}

func newPluginToolExecutor(inner agent.ToolExecutor, host *pluginhost.Host, threadID, cwd string) agent.ToolExecutor {
	if inner == nil || host == nil {
		return inner
	}
	return &pluginToolExecutor{inner: inner, host: host, threadID: threadID, cwd: cwd}
}

func (e *pluginToolExecutor) Definitions() []providers.ToolDefinition {
	inner := e.inner.Definitions()
	registered := e.host.ToolDefinitions()
	plugin := make([]providers.ToolDefinition, 0, len(registered))
	for _, definition := range registered {
		if e.pluginToolAllowed(definition.Name) {
			plugin = append(plugin, definition)
		}
	}
	definitions := make([]providers.ToolDefinition, 0, len(inner)+len(plugin))
	definitions = append(definitions, inner...)
	definitions = append(definitions, plugin...)
	return definitions
}

func (e *pluginToolExecutor) Execute(ctx context.Context, call providers.ToolCall) (string, error) {
	result, err := e.ExecuteResult(ctx, call)
	return result.TextProjection(), err
}

func (e *pluginToolExecutor) ExecuteResult(ctx context.Context, call providers.ToolCall) (toolresult.Result, error) {
	input, err := e.toolInput(ctx, call)
	if err != nil {
		return toolresult.Result{}, err
	}
	before := pluginhost.ToolExecuteBeforeOutput{Arguments: append(json.RawMessage(nil), input.Arguments...)}
	if err := e.host.Run(ctx, pluginhost.HookToolExecuteBefore, input, &before); err != nil {
		return toolresult.Result{}, err
	}
	if len(before.Arguments) == 0 || !json.Valid(before.Arguments) {
		return toolresult.Result{}, fmt.Errorf("plugin tool.execute.before returned invalid arguments for %q", call.Name)
	}
	call.Arguments = string(before.Arguments)
	input.Arguments = append(input.Arguments[:0], before.Arguments...)

	var result toolresult.Result
	var executeErr error
	if e.host.SupportsTool(call.Name) && !e.pluginToolAllowed(call.Name) {
		return toolresult.Result{}, toolerrors.New("tool_unavailable", fmt.Sprintf("plugin tool %q is unavailable in this execution scope", call.Name))
	}
	if e.host.SupportsTool(call.Name) {
		result, executeErr = e.host.ExecuteTool(ctx, call.Name, input)
	} else if rich, ok := e.inner.(agent.RichToolExecutor); ok {
		result, executeErr = rich.ExecuteResult(ctx, call)
	} else {
		var text string
		text, executeErr = e.inner.Execute(ctx, call)
		result = toolresult.FromText(text)
	}
	if executeErr != nil {
		result.IsError = true
	}
	after := pluginhost.ToolExecuteAfterOutput{Result: result.Clone()}
	if executeErr != nil {
		after.Error = executeErr.Error()
	}
	if err := e.host.Run(ctx, pluginhost.HookToolExecuteAfter, input, &after); err != nil {
		return result, err
	}
	if err := after.Result.Validate(); err != nil {
		return toolresult.Result{}, fmt.Errorf("plugin tool.execute.after returned invalid result for %q: %w", call.Name, err)
	}
	if strings.TrimSpace(after.Error) != "" {
		return after.Result, errors.New(after.Error)
	}
	return after.Result, nil
}

func (e *pluginToolExecutor) toolInput(ctx context.Context, call providers.ToolCall) (pluginhost.ToolExecuteInput, error) {
	arguments := strings.TrimSpace(call.Arguments)
	if arguments == "" {
		arguments = "{}"
	}
	if !json.Valid([]byte(arguments)) {
		return pluginhost.ToolExecuteInput{}, toolerrors.New(
			toolerrors.InvalidArguments,
			fmt.Sprintf("tool %q arguments are invalid JSON", call.Name),
		)
	}
	stepIndex, _ := toolctx.StepIndex(ctx)
	actorID, actorPath := "", ""
	if provider, ok := e.inner.(interface{ ExecutionActor() (string, string) }); ok {
		actorID, actorPath = provider.ExecutionActor()
	}
	return pluginhost.ToolExecuteInput{
		SessionID: e.threadID,
		ThreadID:  e.threadID,
		ActorID:   actorID,
		ActorPath: actorPath,
		CWD:       e.cwd,
		StepIndex: stepIndex,
		CallID:    call.ID,
		Tool:      call.Name,
		Arguments: json.RawMessage(arguments),
	}, nil
}

func (e *pluginToolExecutor) SupportsTool(name string) bool {
	if e.host.SupportsTool(name) {
		return e.pluginToolAllowed(name)
	}
	provider, ok := e.inner.(agent.ToolSupportProvider)
	return ok && provider.SupportsTool(name)
}

func (e *pluginToolExecutor) pluginToolAllowed(name string) bool {
	tool, ok := e.host.Tool(name)
	if !ok || len(tool.Registration.ExecutionScopes) == 0 {
		return ok
	}
	actorPath := ""
	if provider, ok := e.inner.(interface{ ExecutionActor() (string, string) }); ok {
		_, actorPath = provider.ExecutionActor()
	}
	scope := "child"
	if strings.TrimSpace(actorPath) == "" || strings.TrimSpace(actorPath) == agentthread.RootPath {
		scope = "root"
	}
	for _, allowed := range tool.Registration.ExecutionScopes {
		if strings.TrimSpace(allowed) == scope {
			return true
		}
	}
	return false
}

func (e *pluginToolExecutor) ToolMetadata(call providers.ToolCall) (agent.ToolMetadata, bool) {
	if tool, ok := e.host.Tool(call.Name); ok {
		if tool.Registration.Activity == nil {
			return agent.ToolMetadata{}, false
		}
		activity := tool.Registration.Activity
		return agent.ToolMetadata{
			ReadOnly:        activity.ReadOnly,
			ConcurrencySafe: activity.ConcurrencySafe,
			Destructive:     activity.Destructive,
			Risk:            activity.Risk,
			Reason:          activity.Reason,
		}, true
	}
	provider, ok := e.inner.(agent.ToolMetadataProvider)
	if !ok {
		return agent.ToolMetadata{}, false
	}
	return provider.ToolMetadata(call)
}

func (e *pluginToolExecutor) ToolDisplay(call providers.ToolCall) (providers.ToolCallDisplay, bool) {
	if tool, ok := e.host.Tool(call.Name); ok {
		if tool.Registration.Display == nil {
			return providers.ToolCallDisplay{}, false
		}
		return *tool.Registration.Display, true
	}
	provider, ok := e.inner.(agent.ToolDisplayProvider)
	if !ok {
		return providers.ToolCallDisplay{}, false
	}
	return provider.ToolDisplay(call)
}

func (e *pluginToolExecutor) DiscoveredTools(call providers.ToolCall) []providers.LoadableToolDefinition {
	provider, ok := e.inner.(agent.ToolDiscoveryProvider)
	if !ok {
		return nil
	}
	return provider.DiscoveredTools(call)
}

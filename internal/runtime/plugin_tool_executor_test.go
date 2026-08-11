package runtime

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/blueberrycongee/wuu/internal/agent"
	"github.com/blueberrycongee/wuu/internal/pluginhost"
	"github.com/blueberrycongee/wuu/internal/providers"
	"github.com/blueberrycongee/wuu/internal/toolctx"
	"github.com/blueberrycongee/wuu/internal/toolerrors"
	"github.com/blueberrycongee/wuu/internal/toolresult"
)

type recordingToolExecutor struct {
	mu               sync.Mutex
	calls            []providers.ToolCall
	authorized       []providers.ToolCall
	authorizationErr error
}

func (e *recordingToolExecutor) Definitions() []providers.ToolDefinition {
	return []providers.ToolDefinition{{Name: "demo", InputSchema: map[string]any{"type": "object"}}}
}
func (e *recordingToolExecutor) Execute(context.Context, providers.ToolCall) (string, error) {
	panic("rich path expected")
}
func (e *recordingToolExecutor) ExecuteResult(_ context.Context, call providers.ToolCall) (toolresult.Result, error) {
	e.mu.Lock()
	e.calls = append(e.calls, call)
	e.mu.Unlock()
	return toolresult.FromText(call.Arguments), nil
}

func (e *recordingToolExecutor) AuthorizeTool(_ context.Context, call providers.ToolCall, _ agent.ToolMetadata) error {
	e.authorized = append(e.authorized, call)
	return e.authorizationErr
}

type pluginToolTestClient struct {
	executed bool
}

func (c *pluginToolTestClient) ID() string { return "policy-tool" }
func (c *pluginToolTestClient) Status() pluginhost.Status {
	return pluginhost.Status{ID: c.ID(), State: pluginhost.StateActive}
}
func (c *pluginToolTestClient) Close(context.Context) error { return nil }
func (c *pluginToolTestClient) Tools() []pluginhost.ToolRegistration {
	return []pluginhost.ToolRegistration{{ID: "change", Description: "change state", InputSchema: map[string]any{"type": "object"}}}
}
func (c *pluginToolTestClient) ExecuteTool(context.Context, pluginhost.ToolExecuteParams) (pluginhost.ToolExecuteResult, error) {
	c.executed = true
	return pluginhost.ToolExecuteResult{Result: toolresult.FromText("changed")}, nil
}

func TestPluginToolExecutorPreservesArgumentsAndRichResult(t *testing.T) {
	inner := &recordingToolExecutor{}
	executor := newPluginToolExecutor(inner, pluginhost.New(), "thread-1", "/workspace")
	rich := executor.(interface {
		ExecuteResult(context.Context, providers.ToolCall) (toolresult.Result, error)
	})
	result, err := rich.ExecuteResult(toolctx.WithStepIndex(context.Background(), 4), providers.ToolCall{ID: "call-1", Name: "demo", Arguments: `{}`})
	if err != nil {
		t.Fatal(err)
	}
	if result.TextProjection() != `{}` {
		t.Fatalf("result = %q", result.TextProjection())
	}
	if len(inner.calls) != 1 || inner.calls[0].Arguments != `{}` {
		t.Fatalf("calls = %+v", inner.calls)
	}
}

func TestPluginToolExecutorKeepsConcurrentCallsIsolated(t *testing.T) {
	inner := &recordingToolExecutor{}
	executor := newPluginToolExecutor(inner, pluginhost.New(), "thread", "/workspace")
	rich := executor.(interface {
		ExecuteResult(context.Context, providers.ToolCall) (toolresult.Result, error)
	})
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for _, id := range []string{"a", "b"} {
		id := id
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := rich.ExecuteResult(context.Background(), providers.ToolCall{ID: id, Name: "demo", Arguments: `{}`})
			if err != nil {
				errs <- err
				return
			}
			want := `{}`
			if result.TextProjection() != want {
				errs <- fmt.Errorf("call %s result = %q, want %q", id, result.TextProjection(), want)
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
}

func TestPluginToolExecutorRejectsInvalidArgumentsBeforeHooksOrExecution(t *testing.T) {
	inner := &recordingToolExecutor{}
	executor := newPluginToolExecutor(inner, pluginhost.New(), "thread", "/workspace")

	_, err := executor.Execute(context.Background(), providers.ToolCall{
		ID: "call-write", Name: "write_file", Arguments: `{"path":"page.html","content":"truncated`,
	})
	if err == nil {
		t.Fatal("expected invalid arguments error")
	}
	if got := toolerrors.Kind(err); got != toolerrors.InvalidArguments {
		t.Fatalf("error kind = %q, want %q: %v", got, toolerrors.InvalidArguments, err)
	}
	if len(inner.calls) != 0 {
		t.Fatalf("inner executor must not be called, got %+v", inner.calls)
	}
}

func TestPluginToolExecutorAuthorizesBeforeDispatch(t *testing.T) {
	client := &pluginToolTestClient{}
	host := pluginhost.New(client)
	name := host.ToolDefinitions()[0].Name
	inner := &recordingToolExecutor{authorizationErr: fmt.Errorf("policy denied")}
	executor := newPluginToolExecutor(inner, host, "thread", "/workspace")
	_, err := executor.Execute(context.Background(), providers.ToolCall{Name: name, Arguments: `{}`})
	if err == nil || client.executed || len(inner.authorized) != 1 {
		t.Fatalf("error = %v executed = %v authorized = %+v", err, client.executed, inner.authorized)
	}
}

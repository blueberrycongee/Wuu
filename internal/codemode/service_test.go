package codemode

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/blueberrycongee/wuu/internal/providers"
	"github.com/blueberrycongee/wuu/internal/toolctx"
	"github.com/blueberrycongee/wuu/internal/toolresult"
)

func TestToolNameString(t *testing.T) {
	ns := func(value string) *string { return &value }
	cases := []struct {
		name string
		tool ToolName
		want string
	}{
		{name: "plain", tool: ToolName{Name: "read_file"}, want: "read_file"},
		{name: "nil namespace", tool: ToolName{Name: "read_file", Namespace: nil}, want: "read_file"},
		{name: "empty namespace", tool: ToolName{Name: "read_file", Namespace: ns("")}, want: "read_file"},
		{name: "default namespace", tool: ToolName{Name: "read_file", Namespace: ns(DefaultToolNamespace)}, want: "read_file"},
		{name: "underscore suffix", tool: ToolName{Name: "echo", Namespace: ns("tools_")}, want: "tools_echo"},
		{name: "underscore prefix", tool: ToolName{Name: "_echo", Namespace: ns("tools")}, want: "tools_echo"},
		{name: "double underscore", tool: ToolName{Name: "echo", Namespace: ns("mcp")}, want: "mcp__echo"},
	}
	for _, tc := range cases {
		if got := ToolNameString(tc.tool); got != tc.want {
			t.Errorf("%s: ToolNameString(%+v) = %q, want %q", tc.name, tc.tool, got, tc.want)
		}
	}
}

func TestNewServiceValidation(t *testing.T) {
	if _, err := NewService(ServiceConfig{}); err == nil {
		t.Fatal("NewService accepted an empty executable")
	}
	if _, err := NewService(ServiceConfig{Executable: "/tmp/wuu-code-mode-host"}); err == nil {
		t.Fatal("NewService accepted an empty session ID")
	}
}

type recordingExecutor struct {
	mu    sync.Mutex
	calls []providers.ToolCall
}

func (r *recordingExecutor) Invoke(_ context.Context, call providers.ToolCall) (toolresult.Result, error) {
	r.mu.Lock()
	r.calls = append(r.calls, call)
	r.mu.Unlock()
	return toolresult.FromText("ok"), nil
}

func (r *recordingExecutor) snapshot() []providers.ToolCall {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]providers.ToolCall(nil), r.calls...)
}

func newTestService() *Service {
	return &Service{cells: make(map[string]toolctx.NestedExecutor), bound: make(chan struct{})}
}

func TestInvokeRoutesThroughBoundCell(t *testing.T) {
	service := newTestService()
	executor := &recordingExecutor{}
	if err := service.BindCell("cell-1", executor); err != nil {
		t.Fatalf("BindCell: %v", err)
	}
	value, err := service.Invoke(context.Background(), Invocation{
		CellID:            "cell-1",
		RuntimeToolCallID: "call-1",
		ToolName:          ToolName{Name: "mcp", Namespace: strPtr("mcp")},
		Input:             json.RawMessage(`{"value":42}`),
	})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	var result toolresult.Result
	if err := json.Unmarshal(value, &result); err != nil {
		t.Fatalf("result is not a tool result: %v (%s)", err, value)
	}
	calls := executor.snapshot()
	if len(calls) != 1 {
		t.Fatalf("got %d calls, want 1", len(calls))
	}
	call := calls[0]
	if call.ID != "call-1" {
		t.Errorf("call ID = %q, want call-1", call.ID)
	}
	if call.Name != "mcp__mcp" {
		t.Errorf("call name = %q, want mcp__mcp", call.Name)
	}
	if call.Arguments != `{"value":42}` {
		t.Errorf("arguments = %q", call.Arguments)
	}
	if call.Kind != providers.ToolCallKindFunction {
		t.Errorf("kind = %q", call.Kind)
	}
}

func TestInvokeWaitsForExecBinding(t *testing.T) {
	service := newTestService()
	service.mu.Lock()
	service.pendingExec = 1
	service.mu.Unlock()
	executor := &recordingExecutor{}
	done := make(chan error, 1)
	go func() {
		_, err := service.Invoke(context.Background(), Invocation{
			CellID:            "cell-1",
			RuntimeToolCallID: "call-1",
			ToolName:          ToolName{Name: "read_file"},
			Input:             json.RawMessage(`{}`),
		})
		done <- err
	}()
	select {
	case err := <-done:
		t.Fatalf("Invoke completed before the cell was bound: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	if err := service.BindCell("cell-1", executor); err != nil {
		t.Fatalf("BindCell: %v", err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Invoke after binding: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Invoke did not unblock after the cell was bound")
	}
	if calls := executor.snapshot(); len(calls) != 1 || calls[0].Name != "read_file" {
		t.Fatalf("unexpected routed calls: %+v", calls)
	}
}

func TestInvokeFailsWhenNoExecAndNoBinding(t *testing.T) {
	service := newTestService()
	_, err := service.Invoke(context.Background(), Invocation{
		CellID:            "cell-1",
		RuntimeToolCallID: "call-1",
		ToolName:          ToolName{Name: "read_file"},
		Input:             json.RawMessage(`{}`),
	})
	if err == nil || !strings.Contains(err.Error(), "no owning execution scope") {
		t.Fatalf("Invoke error = %v, want no-owning-scope", err)
	}
}

func TestInvokeCancellationWhileWaiting(t *testing.T) {
	service := newTestService()
	service.mu.Lock()
	service.pendingExec = 1
	service.mu.Unlock()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := service.Invoke(ctx, Invocation{
			CellID:            "cell-1",
			RuntimeToolCallID: "call-1",
			ToolName:          ToolName{Name: "read_file"},
			Input:             json.RawMessage(`{}`),
		})
		done <- err
	}()
	time.Sleep(20 * time.Millisecond)
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Invoke error = %v, want context canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Invoke did not unblock on cancellation")
	}
}

func TestBindAndUnbindSemantics(t *testing.T) {
	service := newTestService()
	executor := &recordingExecutor{}
	if err := service.BindCell("", executor); err == nil {
		t.Fatal("BindCell accepted an empty cell ID")
	}
	if err := service.BindCell("cell-1", nil); err == nil {
		t.Fatal("BindCell accepted a nil executor")
	}
	if err := service.BindCell("cell-1", executor); err != nil {
		t.Fatalf("BindCell: %v", err)
	}
	if err := service.BindCell("cell-1", executor); err != nil {
		t.Fatalf("idempotent rebind failed: %v", err)
	}
	other := &recordingExecutor{}
	if err := service.BindCell("cell-1", other); err == nil {
		t.Fatal("BindCell accepted a different executor for a live cell")
	}
	service.unbind("cell-1")
	if got := service.cellScope("cell-1"); got != nil {
		t.Fatalf("cellScope after unbind = %v, want nil", got)
	}
}

func TestCloseWithoutClient(t *testing.T) {
	service := newTestService()
	if err := service.Close(); err != nil {
		t.Fatalf("Close on a fresh service: %v", err)
	}
}

func TestTerminateWithoutClient(t *testing.T) {
	service := newTestService()
	executor := &recordingExecutor{}
	if err := service.BindCell("cell-1", executor); err != nil {
		t.Fatalf("BindCell: %v", err)
	}
	if _, err := service.Terminate(context.Background(), "cell-1"); err != nil {
		t.Fatalf("Terminate without a client: %v", err)
	}
	if got := service.cellScope("cell-1"); got != nil {
		t.Fatal("Terminate did not release the cell binding")
	}
}

func strPtr(value string) *string { return &value }

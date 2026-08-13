package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"

	"github.com/blueberrycongee/wuu/internal/pluginhost"
	"github.com/blueberrycongee/wuu/internal/sessiontrace"
	"github.com/blueberrycongee/wuu/internal/statepath"
)

func TestDataQueryInvokerReadsSessionTrace(t *testing.T) {
	t.Parallel()

	stateDir := t.TempDir()
	threadID := "thread-1"
	artifactDir := statepath.SessionArtifactDir(stateDir, threadID)
	if err := os.MkdirAll(artifactDir, 0o755); err != nil {
		t.Fatal(err)
	}
	tracePath := sessiontrace.Path(artifactDir)
	if err := sessiontrace.AppendTurn(tracePath,
		sessiontrace.TurnRecord{ThreadID: threadID, TurnID: "turn-1", Status: "completed", InputTokens: 12, OutputTokens: 34},
		sessiontrace.FinalRecord{Status: "completed"},
		nil, nil, nil, nil, nil,
	); err != nil {
		t.Fatal(err)
	}

	kernel := newKernelHostServices(nil, nil)
	kernel.bindWorkspaceStateDir(stateDir)
	invoker := &dataQueryInvoker{parent: kernel}

	raw, err := invoker.InvokeService(context.Background(), pluginhost.ServiceInvokeParams{
		Method: pluginhost.KernelServiceMethod,
		Params: json.RawMessage(`{"thread_id":"thread-1"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	var result pluginhost.DataQueryResult
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatal(err)
	}
	if result.Version != pluginhost.DataQueryServiceVersion || result.ThreadID != threadID {
		t.Fatalf("result = %+v", result)
	}
	if len(result.Events) == 0 {
		t.Fatal("query returned no events")
	}
	for _, event := range result.Events {
		if event.ThreadID != threadID {
			t.Fatalf("event thread_id = %q, want %q", event.ThreadID, threadID)
		}
	}
}

func TestDataQueryInvokerMissingTraceReturnsEmpty(t *testing.T) {
	t.Parallel()

	stateDir := t.TempDir()
	kernel := newKernelHostServices(nil, nil)
	kernel.bindWorkspaceStateDir(stateDir)
	invoker := &dataQueryInvoker{parent: kernel}

	raw, err := invoker.InvokeService(context.Background(), pluginhost.ServiceInvokeParams{
		Method: pluginhost.KernelServiceMethod,
		Params: json.RawMessage(`{"thread_id":"thread-missing"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	var result pluginhost.DataQueryResult
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Events) != 0 {
		t.Fatalf("missing trace events = %+v, want empty", result.Events)
	}
}

func TestDataQueryInvokerRejectsUnsafeThreadID(t *testing.T) {
	t.Parallel()

	kernel := newKernelHostServices(nil, nil)
	kernel.bindWorkspaceStateDir(t.TempDir())
	invoker := &dataQueryInvoker{parent: kernel}

	_, err := invoker.InvokeService(context.Background(), pluginhost.ServiceInvokeParams{
		Method: pluginhost.KernelServiceMethod,
		Params: json.RawMessage(`{"thread_id":"../thread-1"}`),
	})
	var hostErr *pluginhost.HostServiceError
	if !errors.As(err, &hostErr) || hostErr.Code != "invalid_params" {
		t.Fatalf("unsafe thread error = %#v, want invalid_params", err)
	}
}

func TestDataQueryInvokerHonorsLimit(t *testing.T) {
	t.Parallel()

	stateDir := t.TempDir()
	threadID := "thread-1"
	artifactDir := statepath.SessionArtifactDir(stateDir, threadID)
	if err := os.MkdirAll(artifactDir, 0o755); err != nil {
		t.Fatal(err)
	}
	tracePath := sessiontrace.Path(artifactDir)
	if err := sessiontrace.AppendTurn(tracePath,
		sessiontrace.TurnRecord{ThreadID: threadID, TurnID: "turn-1", Status: "completed"},
		sessiontrace.FinalRecord{Status: "completed"},
		nil, nil, nil, nil, nil,
	); err != nil {
		t.Fatal(err)
	}

	kernel := newKernelHostServices(nil, nil)
	kernel.bindWorkspaceStateDir(stateDir)
	invoker := &dataQueryInvoker{parent: kernel}

	raw, err := invoker.InvokeService(context.Background(), pluginhost.ServiceInvokeParams{
		Method: pluginhost.KernelServiceMethod,
		Params: json.RawMessage(`{"thread_id":"thread-1","limit":1}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	var result pluginhost.DataQueryResult
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Events) != 1 {
		t.Fatalf("events = %d, want 1", len(result.Events))
	}
}

func TestDataQueryInvokerRequiresStateDir(t *testing.T) {
	t.Parallel()

	kernel := newKernelHostServices(nil, nil)
	invoker := &dataQueryInvoker{parent: kernel}

	_, err := invoker.InvokeService(context.Background(), pluginhost.ServiceInvokeParams{
		Method: pluginhost.KernelServiceMethod,
		Params: json.RawMessage(`{"thread_id":"thread-1"}`),
	})
	var hostErr *pluginhost.HostServiceError
	if !errors.As(err, &hostErr) || hostErr.Code != "service_unavailable" {
		t.Fatalf("unbound state dir error = %#v, want service_unavailable", err)
	}
}

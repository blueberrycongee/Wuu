package singlepass

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"

	"github.com/blueberrycongee/wuu/internal/loopdriver"
	pluginapi "github.com/blueberrycongee/wuu/packages/plugin-go"
)

type fakeHost struct {
	serve func(service, method string, params json.RawMessage) (json.RawMessage, error)
}

func (h *fakeHost) InitializeParams() pluginapi.InitializeParams { return pluginapi.InitializeParams{} }
func (h *fakeHost) CallHost(_ context.Context, method string, params, result any) error {
	if method != pluginapi.HostServiceCallMethod {
		return &pluginapi.HostCallError{Code: "method_not_found", Message: method}
	}
	raw, _ := json.Marshal(params)
	var routed struct {
		Service string          `json:"service"`
		Method  string          `json:"method"`
		Params  json.RawMessage `json:"params"`
	}
	_ = json.Unmarshal(raw, &routed)
	if h.serve == nil {
		return &pluginapi.HostCallError{Code: "service_not_found", Message: "no provider for service " + routed.Service}
	}
	response, err := h.serve(routed.Service, routed.Method, routed.Params)
	if err != nil {
		return err
	}
	if result == nil {
		return nil
	}
	return json.Unmarshal(response, result)
}

func TestHandlerDeclaresDriverServiceAndGatewayRequirements(t *testing.T) {
	handler := Handler()
	if len(handler.Definition.ProvidedServices) != 1 || handler.Definition.ProvidedServices[0].Name != DriverService {
		t.Fatalf("provided = %+v", handler.Definition.ProvidedServices)
	}
	methods := handler.Definition.ProvidedServices[0].Methods
	for _, want := range []string{"descriptor", "create", "resume", "run", "shutdown"} {
		found := false
		for _, method := range methods {
			if method.Name == want {
				found = true
			}
		}
		if !found {
			t.Fatalf("method %q missing from %+v", want, methods)
		}
	}
	var declared []string
	for _, requirement := range handler.Definition.RequiredServices {
		declared = append(declared, requirement.Name)
	}
	if strings.Join(declared, ",") != loopdriver.DriverModelLoopService+","+loopdriver.DriverCheckpointService {
		t.Fatalf("required services = %v", declared)
	}
}

func TestInvokeServiceRunsDriverThroughKernelGateway(t *testing.T) {
	var mu sync.Mutex
	var modelLoopCalls []loopdriver.DriverModelLoopParams
	var checkpointCalls []loopdriver.DriverCheckpointParams
	host := &fakeHost{serve: func(service, _ string, params json.RawMessage) (json.RawMessage, error) {
		mu.Lock()
		defer mu.Unlock()
		switch service {
		case loopdriver.DriverModelLoopService:
			var decoded loopdriver.DriverModelLoopParams
			if err := json.Unmarshal(params, &decoded); err != nil {
				return nil, err
			}
			modelLoopCalls = append(modelLoopCalls, decoded)
			return json.Marshal(loopdriver.DriverModelLoopResult{ReceiptID: "model-loop-1"})
		case loopdriver.DriverCheckpointService:
			var decoded loopdriver.DriverCheckpointParams
			if err := json.Unmarshal(params, &decoded); err != nil {
				return nil, err
			}
			checkpointCalls = append(checkpointCalls, decoded)
			return json.RawMessage(`{}`), nil
		}
		return nil, &pluginapi.HostCallError{Code: "service_not_found", Message: service}
	}}

	handler := Handler()
	if err := handler.Initialize(context.Background(), host, pluginapi.InitializeParams{}); err != nil {
		t.Fatal(err)
	}
	invoke := func(method string, params any) json.RawMessage {
		t.Helper()
		raw, err := json.Marshal(params)
		if err != nil {
			t.Fatal(err)
		}
		result, err := handler.InvokeService(context.Background(), host, pluginapi.ServiceCall{
			Service: DriverService, Method: method, Caller: "kernel", Params: raw,
		})
		if err != nil {
			t.Fatalf("%s: %v", method, err)
		}
		return result
	}

	var descriptor loopdriver.Descriptor
	if err := json.Unmarshal(invoke("descriptor", nil), &descriptor); err != nil {
		t.Fatal(err)
	}
	if descriptor.ID != loopdriver.SinglePassDriverID {
		t.Fatalf("descriptor = %+v", descriptor)
	}

	create, err := json.Marshal(map[string]any{
		"execution": map[string]any{"session_id": "s", "execution_id": "exec-1"},
		"input":     map[string]any{"messages": []any{}},
	})
	if err != nil {
		t.Fatal(err)
	}
	var opened instanceResult
	createResult, err := handler.InvokeService(context.Background(), host, pluginapi.ServiceCall{
		Service: DriverService, Method: "create", Caller: "kernel", Params: create,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(createResult, &opened); err != nil {
		t.Fatal(err)
	}
	if opened.InstanceID == "" {
		t.Fatal("create returned an empty instance id")
	}

	var outcome runResult
	if err := json.Unmarshal(invoke("run", runParams{InstanceID: opened.InstanceID}), &outcome); err != nil {
		t.Fatal(err)
	}
	if outcome.Status != loopdriver.TerminalSucceeded || outcome.ReceiptID != "model-loop-1" {
		t.Fatalf("outcome = %+v", outcome)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(modelLoopCalls) != 1 || modelLoopCalls[0].ExecutionID != "exec-1" {
		t.Fatalf("model loop calls = %+v", modelLoopCalls)
	}
	if !modelLoopCalls[0].Policy.DisableTools || modelLoopCalls[0].Policy.ModelRoundLimit != 1 {
		t.Fatalf("single-pass policy = %+v", modelLoopCalls[0].Policy)
	}
	if len(checkpointCalls) != 1 || checkpointCalls[0].Checkpoint.DriverID != loopdriver.SinglePassDriverID {
		t.Fatalf("checkpoint calls = %+v", checkpointCalls)
	}
	if _, err := handler.InvokeService(context.Background(), host, pluginapi.ServiceCall{
		Service: DriverService, Method: "shutdown", Caller: "kernel",
		Params: json.RawMessage(`{"instance_id":"` + opened.InstanceID + `"}`),
	}); err != nil {
		t.Fatal(err)
	}
}

func TestInvokeServiceResumeValidatesCheckpoint(t *testing.T) {
	host := &fakeHost{}
	handler := Handler()
	if err := handler.Initialize(context.Background(), host, pluginapi.InitializeParams{}); err != nil {
		t.Fatal(err)
	}
	resume, _ := json.Marshal(map[string]any{
		"execution":  map[string]any{"session_id": "s", "execution_id": "exec-2"},
		"input":      map[string]any{"messages": []any{}},
		"checkpoint": map[string]any{"contract_version": loopdriver.ContractVersion, "driver_id": "other", "driver_version": "9.9.9"},
	})
	if _, err := handler.InvokeService(context.Background(), host, pluginapi.ServiceCall{
		Service: DriverService, Method: "resume", Caller: "kernel", Params: resume,
	}); err == nil {
		t.Fatal("resume with a foreign checkpoint must fail")
	}
}

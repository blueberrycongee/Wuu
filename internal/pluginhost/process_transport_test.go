package pluginhost

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type testHostServiceHandler struct {
	services []HostServiceMethod
	handle   func(context.Context, HostServiceMethod, json.RawMessage) (json.RawMessage, error)
}

func (h testHostServiceHandler) SupportedHostServices() []HostServiceMethod {
	return append([]HostServiceMethod(nil), h.services...)
}

func (h testHostServiceHandler) HandleHostService(ctx context.Context, method HostServiceMethod, params json.RawMessage) (json.RawMessage, error) {
	return h.handle(ctx, method, params)
}

func TestProcessHostServiceTransport(t *testing.T) {
	if scenario := os.Getenv("WUU_PLUGINHOST_TRANSPORT_HELPER"); scenario != "" {
		runHostServiceTransportHelper(scenario)
		return
	}

	t.Run("nested call succeeds", func(t *testing.T) {
		handler := testHostServiceHandler{
			services: []HostServiceMethod{HostServiceStorageGet},
			handle: func(_ context.Context, method HostServiceMethod, params json.RawMessage) (json.RawMessage, error) {
				if method != HostServiceStorageGet || string(params) != `{"key":"color"}` {
					t.Fatalf("call = %s %s", method, params)
				}
				return json.RawMessage(`{"value":"blue"}`), nil
			},
		}
		client := startTransportClient(t, "success", 2*time.Second, handler)
		defer client.Close(context.Background())
		result, err := invokeTransportCapability(context.Background(), client)
		if err != nil {
			t.Fatal(err)
		}
		if string(result.Output) != `{"value":"blue"}` {
			t.Fatalf("output = %s", result.Output)
		}
	})

	t.Run("background call succeeds after invocation returns", func(t *testing.T) {
		called := make(chan struct{})
		handler := testHostServiceHandler{
			services: []HostServiceMethod{HostServiceStorageGet},
			handle: func(_ context.Context, method HostServiceMethod, params json.RawMessage) (json.RawMessage, error) {
				if method != HostServiceStorageGet {
					t.Fatalf("call = %s %s", method, params)
				}
				if string(params) == `{"key":"color"}` {
					return json.RawMessage(`{"value":"blue"}`), nil
				}
				if string(params) != `{"key":"background"}` {
					t.Fatalf("params = %s", params)
				}
				close(called)
				return json.RawMessage(`{"value":"awake"}`), nil
			},
		}
		client := startTransportClient(t, "background", 2*time.Second, handler)
		defer client.Close(context.Background())
		if _, err := invokeTransportCapability(context.Background(), client); err != nil {
			t.Fatal(err)
		}
		select {
		case <-called:
		case <-time.After(time.Second):
			t.Fatal("plugin did not call host after invocation returned")
		}
	})

	t.Run("handler error is returned to plugin", func(t *testing.T) {
		handler := testHostServiceHandler{
			services: []HostServiceMethod{HostServiceStorageGet},
			handle: func(context.Context, HostServiceMethod, json.RawMessage) (json.RawMessage, error) {
				return nil, &HostServiceError{Code: "storage_failed", Message: "storage unavailable"}
			},
		}
		client := startTransportClient(t, "handler-error", 2*time.Second, handler)
		defer client.Close(context.Background())
		result, err := invokeTransportCapability(context.Background(), client)
		if err != nil {
			t.Fatal(err)
		}
		if string(result.Output) != `{"code":"storage_failed"}` {
			t.Fatalf("output = %s", result.Output)
		}
	})

	t.Run("undeclared service fails process", func(t *testing.T) {
		handler := testHostServiceHandler{
			services: []HostServiceMethod{HostServiceStorageGet},
			handle: func(context.Context, HostServiceMethod, json.RawMessage) (json.RawMessage, error) {
				t.Fatal("undeclared service reached handler")
				return nil, nil
			},
		}
		client := startTransportClient(t, "undeclared", 2*time.Second, handler)
		_, err := invokeTransportCapability(context.Background(), client)
		if err == nil || !strings.Contains(err.Error(), "was not negotiated") {
			t.Fatalf("error = %v", err)
		}
		if client.Status().State != StateFailed {
			t.Fatalf("status = %+v", client.Status())
		}
	})

	t.Run("malformed request fails process", func(t *testing.T) {
		handler := testHostServiceHandler{
			services: []HostServiceMethod{HostServiceStorageGet},
			handle: func(context.Context, HostServiceMethod, json.RawMessage) (json.RawMessage, error) {
				t.Fatal("malformed request reached handler")
				return nil, nil
			},
		}
		client := startTransportClient(t, "malformed", 2*time.Second, handler)
		_, err := invokeTransportCapability(context.Background(), client)
		if err == nil || !strings.Contains(err.Error(), "params must be a JSON object") {
			t.Fatalf("error = %v", err)
		}
	})

	for _, test := range []struct {
		name    string
		timeout time.Duration
		cancel  bool
	}{
		{name: "timeout", timeout: 80 * time.Millisecond},
		{name: "cancel", timeout: 2 * time.Second, cancel: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			handlerDone := make(chan struct{})
			handler := testHostServiceHandler{
				services: []HostServiceMethod{HostServiceStorageGet},
				handle: func(ctx context.Context, _ HostServiceMethod, _ json.RawMessage) (json.RawMessage, error) {
					<-ctx.Done()
					close(handlerDone)
					return nil, ctx.Err()
				},
			}
			client := startTransportClient(t, "blocked", test.timeout, handler)
			ctx := context.Background()
			var cancel context.CancelFunc
			if test.cancel {
				ctx, cancel = context.WithCancel(ctx)
				time.AfterFunc(30*time.Millisecond, cancel)
			}
			_, err := invokeTransportCapability(ctx, client)
			if test.cancel && !errors.Is(err, context.Canceled) {
				t.Fatalf("error = %v", err)
			}
			if !test.cancel && !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("error = %v", err)
			}
			select {
			case <-handlerDone:
			case <-time.After(time.Second):
				t.Fatal("handler did not observe context cancellation")
			}
			if client.Status().State != StateFailed {
				t.Fatalf("status = %+v", client.Status())
			}
		})
	}
}

func TestProcessHostServiceConfigurationRequiresLiveHandler(t *testing.T) {
	_, err := Start(context.Background(), ProcessConfig{
		ID: "names-only", Command: os.Args[0], SupportedHostServices: []HostServiceMethod{HostServiceStorageGet},
	})
	if err == nil || !strings.Contains(err.Error(), "require a live handler") {
		t.Fatalf("error = %v", err)
	}

	handler := testHostServiceHandler{
		services: []HostServiceMethod{HostServiceStorageGet},
		handle: func(context.Context, HostServiceMethod, json.RawMessage) (json.RawMessage, error) {
			return json.RawMessage(`{}`), nil
		},
	}
	_, err = Start(context.Background(), ProcessConfig{
		ID: "mismatch", Command: os.Args[0], HostServiceHandler: handler,
		SupportedHostServices: []HostServiceMethod{HostServiceStorageSet},
	})
	if err == nil || !strings.Contains(err.Error(), "do not match") {
		t.Fatalf("error = %v", err)
	}
}

func startTransportClient(t *testing.T, scenario string, timeout time.Duration, handler HostServiceHandler) *ProcessClient {
	t.Helper()
	root := t.TempDir()
	client, err := Start(context.Background(), ProcessConfig{
		ID: "transport-plugin", Command: os.Args[0], Args: []string{"-test.run=TestProcessHostServiceTransport"},
		Env: map[string]string{"WUU_PLUGINHOST_TRANSPORT_HELPER": scenario}, PluginRoot: root,
		ProjectRoot: filepath.Dir(root), WuuHome: t.TempDir(), Timeout: timeout, HostServiceHandler: handler,
	})
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func invokeTransportCapability(ctx context.Context, client *ProcessClient) (CapabilityInvokeResult, error) {
	return client.InvokeCapability(ctx, CapabilityInvokeParams{
		Capability: CapabilityPluginClientRequest,
		Input:      json.RawMessage(`{}`),
		Output:     json.RawMessage(`{}`),
	})
}

func runHostServiceTransportHelper(scenario string) {
	scanner := bufio.NewScanner(os.Stdin)
	enc := json.NewEncoder(os.Stdout)
	if !scanner.Scan() {
		os.Exit(2)
	}
	var initialize rpcRequest
	if json.Unmarshal(scanner.Bytes(), &initialize) != nil || initialize.Method != "initialize" {
		os.Exit(3)
	}
	var initParams CapabilityInitializeParams
	data, _ := json.Marshal(initialize.Params)
	_ = json.Unmarshal(data, &initParams)
	if len(initParams.SupportedHostServices) != 1 || initParams.SupportedHostServices[0] != HostServiceStorageGet {
		os.Exit(4)
	}
	required := []HostServiceDescriptor{{ID: string(HostServiceStorageGet), Required: true}}
	if scenario == "undeclared" {
		required = nil
	}
	_ = enc.Encode(rpcResponse{ID: initialize.ID, Result: mustRaw(CapabilityInitializeResult{
		ProtocolVersion:      CapabilityProtocolVersion,
		Capabilities:         []CapabilityDescriptor{{ID: CapabilityPluginClientRequest, Kind: SeamDecision, Version: 1}},
		RequiredHostServices: required,
	})})

	if !scanner.Scan() {
		os.Exit(5)
	}
	var invoke rpcRequest
	if json.Unmarshal(scanner.Bytes(), &invoke) != nil || invoke.Method != "capability.invoke" {
		os.Exit(6)
	}
	if scenario == "malformed" {
		_, _ = os.Stdout.WriteString(`{"id":"nested-1","method":"host.storage.get","params":[]}` + "\n")
		if !scanner.Scan() {
			os.Exit(10)
		}
		var invalid HostServiceResult
		if json.Unmarshal(scanner.Bytes(), &invalid) != nil || invalid.ID != "nested-1" || invalid.Error == nil || invalid.Error.Code != "invalid_request" {
			os.Exit(11)
		}
		time.Sleep(10 * time.Second)
	}
	_ = enc.Encode(HostServiceCall{ID: "nested-1", Method: HostServiceStorageGet, Params: json.RawMessage(`{"key":"color"}`)})
	if !scanner.Scan() {
		os.Exit(7)
	}
	var hostResult HostServiceResult
	if json.Unmarshal(scanner.Bytes(), &hostResult) != nil || hostResult.ID != "nested-1" {
		os.Exit(8)
	}
	switch scenario {
	case "success":
		_ = enc.Encode(rpcResponse{ID: invoke.ID, Result: mustRaw(CapabilityInvokeResult{Output: hostResult.Result})})
	case "background":
		_ = enc.Encode(rpcResponse{ID: invoke.ID, Result: mustRaw(CapabilityInvokeResult{Output: json.RawMessage(`{}`)})})
		_ = enc.Encode(HostServiceCall{ID: "background-1", Method: HostServiceStorageGet, Params: json.RawMessage(`{"key":"background"}`)})
		if !scanner.Scan() {
			os.Exit(12)
		}
		var backgroundResult HostServiceResult
		if json.Unmarshal(scanner.Bytes(), &backgroundResult) != nil || backgroundResult.ID != "background-1" || string(backgroundResult.Result) != `{"value":"awake"}` {
			os.Exit(13)
		}
		if !scanner.Scan() {
			os.Exit(14)
		}
		var shutdown rpcRequest
		if json.Unmarshal(scanner.Bytes(), &shutdown) != nil || shutdown.Method != "shutdown" {
			os.Exit(15)
		}
		_ = enc.Encode(rpcResponse{ID: shutdown.ID, Result: json.RawMessage(`{}`)})
	case "handler-error":
		_ = enc.Encode(rpcResponse{ID: invoke.ID, Result: mustRaw(CapabilityInvokeResult{Output: mustRaw(map[string]string{"code": hostResult.Error.Code})})})
	case "undeclared":
		if hostResult.Error == nil || hostResult.Error.Code != "service_not_negotiated" {
			os.Exit(9)
		}
		time.Sleep(10 * time.Second)
	case "blocked":
		time.Sleep(10 * time.Second)
	}
}

func mustRaw(value any) json.RawMessage {
	data, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return data
}

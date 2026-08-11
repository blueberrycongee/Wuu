package pluginapi

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
)

func TestServeNegotiatesAndInvokesCapability(t *testing.T) {
	input := strings.Join([]string{
		`{"id":"1","method":"initialize","params":{"protocol_version":1,"capability_protocol_version":2,"plugin_id":"test"}}`,
		`{"id":"2","method":"capability.invoke","params":{"capability":"test.decision","input":{"value":1}}}`,
		`{"id":"3","method":"shutdown"}`,
	}, "\n") + "\n"
	var output bytes.Buffer
	err := ServeIO(context.Background(), strings.NewReader(input), &output, Handler{
		Definition: Definition{Capabilities: []Capability{{ID: "test.decision", Kind: "decision", Version: 1}}},
		InvokeCapability: func(_ context.Context, _ Host, call CapabilityCall) (json.RawMessage, error) {
			if call.Capability != "test.decision" {
				t.Fatalf("capability = %q", call.Capability)
			}
			return json.RawMessage(`{"accepted":true}`), nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	if len(lines) != 3 || !strings.Contains(lines[0], `"protocol_version":3`) || !strings.Contains(lines[1], `"accepted":true`) {
		t.Fatalf("responses = %s", output.String())
	}
}

func TestServeNegotiatesAndInvokesService(t *testing.T) {
	input := strings.Join([]string{
		`{"id":"1","method":"initialize","params":{"protocol_version":1,"capability_protocol_version":3,"plugin_id":"search"}}`,
		`{"id":"2","method":"service.invoke","params":{"service":"search.provider","method":"query","caller":"notes","params":{"q":"x"}}}`,
		`{"id":"3","method":"service.changed","params":{"service":"search.provider","reason":"provider_closed"}}`,
		`{"id":"4","method":"shutdown"}`,
	}, "\n") + "\n"
	var output bytes.Buffer
	var invoke ServiceCall
	var notice ServiceChangedNotice
	err := ServeIO(context.Background(), strings.NewReader(input), &output, Handler{
		Definition: Definition{
			ProvidedServices: []Service{{
				Name:    "search.provider",
				Version: "1.0.0",
				Methods: []ServiceMethod{{Name: "query", InputSchema: "search.query.request.v1", OutputSchema: "search.query.response.v1"}},
			}},
			RequiredServices: []ServiceRequirement{{Name: "memory.index", MajorVersion: 1}},
		},
		InvokeService: func(_ context.Context, _ Host, call ServiceCall) (json.RawMessage, error) {
			invoke = call
			return json.RawMessage(`{"hits":["a"]}`), nil
		},
		ServiceChanged: func(_ context.Context, got ServiceChangedNotice) error {
			notice = got
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if invoke.Caller != "notes" || invoke.Method != "query" {
		t.Fatalf("invoke = %+v", invoke)
	}
	if notice.Reason != "provider_closed" {
		t.Fatalf("notice = %+v", notice)
	}
	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	if len(lines) != 4 {
		t.Fatalf("responses = %s", output.String())
	}
	if !strings.Contains(lines[0], `"protocol_version":3`) || !strings.Contains(lines[0], `"provided_services"`) || !strings.Contains(lines[0], `"required_services"`) {
		t.Fatalf("initialize response = %s", lines[0])
	}
	if !strings.Contains(lines[1], `"hits":["a"]`) {
		t.Fatalf("service.invoke response = %s", lines[1])
	}
}

func TestCallServiceUsesGatewayFrame(t *testing.T) {
	requestReader, requestWriter := io.Pipe()
	defer requestReader.Close()
	client := newClient(requestWriter)
	requestSeen := make(chan string, 1)
	go func() {
		scanner := bufio.NewScanner(requestReader)
		if !scanner.Scan() {
			return
		}
		requestSeen <- scanner.Text()
		client.routeResponse(rpcResponse{ID: "plugin-1", Result: json.RawMessage(`{"hits":[]}`)})
	}()
	var result struct {
		Hits []string `json:"hits"`
	}
	if err := CallService(context.Background(), client, "search.provider", "query", map[string]string{"q": "x"}, &result); err != nil {
		t.Fatal(err)
	}
	request := <-requestSeen
	if !strings.Contains(request, `"method":"host.service.call"`) || !strings.Contains(request, `"service":"search.provider"`) || !strings.Contains(request, `"method":"query"`) {
		t.Fatalf("gateway request = %s", request)
	}
}

func TestServeRunsShutdownCleanupBeforeAcknowledging(t *testing.T) {
	input := strings.Join([]string{
		`{"id":"1","method":"initialize","params":{"protocol_version":1,"capability_protocol_version":2,"plugin_id":"test"}}`,
		`{"id":"2","method":"shutdown"}`,
	}, "\n") + "\n"
	var output bytes.Buffer
	cleaned := false
	err := ServeIO(context.Background(), strings.NewReader(input), &output, Handler{
		Shutdown: func(context.Context) error {
			cleaned = true
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !cleaned {
		t.Fatal("shutdown cleanup was not called")
	}
	if lines := strings.Split(strings.TrimSpace(output.String()), "\n"); len(lines) != 2 || !strings.Contains(lines[1], `"id":"2"`) {
		t.Fatalf("responses = %s", output.String())
	}
}

func TestClientCallsHostServiceOnSameChannel(t *testing.T) {
	requestReader, requestWriter := io.Pipe()
	defer requestReader.Close()
	client := newClient(requestWriter)
	requestSeen := make(chan string, 1)
	go func() {
		scanner := bufio.NewScanner(requestReader)
		if !scanner.Scan() {
			return
		}
		requestSeen <- scanner.Text()
		client.routeResponse(rpcResponse{ID: "plugin-1", Result: json.RawMessage(`{"value":"stored"}`)})
	}()
	var result struct {
		Value string `json:"value"`
	}
	if err := client.CallHost(context.Background(), "host.storage.get", map[string]string{"scope": "workspace", "key": "state"}, &result); err != nil {
		t.Fatal(err)
	}
	request := <-requestSeen
	if result.Value != "stored" || !strings.Contains(request, `"method":"host.service.call"`) || !strings.Contains(request, `"service":"host.storage.get"`) {
		t.Fatalf("result = %+v request = %s", result, request)
	}
}

func TestServeAllowsBackgroundHostCallAfterCapabilityReturns(t *testing.T) {
	hostReader, pluginWriter := io.Pipe()
	pluginReader, hostWriter := io.Pipe()
	defer hostReader.Close()
	defer pluginReader.Close()

	release := make(chan struct{})
	backgroundResult := make(chan error, 1)
	serveDone := make(chan error, 1)
	go func() {
		serveDone <- ServeIO(context.Background(), pluginReader, pluginWriter, Handler{
			Definition: Definition{
				Capabilities:         []Capability{{ID: "test.background", Kind: "observe", Version: 1}},
				RequiredHostServices: []HostService{{ID: "host.storage.get", Required: true}},
			},
			InvokeCapability: func(_ context.Context, host Host, _ CapabilityCall) (json.RawMessage, error) {
				go func() {
					<-release
					var result struct {
						Value string `json:"value"`
					}
					err := host.CallHost(context.Background(), "host.storage.get", map[string]string{"key": "background"}, &result)
					if err == nil && result.Value != "awake" {
						err = fmt.Errorf("value = %q", result.Value)
					}
					backgroundResult <- err
				}()
				return json.RawMessage(`{}`), nil
			},
		})
	}()

	scanner := bufio.NewScanner(hostReader)
	writeHost := func(line string) {
		t.Helper()
		if _, err := io.WriteString(hostWriter, line+"\n"); err != nil {
			t.Fatal(err)
		}
	}
	readPlugin := func() string {
		t.Helper()
		if !scanner.Scan() {
			t.Fatalf("plugin output ended: %v", scanner.Err())
		}
		return scanner.Text()
	}

	writeHost(`{"id":"1","method":"initialize","params":{"protocol_version":1,"capability_protocol_version":2,"plugin_id":"test","supported_host_services":["host.storage.get"]}}`)
	if response := readPlugin(); !strings.Contains(response, `"id":"1"`) {
		t.Fatalf("initialize response = %s", response)
	}
	writeHost(`{"id":"2","method":"capability.invoke","params":{"capability":"test.background","input":{}}}`)
	if response := readPlugin(); !strings.Contains(response, `"id":"2"`) {
		t.Fatalf("capability response = %s", response)
	}
	close(release)
	request := readPlugin()
	var hostCall rpcRequest
	if err := json.Unmarshal([]byte(request), &hostCall); err != nil || hostCall.Method != "host.service.call" {
		t.Fatalf("background request = %s err=%v", request, err)
	}
	writeHost(fmt.Sprintf(`{"id":%q,"result":{"value":"awake"}}`, hostCall.ID))
	if err := <-backgroundResult; err != nil {
		t.Fatal(err)
	}
	writeHost(`{"id":"3","method":"shutdown"}`)
	if response := readPlugin(); !strings.Contains(response, `"id":"3"`) {
		t.Fatalf("shutdown response = %s", response)
	}
	_ = hostWriter.Close()
	if err := <-serveDone; err != nil {
		t.Fatal(err)
	}
}

func TestExecutionCancelPreemptsRunningTool(t *testing.T) {
	input := strings.Join([]string{
		`{"id":"1","method":"initialize","params":{"protocol_version":1,"capability_protocol_version":3,"plugin_id":"slow"}}`,
		`{"id":"2","method":"tool.execute","params":{"tool_id":"wait","execution_id":"exec-1","call_id":"c1","tool":"wait","arguments":{}}}`,
		`{"id":"3","method":"execution.cancel","params":{"execution_id":"exec-1"}}`,
		`{"id":"4","method":"shutdown"}`,
	}, "\n") + "\n"
	var output bytes.Buffer
	observedCancel := make(chan error, 1)
	err := ServeIO(context.Background(), strings.NewReader(input), &output, Handler{
		Definition: Definition{Tools: []Tool{{ID: "wait", Description: "block until canceled", InputSchema: map[string]any{"type": "object"}}}},
		ExecuteTool: func(ctx context.Context, _ Host, call ToolCall) (ToolResult, error) {
			if call.ExecutionID != "exec-1" {
				observedCancel <- nil
				return TextResult("wrong execution"), nil
			}
			<-ctx.Done()
			observedCancel <- ctx.Err()
			return TextResult("canceled"), nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if ctxErr := <-observedCancel; ctxErr == nil {
		t.Fatal("tool handler context was not canceled")
	}
	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	if len(lines) != 4 {
		t.Fatalf("responses = %s", output.String())
	}
	// The cancel acknowledgement (id 3) is written before the tool result
	// (id 2): proof the cancel preempted the serial dispatch queue.
	cancelAck, toolResult := -1, -1
	for index, line := range lines {
		if strings.Contains(line, `"id":"3"`) {
			cancelAck = index
		}
		if strings.Contains(line, `"id":"2"`) {
			toolResult = index
		}
	}
	if cancelAck == -1 || toolResult == -1 || cancelAck > toolResult || !strings.Contains(lines[toolResult], "canceled") {
		t.Fatalf("preemption order = %s", output.String())
	}
}

func TestExecutionCancelPreemptsRunningService(t *testing.T) {
	input := strings.Join([]string{
		`{"id":"1","method":"initialize","params":{"protocol_version":1,"capability_protocol_version":3,"plugin_id":"slow-service"}}`,
		`{"id":"2","method":"service.invoke","params":{"service":"search.provider","method":"query","caller":"notes","execution_id":"exec-service"}}`,
		`{"id":"3","method":"execution.cancel","params":{"execution_id":"exec-service"}}`,
		`{"id":"4","method":"shutdown"}`,
	}, "\n") + "\n"
	var output bytes.Buffer
	observedCancel := make(chan error, 1)
	err := ServeIO(context.Background(), strings.NewReader(input), &output, Handler{
		Definition: Definition{ProvidedServices: []Service{{Name: "search.provider", Version: "1.0.0"}}},
		InvokeService: func(ctx context.Context, _ Host, call ServiceCall) (json.RawMessage, error) {
			if call.ExecutionID != "exec-service" {
				observedCancel <- nil
				return json.RawMessage(`{"state":"wrong execution"}`), nil
			}
			<-ctx.Done()
			observedCancel <- ctx.Err()
			return json.RawMessage(`{"state":"cancelled"}`), nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if ctxErr := <-observedCancel; ctxErr == nil {
		t.Fatal("service handler context was not canceled")
	}
	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	cancelAck, serviceResult := -1, -1
	for index, line := range lines {
		if strings.Contains(line, `"id":"3"`) {
			cancelAck = index
		}
		if strings.Contains(line, `"id":"2"`) {
			serviceResult = index
		}
	}
	if cancelAck == -1 || serviceResult == -1 || cancelAck > serviceResult || !strings.Contains(lines[serviceResult], "cancelled") {
		t.Fatalf("service preemption order = %s", output.String())
	}
}

func TestExecutionCancelForUnknownExecutionIsNoop(t *testing.T) {
	input := strings.Join([]string{
		`{"id":"1","method":"initialize","params":{"protocol_version":1,"capability_protocol_version":3,"plugin_id":"slow"}}`,
		`{"id":"2","method":"execution.cancel","params":{"execution_id":"exec-gone"}}`,
		`{"id":"3","method":"shutdown"}`,
	}, "\n") + "\n"
	var output bytes.Buffer
	if err := ServeIO(context.Background(), strings.NewReader(input), &output, Handler{}); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("responses = %s", output.String())
	}
	joined := output.String()
	for _, id := range []string{`"id":"1"`, `"id":"2"`, `"id":"3"`} {
		if !strings.Contains(joined, id) {
			t.Fatalf("missing ack for %s in %s", id, joined)
		}
	}
}

func TestCallHostPreservesTypedErrorCode(t *testing.T) {
	requestReader, requestWriter := io.Pipe()
	defer requestReader.Close()
	client := newClient(requestWriter)
	go func() {
		scanner := bufio.NewScanner(requestReader)
		if !scanner.Scan() {
			return
		}
		client.routeResponse(rpcResponse{ID: "plugin-1", Error: &rpcError{Code: "service_unavailable", Message: "no provider for service memory.session"}})
	}()
	err := client.CallHost(context.Background(), HostServiceCallMethod, map[string]string{"service": "memory.session", "method": "read"}, nil)
	var hostErr *HostCallError
	if !errors.As(err, &hostErr) || hostErr.Code != "service_unavailable" || hostErr.Message != "no provider for service memory.session" {
		t.Fatalf("typed error = %#v", err)
	}
}

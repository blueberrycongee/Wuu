package pluginapi

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
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
	if len(lines) != 3 || !strings.Contains(lines[0], `"protocol_version":2`) || !strings.Contains(lines[1], `"accepted":true`) {
		t.Fatalf("responses = %s", output.String())
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
	if result.Value != "stored" || !strings.Contains(request, `"method":"host.storage.get"`) {
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
	if err := json.Unmarshal([]byte(request), &hostCall); err != nil || hostCall.Method != "host.storage.get" {
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

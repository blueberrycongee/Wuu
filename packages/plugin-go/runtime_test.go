package pluginapi

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
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

func TestClientCallsHostServiceOnSameChannel(t *testing.T) {
	var output bytes.Buffer
	client := &Client{
		scanner: bufio.NewScanner(strings.NewReader(`{"id":"plugin-1","result":{"value":"stored"}}` + "\n")),
		output:  &output,
	}
	var result struct {
		Value string `json:"value"`
	}
	if err := client.CallHost(context.Background(), "host.storage.get", map[string]string{"scope": "workspace", "key": "state"}, &result); err != nil {
		t.Fatal(err)
	}
	if result.Value != "stored" || !strings.Contains(output.String(), `"method":"host.storage.get"`) {
		t.Fatalf("result = %+v request = %s", result, output.String())
	}
}

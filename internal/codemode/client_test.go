package codemode

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"net"
	"sync/atomic"
	"testing"
	"time"
)

func receive(t *testing.T, conn net.Conn) map[string]json.RawMessage {
	t.Helper()
	var value map[string]json.RawMessage
	if err := readFrame(conn, &value); err != nil {
		t.Fatalf("receive host request: %v", err)
	}
	return value
}

func transmit(t *testing.T, conn net.Conn, value any) {
	t.Helper()
	frame, err := encodeFrame(value)
	if err == nil {
		err = writeFrame(conn, frame)
	}
	if err != nil {
		t.Fatalf("send host response: %v", err)
	}
}

func reply(t *testing.T, conn net.Conn, id json.RawMessage, value any) {
	t.Helper()
	transmit(t, conn, map[string]any{"type": "operation/response", "id": id,
		"result": map[string]any{"status": "ok", "value": value}})
}

func runtimeOutput(state, cell string, content ...ContentItem) any {
	if content == nil {
		content = []ContentItem{}
	}
	return map[string]any{state: map[string]any{"cell_id": cell, "content_items": content, "code_mode_host_duration_ns": 0}}
}

func testClient(t *testing.T, delegate Delegate, host func(net.Conn)) *Client {
	t.Helper()
	local, remote := net.Pipe()
	_ = remote.SetDeadline(time.Now().Add(10 * time.Second))
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		defer remote.Close()
		hello := receive(t, remote)
		if string(hello["type"]) != `"connection/hello"` {
			t.Error("missing client handshake")
			return
		}
		transmit(t, remote, map[string]any{"type": "connection/ready", "selectedVersion": 1, "capabilities": []string{resourceLimitsCapability}})
		open := receive(t, remote)
		reply(t, remote, open["id"], map[string]any{"type": "session/ready", "sessionId": "session"})
		host(remote)
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	client, err := Connect(ctx, local, "session", CellLimits{}, delegate)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close(); <-finished })
	return client
}

func TestExecuteYieldWaitKeepsStructuredOutput(t *testing.T) {
	client := testClient(t, nil, func(conn net.Conn) {
		execute := receive(t, conn)
		reply(t, conn, execute["id"], map[string]any{"type": "execution/started", "cellId": "cell"})
		transmit(t, conn, map[string]any{"type": "execute/initialResponse", "id": execute["id"],
			"result": map[string]any{"status": "ok", "value": runtimeOutput("Yielded", "cell", ContentItem{Type: "input_text", Text: "first"})}})
		wait := receive(t, conn)
		reply(t, conn, wait["id"], map[string]any{"type": "wait/completed", "outcome": map[string]any{"LiveCell": runtimeOutput("Result", "cell",
			ContentItem{Type: "input_image", ImageURL: "data:image/png;base64,aGVsbG8=", Detail: "original"},
			ContentItem{Type: "input_audio", AudioURL: "data:audio/wav;base64,aGVsbG8="})}})
		// Stay alive until the client has consumed its last response.
		_, _ = io.Copy(io.Discard, conn)
	})
	yielded, err := client.Execute(context.Background(), ExecuteRequest{ToolCallID: "outer", Source: `text("first"); await yield_control();`})
	if err != nil || yielded.State != "Yielded" || len(yielded.Content) != 1 {
		t.Fatalf("execute: %+v, %v", yielded, err)
	}
	result, err := client.Wait(context.Background(), yielded.CellID, 100)
	if err != nil || result.State != "Result" || len(result.Content) != 2 {
		t.Fatalf("wait: %+v, %v", result, err)
	}
	if result.Content[0].Detail != "original" || result.Content[1].AudioURL == "" {
		t.Fatalf("multimodal output lost: %+v", result)
	}
}

type testDelegate struct {
	invoke func(context.Context, Invocation) (json.RawMessage, error)
	notify func(context.Context, string, string, string) error
}

func (d testDelegate) Invoke(ctx context.Context, i Invocation) (json.RawMessage, error) {
	return d.invoke(ctx, i)
}
func (d testDelegate) Notify(ctx context.Context, call, cell, text string) error {
	return d.notify(ctx, call, cell, text)
}

func TestDelegateCancellationDoesNotBlockOtherCallbacks(t *testing.T) {
	started := make(chan struct{})
	canceled := make(chan struct{})
	delivered := make(chan struct{})
	delegate := testDelegate{
		invoke: func(ctx context.Context, invocation Invocation) (json.RawMessage, error) {
			if invocation.ToolName.Name != "read_file" {
				t.Error("wrong nested tool")
			}
			close(started)
			<-ctx.Done()
			close(canceled)
			return nil, ctx.Err()
		},
		notify: func(ctx context.Context, call, cell, text string) error { close(delivered); return nil },
	}
	client := testClient(t, delegate, func(conn net.Conn) {
		execute := receive(t, conn)
		reply(t, conn, execute["id"], map[string]any{"type": "execution/started", "cellId": "cell"})
		transmit(t, conn, map[string]any{"type": "delegate/request", "id": 11, "sessionId": "session", "request": map[string]any{
			"type": "tool/invoke", "invocation": Invocation{CellID: "cell", RuntimeToolCallID: "nested", ToolName: ToolName{Name: "read_file"}, ToolKind: "function", Input: json.RawMessage(`{"path":"a.go"}`)}}})
		<-started
		transmit(t, conn, map[string]any{"type": "delegate/request", "id": 12, "sessionId": "session", "request": map[string]any{
			"type": "notification/send", "callId": "outer", "cellId": "cell", "text": "progress"}})
		<-delivered
		notification := receive(t, conn)
		if string(notification["id"]) != "12" {
			t.Error("notification blocked by invoke")
		}
		transmit(t, conn, map[string]any{"type": "delegate/cancel", "id": 11})
		<-canceled
		invocation := receive(t, conn)
		var result wireResult
		_ = json.Unmarshal(invocation["result"], &result)
		if string(invocation["id"]) != "11" || result.Status != "error" {
			t.Error("canceled invoke not acknowledged")
		}
		transmit(t, conn, map[string]any{"type": "execute/initialResponse", "id": execute["id"],
			"result": map[string]any{"status": "ok", "value": runtimeOutput("Result", "cell")}})
		_, _ = io.Copy(io.Discard, conn)
	})
	if _, err := client.Execute(context.Background(), ExecuteRequest{ToolCallID: "outer"}); err != nil {
		t.Fatal(err)
	}
}

func TestCanceledExecuteInvalidatesSessionWithoutReplay(t *testing.T) {
	accepted := make(chan struct{})
	var executions atomic.Int32
	client := testClient(t, nil, func(conn net.Conn) {
		_ = receive(t, conn)
		executions.Add(1)
		close(accepted)
		// Deliberately never acknowledge: execution may already have written.
		var message any
		if err := readFrame(conn, &message); err == nil {
			t.Error("unexpected replay after cancellation")
		}
	})
	ctx, cancel := context.WithCancel(context.Background())
	go func() { <-accepted; cancel() }()
	if _, err := client.Execute(ctx, ExecuteRequest{ToolCallID: "outer"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel: %v", err)
	}
	if _, err := client.Execute(context.Background(), ExecuteRequest{ToolCallID: "retry"}); err == nil {
		t.Fatal("dead session accepted retry")
	}
	if executions.Load() != 1 {
		t.Fatal("execute was replayed")
	}
}

func TestCrashCancelsPendingDelegate(t *testing.T) {
	started, stopped := make(chan struct{}), make(chan struct{})
	client := testClient(t, testDelegate{invoke: func(ctx context.Context, _ Invocation) (json.RawMessage, error) {
		close(started)
		<-ctx.Done()
		close(stopped)
		return nil, ctx.Err()
	}}, func(conn net.Conn) {
		_ = receive(t, conn)
		transmit(t, conn, map[string]any{"type": "delegate/request", "id": 1, "sessionId": "session", "request": map[string]any{
			"type": "tool/invoke", "invocation": Invocation{CellID: "cell"}}})
		<-started
		// The helper closes the transport, simulating a runtime crash.
	})
	if _, err := client.Execute(context.Background(), ExecuteRequest{ToolCallID: "outer"}); err == nil {
		t.Fatal("crash reported success")
	}
	select {
	case <-stopped:
	case <-time.After(5 * time.Second):
		t.Fatal("delegate survived host crash")
	}
}

func TestHandshakeRejectsMissingResourceLimits(t *testing.T) {
	local, remote := net.Pipe()
	go func() {
		defer remote.Close()
		_ = receive(t, remote)
		transmit(t, remote, map[string]any{"type": "connection/ready", "selectedVersion": 1, "capabilities": []string{}})
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := Connect(ctx, local, "session", CellLimits{}, nil); err == nil {
		t.Fatal("resource limits silently ignored")
	}
}

type shortWriter struct{ bytes.Buffer }

func (w *shortWriter) Write(p []byte) (int, error) {
	if len(p) > 2 {
		p = p[:2]
	}
	return w.Buffer.Write(p)
}

func TestFramingHandlesFragmentationAndRejectsInvalidLengths(t *testing.T) {
	frame, err := encodeFrame(map[string]string{"source": "text('多行');\ntext('next')"})
	if err != nil {
		t.Fatal(err)
	}
	var writer shortWriter
	if err := writeFrame(&writer, frame); err != nil {
		t.Fatal(err)
	}
	var decoded map[string]string
	if err := readFrame(&writer, &decoded); err != nil || decoded["source"] != "text('多行');\ntext('next')" {
		t.Fatalf("fragmented frame: %v, %v", decoded, err)
	}
	for _, length := range []uint32{0, maxFrameBytes + 1, ^uint32(0)} {
		var header [4]byte
		binary.LittleEndian.PutUint32(header[:], length)
		if err := readFrame(bytes.NewReader(header[:]), &decoded); err == nil {
			t.Errorf("accepted length %d", length)
		}
	}
	if err := readFrame(bytes.NewReader(frame[:len(frame)-1]), &decoded); !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("truncated frame: %v", err)
	}
}

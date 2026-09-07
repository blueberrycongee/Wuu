package codemode

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"testing"
)

func TestWaitCancellationRetainsOutputAndSession(t *testing.T) {
	requested, release := make(chan struct{}), make(chan struct{})
	client := testClient(t, nil, func(conn net.Conn) {
		wait := receive(t, conn)
		close(requested)
		<-release
		reply(t, conn, wait["id"], map[string]any{"type": "wait/completed", "outcome": map[string]any{"LiveCell": runtimeOutput("Result", "cell", ContentItem{Type: "input_text", Text: "retained output"})}})
		// A fresh execution, rather than a replayed wait, must be next.
		execute := receive(t, conn)
		var operation struct {
			Method string `json:"method"`
		}
		if err := json.Unmarshal(execute["request"], &operation); err != nil || operation.Method != "session/execute" {
			t.Errorf("canceled observer caused a duplicate host wait: %s", execute["request"])
			return
		}
		reply(t, conn, execute["id"], map[string]any{"type": "execution/started", "cellId": "next"})
		transmit(t, conn, map[string]any{"type": "execute/initialResponse", "id": execute["id"], "result": map[string]any{"status": "ok", "value": runtimeOutput("Result", "next")}})
		_, _ = io.Copy(io.Discard, conn)
	})
	ctx, cancel := context.WithCancel(context.Background())
	finished := make(chan error, 1)
	go func() { _, err := client.Wait(ctx, "cell", 100); finished <- err }()
	<-requested
	_, concurrentErr := client.Wait(context.Background(), "cell", 100)
	cancel()
	err := <-finished
	close(release)
	if concurrentErr == nil {
		t.Fatal("concurrent observer accepted")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("detach: %v", err)
	}
	result, err := client.Wait(context.Background(), "cell", 1)
	if err != nil || len(result.Content) != 1 || result.Content[0].Text != "retained output" {
		t.Fatalf("lost detached output: %+v, %v", result, err)
	}
	if _, err := client.Execute(context.Background(), ExecuteRequest{ToolCallID: "next", Source: "1"}); err != nil {
		t.Fatalf("observer canceled session: %v", err)
	}
}

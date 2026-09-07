package codemode

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
)

// Start launches an explicitly selected host binary over stdio. It never
// searches for a model CLI, invokes a shell, downloads a runtime, or falls back
// to executing JavaScript in a trusted extension process. ctx owns the returned
// session lifetime, including yielded cells.
func Start(ctx context.Context, executable, sessionID string, limits CellLimits, delegate Delegate, stderr io.Writer) (*Client, error) {
	if !filepath.IsAbs(executable) {
		return nil, fmt.Errorf("code-mode host must be an absolute executable path")
	}
	cmd := exec.Command(executable, "--listen", "stdio")
	cmd.Stderr = stderr
	input, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	output, err := cmd.StdoutPipe()
	if err != nil {
		_ = input.Close()
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		_ = input.Close()
		_ = output.Close()
		return nil, fmt.Errorf("start code-mode host: %w", err)
	}
	transport := &processTransport{input: input, output: output, cmd: cmd}
	// Attach cancellation before the handshake; a hung startup must also die.
	stop := context.AfterFunc(ctx, func() { _ = cmd.Process.Kill() })
	client, err := Connect(ctx, transport, sessionID, limits, delegate)
	if err != nil {
		stop()
		return nil, err
	}
	go func() {
		<-client.done
		stop()
	}()
	return client, nil
}

type processTransport struct {
	input  io.WriteCloser
	output io.ReadCloser
	cmd    *exec.Cmd
}

func (p *processTransport) Read(data []byte) (int, error)  { return p.output.Read(data) }
func (p *processTransport) Write(data []byte) (int, error) { return p.input.Write(data) }

// Client calls transport.Close exactly once. Kill is intentional: graceful
// shutdown cannot be trusted while an isolate is stuck or its protocol failed.
func (p *processTransport) Close() error {
	_ = p.input.Close()
	_ = p.output.Close()
	_ = p.cmd.Process.Kill()
	_ = p.cmd.Wait()
	return nil
}

// Package codexengine integrates the Codex CLI app-server as an external
// agent engine behind the agentengine seam. The app-server is a long-lived
// child process speaking JSON-RPC over NDJSON on stdio (no jsonrpc:"2.0"
// envelope field); one shared app-server can host many threads.
package codexengine

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// maxLineBytes bounds a single NDJSON line. Codex notifications are normally
// well under 1MB; reasoning deltas can be large, so 16MB matches the reference
// client and guards the parser against pathological input.
const maxLineBytes = 16 * 1024 * 1024

// Transport is the NDJSON framing layer over a codex app-server process.
// It owns the child process lifecycle; the client owns JSON-RPC semantics.
type Transport struct {
	cmd *exec.Cmd

	mu       sync.Mutex
	stdin    io.WriteCloser
	closed   bool
	closeErr error

	lineHandlers   []func(string)
	stderrHandlers []func(string)
	closeHandlers  []func(string)

	// buffered lines received between spawn and the first line handler.
	// This races a slow codex startup: the client subscribes only after
	// spawn returns, and the first NDJSON chunk can arrive immediately.
	lineBuffer []string
	armed      bool
}

// TransportOptions configures a codex app-server subprocess.
type TransportOptions struct {
	// BinaryPath is the absolute path to the codex executable.
	BinaryPath string
	// CWD for the child; empty inherits the parent.
	CWD string
	// Env for the child; nil inherits the parent environment.
	Env []string
	// ExtraArgs are appended after the app-server subcommand.
	ExtraArgs []string
	// ReadyTimeout bounds the time the child may take before the first
	// output line (initialize handshake); zero uses the default.
	ReadyTimeout time.Duration
}

// NewTransport spawns the codex app-server child process.
func NewTransport(opts TransportOptions) (*Transport, error) {
	if strings.TrimSpace(opts.BinaryPath) == "" {
		return nil, errors.New("codex binary path is required")
	}
	args := append([]string{"app-server"}, opts.ExtraArgs...)
	cmd := exec.Command(opts.BinaryPath, args...)
	cmd.Dir = opts.CWD
	cmd.Env = opts.Env
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("codex app-server stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("codex app-server stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("codex app-server stderr pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start codex app-server: %w", err)
	}
	t := &Transport{cmd: cmd, stdin: stdin}
	t.readLoop(stdout)
	t.stderrLoop(stderr)
	return t, nil
}

// readLoop frames stdout into NDJSON lines and fans them out. The first
// handler registration drains the buffer so no early chunk is lost.
func (t *Transport) readLoop(r io.Reader) {
	go func() {
		reader := bufio.NewReaderSize(r, 64*1024)
		for {
			line, err := readLine(reader)
			if len(line) > 0 {
				t.mu.Lock()
				if !t.armed {
					t.lineBuffer = append(t.lineBuffer, line)
				} else {
					handlers := make([]func(string), len(t.lineHandlers))
					copy(handlers, t.lineHandlers)
					t.mu.Unlock()
					for _, h := range handlers {
						h(line)
					}
					continue
				}
				t.mu.Unlock()
			}
			if err != nil {
				if !errors.Is(err, io.EOF) {
					t.fireClose(fmt.Sprintf("read stdout: %v", err))
				} else {
					t.fireClose("codex app-server exited")
				}
				return
			}
		}
	}()
}

// stderrLoop forwards stderr lines to diagnostics handlers.
func (t *Transport) stderrLoop(r io.Reader) {
	go func() {
		reader := bufio.NewReaderSize(r, 64*1024)
		for {
			line, err := readLine(reader)
			if len(line) > 0 {
				t.mu.Lock()
				handlers := make([]func(string), len(t.stderrHandlers))
				copy(handlers, t.stderrHandlers)
				t.mu.Unlock()
				for _, h := range handlers {
					h(line)
				}
			}
			if err != nil {
				return
			}
		}
	}()
}

// readLine reads one line up to maxLineBytes.
func readLine(r *bufio.Reader) (string, error) {
	var sb strings.Builder
	for {
		chunk, err := r.ReadSlice('\n')
		if len(chunk) > 0 {
			line := strings.TrimSuffix(string(chunk), "\n")
			line = strings.TrimSuffix(line, "\r")
			if sb.Len()+len(line) > maxLineBytes {
				return "", fmt.Errorf("codex app-server line exceeds %d bytes", maxLineBytes)
			}
			sb.WriteString(line)
		}
		if err == nil {
			return sb.String(), nil
		}
		if errors.Is(err, bufio.ErrBufferFull) {
			continue
		}
		return sb.String(), err
	}
}

// WriteLine sends one NDJSON line to the child.
func (t *Transport) WriteLine(ctx context.Context, line string) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		if t.closeErr != nil {
			return t.closeErr
		}
		return errors.New("codex app-server transport is closed")
	}
	done := make(chan error, 1)
	go func() {
		_, err := io.WriteString(t.stdin, line+"\n")
		done <- err
	}()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// OnLine registers a handler for each stdout line. Handlers registered after
// the first drain receive only subsequent lines.
func (t *Transport) OnLine(h func(string)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.lineHandlers = append(t.lineHandlers, h)
	if !t.armed {
		t.armed = true
		if len(t.lineBuffer) > 0 {
			buffered := append([]string(nil), t.lineBuffer...)
			t.lineBuffer = nil
			for _, line := range buffered {
				for _, hh := range t.lineHandlers {
					hh(line)
				}
			}
		}
	}
}

// OnStderr registers a handler for child stderr lines (diagnostics only).
func (t *Transport) OnStderr(h func(string)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.stderrHandlers = append(t.stderrHandlers, h)
}

// OnClose registers a handler for transport termination (process exit or
// explicit close). Handlers receive the reason string.
func (t *Transport) OnClose(h func(reason string)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.closeHandlers = append(t.closeHandlers, h)
}

func (t *Transport) fireClose(reason string) {
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return
	}
	t.closed = true
	t.closeErr = errors.New(reason)
	handlers := make([]func(string), len(t.closeHandlers))
	copy(handlers, t.closeHandlers)
	t.mu.Unlock()
	for _, h := range handlers {
		h(reason)
	}
}

// Close terminates the child: stdin EOF first (the Rust app-server exits on
// its own), SIGTERM as a fallback.
func (t *Transport) Close() error {
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return t.closeErr
	}
	t.closed = true
	stdin := t.stdin
	t.mu.Unlock()

	_ = stdin.Close()
	done := make(chan error, 1)
	go func() { done <- t.cmd.Wait() }()
	select {
	case err := <-done:
		if err == nil {
			err = errors.New("codex app-server closed")
		}
		return err
	case <-time.After(2 * time.Second):
		_ = t.cmd.Process.Kill()
		<-done
		return errors.New("codex app-server did not exit after stdin EOF; killed")
	}
}

// ProcessState reports the child's exit state once it has exited.
func (t *Transport) ProcessState() *os.ProcessState {
	if t.cmd.ProcessState == nil {
		return nil
	}
	return t.cmd.ProcessState
}

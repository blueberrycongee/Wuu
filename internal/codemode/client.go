package codemode

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"sync"
)

// Delegate is the owning execution scope's bridge to tools and notifications.
// Implementations must honor cancellation. Invoke may run concurrently; tool
// scheduling, permissions and durable execution records belong to the caller.
type Delegate interface {
	Invoke(context.Context, Invocation) (json.RawMessage, error)
	Notify(context.Context, string, string, string) error
}

type delegateRun struct {
	cellID string
	cancel context.CancelFunc
}

// Client owns one host connection and one in-memory session. A connection must
// not be shared between Wuu sessions. Transport failure or cancellation of an
// in-flight execution invalidates the session and cancels its delegates; the
// client never reconnects or replays code with potentially completed effects.
// Close must also stop the host process when using a process-backed transport.
type Client struct {
	transport io.ReadWriteCloser
	sessionID string
	delegate  Delegate
	ctx       context.Context
	cancel    context.CancelFunc
	outgoing  chan []byte
	hello     chan hostMessage
	done      chan struct{}
	once      sync.Once
	workers   sync.WaitGroup

	mu        sync.Mutex
	err       error
	nextID    int64
	pending   map[int64]chan hostMessage
	delegates map[int64]delegateRun
	waits     map[string]*cellWait
}

// Connect takes ownership of transport even on failure. ctx bounds handshake
// and session opening, not the returned session lifetime. Transport.Close must
// unblock concurrent reads and writes. Resource limits are negotiated rather
// than silently ignored by older hosts.
func Connect(ctx context.Context, transport io.ReadWriteCloser, sessionID string, limits CellLimits, delegate Delegate) (*Client, error) {
	if sessionID == "" || transport == nil {
		if transport != nil {
			_ = transport.Close()
		}
		return nil, errors.New("code-mode connection requires a transport and session ID")
	}
	life, cancel := context.WithCancel(context.Background())
	c := &Client{transport: transport, sessionID: sessionID, delegate: delegate,
		ctx: life, cancel: cancel, outgoing: make(chan []byte, 32), hello: make(chan hostMessage, 1),
		done: make(chan struct{}), pending: make(map[int64]chan hostMessage), delegates: make(map[int64]delegateRun)}
	c.workers.Add(2)
	go c.readLoop()
	go c.writeLoop()
	err := c.send(ctx, map[string]any{"type": "connection/hello", "supportedVersions": []int{1},
		"requiredCapabilities": []string{resourceLimitsCapability}, "optionalCapabilities": []string{}})
	if err == nil {
		select {
		case hello := <-c.hello:
			if hello.Type != "connection/ready" || hello.SelectedVersion != 1 || !slices.Contains(hello.Capabilities, resourceLimitsCapability) {
				err = fmt.Errorf("incompatible code-mode host handshake: %s (version %d, reason %s)", hello.Type, hello.SelectedVersion, hello.Reason)
			}
		case <-ctx.Done():
			err = ctx.Err()
		case <-c.done:
			err = c.failure()
		}
	}
	if err == nil {
		var reply json.RawMessage
		reply, err = c.request(ctx, map[string]any{"method": "session/open", "sessionId": sessionID, "cellExecutionLimits": limits}, false)
		if err == nil {
			var value struct {
				Type      string `json:"type"`
				SessionID string `json:"sessionId"`
			}
			err = json.Unmarshal(reply, &value)
			if err == nil && (value.Type != "session/ready" || value.SessionID != sessionID) {
				err = errors.New("invalid code-mode session-open response")
			}
		}
	}
	if err != nil {
		c.fail(err)
		_ = c.Close()
		return nil, err
	}
	return c, nil
}

func (c *Client) Execute(ctx context.Context, request ExecuteRequest) (Response, error) {
	if request.ToolCallID == "" || (request.MaxOutputTokens != nil && *request.MaxOutputTokens < 0) {
		return Response{}, errors.New("invalid code-mode execute request")
	}
	if request.EnabledTools == nil {
		request.EnabledTools = []ToolDefinition{}
	}
	data, err := c.request(ctx, map[string]any{"method": "session/execute", "sessionId": c.sessionID, "request": request}, true)
	if err != nil {
		return Response{}, err
	}
	return decodeResponse(data)
}

// Wait observes cell output. Canceling ctx detaches only this observer; the
// next Wait for this cell consumes the retained response before issuing a new
// host wait. Concurrent observers of the same cell are rejected. To stop the
// cell itself, use Terminate or Close.
func (c *Client) Wait(ctx context.Context, cellID string, yieldTimeMS uint64) (Response, error) {
	return c.waitCell(ctx, cellID, yieldTimeMS)
}

func (c *Client) Terminate(ctx context.Context, cellID string) (Response, error) {
	return c.observe(ctx, cellID, map[string]any{"method": "session/terminate", "sessionId": c.sessionID, "cellId": cellID})
}

func (c *Client) observe(ctx context.Context, cellID string, request any) (Response, error) {
	if cellID == "" {
		return Response{}, errors.New("code-mode cell ID is required")
	}
	data, err := c.request(ctx, request, false)
	if err != nil {
		return Response{}, err
	}
	var reply struct {
		Type    string                     `json:"type"`
		Outcome map[string]json.RawMessage `json:"outcome"`
	}
	if err := json.Unmarshal(data, &reply); err != nil {
		return Response{}, err
	}
	if reply.Type != "wait/completed" || len(reply.Outcome) != 1 {
		return Response{}, errors.New("invalid code-mode wait response")
	}
	for kind, body := range reply.Outcome {
		if kind != "LiveCell" && kind != "MissingCell" {
			return Response{}, fmt.Errorf("invalid code-mode wait outcome %q", kind)
		}
		response, err := decodeResponse(body)
		if err == nil && response.CellID != cellID {
			return Response{}, errors.New("code-mode wait returned a different cell")
		}
		response.Missing = kind == "MissingCell"
		return response, err
	}
	panic("unreachable")
}

func (c *Client) request(ctx context.Context, request any, execute bool) (json.RawMessage, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	c.mu.Lock()
	if c.err != nil {
		err := c.err
		c.mu.Unlock()
		return nil, err
	}
	if len(c.pending) >= 128 {
		c.mu.Unlock()
		return nil, errors.New("too many pending code-mode operations")
	}
	c.nextID++
	id := c.nextID
	replies := make(chan hostMessage, 2) // execute has start and initial-output frames
	c.pending[id] = replies
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
	}()
	if err := c.send(ctx, map[string]any{"type": "operation/request", "id": id, "request": request}); err != nil {
		return nil, err
	}
	var startedCell string
	var initial json.RawMessage
	for {
		select {
		case <-ctx.Done():
			// Execute can have started before its acknowledgement arrives. Merely
			// canceling the observation can leave JS and writes running unseen.
			c.fail(fmt.Errorf("code-mode session invalidated: %w", ctx.Err()))
			return nil, ctx.Err()
		case <-c.done:
			return nil, c.failure()
		case reply := <-replies:
			value, err := reply.Result.unwrap()
			if err != nil {
				return nil, err
			}
			if !execute {
				if reply.Type != "operation/response" {
					return nil, errors.New("unexpected code-mode initial response")
				}
				return value, nil
			}
			if reply.Type == "execute/initialResponse" {
				initial = value
			} else {
				var started struct {
					Type   string `json:"type"`
					CellID string `json:"cellId"`
				}
				if err := json.Unmarshal(value, &started); err != nil {
					return nil, err
				}
				if started.Type != "execution/started" || started.CellID == "" {
					return nil, errors.New("invalid code-mode execution acknowledgement")
				}
				startedCell = started.CellID
			}
			if startedCell != "" && initial != nil {
				result, err := decodeResponse(initial)
				if err != nil || result.CellID != startedCell {
					c.fail(errors.New("invalid code-mode initial cell response"))
					return nil, c.failure()
				}
				return initial, nil
			}
		}
	}
}

func (c *Client) send(ctx context.Context, value any) error {
	frame, err := encodeFrame(value)
	if err != nil {
		return err
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-c.done:
		return c.failure()
	case c.outgoing <- frame:
		return nil
	}
}

func (c *Client) writeLoop() {
	defer c.workers.Done()
	for {
		select {
		case <-c.done:
			return
		case frame := <-c.outgoing:
			if err := writeFrame(c.transport, frame); err != nil {
				c.fail(fmt.Errorf("write code-mode host: %w", err))
				return
			}
		}
	}
}

func (c *Client) readLoop() {
	defer c.workers.Done()
	handshaken := false
	for {
		var message hostMessage
		if err := readFrame(c.transport, &message); err != nil {
			c.fail(fmt.Errorf("read code-mode host: %w", err))
			return
		}
		if !handshaken {
			if message.Type != "connection/ready" && message.Type != "connection/rejected" {
				c.fail(errors.New("code-mode host did not send a handshake"))
				return
			}
			handshaken = true
			c.hello <- message
			continue
		}
		switch message.Type {
		case "operation/response", "execute/initialResponse":
			c.mu.Lock()
			replies := c.pending[message.ID]
			c.mu.Unlock()
			if replies != nil {
				select {
				case replies <- message:
				default:
					c.fail(errors.New("too many code-mode response frames"))
					return
				}
			}
		case "delegate/request":
			c.startDelegate(message)
		case "delegate/cancel":
			c.mu.Lock()
			if run, ok := c.delegates[message.ID]; ok {
				run.cancel()
			}
			c.mu.Unlock()
		case "cell/closed":
			if message.SessionID != c.sessionID {
				c.fail(errors.New("code-mode cell belongs to another session"))
				return
			}
			c.mu.Lock()
			for _, run := range c.delegates {
				if run.cellID == message.CellID {
					run.cancel()
				}
			}
			c.mu.Unlock()
		default:
			c.fail(fmt.Errorf("unknown code-mode host message %q", message.Type))
			return
		}
	}
}

func (c *Client) startDelegate(message hostMessage) {
	if message.SessionID != c.sessionID {
		c.fail(errors.New("code-mode delegate belongs to another session"))
		return
	}
	c.mu.Lock()
	_, duplicate := c.delegates[message.ID]
	if duplicate || len(c.delegates) >= 128 {
		c.mu.Unlock()
		c.fail(errors.New("invalid or excessive code-mode delegate requests"))
		return
	}
	ctx, cancel := context.WithCancel(c.ctx)
	cellID := message.Request.CellID
	if message.Request.Type == "tool/invoke" {
		cellID = message.Request.Invocation.CellID
	}
	c.delegates[message.ID] = delegateRun{cellID: cellID, cancel: cancel}
	c.mu.Unlock()
	go func() {
		defer func() {
			cancel()
			c.mu.Lock()
			delete(c.delegates, message.ID)
			c.mu.Unlock()
		}()
		value, err := c.invokeDelegate(ctx, message.Request)
		result := map[string]any{"status": "ok", "value": value}
		if err != nil {
			result = map[string]any{"status": "error", "message": err.Error()}
		}
		// Delegate cancellation is acknowledged as an error; it must not block
		// unrelated callbacks or the read loop that delivers cancellation.
		if err := c.send(c.ctx, map[string]any{"type": "delegate/response", "id": message.ID, "result": result}); err != nil && c.ctx.Err() == nil {
			c.fail(err)
		}
	}()
}

func (c *Client) invokeDelegate(ctx context.Context, request delegateRequest) (value any, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			value, err = nil, fmt.Errorf("code-mode delegate panicked: %v", recovered)
		}
	}()
	if c.delegate == nil {
		return nil, errors.New("code-mode execution delegate is unavailable")
	}
	switch request.Type {
	case "tool/invoke":
		result, err := c.delegate.Invoke(ctx, request.Invocation)
		if err != nil {
			return nil, err
		}
		if !json.Valid(result) {
			return nil, errors.New("code-mode delegate returned invalid JSON")
		}
		return map[string]any{"type": "tool/result", "result": result}, nil
	case "notification/send":
		if err := c.delegate.Notify(ctx, request.CallID, request.CellID, request.Text); err != nil {
			return nil, err
		}
		return map[string]any{"type": "notification/delivered"}, nil
	default:
		return nil, fmt.Errorf("unsupported code-mode delegate request %q", request.Type)
	}
}

func (c *Client) fail(err error) {
	c.once.Do(func() {
		c.mu.Lock()
		c.err = err
		c.mu.Unlock()
		c.cancel()
		close(c.done)
		_ = c.transport.Close()
	})
}

func (c *Client) failure() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.err
}

// Close cancels all cells and delegates and joins transport workers. It does
// not wait on arbitrary delegate code that ignores its cancellation context.
func (c *Client) Close() error {
	c.fail(errors.New("code-mode session closed"))
	c.workers.Wait()
	return nil
}

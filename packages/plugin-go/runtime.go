// Package pluginapi implements the public multiplexed JSON-lines runtime used
// by Wuu plugin helper processes. Host requests and plugin-initiated host
// service calls share one full-duplex channel and may be in flight at the same
// time. The package intentionally contains no imports from Wuu's internal
// Agent, app-server, or Desktop implementations.
package pluginapi

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"sync/atomic"
)

const CapabilityProtocolVersion = 2

type InitializeParams struct {
	ProtocolVersion           int      `json:"protocol_version"`
	CapabilityProtocolVersion int      `json:"capability_protocol_version,omitempty"`
	PluginID                  string   `json:"plugin_id"`
	PluginRoot                string   `json:"plugin_root"`
	ProjectRoot               string   `json:"project_root"`
	WuuHome                   string   `json:"wuu_home"`
	SupportedHostServices     []string `json:"supported_host_services,omitempty"`
}

type Capability struct {
	ID          string `json:"id"`
	Kind        string `json:"kind"`
	ErrorPolicy string `json:"error_policy,omitempty"`
	Version     int    `json:"version"`
	Priority    int    `json:"priority,omitempty"`
}

type HostService struct {
	ID       string `json:"id"`
	Required bool   `json:"required,omitempty"`
}

type Tool struct {
	ID              string         `json:"id"`
	Description     string         `json:"description"`
	InputSchema     map[string]any `json:"input_schema"`
	ExecutionScopes []string       `json:"execution_scopes,omitempty"`
	Activity        *ToolActivity  `json:"activity,omitempty"`
}

type ToolActivity struct {
	ReadOnly        bool   `json:"read_only,omitempty"`
	ConcurrencySafe bool   `json:"concurrency_safe,omitempty"`
	Destructive     bool   `json:"destructive,omitempty"`
	Risk            string `json:"risk,omitempty"`
	Reason          string `json:"reason,omitempty"`
}

type Definition struct {
	Tools                []Tool        `json:"tools,omitempty"`
	Capabilities         []Capability  `json:"capabilities,omitempty"`
	RequiredHostServices []HostService `json:"required_host_services,omitempty"`
}

type ToolCall struct {
	ToolID    string          `json:"tool_id"`
	SessionID string          `json:"session_id,omitempty"`
	ThreadID  string          `json:"thread_id,omitempty"`
	ActorID   string          `json:"actor_id,omitempty"`
	ActorPath string          `json:"actor_path,omitempty"`
	CWD       string          `json:"cwd"`
	StepIndex int             `json:"step_index,omitempty"`
	CallID    string          `json:"call_id"`
	Tool      string          `json:"tool"`
	Arguments json.RawMessage `json:"arguments"`
}

type ContentPart struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type ToolResult struct {
	Content           []ContentPart   `json:"content,omitempty"`
	StructuredContent json.RawMessage `json:"structured_content,omitempty"`
	Meta              json.RawMessage `json:"meta,omitempty"`
	IsError           bool            `json:"is_error,omitempty"`
}

func TextResult(text string) ToolResult {
	if text == "" {
		return ToolResult{}
	}
	return ToolResult{Content: []ContentPart{{Type: "text", Text: text}}}
}

type CapabilityCall struct {
	Capability string          `json:"capability"`
	Input      json.RawMessage `json:"input"`
	Output     json.RawMessage `json:"output"`
}

type Host interface {
	InitializeParams() InitializeParams
	CallHost(context.Context, string, any, any) error
}

type Handler struct {
	Definition       Definition
	Initialize       func(context.Context, Host, InitializeParams) error
	ExecuteTool      func(context.Context, Host, ToolCall) (ToolResult, error)
	InvokeCapability func(context.Context, Host, CapabilityCall) (json.RawMessage, error)
}

type Client struct {
	output    io.Writer
	seq       atomic.Uint64
	initMu    sync.RWMutex
	init      InitializeParams
	writeMu   sync.Mutex
	pendingMu sync.Mutex
	pending   map[string]chan rpcResponse
	done      chan struct{}
	doneOnce  sync.Once
	errMu     sync.Mutex
	readErr   error
}

func (c *Client) InitializeParams() InitializeParams {
	c.initMu.RLock()
	defer c.initMu.RUnlock()
	return c.init
}

func (c *Client) CallHost(ctx context.Context, method string, params, result any) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	id := fmt.Sprintf("plugin-%d", c.seq.Add(1))
	rawParams, err := json.Marshal(params)
	if err != nil {
		return err
	}
	responseCh := make(chan rpcResponse, 1)
	c.pendingMu.Lock()
	c.pending[id] = responseCh
	c.pendingMu.Unlock()
	defer c.removePending(id)
	if err := c.write(rpcRequest{ID: id, Method: strings.TrimSpace(method), Params: rawParams}); err != nil {
		return err
	}
	select {
	case response := <-responseCh:
		if response.Error != nil {
			return errors.New(strings.TrimSpace(response.Error.Message))
		}
		if result == nil {
			return nil
		}
		if len(response.Result) == 0 {
			return errors.New("host service response is missing result")
		}
		return json.Unmarshal(response.Result, result)
	case <-ctx.Done():
		return ctx.Err()
	case <-c.done:
		return c.transportError()
	}
}

func newClient(output io.Writer) *Client {
	return &Client{output: output, pending: make(map[string]chan rpcResponse), done: make(chan struct{})}
}

func (c *Client) write(value any) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return writeJSONLine(c.output, value)
}

func (c *Client) removePending(id string) {
	c.pendingMu.Lock()
	delete(c.pending, id)
	c.pendingMu.Unlock()
}

func (c *Client) routeResponse(response rpcResponse) {
	c.pendingMu.Lock()
	responseCh := c.pending[response.ID]
	c.pendingMu.Unlock()
	if responseCh != nil {
		responseCh <- response
	}
}

func (c *Client) closeTransport(err error) {
	c.errMu.Lock()
	if c.readErr == nil {
		if err == nil {
			err = io.EOF
		}
		c.readErr = err
	}
	c.errMu.Unlock()
	c.doneOnce.Do(func() { close(c.done) })
}

func (c *Client) transportError() error {
	c.errMu.Lock()
	defer c.errMu.Unlock()
	if c.readErr == nil {
		return io.EOF
	}
	return c.readErr
}

type rpcRequest struct {
	ID     string          `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
}

type rpcResponse struct {
	ID     string          `json:"id"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    string `json:"code,omitempty"`
	Message string `json:"message"`
}

func Serve(ctx context.Context, handler Handler) error {
	return ServeIO(ctx, os.Stdin, os.Stdout, handler)
}

func ServeIO(ctx context.Context, input io.Reader, output io.Writer, handler Handler) error {
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 64*1024), 4<<20)
	client := newClient(output)
	requests := make(chan rpcRequest)
	workerDone := make(chan error, 1)
	go func() {
		for request := range requests {
			result, stop, err := dispatch(ctx, client, handler, request)
			response := rpcResponse{ID: request.ID, Result: result}
			if err != nil {
				response = rpcResponse{ID: request.ID, Error: &rpcError{Message: err.Error()}}
			}
			if writeErr := client.write(response); writeErr != nil {
				workerDone <- writeErr
				return
			}
			if stop {
				workerDone <- nil
				return
			}
		}
		workerDone <- nil
	}()

	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			client.closeTransport(err)
			close(requests)
			return err
		}
		kind, request, response, err := decodeMessage(scanner.Bytes())
		if err != nil {
			client.closeTransport(err)
			close(requests)
			return err
		}
		if kind == messageResponse {
			client.routeResponse(response)
			continue
		}
		select {
		case requests <- request:
		case err := <-workerDone:
			client.closeTransport(err)
			return err
		case <-ctx.Done():
			client.closeTransport(ctx.Err())
			return ctx.Err()
		}
	}
	readErr := scanner.Err()
	client.closeTransport(readErr)
	close(requests)
	workerErr := <-workerDone
	if readErr != nil {
		return readErr
	}
	return workerErr
}

type messageKind int

const (
	messageRequest messageKind = iota
	messageResponse
)

func decodeMessage(line []byte) (messageKind, rpcRequest, rpcResponse, error) {
	var envelope struct {
		ID     string          `json:"id"`
		Method string          `json:"method"`
		Params json.RawMessage `json:"params"`
		Result json.RawMessage `json:"result"`
		Error  *rpcError       `json:"error"`
	}
	if err := json.Unmarshal(line, &envelope); err != nil {
		return 0, rpcRequest{}, rpcResponse{}, fmt.Errorf("decode runtime message: %w", err)
	}
	if strings.TrimSpace(envelope.ID) == "" {
		return 0, rpcRequest{}, rpcResponse{}, errors.New("runtime message id is required")
	}
	if strings.TrimSpace(envelope.Method) != "" {
		if len(envelope.Result) != 0 || envelope.Error != nil {
			return 0, rpcRequest{}, rpcResponse{}, errors.New("host request cannot contain result or error")
		}
		return messageRequest, rpcRequest{ID: envelope.ID, Method: envelope.Method, Params: envelope.Params}, rpcResponse{}, nil
	}
	if len(envelope.Params) != 0 {
		return 0, rpcRequest{}, rpcResponse{}, errors.New("host response cannot contain params")
	}
	if (len(envelope.Result) == 0) == (envelope.Error == nil) {
		return 0, rpcRequest{}, rpcResponse{}, errors.New("host response must contain exactly one of result or error")
	}
	return messageResponse, rpcRequest{}, rpcResponse{ID: envelope.ID, Result: envelope.Result, Error: envelope.Error}, nil
}

func dispatch(ctx context.Context, client *Client, handler Handler, request rpcRequest) (json.RawMessage, bool, error) {
	switch request.Method {
	case "initialize":
		var params InitializeParams
		if err := json.Unmarshal(request.Params, &params); err != nil {
			return nil, false, err
		}
		client.initMu.Lock()
		client.init = params
		client.initMu.Unlock()
		if handler.Initialize != nil {
			if err := handler.Initialize(ctx, client, params); err != nil {
				return nil, false, err
			}
		}
		return marshal(struct {
			Definition
			Hooks           []string `json:"hooks"`
			ProtocolVersion int      `json:"protocol_version"`
		}{Definition: handler.Definition, Hooks: []string{}, ProtocolVersion: CapabilityProtocolVersion})
	case "tool.execute":
		if handler.ExecuteTool == nil {
			return nil, false, errors.New("tool execution is unavailable")
		}
		var call ToolCall
		if err := json.Unmarshal(request.Params, &call); err != nil {
			return nil, false, err
		}
		result, err := handler.ExecuteTool(ctx, client, call)
		if err != nil {
			return nil, false, err
		}
		return marshal(struct {
			Result ToolResult `json:"result"`
		}{Result: result})
	case "capability.invoke":
		if handler.InvokeCapability == nil {
			return nil, false, errors.New("capability invocation is unavailable")
		}
		var call CapabilityCall
		if err := json.Unmarshal(request.Params, &call); err != nil {
			return nil, false, err
		}
		value, err := handler.InvokeCapability(ctx, client, call)
		if err != nil {
			return nil, false, err
		}
		if len(value) == 0 || !json.Valid(value) {
			return nil, false, errors.New("capability returned invalid JSON")
		}
		return marshal(struct {
			Output json.RawMessage `json:"output"`
		}{Output: value})
	case "shutdown":
		return json.RawMessage(`{}`), true, nil
	default:
		return nil, false, fmt.Errorf("method %q is not supported", request.Method)
	}
}

func marshal(value any) (json.RawMessage, bool, error) {
	raw, err := json.Marshal(value)
	return raw, false, err
}

func writeJSONLine(output io.Writer, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = output.Write(append(raw, '\n'))
	return err
}

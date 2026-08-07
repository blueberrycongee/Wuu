// Package pluginapi implements the public JSON-lines runtime used by Wuu
// plugin helper processes. It intentionally contains no imports from Wuu's
// internal Agent, app-server, or Desktop implementations.
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
	ID       string `json:"id"`
	Kind     string `json:"kind"`
	Version  int    `json:"version"`
	Priority int    `json:"priority,omitempty"`
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
	scanner *bufio.Scanner
	output  io.Writer
	seq     atomic.Uint64
	init    InitializeParams
}

func (c *Client) InitializeParams() InitializeParams { return c.init }

func (c *Client) CallHost(ctx context.Context, method string, params, result any) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	id := fmt.Sprintf("plugin-%d", c.seq.Add(1))
	rawParams, err := json.Marshal(params)
	if err != nil {
		return err
	}
	if err := writeJSONLine(c.output, rpcRequest{ID: id, Method: strings.TrimSpace(method), Params: rawParams}); err != nil {
		return err
	}
	if !c.scanner.Scan() {
		if err := c.scanner.Err(); err != nil {
			return err
		}
		return io.EOF
	}
	var response rpcResponse
	if err := json.Unmarshal(c.scanner.Bytes(), &response); err != nil {
		return fmt.Errorf("decode host service response: %w", err)
	}
	if response.ID != id {
		return fmt.Errorf("host service response id %q does not match %q", response.ID, id)
	}
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
	client := &Client{scanner: scanner, output: output}
	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return err
		}
		var request rpcRequest
		if err := json.Unmarshal(scanner.Bytes(), &request); err != nil {
			return fmt.Errorf("decode host request: %w", err)
		}
		result, stop, err := dispatch(ctx, client, handler, request)
		if err != nil {
			if writeErr := writeJSONLine(output, rpcResponse{ID: request.ID, Error: &rpcError{Message: err.Error()}}); writeErr != nil {
				return writeErr
			}
		} else if err := writeJSONLine(output, rpcResponse{ID: request.ID, Result: result}); err != nil {
			return err
		}
		if stop {
			return nil
		}
	}
	return scanner.Err()
}

func dispatch(ctx context.Context, client *Client, handler Handler, request rpcRequest) (json.RawMessage, bool, error) {
	switch request.Method {
	case "initialize":
		var params InitializeParams
		if err := json.Unmarshal(request.Params, &params); err != nil {
			return nil, false, err
		}
		client.init = params
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

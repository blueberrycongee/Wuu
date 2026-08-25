package appserver

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/blueberrycongee/wuu/internal/providers"
)

const namedAgentMCPServerName = "wuu_collaboration"

var namedAgentMCPTools = map[string]struct{}{
	"chat_check": {}, "chat_read": {}, "chat_send": {}, "collaboration_send": {},
	"chat_draft": {}, "chat_task": {}, "chat_remind": {},
}

type namedAgentMCPRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type namedAgentMCPResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   any             `json:"error,omitempty"`
}

func (s *Server) namedAgentMCPURL(agentID string) (string, error) {
	agentID = strings.TrimSpace(agentID)
	if s == nil || s.closed.Load() || agentID == "" {
		return "", errors.New("named agent MCP bridge is unavailable")
	}
	if s.channelService == nil {
		return "", errors.New("channels service is unavailable")
	}
	if _, err := s.channelService.GetNamedAgent(context.Background(), agentID); err != nil {
		return "", err
	}

	s.namedAgentMCPMu.Lock()
	defer s.namedAgentMCPMu.Unlock()
	if s.namedAgentMCPServer == nil {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return "", fmt.Errorf("listen for named agent MCP: %w", err)
		}
		secretBytes := make([]byte, 32)
		if _, err := rand.Read(secretBytes); err != nil {
			_ = listener.Close()
			return "", fmt.Errorf("create named agent MCP capability: %w", err)
		}
		s.namedAgentMCPSecret = hex.EncodeToString(secretBytes)
		s.namedAgentMCPBaseURL = "http://" + listener.Addr().String()
		s.namedAgentMCPServer = &http.Server{
			Handler:           http.HandlerFunc(s.handleNamedAgentMCP),
			ReadHeaderTimeout: 5 * time.Second,
		}
		server := s.namedAgentMCPServer
		go func() {
			if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
				providers.DebugLogf("named agent MCP bridge: %v", err)
			}
		}()
	}
	return fmt.Sprintf("%s/%s/%s", s.namedAgentMCPBaseURL, s.namedAgentMCPSecret, agentID), nil
}

func (s *Server) closeNamedAgentMCP() {
	if s == nil {
		return
	}
	s.namedAgentMCPMu.Lock()
	server := s.namedAgentMCPServer
	s.namedAgentMCPServer = nil
	s.namedAgentMCPBaseURL = ""
	s.namedAgentMCPSecret = ""
	s.namedAgentMCPMu.Unlock()
	if server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
	}
}

func (s *Server) handleNamedAgentMCP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	s.namedAgentMCPMu.Lock()
	prefix := "/" + s.namedAgentMCPSecret + "/"
	s.namedAgentMCPMu.Unlock()
	if prefix == "//" || !strings.HasPrefix(r.URL.Path, prefix) {
		http.NotFound(w, r)
		return
	}
	agentID := strings.Trim(strings.TrimPrefix(r.URL.Path, prefix), "/")
	if agentID == "" || strings.Contains(agentID, "/") {
		http.NotFound(w, r)
		return
	}
	var request namedAgentMCPRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&request); err != nil {
		writeNamedAgentMCPResponse(w, namedAgentMCPResponse{JSONRPC: "2.0", ID: request.ID, Error: map[string]any{"code": -32700, "message": "invalid JSON-RPC request"}})
		return
	}
	if len(request.ID) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	response := namedAgentMCPResponse{JSONRPC: "2.0", ID: request.ID}
	switch request.Method {
	case "initialize":
		response.Result = map[string]any{
			"protocolVersion": "2025-03-26",
			"capabilities":    map[string]any{"tools": map[string]any{"listChanged": false}},
			"serverInfo":      map[string]any{"name": namedAgentMCPServerName, "version": "1"},
		}
	case "tools/list":
		toolkit, err := s.namedAgentToolkit(agentID)
		if err != nil {
			response.Error = map[string]any{"code": -32000, "message": err.Error()}
			break
		}
		definitions := make([]map[string]any, 0, len(namedAgentMCPTools))
		for _, definition := range toolkit.Definitions() {
			if _, ok := namedAgentMCPTools[definition.Name]; !ok {
				continue
			}
			definitions = append(definitions, map[string]any{
				"name": definition.Name, "description": definition.Description,
				"inputSchema": definition.InputSchema,
			})
		}
		response.Result = map[string]any{"tools": definitions}
	case "tools/call":
		var params struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if err := json.Unmarshal(request.Params, &params); err != nil {
			response.Error = map[string]any{"code": -32602, "message": "invalid tool arguments"}
			break
		}
		if _, ok := namedAgentMCPTools[params.Name]; !ok {
			response.Error = map[string]any{"code": -32602, "message": "unknown collaboration tool"}
			break
		}
		toolkit, err := s.namedAgentToolkit(agentID)
		if err != nil {
			response.Error = map[string]any{"code": -32000, "message": err.Error()}
			break
		}
		arguments := strings.TrimSpace(string(params.Arguments))
		if arguments == "" || arguments == "null" {
			arguments = "{}"
		}
		result, callErr := toolkit.Execute(r.Context(), providers.ToolCall{Name: params.Name, Arguments: arguments})
		content := []map[string]string{{"type": "text", "text": result}}
		response.Result = map[string]any{"content": content, "isError": callErr != nil}
		if callErr != nil {
			response.Result = map[string]any{"content": []map[string]string{{"type": "text", "text": callErr.Error()}}, "isError": true}
		}
	default:
		response.Error = map[string]any{"code": -32601, "message": "method not found"}
	}
	writeNamedAgentMCPResponse(w, response)
}

func (s *Server) namedAgentToolkit(agentID string) (interface {
	Definitions() []providers.ToolDefinition
	Execute(context.Context, providers.ToolCall) (string, error)
}, error) {
	if s == nil {
		return nil, errors.New("named agent MCP bridge is unavailable")
	}
	if s.channelService == nil {
		return nil, errors.New("channels service is unavailable")
	}
	agent, err := s.channelService.GetNamedAgent(context.Background(), agentID)
	if err != nil {
		return nil, err
	}
	thread := s.thread(namedAgentSessionID(agent))
	if thread == nil {
		return nil, errors.New("named agent runtime is unavailable")
	}
	thread.mu.Lock()
	defer thread.mu.Unlock()
	if thread.NamedAgentID != agentID || thread.execRuntime == nil || thread.execRuntime.Toolkit == nil {
		return nil, errors.New("named agent runtime is unavailable")
	}
	return thread.execRuntime.Toolkit, nil
}

func writeNamedAgentMCPResponse(w http.ResponseWriter, response namedAgentMCPResponse) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

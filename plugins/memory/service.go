package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	pluginapi "github.com/blueberrycongee/wuu/packages/plugin-go"
)

// SessionMemoryService is the stable registry name other plugins resolve to
// share session memory semantics instead of touching the files directly.
const SessionMemoryService = "memory.session"

func sessionMemoryServiceDescriptor() pluginapi.Service {
	return pluginapi.Service{
		Name:    SessionMemoryService,
		Version: "1.0.0",
		Methods: []pluginapi.ServiceMethod{
			{Name: "status", InputSchema: "memory.session.status.request.v1", OutputSchema: "memory.session.status.response.v1"},
			{Name: "read", InputSchema: "memory.session.read.request.v1", OutputSchema: "memory.session.read.response.v1"},
			{Name: "append", InputSchema: "memory.session.append.request.v1", OutputSchema: "memory.session.append.response.v1"},
			{Name: "replace", InputSchema: "memory.session.replace.request.v1", OutputSchema: "memory.session.replace.response.v1"},
		},
	}
}

// sessionMemoryServiceParams carries one memory.session call. thread_id is
// required for the session-scoped targets (summary, checkpoint, notes) and
// optional for the workspace-scoped project_memory target.
type sessionMemoryServiceParams struct {
	Target   string `json:"target,omitempty"`
	ThreadID string `json:"thread_id,omitempty"`
	Content  string `json:"content,omitempty"`
	Source   string `json:"source,omitempty"`
}

func (c *controller) invokeService(_ context.Context, _ pluginapi.Host, call pluginapi.ServiceCall) (json.RawMessage, error) {
	if call.Service != SessionMemoryService {
		return nil, fmt.Errorf("memory plugin does not provide service %q", call.Service)
	}
	var params sessionMemoryServiceParams
	if len(call.Params) != 0 {
		if err := json.Unmarshal(call.Params, &params); err != nil {
			return nil, fmt.Errorf("memory.session params: %w", err)
		}
	}
	source := strings.TrimSpace(params.Source)
	if source == "" {
		source = strings.TrimSpace(call.Caller)
	}
	result, err := c.runSessionMemory(sessionMemoryArgs{
		Action:  call.Method,
		Target:  params.Target,
		Content: params.Content,
		Source:  source,
	}, params.ThreadID)
	if err != nil {
		return nil, err
	}
	return json.Marshal(result)
}

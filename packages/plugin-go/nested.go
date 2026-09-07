package pluginapi

import (
	"context"
	"encoding/json"
)

const InvokeToolService = "execution.invoke-tool"

// InvokeToolParams identifies a child invocation of a live orchestrator. The
// scope-local CallID must retain identical name and arguments when retried.
type InvokeToolParams struct {
	ExecutionID string          `json:"execution_id"`
	CallID      string          `json:"call_id"`
	Name        string          `json:"name"`
	Arguments   json.RawMessage `json:"arguments"`
}

// InvokeTool invokes an enabled tool through normal scheduling, authorization
// and durable recording. Requires execution.invoke-tool v1. The child cannot
// outlive the owning execution or its plugin generation.
func InvokeTool(ctx context.Context, host Host, params InvokeToolParams) (ToolResult, error) {
	var result ToolResult
	err := CallService(ctx, host, InvokeToolService, KernelServiceMethod, params, &result)
	return result, err
}

package tools

import (
	"context"

	"github.com/blueberrycongee/wuu/internal/providers"
)

const newContextToolName = "new_context"

// NewContextTool is a model control signal. The agent loop observes the
// completed call at the tool-batch boundary and performs the actual context
// replacement there; executing the tool itself never mutates environment state.
type NewContextTool struct{}

func NewNewContextTool() *NewContextTool        { return &NewContextTool{} }
func (*NewContextTool) Name() string            { return newContextToolName }
func (*NewContextTool) IsReadOnly() bool        { return true }
func (*NewContextTool) IsConcurrencySafe() bool { return true }
func (*NewContextTool) Execute(context.Context, string) (string, error) {
	return `{"requested":true,"message":"Wuu will evaluate the context-window transition after this tool batch. Environment state is unchanged."}`, nil
}

func (*NewContextTool) Definition() providers.ToolDefinition {
	return providers.ToolDefinition{
		Name: newContextToolName,
		Description: "Start a fresh context window at the next safe tool-loop boundary. This releases old active model context but does not clear or reset files, processes, permissions, or other environment state. " +
			"Do not write a summary first; Wuu maintains the continuation note in a background fork.",
		InputSchema: map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"properties":           map[string]any{},
		},
	}
}

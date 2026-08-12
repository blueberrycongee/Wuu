package todo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	pluginapi "github.com/blueberrycongee/wuu/packages/plugin-go"
)

type Status string

const (
	StatusPending    Status = "pending"
	StatusInProgress Status = "in_progress"
	StatusCompleted  Status = "completed"
)

type Item struct {
	Content string `json:"content"`
	Status  Status `json:"status"`
}

type Update struct {
	Explanation string `json:"explanation,omitempty"`
	Todos       []Item `json:"todos"`
}

func Handler() pluginapi.Handler {
	return pluginapi.Handler{
		Definition: pluginapi.Definition{Tools: []pluginapi.Tool{{
			ID:          "update_todo",
			Description: "Update the current task TODO list. Provide the full TODO list every time. Keep exactly one item in_progress until all items are completed.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"explanation": map[string]any{"type": "string", "description": "Optional short explanation for why the TODO list changed."},
					"todos": map[string]any{
						"type": "array", "minItems": 1, "description": "Full current TODO list.",
						"items": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"content": map[string]any{"type": "string", "description": "Concrete task to complete."},
								"status":  map[string]any{"type": "string", "enum": []string{"pending", "in_progress", "completed"}},
							},
							"required":             []string{"content", "status"},
							"additionalProperties": false,
						},
					},
				},
				"required":             []string{"todos"},
				"additionalProperties": false,
			},
			Display: &pluginapi.ToolDisplay{Kind: "todo", Text: "Updating TODO", Capability: "todo"},
		}}},
		ExecuteTool: executeTool,
	}
}

func executeTool(_ context.Context, _ pluginapi.Host, call pluginapi.ToolCall) (pluginapi.ToolResult, error) {
	if call.ToolID != "update_todo" {
		return pluginapi.ToolResult{}, fmt.Errorf("unknown TODO tool %q", call.ToolID)
	}
	update, err := decodeUpdate(call.Arguments)
	if err != nil {
		return pluginapi.ToolResult{}, err
	}
	if err := validateUpdate(update); err != nil {
		return pluginapi.ToolResult{}, err
	}
	result, err := json.Marshal(map[string]any{
		"action": "update_todo",
		"status": "updated",
		"todos":  update.Todos,
	})
	if err != nil {
		return pluginapi.ToolResult{}, err
	}
	return pluginapi.TextResult(string(result)), nil
}

func decodeUpdate(raw json.RawMessage) (Update, error) {
	if len(strings.TrimSpace(string(raw))) == 0 {
		raw = json.RawMessage(`{}`)
	}
	var update Update
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&update); err != nil {
		return Update{}, fmt.Errorf("invalid tool arguments: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return Update{}, errors.New("invalid tool arguments: multiple JSON values")
	}
	return update, nil
}

func validateUpdate(update Update) error {
	if len(update.Todos) == 0 {
		return errors.New("update_todo requires at least one TODO item")
	}
	inProgress := 0
	completed := 0
	for index, item := range update.Todos {
		if strings.TrimSpace(item.Content) == "" {
			return fmt.Errorf("TODO item %d requires content", index)
		}
		switch item.Status {
		case StatusPending:
		case StatusInProgress:
			inProgress++
		case StatusCompleted:
			completed++
		default:
			return fmt.Errorf("TODO item %d has invalid status %q", index, item.Status)
		}
	}
	if completed == len(update.Todos) {
		if inProgress != 0 {
			return errors.New("completed TODO list cannot have an in_progress item")
		}
		return nil
	}
	if inProgress != 1 {
		return errors.New("unfinished TODO list requires exactly one in_progress item")
	}
	return nil
}

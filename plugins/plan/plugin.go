package plan

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
	Step   string `json:"step"`
	Status Status `json:"status"`
}

type Update struct {
	Explanation string `json:"explanation,omitempty"`
	Plan        []Item `json:"plan"`
}

func Handler() pluginapi.Handler {
	return pluginapi.Handler{
		Definition: pluginapi.Definition{Tools: []pluginapi.Tool{{
			ID:          "update_plan",
			Description: "Update the current task plan. Provide the full plan every time. Keep exactly one item in_progress until all items are completed.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"explanation": map[string]any{"type": "string", "description": "Optional short explanation for why the plan changed."},
					"plan": map[string]any{
						"type": "array", "minItems": 1, "description": "Full current plan.",
						"items": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"step":   map[string]any{"type": "string", "description": "Concrete task step."},
								"status": map[string]any{"type": "string", "enum": []string{"pending", "in_progress", "completed"}},
							},
							"required":             []string{"step", "status"},
							"additionalProperties": false,
						},
					},
				},
				"required":             []string{"plan"},
				"additionalProperties": false,
			},
			Display: &pluginapi.ToolDisplay{Kind: "plan", Text: "Updating plan", Capability: "plan"},
		}}},
		ExecuteTool: executeTool,
	}
}

func executeTool(_ context.Context, _ pluginapi.Host, call pluginapi.ToolCall) (pluginapi.ToolResult, error) {
	if call.ToolID != "update_plan" {
		return pluginapi.ToolResult{}, fmt.Errorf("unknown plan tool %q", call.ToolID)
	}
	update, err := decodeUpdate(call.Arguments)
	if err != nil {
		return pluginapi.ToolResult{}, err
	}
	if err := validateUpdate(update); err != nil {
		return pluginapi.ToolResult{}, err
	}
	result, err := json.Marshal(map[string]any{
		"action": "update_plan",
		"status": "updated",
		"plan":   update.Plan,
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
	if len(update.Plan) == 0 {
		return errors.New("update_plan requires at least one plan item")
	}
	inProgress := 0
	completed := 0
	for index, item := range update.Plan {
		if strings.TrimSpace(item.Step) == "" {
			return fmt.Errorf("plan item %d requires step", index)
		}
		switch item.Status {
		case StatusPending:
		case StatusInProgress:
			inProgress++
		case StatusCompleted:
			completed++
		default:
			return fmt.Errorf("plan item %d has invalid status %q", index, item.Status)
		}
	}
	if completed == len(update.Plan) {
		if inProgress != 0 {
			return errors.New("completed plan cannot have an in_progress item")
		}
		return nil
	}
	if inProgress != 1 {
		return errors.New("unfinished plan requires exactly one in_progress item")
	}
	return nil
}

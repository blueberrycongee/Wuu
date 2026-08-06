package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/blueberrycongee/wuu/internal/goalruntime"
	"github.com/blueberrycongee/wuu/internal/providers"
	"github.com/blueberrycongee/wuu/internal/session"
)

// Goal is one thread-scoped durable objective, exposed to the model through
// three narrow tools instead of one action-dispatch schema.

type GetGoalTool struct{ env *Env }

func NewGetGoalTool(env *Env) *GetGoalTool { return &GetGoalTool{env: env} }

func (t *GetGoalTool) Name() string            { return "get_goal" }
func (t *GetGoalTool) IsReadOnly() bool        { return true }
func (t *GetGoalTool) IsConcurrencySafe() bool { return true }
func (t *GetGoalTool) Definition() providers.ToolDefinition {
	return providers.ToolDefinition{
		Name:        t.Name(),
		Description: "Get the current goal for this thread, including status, token usage, and elapsed-time usage.",
		InputSchema: emptyGoalToolSchema(),
	}
}
func (t *GetGoalTool) Execute(_ context.Context, argsJSON string) (string, error) {
	if err := decodeEmptyGoalArgs(argsJSON); err != nil {
		return "", err
	}
	goal, ok, err := currentGoalRuntime(t.env)
	if err != nil {
		return "", err
	}
	if !ok {
		return mustJSON(map[string]any{"goal": nil})
	}
	return goalToolResult(goal)
}

type CreateGoalTool struct{ env *Env }

func NewCreateGoalTool(env *Env) *CreateGoalTool { return &CreateGoalTool{env: env} }

func (t *CreateGoalTool) Name() string            { return "create_goal" }
func (t *CreateGoalTool) IsReadOnly() bool        { return false }
func (t *CreateGoalTool) IsConcurrencySafe() bool { return false }
func (t *CreateGoalTool) Definition() providers.ToolDefinition {
	return providers.ToolDefinition{
		Name: t.Name(),
		Description: "Create a goal only when explicitly requested by the user or system/developer instructions; do not infer goals from ordinary tasks. " +
			"This starts a new active goal when no goal exists or replaces the current goal when it is complete. Fails if an unfinished goal exists; use update_goal only for status.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"objective": map[string]any{
					"type":        "string",
					"description": "Required. The concrete objective to start pursuing. State the outcome, not implementation details.",
				},
			},
			"required":             []string{"objective"},
			"additionalProperties": false,
		},
	}
}
func (t *CreateGoalTool) Execute(_ context.Context, argsJSON string) (string, error) {
	var args struct {
		Objective string `json:"objective"`
	}
	if err := decodeArgs(argsJSON, &args); err != nil {
		return "", err
	}
	objective := strings.TrimSpace(args.Objective)
	if err := goalruntime.ValidateObjective(objective); err != nil {
		return "", err
	}
	if t == nil || t.env == nil || t.env.GoalRuntime == nil {
		return "", errors.New("create_goal requires a thread runtime goal store")
	}
	threadID := strings.TrimSpace(t.env.SessionID)
	if threadID == "" {
		return "", errors.New("thread session_id is required for runtime goal")
	}
	goal, err := t.env.GoalRuntime.Create(goalruntime.Spec{
		ThreadID:  threadID,
		GoalID:    "goal-" + session.NewID(),
		Objective: objective,
	})
	if err != nil {
		return "", err
	}
	return goalToolResult(goal)
}

type UpdateGoalTool struct{ env *Env }

func NewUpdateGoalTool(env *Env) *UpdateGoalTool { return &UpdateGoalTool{env: env} }

func (t *UpdateGoalTool) Name() string            { return "update_goal" }
func (t *UpdateGoalTool) IsReadOnly() bool        { return false }
func (t *UpdateGoalTool) IsConcurrencySafe() bool { return false }
func (t *UpdateGoalTool) Definition() providers.ToolDefinition {
	return providers.ToolDefinition{
		Name: t.Name(),
		Description: "Update the existing goal. Use this tool only to mark the goal achieved or genuinely blocked. " +
			"Set status to complete only when the objective has actually been achieved and no required work remains. " +
			"Set status to blocked only when the same blocking condition has repeated for at least three consecutive goal turns and the agent cannot make meaningful progress without user input or an external-state change. " +
			"After a previously blocked goal is resumed, treat the resumed run as a fresh blocked audit. " +
			"Do not use blocked merely because the work is hard, slow, uncertain, incomplete, or would benefit from clarification. " +
			"You cannot pause or resume a goal; those status changes are controlled by the user or system.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"status": map[string]any{
					"type":        "string",
					"enum":        []string{"complete", "blocked"},
					"description": "Required. Set to complete only when the objective is achieved; set to blocked only after the repeated-blocker audit is satisfied.",
				},
			},
			"required":             []string{"status"},
			"additionalProperties": false,
		},
	}
}
func (t *UpdateGoalTool) Execute(_ context.Context, argsJSON string) (string, error) {
	var args struct {
		Status string `json:"status"`
	}
	if err := decodeArgs(argsJSON, &args); err != nil {
		return "", err
	}
	status := goalruntime.Status(strings.TrimSpace(args.Status))
	if status != goalruntime.StatusComplete && status != goalruntime.StatusBlocked {
		return "", errors.New("update_goal requires status=complete or status=blocked")
	}
	if _, err := currentActiveRuntimeGoalForTool(t.env); err != nil {
		return "", err
	}
	goal, err := t.env.GoalRuntime.SetModelStatus(status, time.Now().UTC())
	if err != nil {
		return "", err
	}
	return goalToolResult(goal)
}

func emptyGoalToolSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"properties":           map[string]any{},
		"additionalProperties": false,
	}
}

func decodeEmptyGoalArgs(argsJSON string) error {
	if strings.TrimSpace(argsJSON) == "" {
		return nil
	}
	var args map[string]json.RawMessage
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return err
	}
	if len(args) > 0 {
		return errors.New("get_goal does not accept arguments")
	}
	return nil
}

func currentActiveRuntimeGoalForTool(env *Env) (goalruntime.Goal, error) {
	goal, ok, err := currentGoalRuntime(env)
	if err != nil {
		return goalruntime.Goal{}, err
	}
	if !ok {
		return goalruntime.Goal{}, errors.New("active runtime goal not found")
	}
	if goal.Status != goalruntime.StatusActive {
		return goalruntime.Goal{}, fmt.Errorf("active runtime goal is currently %s; ask the user to resume the goal before updating it", goal.Status)
	}
	return goal, nil
}

func currentGoalRuntime(env *Env) (goalruntime.Goal, bool, error) {
	if env == nil || env.GoalRuntime == nil {
		return goalruntime.Goal{}, false, nil
	}
	goal, err := env.GoalRuntime.CurrentGoal()
	if errors.Is(err, os.ErrNotExist) {
		return goalruntime.Goal{}, false, nil
	}
	if err != nil {
		return goalruntime.Goal{}, false, err
	}
	return goal, true, nil
}

// goalToolView is Wuu's public thread Goal contract. GoalID and the persistence
// schema stay private to the runtime, and Wuu has no token-budget fields.
type goalToolView struct {
	ThreadID        string             `json:"thread_id"`
	Objective       string             `json:"objective"`
	Status          goalruntime.Status `json:"status"`
	TokensUsed      int                `json:"tokens_used"`
	TimeUsedSeconds int64              `json:"time_used_seconds"`
	CreatedAt       int64              `json:"created_at"`
	UpdatedAt       int64              `json:"updated_at"`
}

func goalToolResult(goal goalruntime.Goal) (string, error) {
	return mustJSON(map[string]any{
		"goal": goalToolView{
			ThreadID:        goal.ThreadID,
			Objective:       goal.Objective,
			Status:          goal.Status,
			TokensUsed:      goal.TokensUsed,
			TimeUsedSeconds: goal.TimeUsedSeconds,
			CreatedAt:       goal.CreatedAt.Unix(),
			UpdatedAt:       goal.UpdatedAt.Unix(),
		},
	})
}

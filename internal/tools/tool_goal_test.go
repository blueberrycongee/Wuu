package tools

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/blueberrycongee/wuu/internal/goalruntime"
	"github.com/blueberrycongee/wuu/internal/providers"
)

type decodedGoalToolResult struct {
	Goal *goalToolView `json:"goal"`
}

func newGoalToolTestEnv(t *testing.T, threadID string) *Env {
	t.Helper()
	return &Env{
		RootDir:     t.TempDir(),
		StateDir:    filepath.Join(t.TempDir(), "state"),
		SessionID:   threadID,
		GoalRuntime: goalruntime.NewRuntime(goalruntime.NewStore(filepath.Join(t.TempDir(), "goal_runtime.json"))),
	}
}

func decodeGoalToolResult(t *testing.T, raw string) decodedGoalToolResult {
	t.Helper()
	var result decodedGoalToolResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("decode goal tool result: %v\n%s", err, raw)
	}
	return result
}

func TestGoalToolsLifecycle(t *testing.T) {
	env := newGoalToolTestEnv(t, "thread-goal-tools")

	gotRaw, err := NewGetGoalTool(env).Execute(context.Background(), `{}`)
	if err != nil {
		t.Fatalf("get missing goal: %v", err)
	}
	if got := decodeGoalToolResult(t, gotRaw); got.Goal != nil {
		t.Fatalf("missing goal result = %+v, want nil", got.Goal)
	}

	createdRaw, err := NewCreateGoalTool(env).Execute(context.Background(), `{"objective":"Ship goal tools"}`)
	if err != nil {
		t.Fatalf("create goal: %v", err)
	}
	created := decodeGoalToolResult(t, createdRaw)
	if created.Goal == nil || created.Goal.ThreadID != env.SessionID ||
		created.Goal.Objective != "Ship goal tools" || created.Goal.Status != goalruntime.StatusActive {
		t.Fatalf("unexpected create result: %+v", created.Goal)
	}
	if strings.Contains(createdRaw, "goal_id") || strings.Contains(createdRaw, "schema_version") {
		t.Fatalf("runtime implementation fields leaked into model result: %s", createdRaw)
	}
	persisted, err := env.GoalRuntime.CurrentGoal()
	if err != nil || !strings.HasPrefix(persisted.GoalID, "goal-") {
		t.Fatalf("runtime should persist a private goal id: goal=%+v err=%v", persisted, err)
	}

	if _, err := NewCreateGoalTool(env).Execute(context.Background(), `{"objective":"Duplicate"}`); err == nil {
		t.Fatal("create_goal should reject an unfinished goal")
	}
	if _, err := NewUpdateGoalTool(env).Execute(context.Background(), `{"status":"active"}`); err == nil {
		t.Fatal("update_goal should reject user-owned statuses")
	}
	if _, err := NewGetGoalTool(env).Execute(context.Background(), `{"extra":true}`); err == nil {
		t.Fatal("get_goal should reject arguments")
	}

	gotRaw, err = NewGetGoalTool(env).Execute(context.Background(), `{}`)
	if err != nil {
		t.Fatalf("get active goal: %v", err)
	}
	got := decodeGoalToolResult(t, gotRaw)
	if got.Goal == nil || got.Goal.Objective != created.Goal.Objective || got.Goal.Status != goalruntime.StatusActive {
		t.Fatalf("unexpected get result: %+v", got.Goal)
	}

	completedRaw, err := NewUpdateGoalTool(env).Execute(context.Background(), `{"status":"complete"}`)
	if err != nil {
		t.Fatalf("complete goal: %v", err)
	}
	if completed := decodeGoalToolResult(t, completedRaw); completed.Goal == nil || completed.Goal.Status != goalruntime.StatusComplete {
		t.Fatalf("unexpected complete result: %+v", completed.Goal)
	}
	if _, err := NewCreateGoalTool(env).Execute(context.Background(), `{"objective":"Replacement goal"}`); err != nil {
		t.Fatalf("create replacement after terminal goal: %v", err)
	}
}

func TestUpdateGoalBlockedRequiresResumeBeforeComplete(t *testing.T) {
	env := newGoalToolTestEnv(t, "thread-goal-blocked")
	if _, err := NewCreateGoalTool(env).Execute(context.Background(), `{"objective":"Recover after blocker"}`); err != nil {
		t.Fatalf("create goal: %v", err)
	}
	blockedRaw, err := NewUpdateGoalTool(env).Execute(context.Background(), `{"status":"blocked"}`)
	if err != nil {
		t.Fatalf("block goal: %v", err)
	}
	if blocked := decodeGoalToolResult(t, blockedRaw); blocked.Goal == nil || blocked.Goal.Status != goalruntime.StatusBlocked {
		t.Fatalf("unexpected blocked result: %+v", blocked.Goal)
	}

	_, err = NewUpdateGoalTool(env).Execute(context.Background(), `{"status":"complete"}`)
	if err == nil || !strings.Contains(err.Error(), "currently blocked") || !strings.Contains(err.Error(), "resume") {
		t.Fatalf("blocked goal should explain user resume requirement: %v", err)
	}
}

func TestGoalToolDefinitionsAreSeparatedAndBudgetFree(t *testing.T) {
	definitions := []providers.ToolDefinition{
		NewGetGoalTool(&Env{}).Definition(),
		NewCreateGoalTool(&Env{}).Definition(),
		NewUpdateGoalTool(&Env{}).Definition(),
	}
	wantNames := []string{"get_goal", "create_goal", "update_goal"}
	for i, def := range definitions {
		if def.Name != wantNames[i] {
			t.Fatalf("definition %d name = %q, want %q", i, def.Name, wantNames[i])
		}
		assertToolSchemaOmits(t, def, "action", "goal_id", "token_budget", "usage_limit", "budget_limit", "reason")
	}
	if !NewGetGoalTool(&Env{}).IsReadOnly() || !NewGetGoalTool(&Env{}).IsConcurrencySafe() {
		t.Fatal("get_goal should be read-only and concurrency-safe")
	}
	if !strings.Contains(NewCreateGoalTool(&Env{}).Definition().Description, "explicitly requested") {
		t.Fatal("create_goal must preserve the explicit-request boundary")
	}
	updateDesc := NewUpdateGoalTool(&Env{}).Definition().Description
	for _, want := range []string{"complete", "three consecutive goal turns", "blocked", "pause or resume"} {
		if !strings.Contains(updateDesc, want) {
			t.Fatalf("update_goal description missing %q: %q", want, updateDesc)
		}
	}
}

func assertToolSchemaOmits(t *testing.T, def providers.ToolDefinition, names ...string) {
	t.Helper()
	properties, ok := def.InputSchema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("%s schema properties have unexpected type %T", def.Name, def.InputSchema["properties"])
	}
	for _, name := range names {
		if _, ok := properties[name]; ok {
			t.Fatalf("%s schema should omit %q", def.Name, name)
		}
	}
}

package automation

import (
	"context"
	"errors"
	"testing"
)

type recordingExecutor struct {
	result ExecutionResult
	err    error
}

func (e *recordingExecutor) ExecuteAutomation(_ context.Context, _ Task, _ Run) (ExecutionResult, error) {
	return e.result, e.err
}

func TestManagerAddsHeartbeatTaskWithCreatorThread(t *testing.T) {
	manager := NewManager(Config{StateDir: t.TempDir()})
	task, err := manager.AddTask(AddTaskParams{
		Prompt:          "check cache health",
		Schedule:        "*/5 * * * *",
		Timezone:        "UTC",
		Mode:            ModeThreadHeartbeat,
		CreatorThreadID: "thread-1",
		Recurring:       true,
		Durable:         true,
	})
	if err != nil {
		t.Fatalf("AddTask() error = %v", err)
	}
	if task.HeartbeatThreadID != "thread-1" {
		t.Fatalf("HeartbeatThreadID = %q, want thread-1", task.HeartbeatThreadID)
	}
	if task.Mode != string(ModeThreadHeartbeat) {
		t.Fatalf("Mode = %q", task.Mode)
	}
	tasks, err := manager.ListTasks()
	if err != nil {
		t.Fatalf("ListTasks() error = %v", err)
	}
	if len(tasks) != 1 || tasks[0].ID != task.ID {
		t.Fatalf("ListTasks() = %#v", tasks)
	}
}

func TestManagerRejectsHeartbeatWithoutThread(t *testing.T) {
	manager := NewManager(Config{StateDir: t.TempDir()})
	_, err := manager.AddTask(AddTaskParams{
		Prompt:   "check cache health",
		Schedule: "*/5 * * * *",
		Timezone: "UTC",
		Mode:     ModeThreadHeartbeat,
	})
	if err == nil {
		t.Fatal("AddTask() error = nil")
	}
}

func TestManagerRecordsAdmissionAndCompletion(t *testing.T) {
	manager := NewManager(Config{StateDir: t.TempDir()})
	manager.executor = &recordingExecutor{result: ExecutionResult{
		Status:   RunStatusRunning,
		ThreadID: "thread-2",
		TurnID:   "turn-1",
	}}
	if err := manager.Fire(context.Background(), Task{ID: "task-1", Prompt: "inspect", Mode: string(ModeNewThread)}); err != nil {
		t.Fatalf("Fire() error = %v", err)
	}
	runs, err := manager.ListRuns()
	if err != nil {
		t.Fatalf("ListRuns() error = %v", err)
	}
	if len(runs) != 1 || runs[0].Status != RunStatusRunning || runs[0].ThreadID != "thread-2" {
		t.Fatalf("runs = %#v", runs)
	}
	if err := manager.CompleteRun(runs[0].ID, "thread-2", "turn-1", nil); err != nil {
		t.Fatalf("CompleteRun() error = %v", err)
	}
	runs, _ = manager.ListRuns()
	if runs[0].Status != RunStatusCompleted || runs[0].CompletedAt.IsZero() {
		t.Fatalf("completed run = %#v", runs[0])
	}
}

func TestManagerRecordsExecutionFailure(t *testing.T) {
	manager := NewManager(Config{StateDir: t.TempDir()})
	manager.executor = &recordingExecutor{err: errors.New("start failed")}
	err := manager.Fire(context.Background(), Task{ID: "task-1", Prompt: "inspect"})
	if err == nil {
		t.Fatal("Fire() error = nil")
	}
	runs, listErr := manager.ListRuns()
	if listErr != nil {
		t.Fatalf("ListRuns() error = %v", listErr)
	}
	if len(runs) != 1 || runs[0].Status != RunStatusFailed || runs[0].Error != "start failed" {
		t.Fatalf("runs = %#v", runs)
	}
}

func TestManagerPausesDurableTask(t *testing.T) {
	manager := NewManager(Config{StateDir: t.TempDir()})
	task, err := manager.AddTask(AddTaskParams{
		Prompt: "inspect", Schedule: "*/5 * * * *", Timezone: "UTC", Durable: true,
	})
	if err != nil {
		t.Fatalf("AddTask() error = %v", err)
	}
	paused, err := manager.SetPaused(task.ID, true)
	if err != nil {
		t.Fatalf("SetPaused() error = %v", err)
	}
	if !paused.Paused {
		t.Fatalf("paused task = %#v", paused)
	}
	tasks, err := manager.ListTasks()
	if err != nil || len(tasks) != 1 || !tasks[0].Paused {
		t.Fatalf("ListTasks() = %#v, %v", tasks, err)
	}
}

func TestRunAdmissionDoesNotOverwriteTerminalStatus(t *testing.T) {
	store := NewRunStore(t.TempDir() + "/runs.json")
	run := Run{ID: "run-1", Status: RunStatusRunning}
	if err := store.Add(run); err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if err := store.Finish(run.ID, RunStatusCompleted, "thread-1", "turn-1", ""); err != nil {
		t.Fatalf("Finish() error = %v", err)
	}
	if err := store.UpdateAdmission(run.ID, RunStatusRunning, "thread-1", "turn-1", ""); err != nil {
		t.Fatalf("UpdateAdmission() error = %v", err)
	}
	runs, err := store.List()
	if err != nil || len(runs) != 1 || runs[0].Status != RunStatusCompleted {
		t.Fatalf("runs = %#v, %v", runs, err)
	}
}

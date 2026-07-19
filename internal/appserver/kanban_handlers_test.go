package appserver

import (
	"strings"
	"testing"

	"github.com/blueberrycongee/wuu/internal/kanban"
)

func TestKanbanRunRequestPromptExecute(t *testing.T) {
	task := kanban.Task{
		ID: "kt-1", Title: "Build the landing page", Brief: "goal: ship it\ndone when: deployed",
		SourceThreadID: "cth-src",
	}
	prompt := kanbanRunRequestPrompt(task, kanban.Run{Kind: kanban.RunKindExecute}, []kanban.Artifact{
		{Path: "design/mock.png"},
	}, "/tmp/out/kt-1")
	for _, want := range []string{
		"# Task: Build the landing page",
		"goal: ship it",
		"- design/mock.png",
		"cth-src",
		"/tmp/out/kt-1",
		"final message must be a short summary",
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt missing %q:\n%s", want, prompt)
		}
	}
	if strings.Contains(prompt, "second pair of eyes") {
		t.Error("execute prompt must not contain review framing")
	}
}

func TestKanbanRunRequestPromptReview(t *testing.T) {
	task := kanban.Task{ID: "kt-1", Title: "Build the landing page", Brief: "done when: deployed"}
	prompt := kanbanRunRequestPrompt(task, kanban.Run{Kind: kanban.RunKindReview}, nil, "/tmp/out/kt-1")
	for _, want := range []string{
		"# Verification task: Build the landing page",
		"second pair of eyes",
		"No prior artifacts",
		"Do not",
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("review prompt missing %q:\n%s", want, prompt)
		}
	}
	if strings.Contains(prompt, "# Task:") {
		t.Error("review prompt must not use the execute header")
	}
}

func TestKanbanRunRequestPromptNoSourceThread(t *testing.T) {
	task := kanban.Task{ID: "kt-1", Title: "T"}
	prompt := kanbanRunRequestPrompt(task, kanban.Run{Kind: kanban.RunKindExecute}, nil, "/tmp/out")
	if strings.Contains(prompt, "crystallized from conversation") {
		t.Error("source-thread note must be omitted when there is no source thread")
	}
}

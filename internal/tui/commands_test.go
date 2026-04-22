package tui

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/blueberrycongee/wuu/internal/agent"
	"github.com/blueberrycongee/wuu/internal/cron"
	"github.com/blueberrycongee/wuu/internal/providers"
)

func TestParseSlashCommand(t *testing.T) {
	tests := []struct {
		input string
		ok    bool
		name  string
		args  string
	}{
		{input: "", ok: false},
		{input: "hello", ok: false},
		{input: "/", ok: false},
		{input: "/resume", ok: true, name: "resume", args: ""},
		{input: " /WORKTREE  ", ok: true, name: "worktree", args: ""},
		{input: "/insight latest", ok: true, name: "insight", args: "latest"},
	}

	for _, tc := range tests {
		cmd, ok := parseSlashCommand(tc.input)
		if ok != tc.ok {
			t.Fatalf("input=%q: expected ok=%v got %v", tc.input, tc.ok, ok)
		}
		if !ok {
			continue
		}
		if cmd.Name != tc.name || cmd.Args != tc.args {
			t.Fatalf("input=%q: unexpected parse result: %#v", tc.input, cmd)
		}
	}
}

func TestHandleSlash(t *testing.T) {
	m := NewModel(Config{
		Provider:   "test",
		Model:      "test-model",
		ConfigPath: "/tmp/.wuu.json",
		StreamRunner: &agent.StreamRunner{
			Client: &echoStreamClient{answer: func(_ []providers.ChatMessage) string { return "" }},
			Model:  "test-model",
		},
	})

	msg, handled := m.handleSlash("/skills")
	if !handled {
		t.Fatal("expected /skills to be handled")
	}
	if msg == "" {
		t.Fatal("expected non-empty message")
	}

	msg, handled = m.handleSlash("/unknown")
	if !handled {
		t.Fatal("expected unknown slash command to be handled")
	}
	if msg == "" {
		t.Fatal("expected unknown slash command message")
	}
}

func TestHandleSlashNewResetsChatHistoryButKeepsSystemPrompt(t *testing.T) {
	m := NewModel(Config{
		Provider:   "test",
		Model:      "test-model",
		ConfigPath: "/tmp/.wuu.json",
		StreamRunner: &agent.StreamRunner{
			Client:       &echoStreamClient{answer: func(_ []providers.ChatMessage) string { return "" }},
			Model:        "test-model",
			SystemPrompt: "system rules",
		},
	})
	m.chatHistory = []providers.ChatMessage{
		{Role: "system", Content: "system rules"},
		{Role: "user", Content: "old user"},
		{Role: "assistant", Content: "old assistant"},
		{Role: "tool", Content: "old tool"},
	}
	m.entries = []transcriptEntry{{Role: "USER", Content: "visible old entry"}}

	msg, handled := m.handleSlash("/new")
	if !handled {
		t.Fatal("expected /new to be handled")
	}
	if msg == "" {
		t.Fatal("expected /new response message")
	}
	if len(m.entries) != 0 {
		t.Fatalf("expected /new to clear visible entries, got %d", len(m.entries))
	}
	if len(m.chatHistory) != 1 {
		t.Fatalf("expected /new to keep only system prompt in chat history, got %+v", m.chatHistory)
	}
	if m.chatHistory[0].Role != "system" || m.chatHistory[0].Content != "system rules" {
		t.Fatalf("expected /new to preserve system prompt, got %+v", m.chatHistory[0])
	}
}

func TestHandleSlashNewClearsChatHistoryWithoutSystemPrompt(t *testing.T) {
	m := NewModel(Config{
		Provider:   "test",
		Model:      "test-model",
		ConfigPath: "/tmp/.wuu.json",
		StreamRunner: &agent.StreamRunner{
			Client: &echoStreamClient{answer: func(_ []providers.ChatMessage) string { return "" }},
			Model:  "test-model",
		},
	})
	m.chatHistory = []providers.ChatMessage{
		{Role: "user", Content: "old user"},
		{Role: "assistant", Content: "old assistant"},
	}

	_, handled := m.handleSlash("/new")
	if !handled {
		t.Fatal("expected /new to be handled")
	}
	if len(m.chatHistory) != 0 {
		t.Fatalf("expected /new to clear all chat history without system prompt, got %+v", m.chatHistory)
	}
}

func TestCmdLoopStoresSessionOnlyTask(t *testing.T) {
	root := t.TempDir()
	m := NewModel(Config{
		Provider:      "test",
		Model:         "test-model",
		WorkspaceRoot: root,
		ConfigPath:    filepath.Join(root, ".wuu.json"),
		StreamRunner: &agent.StreamRunner{
			Client: &echoStreamClient{answer: func(_ []providers.ChatMessage) string { return "" }},
			Model:  "test-model",
		},
	})

	out := cmdLoop("5m check deploy", &m)
	if !strings.Contains(out, "in this session only") {
		t.Fatalf("expected session-only message, got %q", out)
	}

	fileTasks, err := cron.NewTaskStore(filepath.Join(root, ".wuu", "scheduled_tasks.json")).List()
	if err != nil {
		t.Fatalf("file store list: %v", err)
	}
	if len(fileTasks) != 0 {
		t.Fatalf("expected no durable tasks, got %d", len(fileTasks))
	}

	sessionTasks, err := cron.NewSessionTaskStore(root).List()
	if err != nil {
		t.Fatalf("session store list: %v", err)
	}
	if len(sessionTasks) != 1 {
		t.Fatalf("expected 1 session task, got %d", len(sessionTasks))
	}
}

func TestCmdTasksShowsSessionOnlyTasks(t *testing.T) {
	root := t.TempDir()
	store := cron.NewSessionTaskStore(root)
	if err := store.Add(cron.Task{
		ID:        "abc123",
		Cron:      "*/5 * * * *",
		Prompt:    "check deploy",
		CreatedAt: 1,
		Recurring: true,
	}); err != nil {
		t.Fatalf("store.Add: %v", err)
	}

	m := NewModel(Config{
		Provider:      "test",
		Model:         "test-model",
		WorkspaceRoot: root,
		ConfigPath:    filepath.Join(root, ".wuu.json"),
		StreamRunner: &agent.StreamRunner{
			Client: &echoStreamClient{answer: func(_ []providers.ChatMessage) string { return "" }},
			Model:  "test-model",
		},
	})

	out := cmdTasks("", &m)
	if !strings.Contains(out, "[session-only]") {
		t.Fatalf("expected session-only label, got %q", out)
	}
}

func TestCommandCompletionEnterBehavior(t *testing.T) {
	tests := []struct {
		name string
		want slashCompletionEnterBehavior
	}{
		{name: "help", want: slashCompletionExecute},
		{name: "exit", want: slashCompletionExecute},
		{name: "model", want: slashCompletionInsertOnly},
		{name: "resume", want: slashCompletionInsertOnly},
		{name: "worktree", want: slashCompletionInsertOnly},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var found *command
			for i := range commandRegistry {
				if commandRegistry[i].Name == tc.name {
					found = &commandRegistry[i]
					break
				}
			}
			if found == nil {
				t.Fatalf("command %q not found", tc.name)
			}
			if got := found.completionEnterBehavior(); got != tc.want {
				t.Fatalf("completionEnterBehavior() = %v, want %v", got, tc.want)
			}
		})
	}
}

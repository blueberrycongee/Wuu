package tui

import (
	"context"
	"testing"
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
		RunPrompt: func(_ctx context.Context, _prompt string) (string, error) {
			return "", nil
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

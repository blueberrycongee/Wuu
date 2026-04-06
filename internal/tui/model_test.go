package tui

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestSubmitPromptFlow(t *testing.T) {
	m := NewModel(Config{
		Provider:   "test",
		Model:      "test-model",
		ConfigPath: "/tmp/.wuu.json",
		RunPrompt: func(_ctx context.Context, prompt string) (string, error) {
			return "answer to: " + prompt, nil
		},
	})
	m.width = 120
	m.height = 40
	m.relayout()

	m.input.SetValue("hello world")
	nextModel, cmd := m.submit()
	if cmd == nil {
		t.Fatal("expected async command from submit")
	}
	next := nextModel.(Model)
	if !next.pending {
		t.Fatal("expected pending=true after submit")
	}

	msg := cmd()
	afterModel, _ := next.Update(msg)
	after := afterModel.(Model)
	if after.pending {
		t.Fatal("expected pending=false after response")
	}
	content := strings.Join(after.transcript, "\n\n")
	if !strings.Contains(content, "USER\nhello world") {
		t.Fatalf("missing user entry: %s", content)
	}
	if !strings.Contains(content, "ASSISTANT\nanswer to: hello world") {
		t.Fatalf("missing assistant entry: %s", content)
	}
}

func TestJumpToBottomToggle(t *testing.T) {
	m := NewModel(Config{
		Provider:   "test",
		Model:      "test-model",
		ConfigPath: "/tmp/.wuu.json",
		RunPrompt: func(_ctx context.Context, prompt string) (string, error) {
			return prompt, nil
		},
	})
	m.width = 100
	m.height = 16
	for i := 0; i < 30; i++ {
		m.appendEntry("assistant", "line")
	}
	m.relayout()
	m.refreshViewport(true)

	if m.showJump {
		t.Fatal("expected no jump hint at bottom")
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyPgUp})
	paged := updated.(Model)
	if !paged.showJump {
		t.Fatal("expected jump hint after page up")
	}

	updated, _ = paged.Update(tea.KeyMsg{Type: tea.KeyCtrlJ})
	jumped := updated.(Model)
	if jumped.showJump {
		t.Fatal("expected jump hint cleared after jump-to-bottom")
	}
}

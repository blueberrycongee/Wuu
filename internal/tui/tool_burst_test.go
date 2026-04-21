package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestView_CollapsesReadSearchListBurst(t *testing.T) {
	m := NewModel(Config{Provider: "test", Model: "test-model", ConfigPath: "/tmp/.wuu.json"})
	m.width = 120
	m.height = 24

	idx := m.appendEntry("assistant", "")
	m.entries[idx].ToolCalls = []ToolCallEntry{
		{Name: "read_file", Args: `{"path":"internal/tui/model.go"}`, Status: ToolCallDone, Collapsed: true},
		{Name: "grep", Args: `{"pattern":"collapseReadSearchGroups"}`, Status: ToolCallDone, Collapsed: true},
		{Name: "list_files", Args: `{"path":"internal/tui"}`, Status: ToolCallDone, Collapsed: true},
	}

	m.relayout()
	m.refreshViewport(true)

	view := ansi.Strip(m.View())
	if !strings.Contains(view, "Called read/search/list") {
		t.Fatalf("expected collapsed burst summary, got: %s", view)
	}
	if !strings.Contains(view, "1 read, 1 search, 1 list") {
		t.Fatalf("expected burst counts, got: %s", view)
	}
	if !strings.Contains(view, "press 'o' to expand") {
		t.Fatalf("expected expand hint, got: %s", view)
	}
	if strings.Contains(view, "\n  └ • read_file") || strings.Contains(view, "\n  └ • grep") {
		t.Fatalf("expected child details hidden while collapsed, got: %s", view)
	}
}

func TestToggleLastToolBurstGroup_ExpandsDetails(t *testing.T) {
	m := NewModel(Config{Provider: "test", Model: "test-model", ConfigPath: "/tmp/.wuu.json"})
	m.width = 120
	m.height = 24

	idx := m.appendEntry("assistant", "")
	m.entries[idx].ToolCalls = []ToolCallEntry{
		{Name: "read_file", Args: `{"path":"internal/tui/model.go"}`, Status: ToolCallDone, Collapsed: true},
		{Name: "read_file", Args: `{"path":"internal/tui/message_pipeline.go"}`, Status: ToolCallDone, Collapsed: true},
		{Name: "grep", Args: `{"pattern":"collapseReadSearchGroups"}`, Status: ToolCallDone, Collapsed: true},
	}

	m.relayout()
	m.refreshViewport(true)

	if !m.toggleLastToolBurstGroup() {
		t.Fatal("expected tool burst group to toggle")
	}
	if m.entries[idx].ToolCalls[0].Collapsed {
		t.Fatal("expected first tool in burst to expand")
	}
	m.refreshViewport(false)

	view := ansi.Strip(m.View())
	if !strings.Contains(view, "press 'o' to collapse") {
		t.Fatalf("expected collapse hint after toggle, got: %s", view)
	}
	if !strings.Contains(view, "read_file · internal/tui/model.go") {
		t.Fatalf("expected first tool detail after expand, got: %s", view)
	}
	if !strings.Contains(view, "grep · collapseReadSearchGroups") {
		t.Fatalf("expected search detail after expand, got: %s", view)
	}
}

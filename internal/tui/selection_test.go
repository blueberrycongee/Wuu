package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
)

// TestMain forces lipgloss into TrueColor mode so style.Render
// actually emits ANSI escapes during the test run. Without this the
// renderer auto-detects "no terminal attached" and silently strips
// all color, which makes our highlight assertions impossible.
func init() {
	lipgloss.SetColorProfile(termenv.TrueColor)
}

// fullContent fixture with 10 distinct lines so we can verify
// content-row addressing across scroll positions.
const fixtureContent = "line0\nline1\nline2\nline3\nline4\nline5\nline6\nline7\nline8\nline9"

func TestScreenToViewportCoords_AccountsForContentPadding(t *testing.T) {
	m := &Model{}
	m.layout.Chat.X = 10
	m.layout.Chat.Y = 4
	m.layout.Chat.Height = 5
	m.viewport.YOffset = 7

	row, col := m.screenToViewportCoords(10+contentPadLeft+3, 6)
	if row != 9 {
		t.Fatalf("row: got %d, want %d", row, 9)
	}
	if col != 3 {
		t.Fatalf("col: got %d, want %d", col, 3)
	}

	_, col = m.screenToViewportCoords(10, 6)
	if col != 0 {
		t.Fatalf("col before content area: got %d, want 0", col)
	}
}

func TestMouseSelectionStartsAtContentAreaAfterLeftPadding(t *testing.T) {
	m := NewModel(Config{Provider: "test", Model: "test-model", ConfigPath: "/tmp/.wuu.json"})
	m.width = 100
	m.height = 20
	m.relayout()

	updated, _ := m.Update(tea.MouseMsg{
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
		X:      m.layout.Chat.X + contentPadLeft,
		Y:      m.layout.Chat.Y,
	})
	after := updated.(Model)

	if after.selection.Anchor == nil {
		t.Fatal("expected selection anchor after mouse press")
	}
	if after.selection.Anchor.Col != 0 {
		t.Fatalf("expected selection anchor col 0 at content start, got %d", after.selection.Anchor.Col)
	}
}

func TestSelection_SelectedTextUsesVisualColumnsForWideRunes(t *testing.T) {
	sel := &selectionState{
		Anchor: &selectionPoint{Row: 0, Col: 0},
		Focus:  &selectionPoint{Row: 0, Col: 1},
	}

	got := sel.selectedText("你好a")
	want := "你"
	if got != want {
		t.Fatalf("selectedText wide rune: got %q, want %q", got, want)
	}
}

func TestSelection_SelectedTextUsesVisualColumnsWithANSI(t *testing.T) {
	line := lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF00")).Render("你好a")
	sel := &selectionState{
		Anchor: &selectionPoint{Row: 0, Col: 2},
		Focus:  &selectionPoint{Row: 0, Col: 3},
	}

	got := sel.selectedText(line)
	want := "好"
	if got != want {
		t.Fatalf("selectedText ANSI wide rune: got %q, want %q", got, want)
	}
}

func TestHighlightLineRange_UsesVisualColumnsForWideRunes(t *testing.T) {
	style := lipgloss.NewStyle().Background(lipgloss.Color("#FF0000"))
	out := highlightLineRange("你好a", 2, 4, style)
	stripped := ansi.Strip(out)
	if stripped != "你好a" {
		t.Fatalf("strip: got %q, want %q", stripped, "你好a")
	}
	if !strings.Contains(out, style.Render("好")) {
		t.Fatalf("expected highlighted wide rune, got %q", out)
	}
}

func TestHighlightLineRange_PreservesPaddingAlignment(t *testing.T) {
	style := lipgloss.NewStyle().Background(lipgloss.Color("#FF0000"))
	line := strings.Repeat(" ", contentPadLeft) + "你好a"

	out := highlightLineRange(line, contentPadLeft+2, contentPadLeft+4, style)
	stripped := ansi.Strip(out)
	if stripped != line {
		t.Fatalf("strip: got %q, want %q", stripped, line)
	}
	if !strings.Contains(out, style.Render("好")) {
		t.Fatalf("expected highlighted wide rune after padding, got %q", out)
	}
}

func TestSelection_SelectedTextAcrossMultipleLines(t *testing.T) {
	sel := &selectionState{
		Anchor: &selectionPoint{Row: 2, Col: 0},
		Focus:  &selectionPoint{Row: 4, Col: 4},
	}
	got := sel.selectedText(fixtureContent)
	want := "line2\nline3\nline4"
	if got != want {
		t.Fatalf("selectedText: got %q, want %q", got, want)
	}
}

func TestSelection_SelectedTextSurvivesAcrossScrollPosition(t *testing.T) {
	// Selection covers absolute content rows 5-7. Even though the
	// "visible window" the renderer might produce is row 0-4 (after
	// the user has scrolled), selectedText reads from the FULL
	// content and returns the right substring.
	sel := &selectionState{
		Anchor: &selectionPoint{Row: 5, Col: 0},
		Focus:  &selectionPoint{Row: 7, Col: 4},
	}
	got := sel.selectedText(fixtureContent)
	want := "line5\nline6\nline7"
	if got != want {
		t.Fatalf("selectedText after scroll: got %q, want %q", got, want)
	}
}

func TestOverlaySelection_TranslatesContentRowsToVisibleWindow(t *testing.T) {
	// Visible window contains content rows [3, 4, 5] because the
	// view was scrolled to YOffset=3. The selection covers content
	// rows 4-5, which should map to local viewport rows 1-2.
	visibleWindow := "line3\nline4\nline5"
	sel := &selectionState{
		Anchor: &selectionPoint{Row: 4, Col: 0},
		Focus:  &selectionPoint{Row: 5, Col: 4},
	}
	style := lipgloss.NewStyle().Background(lipgloss.Color("#FF0000"))

	out := overlaySelection(visibleWindow, sel, 3, style)
	lines := strings.Split(out, "\n")

	// Row 0 of the visible window (content row 3) is NOT in the
	// selection range and should be untouched.
	if lines[0] != "line3" {
		t.Errorf("row 0 should be untouched, got %q", lines[0])
	}
	// Rows 1 and 2 should have ANSI codes injected (the highlight).
	if !strings.Contains(lines[1], "\x1b[") {
		t.Errorf("row 1 should be highlighted (got %q)", lines[1])
	}
	if !strings.Contains(lines[2], "\x1b[") {
		t.Errorf("row 2 should be highlighted (got %q)", lines[2])
	}
}

func TestOverlaySelection_ClipsBeyondVisibleWindow(t *testing.T) {
	// Selection covers content rows 0-9 but only rows 3-5 are visible.
	// overlaySelection should silently skip the off-screen rows
	// without panicking.
	visibleWindow := "line3\nline4\nline5"
	sel := &selectionState{
		Anchor: &selectionPoint{Row: 0, Col: 0},
		Focus:  &selectionPoint{Row: 9, Col: 4},
	}
	style := lipgloss.NewStyle().Background(lipgloss.Color("#FF0000"))

	out := overlaySelection(visibleWindow, sel, 3, style)
	if strings.Count(out, "\n") != 2 {
		t.Fatalf("expected 3 visible lines, got: %q", out)
	}
	// All three visible rows are inside [0, 9], so all three should
	// be highlighted.
	for i, line := range strings.Split(out, "\n") {
		if !strings.Contains(line, "\x1b[") {
			t.Errorf("row %d should be highlighted (got %q)", i, line)
		}
	}
}

func TestOverlaySelection_SelectionAboveVisibleWindow(t *testing.T) {
	// Selection covers rows 0-1, but visible window starts at row 5.
	// Nothing should be highlighted; the visible window comes back
	// completely unchanged.
	visibleWindow := "line5\nline6\nline7"
	sel := &selectionState{
		Anchor: &selectionPoint{Row: 0, Col: 0},
		Focus:  &selectionPoint{Row: 1, Col: 4},
	}
	style := lipgloss.NewStyle().Background(lipgloss.Color("#FF0000"))

	out := overlaySelection(visibleWindow, sel, 5, style)
	if out != visibleWindow {
		t.Fatalf("off-screen selection should leave window untouched, got: %q", out)
	}
}

func TestOverlaySelection_SelectionBelowVisibleWindow(t *testing.T) {
	// Mirror of the above: selection on rows 8-9, visible window
	// shows rows 0-2. Nothing should be highlighted.
	visibleWindow := "line0\nline1\nline2"
	sel := &selectionState{
		Anchor: &selectionPoint{Row: 8, Col: 0},
		Focus:  &selectionPoint{Row: 9, Col: 4},
	}
	style := lipgloss.NewStyle().Background(lipgloss.Color("#FF0000"))

	out := overlaySelection(visibleWindow, sel, 0, style)
	if out != visibleWindow {
		t.Fatalf("off-screen selection should leave window untouched, got: %q", out)
	}
}

func TestOverlaySelection_NoSelectionPassThrough(t *testing.T) {
	out := overlaySelection("hello\nworld", nil, 0, lipgloss.NewStyle())
	if out != "hello\nworld" {
		t.Fatalf("nil selection should pass through, got %q", out)
	}
}

package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
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

func TestSelection_SelectedTextUsesAbsoluteRows(t *testing.T) {
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

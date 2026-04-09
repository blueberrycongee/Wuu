package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestScrollbarContentFitsViewport(t *testing.T) {
	// When content fits in viewport, no scrollbar should be shown.
	result := renderScrollbar(10, 10, 10, 0)
	if result != "" {
		t.Fatalf("expected empty string when content fits viewport, got %q", result)
	}
}

func TestScrollbarContentSmallerThanViewport(t *testing.T) {
	result := renderScrollbar(10, 5, 10, 0)
	if result != "" {
		t.Fatalf("expected empty string when content < viewport, got %q", result)
	}
}

func TestScrollbarZeroHeight(t *testing.T) {
	result := renderScrollbar(0, 20, 10, 0)
	if result != "" {
		t.Fatalf("expected empty string for zero height, got %q", result)
	}
}

func TestScrollbarBasicRendering(t *testing.T) {
	// 10-line viewport, 20 lines of content, at top.
	result := renderScrollbar(10, 20, 10, 0)
	if result == "" {
		t.Fatal("expected non-empty scrollbar")
	}
	lines := strings.Split(result, "\n")
	if len(lines) != 10 {
		t.Fatalf("expected 10 lines, got %d", len(lines))
	}
}

func TestScrollbarThumbAtTop(t *testing.T) {
	// At offset 0, thumb should start at line 0.
	result := renderScrollbar(10, 20, 10, 0)
	lines := strings.Split(result, "\n")
	// First line should be thumb.
	if !strings.Contains(lines[0], scrollbarThumb) {
		t.Fatalf("expected thumb at line 0, got %q", lines[0])
	}
}

func TestScrollbarThumbAtBottom(t *testing.T) {
	// At max offset, thumb should end at last line.
	result := renderScrollbar(10, 20, 10, 10) // offset = contentSize - viewportSize
	lines := strings.Split(result, "\n")
	lastLine := lines[len(lines)-1]
	if !strings.Contains(lastLine, scrollbarThumb) {
		t.Fatalf("expected thumb at last line, got %q", lastLine)
	}
}

func TestScrollbarThumbMinSize(t *testing.T) {
	// Very large content: thumb should be at least 1 character.
	result := renderScrollbar(10, 1000, 10, 0)
	lines := strings.Split(result, "\n")
	thumbCount := 0
	for _, line := range lines {
		if strings.Contains(line, scrollbarThumb) {
			thumbCount++
		}
	}
	if thumbCount < 1 {
		t.Fatal("expected at least 1 thumb line")
	}
}

func TestScrollbarThumbProportional(t *testing.T) {
	// 20-line viewport, 40 lines of content: thumb should be ~10 lines (half).
	result := renderScrollbar(20, 40, 20, 0)
	lines := strings.Split(result, "\n")
	thumbCount := 0
	for _, line := range lines {
		if strings.Contains(line, scrollbarThumb) {
			thumbCount++
		}
	}
	if thumbCount != 10 {
		t.Fatalf("expected 10 thumb lines for 50%% ratio, got %d", thumbCount)
	}
}

func TestScrollbarTrackLines(t *testing.T) {
	// Non-thumb lines should be track.
	result := renderScrollbar(10, 20, 10, 0)
	lines := strings.Split(result, "\n")
	for _, line := range lines {
		if !strings.Contains(line, scrollbarThumb) && !strings.Contains(line, scrollbarTrack) {
			t.Fatalf("line should be either thumb or track, got %q", line)
		}
	}
}

func TestScrollbarRendersMarkerOnTrack(t *testing.T) {
	result := renderScrollbarWithMarkers(10, 100, 10, 0, []int{50})
	if !strings.Contains(result, scrollbarMarker) {
		t.Fatalf("expected scrollbar marker in output, got %q", result)
	}
}

func TestScrollbarRendersMarkerOnThumb(t *testing.T) {
	result := renderScrollbarWithMarkers(10, 100, 10, 0, []int{0})
	lines := strings.Split(result, "\n")
	if len(lines) == 0 || !strings.Contains(lines[0], scrollbarMarkerOnThumb) {
		t.Fatalf("expected marker-on-thumb at first row, got %q", result)
	}
}

func TestOverlayScrollbar_PadsAfterWideRuneTruncation(t *testing.T) {
	// First line width is exactly 10 and ends with a CJK rune (width 2).
	viewport := "12345678你\nabcdefghij"
	scrollbar := "│\n│"

	result := overlayScrollbar(viewport, scrollbar, 10)
	lines := strings.Split(result, "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}

	for i, line := range lines {
		if w := ansi.StringWidth(line); w != 10 {
			t.Fatalf("line %d width mismatch: got %d want 10", i, w)
		}
		right := ansi.Cut(line, 9, 10)
		if !strings.Contains(right, "│") {
			t.Fatalf("line %d rightmost cell should contain scrollbar, got %q", i, right)
		}
	}
}

func TestScrollbarOffsetForThumbPos_Clamps(t *testing.T) {
	got := scrollbarOffsetForThumbPos(-10, 20, 200)
	if got != 0 {
		t.Fatalf("expected clamped low thumb offset 0, got %d", got)
	}
	got = scrollbarOffsetForThumbPos(30, 20, 200)
	if got != 200 {
		t.Fatalf("expected clamped high thumb offset 200, got %d", got)
	}
}

func TestRoundDiv(t *testing.T) {
	tests := []struct {
		num  int
		den  int
		want int
	}{
		{num: 5, den: 2, want: 3},
		{num: 4, den: 2, want: 2},
		{num: -5, den: 2, want: -3},
		{num: -4, den: 2, want: -2},
		{num: 9, den: 0, want: 0},
	}
	for _, tc := range tests {
		got := roundDiv(tc.num, tc.den)
		if got != tc.want {
			t.Fatalf("roundDiv(%d,%d)=%d want %d", tc.num, tc.den, got, tc.want)
		}
	}
}

package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestWrapTextHardWrapsLongToken(t *testing.T) {
	input := strings.Repeat("a", 40)
	out := wrapText(input, 10)
	lines := strings.Split(out, "\n")

	if len(lines) < 4 {
		t.Fatalf("expected wrapped lines, got %d: %q", len(lines), out)
	}

	for i, line := range lines {
		if w := ansi.StringWidth(line); w > 10 {
			t.Fatalf("line %d too wide: got %d, want <= 10", i, w)
		}
	}

	if strings.ReplaceAll(out, "\n", "") != input {
		t.Fatalf("wrapped output should preserve content: %q", out)
	}
}

func TestWrapTextRespectsCJKWidth(t *testing.T) {
	input := "这是一个没有空格的很长中文字符串用于换行测试"
	out := wrapText(input, 12)
	lines := strings.Split(out, "\n")

	if len(lines) < 2 {
		t.Fatalf("expected wrapped CJK lines, got %d: %q", len(lines), out)
	}

	for i, line := range lines {
		if w := ansi.StringWidth(line); w > 12 {
			t.Fatalf("line %d too wide: got %d, want <= 12", i, w)
		}
	}
}

package tui

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// selectionPoint is a screen-buffer coordinate (0-indexed col/row within the
// chat viewport). Matches the anchor/focus model from claude-code's selection.ts.
type selectionPoint struct {
	Row int
	Col int
}

// selectionState tracks a linear text selection in the chat viewport.
type selectionState struct {
	Anchor     *selectionPoint
	Focus      *selectionPoint
	IsDragging bool
}

func (s *selectionState) hasSelection() bool {
	return s != nil && s.Anchor != nil && s.Focus != nil
}

func (s *selectionState) clear() {
	s.Anchor = nil
	s.Focus = nil
	s.IsDragging = false
}

func (s *selectionState) start(col, row int) {
	s.Anchor = &selectionPoint{Row: row, Col: col}
	s.Focus = nil
	s.IsDragging = true
}

func (s *selectionState) update(col, row int) {
	if !s.IsDragging {
		return
	}
	if s.Focus == nil && s.Anchor != nil &&
		s.Anchor.Col == col && s.Anchor.Row == row {
		return
	}
	s.Focus = &selectionPoint{Row: row, Col: col}
}

func (s *selectionState) finish() {
	s.IsDragging = false
}

func (s *selectionState) bounds() (start, end *selectionPoint) {
	if !s.hasSelection() {
		return nil, nil
	}
	if s.Anchor.Row < s.Focus.Row ||
		(s.Anchor.Row == s.Focus.Row && s.Anchor.Col <= s.Focus.Col) {
		return s.Anchor, s.Focus
	}
	return s.Focus, s.Anchor
}

func (s *selectionState) selectedText(viewportContent string) string {
	start, end := s.bounds()
	if start == nil {
		return ""
	}
	lines := strings.Split(viewportContent, "\n")
	var sb strings.Builder
	for row := start.Row; row <= end.Row && row < len(lines); row++ {
		if row < 0 {
			continue
		}
		stripped := ansi.Strip(lines[row])
		runes := []rune(stripped)
		colStart := 0
		if row == start.Row {
			colStart = start.Col
		}
		colEnd := len(runes)
		if row == end.Row {
			colEnd = end.Col + 1
		}
		if colStart < 0 {
			colStart = 0
		}
		if colEnd > len(runes) {
			colEnd = len(runes)
		}
		if row > start.Row {
			sb.WriteByte('\n')
		}
		if colStart < colEnd {
			sb.WriteString(string(runes[colStart:colEnd]))
		}
	}
	return sb.String()
}

// --- Model methods for text selection ---

// isInChatArea reports whether screen coordinates fall within the chat viewport.
func (m *Model) isInChatArea(x, y int) bool {
	top := m.layout.Chat.Y
	bottom := top + m.layout.Chat.Height
	left := m.layout.Chat.X
	right := m.layout.Chat.X + m.layout.Chat.Width - 2
	return x >= left && x <= right && y >= top && y < bottom
}

// screenToViewportCoords converts screen (x, y) to viewport-relative (row, col).
func (m *Model) screenToViewportCoords(x, y int) (vpRow, vpCol int) {
	vpRow = y - m.layout.Chat.Y
	vpCol = x - m.layout.Chat.X
	if vpRow < 0 {
		vpRow = 0
	}
	if vpRow >= m.layout.Chat.Height {
		vpRow = m.layout.Chat.Height - 1
	}
	if vpCol < 0 {
		vpCol = 0
	}
	return vpRow, vpCol
}

// copySelectionToClipboard copies selected text to clipboard via OSC 52.
func (m *Model) copySelectionToClipboard() {
	content := m.viewport.View()
	text := m.selection.selectedText(content)
	if text == "" {
		return
	}
	encoded := base64.StdEncoding.EncodeToString([]byte(text))
	seq := fmt.Sprintf("\x1b]52;c;%s\x1b\\", encoded)
	os.Stdout.WriteString(seq)
}

// --- Rendering helpers ---

func overlaySelection(output string, sel *selectionState, style lipgloss.Style) string {
	if sel == nil || !sel.hasSelection() {
		return output
	}
	start, end := sel.bounds()
	if start == nil {
		return output
	}
	lines := strings.Split(output, "\n")
	for row := start.Row; row <= end.Row && row < len(lines); row++ {
		if row < 0 {
			continue
		}
		colStart := 0
		if row == start.Row {
			colStart = start.Col
		}
		colEnd := lipgloss.Width(lines[row])
		if row == end.Row {
			colEnd = end.Col + 1
		}
		lines[row] = highlightLineRange(lines[row], colStart, colEnd, style)
	}
	return strings.Join(lines, "\n")
}

func highlightLineRange(line string, colStart, colEnd int, style lipgloss.Style) string {
	if colStart >= colEnd {
		return line
	}
	stripped := ansi.Strip(line)
	runes := []rune(stripped)
	lineWidth := len(runes)
	if colStart >= lineWidth {
		return line
	}
	if colEnd > lineWidth {
		colEnd = lineWidth
	}
	before := ""
	if colStart > 0 {
		before = ansi.Truncate(line, colStart, "")
	}
	middle := style.Render(string(runes[colStart:colEnd]))
	after := ""
	if colEnd < lineWidth {
		after = cutLeadingVisualCols(line, colEnd)
	}
	return before + middle + after
}

func cutLeadingVisualCols(s string, n int) string {
	visualCol := 0
	i := 0
	bytes := []byte(s)
	for i < len(bytes) && visualCol < n {
		if bytes[i] == 0x1b && i+1 < len(bytes) && bytes[i+1] == '[' {
			j := i + 2
			for j < len(bytes) && bytes[j] >= 0x30 && bytes[j] <= 0x3F {
				j++
			}
			for j < len(bytes) && bytes[j] >= 0x20 && bytes[j] <= 0x2F {
				j++
			}
			if j < len(bytes) {
				j++
			}
			i = j
		} else if bytes[i] == 0x1b && i+1 < len(bytes) && bytes[i+1] == ']' {
			j := i + 2
			for j < len(bytes) {
				if bytes[j] == 0x1b && j+1 < len(bytes) && bytes[j+1] == '\\' {
					j += 2
					break
				}
				if bytes[j] == 0x07 {
					j++
					break
				}
				j++
			}
			i = j
		} else {
			sz := 1
			b := bytes[i]
			if b >= 0xC0 && b < 0xE0 {
				sz = 2
			} else if b >= 0xE0 && b < 0xF0 {
				sz = 3
			} else if b >= 0xF0 {
				sz = 4
			}
			if i+sz > len(bytes) {
				sz = len(bytes) - i
			}
			i += sz
			visualCol++
		}
	}
	if i >= len(bytes) {
		return ""
	}
	return string(bytes[i:])
}

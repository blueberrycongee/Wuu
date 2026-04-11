package tui

import (
	"fmt"
	"strings"
)

const inlineSweepWidth = 7

var inlineSweepChars = []rune{'░', '▒', '▓', '█', '▓', '▒', '░'}

// renderInlineStatus renders an animated status line for display below the
// user message in the chat viewport.
func renderInlineStatus(status string, frame int) string {
	if !isInlineWaitingStatus(status) {
		return ""
	}

	label := statusLabel(status)
	sweep := renderInlineSweep(frame)
	styledLabel := inlineStatusLabelStyle.Render(label)

	return fmt.Sprintf("%s %s", sweep, styledLabel)
}

func renderInlineSweep(frame int) string {
	width := inlineSweepWidth * 2
	track := make([]rune, width)
	for i := range track {
		track[i] = '─'
	}

	start := frame % (width + inlineSweepWidth)
	start -= inlineSweepWidth
	for i, ch := range inlineSweepChars {
		pos := start + i
		if pos < 0 || pos >= width {
			continue
		}
		track[pos] = ch
	}

	var b strings.Builder
	for _, ch := range track {
		style := inlineStatusTrackStyle
		if ch != '─' {
			style = inlineStatusSweepStyle
		}
		b.WriteString(style.Render(string(ch)))
	}
	return b.String()
}

func isInlineWaitingStatus(status string) bool {
	switch {
	case status == "thinking":
		return true
	case status == "streaming" || status == "streaming response":
		return true
	case strings.HasPrefix(status, "tool:"):
		return true
	case strings.HasPrefix(status, "executing tool:"):
		return true
	default:
		return false
	}
}

// statusLabel maps internal statusLine values to user-friendly display labels.
func statusLabel(status string) string {
	switch {
	case status == "thinking":
		return "Thinking"
	case status == "streaming" || status == "streaming response":
		return "Generating"
	case strings.HasPrefix(status, "tool:"):
		return "Running " + strings.TrimPrefix(status, "tool: ")
	case strings.HasPrefix(status, "executing tool:"):
		return "Running " + strings.TrimPrefix(status, "executing tool: ")
	default:
		return ""
	}
}

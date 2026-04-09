package tui

import (
	"fmt"
	"strings"
)

// braille spinner frames — smooth rotation effect.
var spinFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// pulseDots produces a breathing/pulsing dots animation.
// Cycles: .   ..  ... ..  .   (repeat)
var pulseDots = []string{"·  ", "·· ", "···", "·· ", "·  ", "   "}

// renderInlineStatus renders an animated status line for display below the
// user message in the chat viewport.
func renderInlineStatus(status string, frame int) string {
	label := statusLabel(status)
	spin := spinFrames[frame%len(spinFrames)]
	dots := pulseDots[frame%len(pulseDots)]

	styledDot := inlineStatusDotStyle.Render(spin)
	styledLabel := inlineStatusLabelStyle.Render(label)
	styledDots := inlineStatusDotStyle.Render(dots)

	return fmt.Sprintf("%s %s %s", styledDot, styledLabel, styledDots)
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
		return "Working"
	}
}

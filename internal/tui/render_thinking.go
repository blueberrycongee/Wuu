package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

var spinnerFrames = []string{"◐", "◑", "◒", "◓"}

// renderThinkingBlock renders the thinking indicator and optional content.
func renderThinkingBlock(content string, done bool, expanded bool, duration time.Duration, width int, tick int) string {
	spinnerStyle := lipgloss.NewStyle().Foreground(currentTheme.Brand)
	labelStyle := lipgloss.NewStyle().Foreground(currentTheme.Subtle)
	timeStyle := lipgloss.NewStyle().Foreground(currentTheme.Inactive)
	contentStyle := lipgloss.NewStyle().Foreground(currentTheme.Subtle)
	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.Border{
			Top:         "╌",
			Bottom:      "╌",
			Left:        "│",
			Right:       "│",
			TopLeft:     "╭",
			TopRight:    "╮",
			BottomLeft:  "╰",
			BottomRight: "╯",
		}).
		BorderForeground(currentTheme.Subtle)

	var b strings.Builder

	if !done {
		// Active: spinner + "Thinking..." + elapsed
		frame := spinnerFrames[tick%len(spinnerFrames)]
		b.WriteString(" ")
		b.WriteString(spinnerStyle.Render(frame))
		b.WriteString(" ")
		b.WriteString(labelStyle.Render("Thinking..."))
		if duration > 0 {
			b.WriteString("  ")
			b.WriteString(timeStyle.Render(fmt.Sprintf("%.1fs", duration.Seconds())))
		}
	} else {
		// Completed: diamond + "Thought for Xs"
		b.WriteString(" ")
		b.WriteString(spinnerStyle.Render("◆"))
		b.WriteString(" ")
		label := fmt.Sprintf("Thought for %.1fs", duration.Seconds())
		b.WriteString(labelStyle.Render(label))
	}

	if expanded && strings.TrimSpace(content) != "" {
		innerW := width - 4
		if innerW < 20 {
			innerW = 20
		}
		wrapped := wrapText(content, innerW)
		styled := contentStyle.Render(wrapped)
		box := borderStyle.Width(innerW).Render(styled)
		b.WriteString("\n")
		b.WriteString(box)
	}

	return b.String()
}

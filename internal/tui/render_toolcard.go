package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// renderToolCard renders a single tool call card.
func renderToolCard(tc ToolCallEntry, width int) string {
	iconStyle := lipgloss.NewStyle().Foreground(currentTheme.ToolBorder)
	nameStyle := lipgloss.NewStyle().Bold(true).Foreground(currentTheme.ToolBorder)
	statusDone := lipgloss.NewStyle().Foreground(currentTheme.Success)
	statusRunning := lipgloss.NewStyle().Foreground(currentTheme.Warning)
	statusError := lipgloss.NewStyle().Foreground(currentTheme.Error)
	contentStyle := lipgloss.NewStyle().Foreground(currentTheme.Inactive)
	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(currentTheme.ToolBorder)

	var b strings.Builder

	// Header line: icon + name + status
	b.WriteString(" ")
	b.WriteString(iconStyle.Render("⚡"))
	b.WriteString(" ")
	b.WriteString(nameStyle.Render(tc.Name))

	switch tc.Status {
	case ToolCallDone:
		b.WriteString("  ")
		b.WriteString(statusDone.Render("✓ done"))
	case ToolCallRunning:
		b.WriteString("  ")
		b.WriteString(statusRunning.Render("⏳ running"))
	case ToolCallError:
		b.WriteString("  ")
		b.WriteString(statusError.Render("✗ error"))
	}

	// Collapsed: just the header line with brief args summary
	if tc.Collapsed {
		summary := toolArgsSummary(tc.Args, width-30)
		if summary != "" {
			b.WriteString(" ── ")
			b.WriteString(contentStyle.Render(summary))
		}
		return b.String()
	}

	// Expanded: show args and result in a bordered box
	innerW := width - 4
	if innerW < 20 {
		innerW = 20
	}

	var content strings.Builder
	if tc.Args != "" {
		content.WriteString(contentStyle.Render(wrapText(tc.Args, innerW)))
	}
	if tc.Result != "" {
		if content.Len() > 0 {
			content.WriteString("\n")
			content.WriteString(contentStyle.Render(strings.Repeat("─", min(innerW, 40))))
			content.WriteString("\n")
		}
		content.WriteString(contentStyle.Render(wrapText(truncateToolResult(tc.Result, 500), innerW)))
	}

	if content.Len() > 0 {
		box := borderStyle.Width(innerW).Render(content.String())
		b.WriteString("\n")
		b.WriteString(box)
	}

	return b.String()
}

// toolArgsSummary extracts a brief summary from tool arguments.
func toolArgsSummary(args string, maxWidth int) string {
	if args == "" || maxWidth <= 0 {
		return ""
	}
	summary := args
	// Strip JSON braces for readability.
	summary = strings.TrimPrefix(summary, "{")
	summary = strings.TrimSuffix(summary, "}")
	summary = strings.TrimSpace(summary)
	if len(summary) > maxWidth {
		summary = summary[:maxWidth] + "…"
	}
	return summary
}

// truncateToolResult shortens tool output for display.
func truncateToolResult(result string, maxLen int) string {
	if len(result) <= maxLen {
		return result
	}
	return result[:maxLen] + "\n… (truncated)"
}

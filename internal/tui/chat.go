package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// messageItemType identifies the kind of message.
type messageItemType int

const (
	itemUser messageItemType = iota
	itemAssistant
	itemTool
	itemSystem
)

// messageItem represents one entry in the chat transcript.
type messageItem struct {
	Type      messageItemType
	Content   string
	ToolName  string // for tool items
	ToolID    string // for tool items
	Collapsed bool   // for tool items, collapsible
	Streaming bool   // currently receiving content
}

// Styles for different message types.
var (
	userStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("12")). // blue
			Bold(true)

	assistantStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("15")) // white

	toolHeaderStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("11")). // yellow
			Bold(true)

	toolBodyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("8")) // gray

	systemStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("8")). // gray
			Italic(true)

	roleLabelStyle = lipgloss.NewStyle().
			Bold(true).
			PaddingRight(1)
)

// renderItem renders a single message item for display.
func renderItem(item messageItem, width int) string {
	if width <= 0 {
		width = 80
	}

	switch item.Type {
	case itemUser:
		label := roleLabelStyle.Copy().Foreground(lipgloss.Color("12")).Render("USER")
		content := userStyle.Render(item.Content)
		return fmt.Sprintf("%s\n%s", label, content)

	case itemAssistant:
		label := roleLabelStyle.Copy().Foreground(lipgloss.Color("10")).Render("ASSISTANT")
		content := assistantStyle.Render(item.Content)
		if item.Streaming {
			content += lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render("▊")
		}
		return fmt.Sprintf("%s\n%s", label, content)

	case itemTool:
		return renderToolItem(item, width)

	case itemSystem:
		label := roleLabelStyle.Copy().Foreground(lipgloss.Color("8")).Render("SYSTEM")
		content := systemStyle.Render(item.Content)
		return fmt.Sprintf("%s\n%s", label, content)

	default:
		return item.Content
	}
}

func renderToolItem(item messageItem, width int) string {
	icon := "├─"
	header := toolHeaderStyle.Render(fmt.Sprintf("%s [tool] %s", icon, item.ToolName))

	if item.Collapsed || strings.TrimSpace(item.Content) == "" {
		return header
	}

	// Indent tool output
	lines := strings.Split(item.Content, "\n")
	maxLines := 20
	truncated := false
	if len(lines) > maxLines {
		lines = lines[:maxLines]
		truncated = true
	}

	var body strings.Builder
	for _, line := range lines {
		body.WriteString("│  ")
		if len(line) > width-6 {
			line = line[:width-6] + "…"
		}
		body.WriteString(toolBodyStyle.Render(line))
		body.WriteString("\n")
	}
	if truncated {
		body.WriteString("│  ")
		body.WriteString(toolBodyStyle.Render(fmt.Sprintf("... (%d more lines)", len(lines)-maxLines)))
	}

	return header + "\n" + strings.TrimRight(body.String(), "\n")
}

// renderTranscript renders the full chat transcript from a list of items.
func renderTranscript(items []messageItem, width int) string {
	if len(items) == 0 {
		return ""
	}

	var b strings.Builder
	for i, item := range items {
		if i > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString(renderItem(item, width))
	}
	return b.String()
}

// transcriptEntryToItem converts a legacy transcriptEntry to a messageItem.
func transcriptEntryToItem(entry transcriptEntry) messageItem {
	switch strings.ToUpper(entry.Role) {
	case "USER":
		return messageItem{Type: itemUser, Content: entry.Content}
	case "ASSISTANT":
		return messageItem{Type: itemAssistant, Content: entry.Content}
	case "TOOL":
		return messageItem{Type: itemTool, Content: entry.Content, ToolName: "tool"}
	default:
		return messageItem{Type: itemSystem, Content: entry.Content}
	}
}

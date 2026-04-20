package tui

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/blueberrycongee/wuu/internal/stringutil"
	"github.com/charmbracelet/lipgloss"
)

// renderToolCard renders a single tool call with a compact tree layout.
func renderToolCard(tc *ToolCallEntry, width int, frame int) string {
	// Running tools have animated spinner — don't cache those.
	if tc.Status != ToolCallRunning {
		key := fmt.Sprintf("%s:%v:%d:%d", tc.Status, tc.Collapsed, len(tc.Args), len(tc.Result))
		if tc.cachedCard != "" && tc.cachedCardKey == key && tc.cachedCardWidth == width {
			return tc.cachedCard
		}
	}

	// ask_user has its own dedicated card layout.
	if tc.Name == "ask_user" {
		return renderAskUserCard(*tc, width)
	}

	var result string
	if tc.Collapsed {
		result = renderToolCardCollapsed(tc, width, frame)
	} else {
		result = renderToolCardExpanded(tc, width, frame)
	}

	// Cache for non-running tools.
	if tc.Status != ToolCallRunning {
		tc.cachedCard = result
		tc.cachedCardKey = fmt.Sprintf("%s:%v:%d:%d", tc.Status, tc.Collapsed, len(tc.Args), len(tc.Result))
		tc.cachedCardWidth = width
	}
	return result
}

// toolVerb maps a raw tool name to a user-facing verb.
func toolVerb(name string) string {
	switch strings.TrimSpace(name) {
	case "read_file":
		return "Read"
	case "write_file":
		return "Write"
	case "edit_file":
		return "Edit"
	case "list_files":
		return "List"
	case "run_shell":
		return "Shell"
	case "grep":
		return "Grep"
	case "glob":
		return "Glob"
	case "web_search":
		return "Search"
	case "web_fetch":
		return "Fetch"
	case "spawn_agent":
		return "Spawn"
	case "fork_agent":
		return "Fork"
	case "ask_user":
		return "Ask"
	default:
		return name
	}
}

// toolStatusIcon returns the icon and style for a tool status.
func toolStatusIcon(status ToolCallStatus, frame int) (string, lipgloss.Style) {
	switch status {
	case ToolCallRunning:
		return statusSpinner(frame), lipgloss.NewStyle().Foreground(currentTheme.Brand)
	case ToolCallError:
		return "✗", lipgloss.NewStyle().Foreground(currentTheme.Error)
	default:
		return "✓", lipgloss.NewStyle().Foreground(currentTheme.Success)
	}
}

// toolPrimaryArg extracts the most important argument for display.
func toolPrimaryArg(name, args string) string {
	if args == "" {
		return ""
	}
	var parsed map[string]any
	if json.Unmarshal([]byte(args), &parsed) != nil {
		return ""
	}

	switch name {
	case "read_file", "write_file", "edit_file", "list_files":
		if p, ok := parsed["path"].(string); ok {
			return p
		}
	case "run_shell":
		if c, ok := parsed["command"].(string); ok {
			return c
		}
	case "grep":
		if p, ok := parsed["pattern"].(string); ok {
			return p
		}
	case "glob":
		if p, ok := parsed["pattern"].(string); ok {
			return p
		}
	case "web_search":
		if q, ok := parsed["query"].(string); ok {
			return q
		}
	case "web_fetch":
		if u, ok := parsed["url"].(string); ok {
			return u
		}
	}
	return ""
}

// toolCompactParams returns a compact "key: val, key: val" summary
// excluding the primary argument.
func toolCompactParams(name, args string) string {
	if args == "" {
		return ""
	}
	var parsed map[string]any
	if json.Unmarshal([]byte(args), &parsed) != nil {
		return ""
	}

	// Remove primary key.
	primaryKey := ""
	switch name {
	case "read_file", "write_file", "edit_file", "list_files":
		primaryKey = "path"
	case "run_shell":
		primaryKey = "command"
	case "grep":
		primaryKey = "pattern"
	case "glob":
		primaryKey = "pattern"
	case "web_search":
		primaryKey = "query"
	case "web_fetch":
		primaryKey = "url"
	}
	delete(parsed, primaryKey)

	var parts []string
	for k, v := range parsed {
		switch val := v.(type) {
		case string:
			if len(val) > 50 {
				val = val[:47] + "…"
			}
			parts = append(parts, fmt.Sprintf("%s: %s", k, val))
		case float64:
			parts = append(parts, fmt.Sprintf("%s: %v", k, val))
		case bool:
			parts = append(parts, fmt.Sprintf("%s: %v", k, val))
		default:
			b, _ := json.Marshal(val)
			if len(b) > 50 {
				b = append(b[:47], '.', '.', '.')
			}
			parts = append(parts, fmt.Sprintf("%s: %s", k, string(b)))
		}
	}
	return strings.Join(parts, ", ")
}

func renderToolCardCollapsed(tc *ToolCallEntry, width int, frame int) string {
	icon, iconStyle := toolStatusIcon(tc.Status, frame)
	verb := toolVerb(tc.Name)
	primary := toolPrimaryArg(tc.Name, tc.Args)

	metaStyle := lipgloss.NewStyle().Foreground(currentTheme.Subtle)
	verbStyle := lipgloss.NewStyle().Bold(true).Foreground(currentTheme.Text)

	var b strings.Builder
	b.WriteString(iconStyle.Render(icon))
	b.WriteString(" ")
	b.WriteString(verbStyle.Render(verb))
	if primary != "" {
		b.WriteString(" ")
		b.WriteString(metaStyle.Render("(" + trimToWidth(primary, max(20, width-20)) + ")"))
	}

	// Inline diff stats for edit/write operations.
	if tc.Result != "" {
		if dr := diffResultFromJSON(tc.Result); dr != nil {
			b.WriteString("  ")
			b.WriteString(diffStats(dr))
		} else if tc.Status == ToolCallDone {
			// Show result length hint.
			lines := strings.Count(tc.Result, "\n")
			if lines > 0 {
				b.WriteString(" ")
				b.WriteString(metaStyle.Render(fmt.Sprintf("%d lines", lines+1)))
			}
		}
	}
	return b.String()
}

func renderToolCardExpanded(tc *ToolCallEntry, width int, frame int) string {
	header := renderToolCardCollapsed(tc, width, frame)
	if header == "" {
		return ""
	}

	var bodyParts []string
	innerW := width - 4
	if innerW < 20 {
		innerW = 20
	}

	// Compact params line.
	params := toolCompactParams(tc.Name, tc.Args)
	if params != "" {
		bodyParts = append(bodyParts, "├─ "+params)
	}

	// Result line.
	if tc.Result != "" {
		if dr := diffResultFromJSON(tc.Result); dr != nil {
			bodyParts = append(bodyParts, "└─ "+diffStats(dr))
		} else {
			truncated := truncateToolResult(tc.Result, 300)
			lines := strings.Split(truncated, "\n")
			if len(lines) == 1 {
				bodyParts = append(bodyParts, "└─ "+lines[0])
			} else {
				for i, line := range lines {
					prefix := "└─ "
					if i > 0 {
						prefix = "   "
					}
					bodyParts = append(bodyParts, prefix+line)
				}
			}
		}
	}

	if len(bodyParts) == 0 {
		return header
	}

	// Style the tree lines.
	metaStyle := lipgloss.NewStyle().Foreground(currentTheme.Inactive)
	styledBody := metaStyle.Render(strings.Join(bodyParts, "\n"))

	return header + "\n" + styledBody
}

// toolArgsSummary extracts a human-readable one-line summary from tool arguments.
// Kept for backward compatibility — now delegates to toolPrimaryArg.
func toolArgsSummary(toolName, args string, maxWidth int) string {
	summary := toolPrimaryArg(toolName, args)
	if summary == "" {
		// Generic fallback: first string value.
		var parsed map[string]any
		if json.Unmarshal([]byte(args), &parsed) == nil {
			for _, v := range parsed {
				if s, ok := v.(string); ok && s != "" {
					summary = s
					break
				}
			}
		}
	}
	if len(summary) > maxWidth {
		summary = summary[:maxWidth] + "…"
	}
	return summary
}

// formatToolArgs returns a human-readable multi-line format for expanded view.
func formatToolArgs(toolName, args string) string {
	var parsed map[string]any
	if json.Unmarshal([]byte(args), &parsed) != nil {
		return args
	}

	var lines []string
	for k, v := range parsed {
		switch val := v.(type) {
		case string:
			if len(val) > 200 {
				lines = append(lines, k+": ("+string(rune(len(val)))+" chars)")
			} else {
				lines = append(lines, k+": "+val)
			}
		default:
			b, _ := json.Marshal(val)
			lines = append(lines, k+": "+string(b))
		}
	}
	return strings.Join(lines, "\n")
}

// truncateToolResult shortens tool output for display.
func truncateToolResult(result string, maxLen int) string {
	return stringutil.Truncate(result, maxLen, "\n… (truncated)")
}

package tui

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/blueberrycongee/wuu/internal/tools"
)

func diffStyles() (
	add lipgloss.Style,
	del lipgloss.Style,
	context lipgloss.Style,
	gutter lipgloss.Style,
	hunkSep lipgloss.Style,
	newFile lipgloss.Style,
) {
	add = lipgloss.NewStyle().
		Foreground(currentTheme.DiffAddFg).
		Background(currentTheme.DiffAddBg)
	del = lipgloss.NewStyle().
		Foreground(currentTheme.DiffDeleteFg).
		Background(currentTheme.DiffDeleteBg)
	context = lipgloss.NewStyle().
		Foreground(currentTheme.Text)
	gutter = lipgloss.NewStyle().
		Foreground(currentTheme.Subtle)
	hunkSep = lipgloss.NewStyle().
		Foreground(currentTheme.Inactive)
	newFile = lipgloss.NewStyle().
		Foreground(currentTheme.Success).
		Italic(true)
	return
}

// diffResultFromJSON attempts to parse a DiffResult from a tool result JSON string.
// Returns nil if the result doesn't contain a diff field.
func diffResultFromJSON(resultJSON string) *tools.DiffResult {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(resultJSON), &raw); err != nil {
		return nil
	}
	diffBytes, ok := raw["diff"]
	if !ok {
		return nil
	}
	var dr tools.DiffResult
	if err := json.Unmarshal(diffBytes, &dr); err != nil {
		return nil
	}
	return &dr
}

// diffStats returns "+N/-M" summary string from a DiffResult.
func diffStats(dr *tools.DiffResult) string {
	diffAddStyle, diffDeleteStyle, _, _, _, _ := diffStyles()
	if dr.NewFile {
		return fmt.Sprintf("+%d (new)", dr.Lines)
	}
	var added, deleted int
	for _, h := range dr.Hunks {
		for _, l := range h.Lines {
			switch l.Op {
			case "insert":
				added++
			case "delete":
				deleted++
			}
		}
	}
	addStr := diffAddStyle.Render(fmt.Sprintf("+%d", added))
	delStr := diffDeleteStyle.Render(fmt.Sprintf("-%d", deleted))
	return addStr + " " + delStr
}

// renderDiff renders a full diff view with gutter, colors, and hunk separators.
func renderDiff(dr *tools.DiffResult, width int) string {
	diffAddStyle, diffDeleteStyle, diffContextStyle, diffGutterStyle, diffHunkSepStyle, diffNewFileStyle := diffStyles()
	if dr.NewFile {
		return diffNewFileStyle.Render(fmt.Sprintf("  new file (%d lines)", dr.Lines))
	}
	if len(dr.Hunks) == 0 {
		return diffContextStyle.Render("  (no changes)")
	}

	// Find max line number for gutter width.
	maxLine := 0
	for _, h := range dr.Hunks {
		oldLine := h.OldStart
		newLine := h.NewStart
		for _, l := range h.Lines {
			switch l.Op {
			case "equal":
				if oldLine > maxLine {
					maxLine = oldLine
				}
				if newLine > maxLine {
					maxLine = newLine
				}
				oldLine++
				newLine++
			case "delete":
				if oldLine > maxLine {
					maxLine = oldLine
				}
				oldLine++
			case "insert":
				if newLine > maxLine {
					maxLine = newLine
				}
				newLine++
			}
		}
	}

	gutterWidth := len(fmt.Sprintf("%d", maxLine))
	if gutterWidth < 3 {
		gutterWidth = 3
	}

	var b strings.Builder
	for i, h := range dr.Hunks {
		if i > 0 {
			b.WriteString(diffHunkSepStyle.Render(
				strings.Repeat(" ", gutterWidth) + " ⋮"))
			b.WriteString("\n")
		}

		oldLine := h.OldStart
		newLine := h.NewStart

		for _, l := range h.Lines {
			var lineNum string
			var marker string
			var style lipgloss.Style

			switch l.Op {
			case "equal":
				lineNum = fmt.Sprintf("%*d", gutterWidth, newLine)
				marker = " "
				style = diffContextStyle
				oldLine++
				newLine++
			case "delete":
				lineNum = fmt.Sprintf("%*d", gutterWidth, oldLine)
				marker = "-"
				style = diffDeleteStyle
				oldLine++
			case "insert":
				lineNum = fmt.Sprintf("%*d", gutterWidth, newLine)
				marker = "+"
				style = diffAddStyle
				newLine++
			}

			gutter := diffGutterStyle.Render(lineNum) + " "

			// Truncate content to fit width.
			contentWidth := width - gutterWidth - 3 // gutter + space + marker + space
			content := l.Content
			if lipgloss.Width(content) > contentWidth && contentWidth > 3 {
				content = content[:contentWidth-3] + "..."
			}

			line := style.Render(marker + " " + content)
			b.WriteString(gutter + line + "\n")
		}
	}

	return strings.TrimRight(b.String(), "\n")
}

package tui

import (
	"fmt"
	"strings"
)

type toolBurstKind uint8

const (
	toolBurstNone toolBurstKind = iota
	toolBurstRead
	toolBurstSearch
	toolBurstList
)

type toolBurstSummary struct {
	readCount   int
	searchCount int
	listCount   int
	latestHint  string
	status      ToolCallStatus
}

func classifyToolBurstTool(name string) toolBurstKind {
	switch name {
	case "read_file":
		return toolBurstRead
	case "grep", "glob":
		return toolBurstSearch
	case "list_files":
		return toolBurstList
	default:
		return toolBurstNone
	}
}

func shouldCollapseToolBurstByDefault(name string) bool {
	return classifyToolBurstTool(name) != toolBurstNone
}

func parseToolBlockIndex(block string) (int, bool) {
	if !strings.HasPrefix(block, "tool:") {
		return 0, false
	}
	var idx int
	if _, err := fmt.Sscanf(block, "tool:%d", &idx); err != nil {
		return 0, false
	}
	return idx, true
}

func summarizeToolBurst(calls []ToolCallEntry, width int) toolBurstSummary {
	summary := toolBurstSummary{status: ToolCallDone}
	hintWidth := max(24, width/2)
	for _, tc := range calls {
		switch classifyToolBurstTool(tc.Name) {
		case toolBurstRead:
			summary.readCount++
		case toolBurstSearch:
			summary.searchCount++
		case toolBurstList:
			summary.listCount++
		}
		switch tc.Status {
		case ToolCallError:
			summary.status = ToolCallError
		case ToolCallRunning:
			if summary.status != ToolCallError {
				summary.status = ToolCallRunning
			}
		}
		if hint := toolArgsSummary(tc.Name, tc.Args, hintWidth); hint != "" {
			summary.latestHint = hint
		}
	}
	return summary
}

func toolBurstName(summary toolBurstSummary) string {
	var parts []string
	if summary.readCount > 0 {
		parts = append(parts, "read")
	}
	if summary.searchCount > 0 {
		parts = append(parts, "search")
	}
	if summary.listCount > 0 {
		parts = append(parts, "list")
	}
	if len(parts) == 0 {
		return "tools"
	}
	return strings.Join(parts, "/")
}

func toolBurstCountSummary(summary toolBurstSummary) string {
	var parts []string
	if summary.readCount > 0 {
		parts = append(parts, pluralize(summary.readCount, "read"))
	}
	if summary.searchCount > 0 {
		parts = append(parts, pluralize(summary.searchCount, "search"))
	}
	if summary.listCount > 0 {
		parts = append(parts, pluralize(summary.listCount, "list"))
	}
	return strings.Join(parts, ", ")
}

func pluralize(n int, singular string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, singular)
	}
	return fmt.Sprintf("%d %ss", n, singular)
}

func renderToolBurstGroup(calls []ToolCallEntry, width int, frame int) string {
	if len(calls) == 0 {
		return ""
	}
	if len(calls) == 1 {
		return renderToolCard(&calls[0], width, frame)
	}

	summary := summarizeToolBurst(calls, width)
	bullet := toolBullet(summary.status, frame)
	verb := "Called"
	if summary.status == ToolCallRunning {
		verb = "Calling"
	}

	headerParts := []string{
		bullet,
		toolVerbStyle.Render(verb),
		toolNameStyle.Render(toolBurstName(summary)),
	}
	if counts := toolBurstCountSummary(summary); counts != "" {
		headerParts = append(headerParts, toolMetaDim.Render("· "+counts))
	}
	if summary.latestHint != "" {
		headerParts = append(headerParts, toolMetaDim.Render("· "+summary.latestHint))
	}

	collapsed := calls[0].Collapsed
	if collapsed {
		headerParts = append(headerParts, toolMetaDim.Render("· press 'o' to expand"))
	} else {
		headerParts = append(headerParts, toolMetaDim.Render("· press 'o' to collapse"))
	}
	header := strings.Join(headerParts, " ")
	if collapsed {
		return header
	}

	lines := []string{header}
	for _, tc := range calls {
		lines = append(lines, renderToolBurstChild(tc, width, frame))
	}
	return strings.Join(lines, "\n")
}

func renderToolBurstChild(tc ToolCallEntry, width int, frame int) string {
	parts := []string{toolBullet(tc.Status, frame), toolNameStyle.Render(tc.Name)}
	if summary := toolArgsSummary(tc.Name, tc.Args, max(20, width-12)); summary != "" {
		parts = append(parts, toolMetaDim.Render("· "+summary))
	}
	line := wrapText(strings.Join(parts, " "), max(20, width-4))
	return indentTreeResult(line, width)
}

func (m *Model) toggleLastToolBurstGroup() bool {
	for entryIdx := len(m.entries) - 1; entryIdx >= 0; entryIdx-- {
		entry := &m.entries[entryIdx]
		if start, ok := lastToolBurstStart(entry); ok {
			entry.ToolCalls[start].Collapsed = !entry.ToolCalls[start].Collapsed
			entry.composited = ""
			entry.compositedKey = 0
			entry.compositedH = 0
			return true
		}
	}
	return false
}

func lastToolBurstStart(entry *transcriptEntry) (int, bool) {
	if entry == nil || len(entry.ToolCalls) < 2 {
		return 0, false
	}

	if len(entry.blockOrder) > 0 {
		runStart := -1
		runCount := 0
		for i := len(entry.blockOrder) - 1; i >= 0; i-- {
			idx, ok := parseToolBlockIndex(entry.blockOrder[i])
			if !ok || idx < 0 || idx >= len(entry.ToolCalls) || classifyToolBurstTool(entry.ToolCalls[idx].Name) == toolBurstNone {
				if runCount > 1 {
					return runStart, true
				}
				runStart = -1
				runCount = 0
				continue
			}
			runStart = idx
			runCount++
		}
		if runCount > 1 {
			return runStart, true
		}
	}

	runCount := 0
	runStart := -1
	for i := len(entry.ToolCalls) - 1; i >= 0; i-- {
		if classifyToolBurstTool(entry.ToolCalls[i].Name) == toolBurstNone {
			if runCount > 1 {
				return runStart, true
			}
			runStart = -1
			runCount = 0
			continue
		}
		runStart = i
		runCount++
	}
	if runCount > 1 {
		return runStart, true
	}
	return 0, false
}

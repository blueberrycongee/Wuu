package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	processruntime "github.com/blueberrycongee/wuu/internal/process"
)

const processPanelMaxRows = 6

func (m Model) processPanelHeight() int {
	processes := m.visibleProcesses()
	if len(processes) == 0 {
		return 0
	}
	rows := len(processes)
	if rows > processPanelMaxRows {
		rows = processPanelMaxRows
	}
	return rows + 1
}

func (m Model) visibleProcesses() []processruntime.Process {
	if m.processManager == nil {
		return nil
	}
	list, err := m.processManager.List()
	if err != nil {
		return nil
	}
	out := make([]processruntime.Process, 0, len(list))
	for _, p := range list {
		if p.Status == processruntime.StatusRunning || p.Status == processruntime.StatusStarting || p.Status == processruntime.StatusStopping {
			out = append(out, p)
		}
	}
	return out
}

func (m Model) renderProcessPanel(width int) string {
	processes := m.visibleProcesses()
	if len(processes) == 0 {
		return ""
	}

	var b strings.Builder
	titleStatus := workerRunningStatus(fmt.Sprintf("%d background process(es)", len(processes)))
	title := fmt.Sprintf(" Processes · %s", renderStatusHeader(titleStatus, m.spinnerFrame))
	b.WriteString(processPanelTitleStyle.Render(fitToWidth(title, width)))

	now := time.Now()
	limit := len(processes)
	if limit > processPanelMaxRows {
		limit = processPanelMaxRows
	}
	for i := 0; i < limit; i++ {
		p := processes[i]
		row := formatProcessPanelRow(p, now)
		b.WriteString("\n")
		b.WriteString(processPanelRowStyle.Render(trimToWidth(row, width)))
	}
	if len(processes) > limit {
		b.WriteString("\n")
		b.WriteString(processPanelRowStyle.Render(trimToWidth(fmt.Sprintf("+%d more", len(processes)-limit), width)))
	}
	return b.String()
}

func formatProcessPanelRow(p processruntime.Process, now time.Time) string {
	name := processDisplayName(p)
	status := string(p.Status)
	owner := processOwnerLabel(p)
	lifecycle := string(p.Lifecycle)
	uptime := processUptime(p, now)
	parts := []string{name, status, lifecycle, owner, uptime}
	return strings.Join(parts, " · ")
}

func processDisplayName(p processruntime.Process) string {
	name := strings.TrimSpace(p.Command)
	if name == "" {
		name = p.ID
	}
	name = strings.Join(strings.Fields(name), " ")
	if len(name) > 40 {
		name = truncate(name, 40)
	}
	return name
}

func processOwnerLabel(p processruntime.Process) string {
	switch p.OwnerKind {
	case processruntime.OwnerMainAgent:
		return "owner:main"
	case processruntime.OwnerSubagent:
		if strings.TrimSpace(p.OwnerID) != "" {
			return "owner:subagent"
		}
		return "owner:subagent"
	default:
		return "owner:unknown"
	}
}

func processUptime(p processruntime.Process, now time.Time) string {
	if p.StartedAt.IsZero() {
		return "0s"
	}
	end := now
	if !p.StoppedAt.IsZero() {
		end = p.StoppedAt
	}
	if end.Before(p.StartedAt) {
		end = p.StartedAt
	}
	return formatElapsed(end.Sub(p.StartedAt))
}

var (
	processPanelTitleStyle lipgloss.Style
	processPanelRowStyle   lipgloss.Style
)

func initProcessPanelStyles() {
	processPanelTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(currentTheme.Subtle)
	processPanelRowStyle = lipgloss.NewStyle().Foreground(currentTheme.Subtle)
}

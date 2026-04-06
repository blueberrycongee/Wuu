package tui

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	minOutputHeight = 6
	inputHeight     = 3
	headerHeight    = 2
	footerHeight    = 2
)

type tickMsg struct {
	now time.Time
}

type responseMsg struct {
	prompt  string
	answer  string
	err     error
	elapsed time.Duration
}

// Model implements the terminal UI state machine.
type Model struct {
	provider      string
	modelName     string
	configPath    string
	workspaceRoot string
	runPrompt     func(ctx context.Context, prompt string) (string, error)

	viewport viewport.Model
	input    textarea.Model

	width       int
	height      int
	transcript  []string
	pending     bool
	autoFollow  bool
	showJump    bool
	clock       string
	statusLine  string
	pendingText string
}

// NewModel builds the initial UI model.
func NewModel(cfg Config) Model {
	vp := viewport.New(80, minOutputHeight)
	vp.SetContent("")

	in := textarea.New()
	in.Placeholder = "Type prompt or slash command (/resume /fork /worktree /skills /insight)"
	in.Focus()
	in.SetWidth(80)
	in.SetHeight(inputHeight)
	in.ShowLineNumbers = false
	in.Prompt = "> "
	in.CharLimit = 0

	return Model{
		provider:      cfg.Provider,
		modelName:     cfg.Model,
		configPath:    cfg.ConfigPath,
		workspaceRoot: filepath.Dir(cfg.ConfigPath),
		runPrompt:     cfg.RunPrompt,
		viewport:      vp,
		input:         in,
		autoFollow:    true,
		clock:         time.Now().Format("15:04:05"),
		statusLine:    "ready",
	}
}

// Init starts the clock ticker.
func (m Model) Init() tea.Cmd {
	return tickCmd()
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg{now: t}
	})
}

// Update handles events.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.relayout()
		return m, nil

	case tickMsg:
		m.clock = msg.now.Format("15:04:05")
		return m, tickCmd()

	case responseMsg:
		m.pending = false
		m.pendingText = ""
		if msg.err != nil {
			m.appendEntry("system", fmt.Sprintf("error: %v", msg.err))
			m.statusLine = "request failed"
		} else {
			m.appendEntry("assistant", msg.answer)
			m.statusLine = fmt.Sprintf("response in %s", msg.elapsed.Truncate(10*time.Millisecond))
		}
		m.refreshViewport(true)
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "enter":
			return m.submit()
		case "ctrl+j", "end":
			m.viewport.GotoBottom()
			m.autoFollow = true
			m.showJump = false
			return m, nil
		case "pgup", "ctrl+u":
			m.viewport.ViewUp()
			m.autoFollow = false
			m.showJump = !m.viewport.AtBottom()
			return m, nil
		case "pgdown", "ctrl+d":
			m.viewport.ViewDown()
			m.showJump = !m.viewport.AtBottom()
			m.autoFollow = !m.showJump
			return m, nil
		}
	}

	var cmds []tea.Cmd
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	if cmd != nil {
		cmds = append(cmds, cmd)
	}

	m.viewport, cmd = m.viewport.Update(msg)
	if cmd != nil {
		cmds = append(cmds, cmd)
	}
	m.showJump = !m.viewport.AtBottom()

	return m, tea.Batch(cmds...)
}

func (m Model) submit() (tea.Model, tea.Cmd) {
	raw := strings.TrimSpace(m.input.Value())
	if raw == "" {
		return m, nil
	}

	if output, handled := m.handleSlash(raw); handled {
		m.appendEntry("system", output)
		m.input.Reset()
		m.statusLine = "command executed"
		m.refreshViewport(true)
		return m, nil
	}

	if m.pending {
		return m, nil
	}

	m.appendEntry("user", raw)
	m.input.Reset()
	m.pending = true
	m.pendingText = "thinking..."
	m.statusLine = "running prompt"
	m.refreshViewport(true)

	start := time.Now()
	return m, func() tea.Msg {
		answer, err := m.runPrompt(context.Background(), raw)
		return responseMsg{
			prompt:  raw,
			answer:  answer,
			err:     err,
			elapsed: time.Since(start),
		}
	}
}

func (m *Model) appendEntry(role, content string) {
	text := strings.TrimSpace(content)
	if text == "" {
		text = "(empty)"
	}
	prefix := strings.ToUpper(role)
	m.transcript = append(m.transcript, fmt.Sprintf("%s\n%s", prefix, text))
}

func (m Model) entryCount() int {
	return len(m.transcript)
}

func (m *Model) refreshViewport(forceBottom bool) {
	content := strings.Join(m.transcript, "\n\n")
	if m.pending && m.pendingText != "" {
		if content != "" {
			content += "\n\n"
		}
		content += "ASSISTANT\n" + m.pendingText
	}
	m.viewport.SetContent(content)
	if forceBottom || m.autoFollow {
		m.viewport.GotoBottom()
	}
	m.showJump = !m.viewport.AtBottom()
}

func (m *Model) relayout() {
	if m.width <= 0 || m.height <= 0 {
		return
	}

	m.input.SetWidth(max(16, m.width-2))
	outputHeight := m.height - headerHeight - footerHeight - inputHeight
	if outputHeight < minOutputHeight {
		outputHeight = minOutputHeight
	}
	m.viewport.Width = max(16, m.width-2)
	m.viewport.Height = outputHeight
	m.refreshViewport(false)
}

// View renders the full terminal.
func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return "loading..."
	}

	title := lipgloss.NewStyle().Bold(true).Render(
		fmt.Sprintf("wuu tui | provider=%s | model=%s", m.provider, m.modelName),
	)
	meta := lipgloss.NewStyle().Faint(true).Render(fmt.Sprintf("config: %s", m.configPath))
	jumpHint := ""
	if m.showJump {
		jumpHint = " | Ctrl+J jump to bottom"
	}
	clock := lipgloss.NewStyle().Faint(true).Render(m.clock)
	status := lipgloss.NewStyle().Faint(true).Render(m.statusLine + jumpHint)
	footer := lipgloss.JoinHorizontal(lipgloss.Left, status, strings.Repeat(" ", max(1, m.width-lipgloss.Width(status)-lipgloss.Width(clock)-2)), clock)

	outputBox := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		Padding(0, 1).
		Width(max(16, m.width-2)).
		Height(m.viewport.Height).
		Render(m.viewport.View())

	inputBox := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		Padding(0, 1).
		Width(max(16, m.width-2)).
		Render(m.input.View())

	return strings.Join([]string{
		title,
		meta,
		outputBox,
		inputBox,
		footer,
	}, "\n")
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

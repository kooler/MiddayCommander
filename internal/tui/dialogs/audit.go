package dialogs

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/kooler/MiddayCommander/internal/audit"
	"github.com/kooler/MiddayCommander/internal/ui/overlay"
	"github.com/kooler/MiddayCommander/internal/ui/theme"
)

type AuditDismissMsg struct{}
type AuditRefreshMsg struct{}

type AuditModel struct {
	path    string
	entries []audit.Event
	offset  int
	width   int
	height  int
	errText string
}

func NewAudit(path string, entries []audit.Event, err error, width, height int) AuditModel {
	model := AuditModel{
		path:    path,
		entries: entries,
		width:   width,
		height:  height,
	}
	if err != nil {
		model.errText = err.Error()
	}
	return model
}

func (m AuditModel) Update(msg tea.KeyMsg) (AuditModel, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		return m, func() tea.Msg { return AuditDismissMsg{} }
	case "r":
		return m, func() tea.Msg { return AuditRefreshMsg{} }
	case "up", "k":
		if m.offset > 0 {
			m.offset--
		}
	case "down", "j":
		if m.offset < max(0, len(m.entries)-m.visibleRows()) {
			m.offset++
		}
	case "pgup":
		m.offset -= m.visibleRows()
		if m.offset < 0 {
			m.offset = 0
		}
	case "pgdown":
		m.offset += m.visibleRows()
		maxOffset := max(0, len(m.entries)-m.visibleRows())
		if m.offset > maxOffset {
			m.offset = maxOffset
		}
	case "home":
		m.offset = 0
	case "end":
		m.offset = max(0, len(m.entries)-m.visibleRows())
	}
	return m, nil
}

func (m AuditModel) BoxSize(screenWidth, screenHeight int) (int, int) {
	width := screenWidth * 4 / 5
	if width < 72 {
		width = min(72, screenWidth)
	}
	height := screenHeight * 3 / 4
	if height < 12 {
		height = min(12, screenHeight)
	}
	return width, height
}

func (m AuditModel) visibleRows() int {
	_, boxHeight := m.BoxSize(m.width, m.height)
	rows := boxHeight - 4
	if rows < 1 {
		rows = 1
	}
	return rows
}

func (m AuditModel) View(th theme.Theme, screenWidth, screenHeight int) string {
	boxWidth, boxHeight := m.BoxSize(screenWidth, screenHeight)
	innerWidth := boxWidth - 2

	bg := lipgloss.Color("#1e1e2e")
	fg := lipgloss.Color("#cdd6f4")
	subtle := lipgloss.Color("#a6adc8")
	accent := lipgloss.Color("#89b4fa")
	highlight := lipgloss.Color("#f9e2af")
	success := lipgloss.Color("#a6e3a1")
	failure := lipgloss.Color("#f38ba8")

	bgStyle := lipgloss.NewStyle().Background(bg).Foreground(fg)
	dimStyle := lipgloss.NewStyle().Background(bg).Foreground(subtle)
	keyStyle := lipgloss.NewStyle().Background(bg).Foreground(accent).Bold(true)
	successStyle := lipgloss.NewStyle().Background(bg).Foreground(success)
	failureStyle := lipgloss.NewStyle().Background(bg).Foreground(failure)
	warnStyle := lipgloss.NewStyle().Background(bg).Foreground(highlight)

	lines := make([]string, 0, boxHeight-2)
	if m.errText != "" {
		lines = append(lines, failureStyle.Render(" "+padRight(m.errText, innerWidth-1)))
	} else if len(m.entries) == 0 {
		lines = append(lines, dimStyle.Render(" "+padRight("No audit events yet.", innerWidth-1)))
	} else {
		end := min(len(m.entries), m.offset+m.visibleRows())
		for _, entry := range m.entries[m.offset:end] {
			style := dimStyle
			switch entry.Status {
			case "completed":
				style = successStyle
			case "failed":
				style = failureStyle
			case "canceled", "paused", "retrying":
				style = warnStyle
			}
			lines = append(lines, style.Render(" "+padRight(formatAuditEvent(entry), innerWidth-1)))
		}
	}

	footer := keyStyle.Render(" ↑↓") + dimStyle.Render(":Scroll") +
		dimStyle.Render("  ") +
		keyStyle.Render("PgUp/PgDn") + dimStyle.Render(":Page") +
		dimStyle.Render("  ") +
		keyStyle.Render("r") + dimStyle.Render(":Refresh") +
		dimStyle.Render("  ") +
		keyStyle.Render("Esc") + dimStyle.Render(":Close")
	footerWidth := lipgloss.Width(footer)
	if footerWidth < innerWidth {
		footer += bgStyle.Render(strings.Repeat(" ", innerWidth-footerWidth))
	}

	title := "Audit Log"
	if m.path != "" {
		title = "Audit Log"
	}
	return overlay.RenderBox(title, lines, footer, boxWidth, boxHeight, accent, bg, highlight)
}

func formatAuditEvent(event audit.Event) string {
	timestamp := "-"
	if !event.Time.IsZero() {
		timestamp = event.Time.Local().Format("2006-01-02 15:04:05")
	}
	parts := []string{timestamp}
	if event.Status != "" {
		parts = append(parts, strings.ToUpper(event.Status))
	}
	if event.Operation != "" {
		parts = append(parts, event.Operation)
	}
	if event.JobID != "" {
		parts = append(parts, event.JobID)
	}
	if event.Message != "" {
		parts = append(parts, event.Message)
	} else if event.Error != "" {
		parts = append(parts, event.Error)
	}
	line := strings.Join(parts, " | ")
	if event.Dest != "" {
		line += fmt.Sprintf(" -> %s", event.Dest)
	}
	return line
}

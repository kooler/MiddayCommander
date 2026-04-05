package dialogs

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/kooler/MiddayCommander/internal/transfer"
	"github.com/kooler/MiddayCommander/internal/ui/overlay"
	"github.com/kooler/MiddayCommander/internal/ui/theme"
)

type TransferDismissMsg struct{}

type TransferModel struct {
	snapshot transfer.Snapshot
	width    int
	height   int
}

func NewTransfer(width, height int) TransferModel {
	return TransferModel{width: width, height: height}
}

func (m *TransferModel) SetSnapshot(snapshot transfer.Snapshot) {
	m.snapshot = snapshot
}

func (m TransferModel) BoxSize(screenWidth, screenHeight int) (int, int) {
	width := screenWidth * 2 / 3
	if width < 54 {
		width = min(54, screenWidth)
	}
	height := 11
	if len(m.snapshot.Queue) > 0 {
		height += min(3, len(m.snapshot.Queue))
	}
	if len(m.snapshot.Recent) > 0 {
		height += min(3, len(m.snapshot.Recent))
	}
	maxHeight := screenHeight * 3 / 4
	if height > maxHeight {
		height = maxHeight
	}
	return width, height
}

func (m TransferModel) Update(msg tea.KeyMsg) (TransferModel, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		return m, func() tea.Msg { return TransferDismissMsg{} }
	}
	return m, nil
}

func (m TransferModel) View(th theme.Theme, screenWidth, screenHeight int) string {
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
	headingStyle := lipgloss.NewStyle().Background(bg).Foreground(accent).Bold(true)
	successStyle := lipgloss.NewStyle().Background(bg).Foreground(success)
	failureStyle := lipgloss.NewStyle().Background(bg).Foreground(failure)
	keyStyle := lipgloss.NewStyle().Background(bg).Foreground(accent).Bold(true)

	lines := make([]string, 0, boxHeight-2)
	if current := m.snapshot.Current; current != nil {
		lines = append(lines, headingStyle.Render(" Current"))
		lines = append(lines, bgStyle.Render(" "+padRight(transferTitle(*current), innerWidth-1)))
		lines = append(lines, renderTransferBar(current.Percent(), innerWidth, successStyle, dimStyle))
		if current.Progress.Current != "" {
			lines = append(lines, dimStyle.Render(" "+padRight(current.Progress.Current, innerWidth-1)))
		}
	} else {
		lines = append(lines, headingStyle.Render(" Current"))
		lines = append(lines, dimStyle.Render(" idle"))
	}

	if len(m.snapshot.Queue) > 0 {
		lines = append(lines, headingStyle.Render(" Queued"))
		for _, queued := range m.snapshot.Queue[:min(3, len(m.snapshot.Queue))] {
			lines = append(lines, dimStyle.Render(" "+padRight(transferTitle(queued), innerWidth-1)))
		}
	}

	if len(m.snapshot.Recent) > 0 {
		lines = append(lines, headingStyle.Render(" Recent"))
		for _, recent := range m.snapshot.Recent[:min(3, len(m.snapshot.Recent))] {
			style := dimStyle
			if recent.State == transfer.StateCompleted {
				style = successStyle
			}
			if recent.State == transfer.StateFailed {
				style = failureStyle
			}
			lines = append(lines, style.Render(" "+padRight(transferTitle(recent), innerWidth-1)))
			if recent.State == transfer.StateFailed && recent.Error != "" {
				lines = append(lines, failureStyle.Render(" "+padRight(recent.Error, innerWidth-1)))
			}
		}
	}

	footer := keyStyle.Render(" Esc") + dimStyle.Render(":Hide") +
		dimStyle.Render("  ") +
		keyStyle.Render("Queue") + dimStyle.Render(fmt.Sprintf(": %d", len(m.snapshot.Queue)))
	footerWidth := lipgloss.Width(footer)
	if footerWidth < innerWidth {
		footer += dimStyle.Render(strings.Repeat(" ", innerWidth-footerWidth))
	}

	return overlay.RenderBox("Transfers", lines, footer, boxWidth, boxHeight, accent, bg, highlight)
}

func transferTitle(status transfer.JobStatus) string {
	progress := ""
	if status.Progress.TotalFiles > 0 {
		progress = fmt.Sprintf(" [%d/%d]", status.Progress.DoneFiles, status.Progress.TotalFiles)
	}
	attempt := ""
	if status.TotalAttempts() > 1 {
		attempt = fmt.Sprintf(" (%d/%d)", max(1, status.Attempt), status.TotalAttempts())
	}
	return fmt.Sprintf("%s -> %s%s%s", strings.ToUpper(string(status.Job.Operation)), status.Job.DestDir.Display(), attempt, progress)
}

func renderTransferBar(progress float64, innerWidth int, fillStyle, emptyStyle lipgloss.Style) string {
	barWidth := innerWidth - 2
	if barWidth < 1 {
		barWidth = 1
	}
	filled := int(progress * float64(barWidth))
	if filled > barWidth {
		filled = barWidth
	}
	return fillStyle.Render(" "+strings.Repeat("█", filled)) +
		emptyStyle.Render(strings.Repeat("░", barWidth-filled)+" ")
}

package dialogs

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	midfs "github.com/kooler/MiddayCommander/internal/fs"
	"github.com/kooler/MiddayCommander/internal/transfer"
	"github.com/kooler/MiddayCommander/internal/ui/overlay"
	"github.com/kooler/MiddayCommander/internal/ui/theme"
)

type TransferOptionsSubmitMsg struct {
	Operation transfer.Operation
	Conflict  transfer.ConflictPolicy
	Verify    transfer.VerifyMode
	Retries   int
}

type TransferOptionsDismissMsg struct{}

type transferOptionField int

const (
	transferOptionConflict transferOptionField = iota
	transferOptionVerify
	transferOptionRetries
)

type TransferOptionsModel struct {
	operation   transfer.Operation
	sourceCount int
	dest        midfs.URI
	conflict    transfer.ConflictPolicy
	verify      transfer.VerifyMode
	retries     int
	activeField int
	width       int
	height      int
}

func NewTransferOptions(op transfer.Operation, sourceCount int, dest midfs.URI, width, height int) TransferOptionsModel {
	return TransferOptionsModel{
		operation:   op,
		sourceCount: sourceCount,
		dest:        dest,
		conflict:    transfer.ConflictOverwrite,
		verify:      transfer.VerifySize,
		retries:     1,
		width:       width,
		height:      height,
	}
}

func (m TransferOptionsModel) Update(msg tea.KeyMsg) (TransferOptionsModel, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		return m, func() tea.Msg { return TransferOptionsDismissMsg{} }
	case "tab", "down":
		m.activeField = (m.activeField + 1) % 3
		return m, nil
	case "shift+tab", "up":
		m.activeField = (m.activeField + 2) % 3
		return m, nil
	case "left", "h":
		m.stepField(-1)
		return m, nil
	case "right", "l":
		m.stepField(1)
		return m, nil
	case "enter":
		return m, func() tea.Msg {
			return TransferOptionsSubmitMsg{
				Operation: m.operation,
				Conflict:  m.conflict,
				Verify:    m.verify,
				Retries:   m.retries,
			}
		}
	}

	if m.activeField == int(transferOptionRetries) {
		switch msg.String() {
		case "0", "1", "2", "3", "4", "5":
			m.retries = int(msg.String()[0] - '0')
			return m, nil
		}
	}

	return m, nil
}

func (m *TransferOptionsModel) stepField(delta int) {
	switch transferOptionField(m.activeField) {
	case transferOptionConflict:
		options := []transfer.ConflictPolicy{
			transfer.ConflictOverwrite,
			transfer.ConflictSkip,
			transfer.ConflictRename,
		}
		m.conflict = rotateConflict(options, m.conflict, delta)
	case transferOptionVerify:
		options := []transfer.VerifyMode{
			transfer.VerifySize,
			transfer.VerifySHA256,
			transfer.VerifyNone,
		}
		m.verify = rotateVerify(options, m.verify, delta)
	case transferOptionRetries:
		m.retries += delta
		if m.retries < 0 {
			m.retries = 5
		}
		if m.retries > 5 {
			m.retries = 0
		}
	}
}

func (m TransferOptionsModel) BoxSize(screenWidth, screenHeight int) (int, int) {
	width := screenWidth * 2 / 3
	if width < 58 {
		width = min(58, screenWidth)
	}
	height := 10
	maxHeight := screenHeight * 3 / 4
	if height > maxHeight {
		height = maxHeight
	}
	return width, height
}

func (m TransferOptionsModel) View(th theme.Theme, screenWidth, screenHeight int) string {
	boxWidth, boxHeight := m.BoxSize(screenWidth, screenHeight)
	innerWidth := boxWidth - 2

	bg := lipgloss.Color("#1e1e2e")
	fg := lipgloss.Color("#cdd6f4")
	subtle := lipgloss.Color("#a6adc8")
	accent := lipgloss.Color("#89b4fa")
	highlight := lipgloss.Color("#f9e2af")
	cursorBg := lipgloss.Color("#45475a")

	bgStyle := lipgloss.NewStyle().Background(bg).Foreground(fg)
	dimStyle := lipgloss.NewStyle().Background(bg).Foreground(subtle)
	labelStyle := lipgloss.NewStyle().Background(bg).Foreground(accent).Bold(true)
	valueStyle := lipgloss.NewStyle().Background(bg).Foreground(highlight)
	activeStyle := lipgloss.NewStyle().Background(cursorBg).Foreground(fg)

	summary := fmt.Sprintf("%s %d item(s) -> %s", strings.ToUpper(string(m.operation)), m.sourceCount, m.dest.Display())
	lines := []string{
		bgStyle.Render(" " + padRight(summary, innerWidth-1)),
		renderTransferOptionLine("Conflict", conflictLabel(m.conflict), m.activeField == int(transferOptionConflict), innerWidth, labelStyle, valueStyle, dimStyle, activeStyle, bgStyle),
		renderTransferOptionLine("Verify", verifyLabel(m.verify), m.activeField == int(transferOptionVerify), innerWidth, labelStyle, valueStyle, dimStyle, activeStyle, bgStyle),
		renderTransferOptionLine("Retries", retryLabel(m.retries), m.activeField == int(transferOptionRetries), innerWidth, labelStyle, valueStyle, dimStyle, activeStyle, bgStyle),
		dimStyle.Render(" " + padRight("Retries are automatic and use short backoff after a failed attempt.", innerWidth-1)),
	}

	keyStyle := lipgloss.NewStyle().Background(bg).Foreground(accent).Bold(true)
	footer := keyStyle.Render(" Tab") + dimStyle.Render(":Field") +
		dimStyle.Render("  ") +
		keyStyle.Render("←→") + dimStyle.Render(":Change") +
		dimStyle.Render("  ") +
		keyStyle.Render("Enter") + dimStyle.Render(":Queue") +
		dimStyle.Render("  ") +
		keyStyle.Render("Esc") + dimStyle.Render(":Cancel")
	footerWidth := lipgloss.Width(footer)
	if footerWidth < innerWidth {
		footer += dimStyle.Render(strings.Repeat(" ", innerWidth-footerWidth))
	}

	return overlay.RenderBox("Transfer Options", lines, footer, boxWidth, boxHeight, accent, bg, highlight)
}

func renderTransferOptionLine(label, value string, active bool, innerWidth int, labelStyle, valueStyle, dimStyle, activeStyle, bgStyle lipgloss.Style) string {
	if active {
		return activeStyle.Render(padRight(fmt.Sprintf(" %s: %s", label, value), innerWidth))
	}
	line := labelStyle.Render(fmt.Sprintf(" %s: ", label)) + valueStyle.Render(value)
	if lipgloss.Width(line) < innerWidth {
		line += bgStyle.Render(strings.Repeat(" ", innerWidth-lipgloss.Width(line)))
	}
	return line
}

func conflictLabel(policy transfer.ConflictPolicy) string {
	switch policy {
	case transfer.ConflictOverwrite:
		return "Overwrite existing destination"
	case transfer.ConflictSkip:
		return "Skip conflicting files"
	case transfer.ConflictRename:
		return "Keep both by renaming new copy"
	default:
		return string(policy)
	}
}

func verifyLabel(mode transfer.VerifyMode) string {
	switch mode {
	case transfer.VerifySize:
		return "Size check"
	case transfer.VerifySHA256:
		return "SHA-256 checksum"
	case transfer.VerifyNone:
		return "No post-copy verification"
	default:
		return string(mode)
	}
}

func retryLabel(retries int) string {
	if retries == 0 {
		return "No automatic retry"
	}
	if retries == 1 {
		return "1 retry (2 attempts max)"
	}
	return fmt.Sprintf("%d retries (%d attempts max)", retries, retries+1)
}

func rotateConflict(options []transfer.ConflictPolicy, current transfer.ConflictPolicy, delta int) transfer.ConflictPolicy {
	index := 0
	for i, option := range options {
		if option == current {
			index = i
			break
		}
	}
	index = (index + delta + len(options)) % len(options)
	return options[index]
}

func rotateVerify(options []transfer.VerifyMode, current transfer.VerifyMode, delta int) transfer.VerifyMode {
	index := 0
	for i, option := range options {
		if option == current {
			index = i
			break
		}
	}
	index = (index + delta + len(options)) % len(options)
	return options[index]
}

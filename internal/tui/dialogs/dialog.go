package dialogs

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/kooler/MiddayCommander/internal/ui/overlay"
	"github.com/kooler/MiddayCommander/internal/ui/theme"
)

type Kind int

const (
	KindConfirm Kind = iota
	KindInput
	KindProgress
	KindError
)

type Result struct {
	Kind      Kind
	Confirmed bool
	Text      string
	Tag       string
}

type Model struct {
	kind    Kind
	title   string
	message string
	tag     string

	input    string
	inputPos int

	progress float64
	current  string

	done   bool
	result Result

	width int
}

func NewConfirm(title, message, tag string) Model {
	return Model{
		kind:    KindConfirm,
		title:   title,
		message: message,
		tag:     tag,
		width:   50,
	}
}

func NewInput(title, message, defaultValue, tag string) Model {
	return Model{
		kind:     KindInput,
		title:    title,
		message:  message,
		tag:      tag,
		input:    defaultValue,
		inputPos: len(defaultValue),
		width:    50,
	}
}

func NewError(title, message string) Model {
	return Model{
		kind:    KindError,
		title:   title,
		message: message,
		width:   50,
	}
}

func NewProgress(title, tag string) Model {
	return Model{
		kind:  KindProgress,
		title: title,
		tag:   tag,
		width: 50,
	}
}

func (m Model) Done() bool {
	return m.done
}

func (m Model) GetResult() Result {
	return m.result
}

func (m *Model) SetProgress(progress float64, current string) {
	m.progress = progress
	m.current = current
}

func (m *Model) Update(msg tea.KeyMsg) tea.Cmd {
	switch m.kind {
	case KindConfirm:
		return m.updateConfirm(msg)
	case KindInput:
		return m.updateInput(msg)
	case KindError:
		return m.updateError(msg)
	case KindProgress:
		return nil
	}
	return nil
}

func (m *Model) Close() {
	m.done = true
	m.result = Result{Kind: m.kind, Tag: m.tag}
}

func (m Model) BoxSize(screenWidth, screenHeight int) (int, int) {
	width := m.width
	if width > screenWidth-4 {
		width = screenWidth - 4
	}
	innerWidth := width - 2
	messageLines := 1
	if m.kind != KindInput {
		messageLines = len(wrapText(m.message, innerWidth-2))
	}
	height := 2 + 1 + messageLines + 1 + 1
	if m.kind == KindProgress {
		height++
		if m.current != "" {
			height++
		}
	}
	maxHeight := screenHeight * 3 / 4
	if height > maxHeight {
		height = maxHeight
	}
	return width, height
}

func (m Model) View(th theme.Theme, screenWidth, screenHeight int) string {
	boxWidth, _ := m.BoxSize(screenWidth, screenHeight)
	innerWidth := boxWidth - 2

	bg := lipgloss.Color("#1e1e2e")
	fg := lipgloss.Color("#cdd6f4")
	subtle := lipgloss.Color("#a6adc8")
	accent := lipgloss.Color("#89b4fa")
	highlight := lipgloss.Color("#f9e2af")

	bgStyle := lipgloss.NewStyle().Background(bg).Foreground(fg)
	dimStyle := lipgloss.NewStyle().Background(bg).Foreground(subtle)
	inputStyle := lipgloss.NewStyle().Background(bg).Foreground(highlight)
	progressStyle := lipgloss.NewStyle().Background(bg).Foreground(lipgloss.Color("#a6e3a1"))
	keyStyle := lipgloss.NewStyle().Background(bg).Foreground(accent).Bold(true)

	var contentLines []string
	contentLines = append(contentLines, bgStyle.Render(strings.Repeat(" ", innerWidth)))

	switch m.kind {
	case KindInput:
		label := " " + m.message + " "
		labelWidth := len(label)
		maxInput := innerWidth - labelWidth
		if maxInput < 1 {
			maxInput = 1
		}

		visibleStart := 0
		visibleEnd := len(m.input)
		if visibleEnd-visibleStart > maxInput {
			visibleStart = m.inputPos - maxInput/2
			if visibleStart < 0 {
				visibleStart = 0
			}
			visibleEnd = visibleStart + maxInput
			if visibleEnd > len(m.input) {
				visibleEnd = len(m.input)
				visibleStart = visibleEnd - maxInput
				if visibleStart < 0 {
					visibleStart = 0
				}
			}
		}

		cursorStyle := lipgloss.NewStyle().Background(highlight).Foreground(bg)
		before := m.input[visibleStart:m.inputPos]
		after := ""
		cursorChar := " "
		if m.inputPos < len(m.input) {
			cursorChar = string(m.input[m.inputPos])
			after = m.input[m.inputPos+1 : visibleEnd]
		} else if visibleEnd < len(m.input) {
			after = m.input[m.inputPos:visibleEnd]
		}
		line := dimStyle.Render(label) +
			inputStyle.Render(before) +
			cursorStyle.Render(cursorChar) +
			inputStyle.Render(after)
		lineWidth := lipgloss.Width(line)
		if lineWidth < innerWidth {
			line += bgStyle.Render(strings.Repeat(" ", innerWidth-lineWidth))
		}
		contentLines = append(contentLines, line)
	default:
		for _, messageLine := range wrapText(m.message, innerWidth-2) {
			line := bgStyle.Render(" " + padRight(messageLine, innerWidth-1))
			contentLines = append(contentLines, line)
		}
	}

	contentLines = append(contentLines, bgStyle.Render(strings.Repeat(" ", innerWidth)))

	if m.kind == KindProgress {
		barWidth := innerWidth - 2
		filled := int(m.progress * float64(barWidth))
		if filled > barWidth {
			filled = barWidth
		}
		bar := progressStyle.Render(" "+strings.Repeat("█", filled)) +
			dimStyle.Render(strings.Repeat("░", barWidth-filled)+" ")
		contentLines = append(contentLines, bar)
		if m.current != "" {
			contentLines = append(contentLines, dimStyle.Render(" "+padRight(m.current, innerWidth-1)))
		}
	}

	var footer string
	switch m.kind {
	case KindConfirm:
		footer = keyStyle.Render(" Y") + dimStyle.Render(":Yes") +
			dimStyle.Render("  ") +
			keyStyle.Render("N") + dimStyle.Render(":No") +
			dimStyle.Render("  ") +
			keyStyle.Render("Esc") + dimStyle.Render(":Cancel")
	case KindInput:
		footer = keyStyle.Render(" Enter") + dimStyle.Render(":OK") +
			dimStyle.Render("  ") +
			keyStyle.Render("Esc") + dimStyle.Render(":Cancel")
	case KindProgress:
		footer = dimStyle.Render(" Working...")
	case KindError:
		footer = keyStyle.Render(" Enter") + dimStyle.Render(":Close") +
			dimStyle.Render("  ") +
			keyStyle.Render("Esc") + dimStyle.Render(":Close")
	}
	footerWidth := lipgloss.Width(footer)
	if footerWidth < innerWidth {
		footer += dimStyle.Render(strings.Repeat(" ", innerWidth-footerWidth))
	}

	boxWidth2, boxHeight := m.BoxSize(screenWidth, screenHeight)
	return overlay.RenderBox(m.title, contentLines, footer, boxWidth2, boxHeight, accent, bg, highlight)
}

func (m *Model) updateConfirm(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "y", "Y", "enter":
		m.done = true
		m.result = Result{Kind: KindConfirm, Confirmed: true, Tag: m.tag}
	case "n", "N", "esc":
		m.done = true
		m.result = Result{Kind: KindConfirm, Confirmed: false, Tag: m.tag}
	}
	return nil
}

func (m *Model) updateInput(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "enter":
		m.done = true
		m.result = Result{Kind: KindInput, Confirmed: true, Text: m.input, Tag: m.tag}
	case "esc":
		m.done = true
		m.result = Result{Kind: KindInput, Confirmed: false, Tag: m.tag}
	case "backspace":
		if m.inputPos > 0 {
			m.input = m.input[:m.inputPos-1] + m.input[m.inputPos:]
			m.inputPos--
		}
	case "delete":
		if m.inputPos < len(m.input) {
			m.input = m.input[:m.inputPos] + m.input[m.inputPos+1:]
		}
	case "left":
		if m.inputPos > 0 {
			m.inputPos--
		}
	case "right":
		if m.inputPos < len(m.input) {
			m.inputPos++
		}
	case "home":
		m.inputPos = 0
	case "end":
		m.inputPos = len(m.input)
	default:
		if len(msg.String()) == 1 && msg.String()[0] >= 32 {
			m.input = m.input[:m.inputPos] + msg.String() + m.input[m.inputPos:]
			m.inputPos++
		}
	}
	return nil
}

func (m *Model) updateError(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "enter", "esc", "q":
		m.done = true
		m.result = Result{Kind: KindError}
	}
	return nil
}

func padRight(value string, width int) string {
	if len(value) >= width {
		return value[:width]
	}
	return value + strings.Repeat(" ", width-len(value))
}

func wrapText(text string, width int) []string {
	if len(text) <= width {
		return []string{text}
	}
	var lines []string
	for len(text) > width {
		cut := width
		for cut > 0 && text[cut] != ' ' {
			cut--
		}
		if cut == 0 {
			cut = width
		}
		lines = append(lines, text[:cut])
		text = strings.TrimLeft(text[cut:], " ")
	}
	if text != "" {
		lines = append(lines, text)
	}
	return lines
}

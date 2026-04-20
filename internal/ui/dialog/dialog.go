package dialog

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/kooler/MiddayCommander/internal/ui/completion"

	"github.com/kooler/MiddayCommander/internal/ui/overlay"
	"github.com/kooler/MiddayCommander/internal/ui/theme"
)

// Kind identifies the dialog type.
type Kind int

const (
	KindConfirm Kind = iota
	KindInput
	KindProgress
	KindError
)

// Result is sent when a dialog closes.
type Result struct {
	Kind      Kind
	Confirmed bool   // for confirm dialogs
	Text      string // for input dialogs
	Tag       string // caller-defined tag to identify which operation triggered the dialog
}

// Model represents a modal dialog overlaid on the panels.
type Model struct {
	kind    Kind
	title   string
	message string
	tag     string // passed back in Result

	// Input dialog
	input       string
	inputPos    int
	basePath    string
	suggestions []string

	// Progress dialog
	progress float64
	current  string

	// State
	done   bool
	result Result

	width int
}

// NewConfirm creates a Yes/No confirmation dialog.
func NewConfirm(title, message, tag string) Model {
	return Model{
		kind:    KindConfirm,
		title:   title,
		message: message,
		tag:     tag,
		width:   50,
	}
}

// NewInput creates a text input dialog.
func NewInput(title, message, defaultValue, tag string) Model {
	return NewInputWithBase(title, message, defaultValue, tag, "")
}

func NewInputWithBase(title, message, defaultValue, tag, basePath string) Model {
	return Model{
		kind:     KindInput,
		title:    title,
		message:  message,
		tag:      tag,
		input:    defaultValue,
		inputPos: len(defaultValue),
		basePath: basePath,
		width:    50,
	}
}

// NewError creates an error display dialog.
func NewError(title, message string) Model {
	return Model{
		kind:    KindError,
		title:   title,
		message: message,
		width:   50,
	}
}

// NewProgress creates a progress dialog.
func NewProgress(title, tag string) Model {
	return Model{
		kind:  KindProgress,
		title: title,
		tag:   tag,
		width: 50,
	}
}

// Done returns true when the dialog has been dismissed.
func (m Model) Done() bool {
	return m.done
}

// GetResult returns the dialog result.
func (m Model) GetResult() Result {
	return m.result
}

// SetProgress updates the progress dialog state.
func (m *Model) SetProgress(progress float64, current string) {
	m.progress = progress
	m.current = current
}

// Update handles key events for the dialog.
func (m *Model) Update(msg tea.KeyMsg) tea.Cmd {
	switch m.kind {
	case KindConfirm:
		return m.updateConfirm(msg)
	case KindInput:
		return m.updateInput(msg)
	case KindError:
		return m.updateError(msg)
	case KindProgress:
		// Progress dialogs can't be dismissed by keyboard (they close when the operation ends)
		return nil
	}
	return nil
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
	case "tab":
		if m.tag == "goto" {
			m.completeGoToPath()
		}
	case "backspace":
		if m.inputPos > 0 {
			m.input = m.input[:m.inputPos-1] + m.input[m.inputPos:]
			m.inputPos--
		}
		m.updateSuggestions()
	case "delete":
		if m.inputPos < len(m.input) {
			m.input = m.input[:m.inputPos] + m.input[m.inputPos+1:]
		}
		m.updateSuggestions()
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
			m.updateSuggestions()
		}
	}
	return nil
}

func (m *Model) updateSuggestions() {
	if m.tag != "goto" {
		m.suggestions = nil
		return
	}
	m.suggestions = completion.CompletePathCandidates(m.input, m.basePath, true)
}

func (m *Model) completeGoToPath() {
	candidates := completion.CompletePathCandidates(m.input, m.basePath, true)
	m.suggestions = candidates
	if len(candidates) == 0 {
		return
	}
	common := completion.CommonPrefix(candidates)
	if len(common) > len(m.input) {
		m.input = common
	}
	if len(candidates) == 1 {
		m.input = candidates[0]
	}
	m.inputPos = len(m.input)
	m.updateSuggestions()
}

func (m *Model) updateError(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "enter", "esc", "q":
		m.done = true
		m.result = Result{Kind: KindError}
	}
	return nil
}

// Close dismisses the dialog (used for progress dialogs when the operation ends).
func (m *Model) Close() {
	m.done = true
	m.result = Result{Kind: m.kind, Tag: m.tag}
}

// BoxSize returns desired box dimensions.
func (m Model) BoxSize(screenWidth, screenHeight int) (int, int) {
	w := m.width
	if w > screenWidth-4 {
		w = screenWidth - 4
	}
	innerW := w - 2
	// Height: borders(2) + blank(1) + content + blank(1) + footer(1)
	var msgLines int
	if m.kind == KindInput {
		msgLines = 1 // label + input on one line
		if len(m.suggestions) > 0 && m.tag == "goto" {
			// For GoTo, format suggestions compactly (multiple per line)
			formatted := completion.FormatSuggestions(m.suggestions, innerW-2, 6, true)
			msgLines += len(formatted)
		}
	} else {
		msgLines = len(wrapText(m.message, innerW-2))
	}
	h := 2 + 1 + msgLines + 1 + 1 // borders + blank + content + blank + footer
	switch m.kind {
	case KindProgress:
		h++ // progress bar
		if m.current != "" {
			h++ // current file
		}
	}
	maxH := screenHeight * 3 / 4
	if h > maxH {
		h = maxH
	}
	return w, h
}

// View renders the dialog as a floating box using the shared overlay style.
func (m Model) View(th theme.Theme, screenWidth, screenHeight int) string {
	boxW, _ := m.BoxSize(screenWidth, screenHeight)
	innerW := boxW - 2

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

	// Empty line
	contentLines = append(contentLines, bgStyle.Render(strings.Repeat(" ", innerW)))

	// Kind-specific content
	switch m.kind {
	case KindInput:
		// Message label and input with cursor at inputPos
		label := " " + m.message + " "
		labelW := len(label)
		maxInput := innerW - labelW
		if maxInput < 1 {
			maxInput = 1
		}

		// Determine visible window of text around the cursor.
		visStart := 0
		visEnd := len(m.input)
		if visEnd-visStart > maxInput {
			// Keep cursor visible with some context on both sides.
			visStart = m.inputPos - maxInput/2
			if visStart < 0 {
				visStart = 0
			}
			visEnd = visStart + maxInput
			if visEnd > len(m.input) {
				visEnd = len(m.input)
				visStart = visEnd - maxInput
				if visStart < 0 {
					visStart = 0
				}
			}
		}

		cursorStyle := lipgloss.NewStyle().Background(highlight).Foreground(bg)
		before := m.input[visStart:m.inputPos]
		after := ""
		cursorCh := " "
		if m.inputPos < len(m.input) {
			cursorCh = string(m.input[m.inputPos])
			after = m.input[m.inputPos+1 : visEnd]
		} else if visEnd < len(m.input) {
			after = m.input[m.inputPos:visEnd]
		}
		line := dimStyle.Render(label) +
			inputStyle.Render(before) +
			cursorStyle.Render(cursorCh) +
			inputStyle.Render(after)
		lineW := lipgloss.Width(line)
		if lineW < innerW {
			line += bgStyle.Render(strings.Repeat(" ", innerW-lineW))
		}
		contentLines = append(contentLines, line)

		if m.kind == KindInput && len(m.suggestions) > 0 && m.tag == "goto" {
			// Format suggestions compactly (multiple per line) like Ctrl+R
			formatted := completion.FormatSuggestions(m.suggestions, innerW-2, 6, true)
			for _, suggLine := range formatted {
				sugLine := completion.PadOrTrim(suggLine, innerW-1)
				contentLines = append(contentLines, bgStyle.Render(" "+sugLine))
			}
		}

	default:
		// Message on its own line(s) for non-input dialogs
		for _, msgLine := range wrapText(m.message, innerW-2) {
			line := bgStyle.Render(" " + padRight(msgLine, innerW-1))
			contentLines = append(contentLines, line)
		}
	}

	// Empty line
	contentLines = append(contentLines, bgStyle.Render(strings.Repeat(" ", innerW)))

	// Kind-specific extra content
	switch m.kind {
	case KindInput:
		// already rendered above

	case KindProgress:
		barWidth := innerW - 2
		filled := int(m.progress * float64(barWidth))
		if filled > barWidth {
			filled = barWidth
		}
		bar := progressStyle.Render(" "+strings.Repeat("█", filled)) +
			dimStyle.Render(strings.Repeat("░", barWidth-filled)+" ")
		contentLines = append(contentLines, bar)
		if m.current != "" {
			cur := dimStyle.Render(" " + padRight(m.current, innerW-1))
			contentLines = append(contentLines, cur)
		}
	}

	// Footer with key hints
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
	if footerWidth < innerW {
		footer += dimStyle.Render(strings.Repeat(" ", innerW-footerWidth))
	}

	boxW2, boxH := m.BoxSize(screenWidth, screenHeight)
	return overlay.RenderBox(m.title, contentLines, footer, boxW2, boxH,
		accent, bg, highlight)
}

func padRight(s string, width int) string {
	if len(s) >= width {
		return s[:width]
	}
	return s + strings.Repeat(" ", width-len(s))
}

func wrapText(text string, width int) []string {
	if len(text) <= width {
		return []string{text}
	}
	var lines []string
	for len(text) > width {
		// Find last space before width
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

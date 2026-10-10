package tablist

import (
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/kooler/MiddayCommander/internal/ui/overlay"
	uitext "github.com/kooler/MiddayCommander/internal/ui/text"
)

// Row is one open tab, with what the list shows and acts on.
type Row struct {
	Number int
	Left   string
	Right  string
	Active bool
}

// JumpMsg is sent when the user picks a tab to switch to.
type JumpMsg struct {
	Index int
}

// NewMsg is sent when the user asks for a new tab based on the one under
// the cursor.
type NewMsg struct {
	From int
}

// CloseMsg is sent when the user asks to close the tab under the cursor.
type CloseMsg struct {
	Index int
}

// DismissMsg is sent when the user closes the list without acting.
type DismissMsg struct{}

// Model is the tab list overlay.
type Model struct {
	rows      []Row
	visible   []int // indexes into rows that pass the filter
	cursor    int   // position within visible
	offset    int
	width     int
	height    int
	filter    string
	filtering bool
	tabCount  int
	maxTabs   int
}

// New starts the cursor on the active tab; tabCount and maxTabs drive the
// footer hints.
func New(rows []Row, activeIndex int, width, height, tabCount, maxTabs int) Model {
	visible := make([]int, 0, len(rows))
	for i := range rows {
		visible = append(visible, i)
	}
	cursor := 0
	if activeIndex >= 0 && activeIndex < len(rows) {
		cursor = activeIndex
	}
	m := Model{rows: rows, visible: visible, cursor: cursor, width: width, height: height, tabCount: tabCount, maxTabs: maxTabs}
	m.clampOffset()
	return m
}

// SetSize updates the screen size the box is laid out against, so a resized
// terminal does not leave the cursor outside the visible window.
func (m *Model) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.clampOffset()
}

func (m Model) Update(msg tea.KeyMsg) (Model, tea.Cmd) {
	if m.filtering {
		return m.updateFiltering(msg)
	}

	switch msg.String() {
	case "esc":
		return m, func() tea.Msg { return DismissMsg{} }
	case "enter":
		if idx := m.currentIndex(); idx >= 0 {
			return m, func() tea.Msg { return JumpMsg{Index: idx} }
		}
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
			m.clampOffset()
		}
	case "down", "j":
		if m.cursor < len(m.visible)-1 {
			m.cursor++
			m.clampOffset()
		}
	case "n":
		if m.tabCount >= m.maxTabs {
			break
		}
		if idx := m.currentIndex(); idx >= 0 {
			return m, func() tea.Msg { return NewMsg{From: idx} }
		}
	case "d", "delete":
		if m.tabCount <= 1 {
			break
		}
		if idx := m.currentIndex(); idx >= 0 {
			return m, func() tea.Msg { return CloseMsg{Index: idx} }
		}
	case "f":
		m.filtering = true
		m.filter = ""
		m.clampOffset() // the filter line shrinks the window
	case "0", "1", "2", "3", "4", "5", "6", "7", "8", "9":
		n := int(msg.String()[0] - '0') // 0 is the tenth
		if n == 0 {
			n = 10
		}
		if n <= len(m.rows) {
			return m, func() tea.Msg { return JumpMsg{Index: n - 1} }
		}
	}
	return m, nil
}

// updateFiltering builds the filter query. Only the arrows move the cursor;
// every other printable key is appended to the query.
func (m Model) updateFiltering(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.filtering = false
		m.filter = ""
		m.refilter()
	case "enter":
		if idx := m.currentIndex(); idx >= 0 {
			return m, func() tea.Msg { return JumpMsg{Index: idx} }
		}
		m.filtering = false
		m.filter = ""
		m.refilter()
	case "backspace":
		if start := uitext.PreviousGraphemeBoundary(m.filter, len(m.filter)); start >= 0 {
			m.filter = m.filter[:start]
			m.refilter()
		} else {
			m.filtering = false
		}
	case "up":
		if m.cursor > 0 {
			m.cursor--
			m.clampOffset()
		}
	case "down":
		if m.cursor < len(m.visible)-1 {
			m.cursor++
			m.clampOffset()
		}
	default:
		if s, ok := uitext.PrintableInput(msg); ok {
			m.filter += s
			m.refilter()
		}
	}
	return m, nil
}

// currentIndex maps the cursor back to the original tab index, or -1.
func (m Model) currentIndex() int {
	if m.cursor < 0 || m.cursor >= len(m.visible) {
		return -1
	}
	return m.visible[m.cursor]
}

func (m *Model) refilter() {
	query := strings.ToLower(m.filter)
	m.visible = nil
	for i := range m.rows {
		if query == "" {
			m.visible = append(m.visible, i)
			continue
		}
		if strings.Contains(strconv.Itoa(m.rows[i].Number), query) {
			m.visible = append(m.visible, i)
			continue
		}
		if fuzzyMatch(m.rows[i].Left, query) || fuzzyMatch(m.rows[i].Right, query) {
			m.visible = append(m.visible, i)
		}
	}
	m.cursor = 0
	m.offset = 0
}

// fuzzyMatch reports whether query matches target as an ordered subsequence.
func fuzzyMatch(target, query string) bool {
	target = strings.ToLower(target)
	rest := target
	for _, r := range query {
		i := strings.IndexRune(rest, r)
		if i < 0 {
			return false
		}
		rest = rest[i+1:]
	}
	return true
}

func (m Model) BoxSize(screenWidth, screenHeight int) (int, int) {
	w := screenWidth * 3 / 4
	if w < 48 {
		w = min(48, screenWidth)
	}
	h := len(m.visible) + 4 // borders(2) + header(1) + footer(1)
	h = max(h, 8)
	maxH := screenHeight * 3 / 4
	h = min(h, maxH)
	return w, h
}

// resultHeight is the row count of the results area, the window that both
// clampOffset and View must agree on. The filter line takes one of the rows.
func (m Model) resultHeight() int {
	_, boxH := m.BoxSize(m.width, m.height)
	h := boxH - 4 // borders(2) + header(1) + footer(1)
	if m.filtering {
		h-- // filter input line
	}
	return max(h, 1)
}

func (m *Model) clampOffset() {
	m.offset = overlay.ClampScroll(m.cursor, m.offset, m.resultHeight(), len(m.visible))
}

func (m Model) columnWidths() (numW, leftW, rightW int) {
	boxW, _ := m.BoxSize(m.width, m.height)
	innerW := boxW - 2
	numW = 4
	pathW := (innerW - numW) / 2
	leftW = pathW
	rightW = innerW - numW - pathW
	return
}

func (m Model) View(screenWidth, screenHeight int) string {
	boxW, boxH := m.BoxSize(screenWidth, screenHeight)
	innerW := boxW - 2

	bg := lipgloss.Color("#1e1e2e")
	fg := lipgloss.Color("#cdd6f4")
	subtle := lipgloss.Color("#a6adc8")
	accent := lipgloss.Color("#89b4fa")
	highlight := lipgloss.Color("#f9e2af")
	cursorBg := lipgloss.Color("#45475a")

	bgStyle := lipgloss.NewStyle().Background(bg).Foreground(fg)
	cursorStyle := lipgloss.NewStyle().Background(cursorBg).Foreground(fg)
	dimStyle := lipgloss.NewStyle().Background(bg).Foreground(subtle)
	numStyle := lipgloss.NewStyle().Background(bg).Foreground(highlight)
	activeStyle := lipgloss.NewStyle().Background(bg).Foreground(accent).Bold(true)

	numW, leftW, rightW := m.columnWidths()

	var contentLines []string

	header := numStyle.Render(overlay.PadOrTrunc("#", numW)) +
		dimStyle.Render(overlay.PadOrTrunc("left path", leftW)) +
		dimStyle.Render(overlay.PadOrTrunc("right path", rightW))
	contentLines = append(contentLines, header)

	if m.filtering {
		promptStyle := lipgloss.NewStyle().Background(bg).Foreground(accent).Bold(true)
		filterLine := promptStyle.Render(" / ") + bgStyle.Render(m.filter+"_")
		fw := lipgloss.Width(filterLine)
		if fw < innerW {
			filterLine += bgStyle.Render(strings.Repeat(" ", innerW-fw))
		}
		contentLines = append(contentLines, filterLine)
	}

	rh := m.resultHeight()
	end := min(m.offset+rh, len(m.visible))
	for i := m.offset; i < end; i++ {
		row := m.rows[m.visible[i]]
		isCursor := i == m.cursor

		num := fmt.Sprintf("%2d", row.Number)
		if row.Active {
			num = "*" + num
		}

		line := overlay.PadOrTrunc(num, numW) +
			overlay.PadOrTruncDots(row.Left, leftW) +
			overlay.PadOrTruncDots(row.Right, rightW)

		if isCursor {
			contentLines = append(contentLines, cursorStyle.Render(overlay.PadOrTrunc(line, innerW)))
		} else {
			numCell := activeStyle.Render(overlay.PadOrTrunc(num, numW))
			if !row.Active {
				numCell = numStyle.Render(overlay.PadOrTrunc(num, numW))
			}
			left := bgStyle.Render(overlay.PadOrTruncDots(row.Left, leftW))
			right := bgStyle.Render(overlay.PadOrTruncDots(row.Right, rightW))
			contentLines = append(contentLines, numCell+left+right)
		}
	}

	if len(m.visible) == 0 {
		empty := dimStyle.Render(" No tabs match.")
		ew := lipgloss.Width(empty)
		if ew < innerW {
			empty += dimStyle.Render(strings.Repeat(" ", innerW-ew))
		}
		contentLines = append(contentLines, empty)
	}

	keyStyle := lipgloss.NewStyle().Background(bg).Foreground(accent).Bold(true)
	sepStyle := dimStyle
	var footer string
	if m.filtering {
		footer = keyStyle.Render(" ↑/↓") + sepStyle.Render(":Move") +
			sepStyle.Render("  ") +
			keyStyle.Render("Backspace") + sepStyle.Render(":Del") +
			sepStyle.Render("  ") +
			keyStyle.Render("Enter") + sepStyle.Render(":Go") +
			sepStyle.Render("  ") +
			keyStyle.Render("Esc") + sepStyle.Render(":Clear")
	} else {
		footer = keyStyle.Render(" j/k") + sepStyle.Render(":Move")
		if m.tabCount < m.maxTabs {
			footer += sepStyle.Render("  ") +
				keyStyle.Render("n") + sepStyle.Render(":New")
		}
		if m.tabCount > 1 {
			footer += sepStyle.Render("  ") +
				keyStyle.Render("d") + sepStyle.Render(":Close")
		}
		footer += sepStyle.Render("  ") +
			keyStyle.Render("f") + sepStyle.Render(":Filter") +
			sepStyle.Render("  ") +
			keyStyle.Render("0-9") + sepStyle.Render(":Jump") +
			sepStyle.Render("  ") +
			keyStyle.Render("Enter") + sepStyle.Render(":Go") +
			sepStyle.Render("  ") +
			keyStyle.Render("Esc") + sepStyle.Render(":Close")
	}
	fw := lipgloss.Width(footer)
	if fw > innerW {
		footer = ansi.Truncate(footer, innerW, "")
	} else if fw < innerW {
		footer += dimStyle.Render(strings.Repeat(" ", innerW-fw))
	}

	return overlay.RenderBox("Tabs", contentLines, footer, boxW, boxH, accent, bg, highlight)
}

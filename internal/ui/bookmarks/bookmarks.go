package bookmarks

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/kooler/MiddayCommander/internal/bookmark"
	"github.com/kooler/MiddayCommander/internal/ui/overlay"
	"github.com/kooler/MiddayCommander/internal/ui/theme"
)

// SelectMsg is sent when the user selects a bookmark.
type SelectMsg struct {
	Path string
}

// DismissMsg is sent when the user closes the bookmark list.
type DismissMsg struct{}

// Model is the bookmark list overlay.
type Model struct {
	store   *bookmark.Store
	items   []bookmark.Bookmark
	cursor  int
	offset  int
	width   int
	height  int
	filter    string // search/filter query
	filtering bool   // true when filter input is active
	adding    bool   // true when prompting for bookmark name
	addPath string // path being bookmarked
	addName string // name being typed
}

// New creates a new bookmark list overlay.
func New(store *bookmark.Store, currentPath string, width, height int) Model {
	items := store.Sorted()
	return Model{
		store:   store,
		items:   items,
		width:   width,
		height:  height,
		addPath: currentPath,
	}
}

// Update handles key events.
func (m Model) Update(msg tea.KeyMsg) (Model, tea.Cmd) {
	if m.adding {
		return m.updateAdding(msg)
	}

	// Filter mode: typing builds the filter query
	if m.filtering {
		switch msg.String() {
		case "esc":
			m.filtering = false
			m.filter = ""
			m.refilter()
		case "enter":
			// Accept filter, select current item
			if m.cursor >= 0 && m.cursor < len(m.items) {
				path := m.items[m.cursor].Path
				m.store.Touch(path)
				_ = m.store.Save()
				return m, func() tea.Msg { return SelectMsg{Path: path} }
			}
			m.filtering = false
		case "backspace":
			if len(m.filter) > 0 {
				m.filter = m.filter[:len(m.filter)-1]
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
			if m.cursor < len(m.items)-1 {
				m.cursor++
				m.clampOffset()
			}
		default:
			s := msg.String()
			if len(s) == 1 && s[0] >= 32 {
				m.filter += s
				m.refilter()
			}
		}
		return m, nil
	}

	// Normal mode
	switch msg.String() {
	case "esc", "ctrl+b":
		return m, func() tea.Msg { return DismissMsg{} }
	case "enter":
		if m.cursor >= 0 && m.cursor < len(m.items) {
			path := m.items[m.cursor].Path
			m.store.Touch(path)
			_ = m.store.Save()
			return m, func() tea.Msg { return SelectMsg{Path: path} }
		}
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
			m.clampOffset()
		}
	case "down", "j":
		if m.cursor < len(m.items)-1 {
			m.cursor++
			m.clampOffset()
		}
	case "d", "delete":
		if m.cursor >= 0 && m.cursor < len(m.items) {
			m.store.Remove(m.items[m.cursor].Path)
			_ = m.store.Save()
			m.refilter()
		}
	case "a":
		m.adding = true
		m.addName = ""
	case "f":
		m.filtering = true
		m.filter = ""
	case "0", "1", "2", "3", "4", "5", "6", "7", "8", "9":
		idx := int(msg.String()[0] - '0')
		if idx < len(m.items) {
			path := m.items[idx].Path
			m.store.Touch(path)
			_ = m.store.Save()
			return m, func() tea.Msg { return SelectMsg{Path: path} }
		}
	}
	return m, nil
}

func (m Model) updateAdding(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.adding = false
		return m, nil
	case "enter":
		m.store.Add(m.addPath, m.addName)
		_ = m.store.Save()
		m.items = m.store.Sorted()
		m.adding = false
		return m, nil
	case "backspace":
		if len(m.addName) > 0 {
			m.addName = m.addName[:len(m.addName)-1]
		}
		return m, nil
	default:
		s := msg.String()
		if len(s) == 1 && s[0] >= 32 {
			m.addName += s
		}
		return m, nil
	}
}

func (m *Model) refilter() {
	all := m.store.Sorted()
	if m.filter == "" {
		m.items = all
	} else {
		query := strings.ToLower(m.filter)
		m.items = nil
		for _, b := range all {
			target := strings.ToLower(b.Path)
			if b.Name != "" {
				target += " " + strings.ToLower(b.Name)
			}
			if strings.Contains(target, query) {
				m.items = append(m.items, b)
			}
		}
	}
	m.cursor = 0
	m.offset = 0
}

// BoxSize returns desired box dimensions.
func (m Model) BoxSize(screenWidth, screenHeight int) (int, int) {
	w := screenWidth * 2 / 3
	if w < 40 {
		w = min(40, screenWidth)
	}
	// Height based on number of bookmarks, capped
	h := len(m.items) + 4 // borders(2) + input/header(1) + footer(1)
	if h < 8 {
		h = 8
	}
	maxH := screenHeight * 3 / 4
	if h > maxH {
		h = maxH
	}
	return w, h
}

func (m Model) resultHeight() int {
	_, boxH := m.BoxSize(m.width, m.height)
	h := boxH - 4 // borders(2) + header(1) + footer(1)
	if h < 1 {
		h = 1
	}
	return h
}

func (m *Model) clampOffset() {
	rh := m.resultHeight()
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+rh {
		m.offset = m.cursor - rh + 1
	}
}

// View renders the bookmark list as a floating box.
func (m Model) View(th theme.Theme, screenWidth, screenHeight int) string {
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

	var contentLines []string

	promptStyle := lipgloss.NewStyle().Background(bg).Foreground(accent).Bold(true)

	// Filter or add input line
	hasExtraLine := false
	if m.filtering {
		filterLine := promptStyle.Render(" Filter: ") + bgStyle.Render(m.filter+"_")
		filterWidth := lipgloss.Width(filterLine)
		if filterWidth < innerW {
			filterLine += bgStyle.Render(strings.Repeat(" ", innerW-filterWidth))
		}
		contentLines = append(contentLines, filterLine)
		hasExtraLine = true
	} else if m.adding {
		addLine := promptStyle.Render(" Name: ") + bgStyle.Render(m.addName+"_")
		addWidth := lipgloss.Width(addLine)
		if addWidth < innerW {
			addLine += bgStyle.Render(strings.Repeat(" ", innerW-addWidth))
		}
		contentLines = append(contentLines, addLine)
		hasExtraLine = true
	}

	// Bookmark list
	rh := m.resultHeight()
	if hasExtraLine {
		rh--
	}
	end := m.offset + rh
	if end > len(m.items) {
		end = len(m.items)
	}

	for i := m.offset; i < end; i++ {
		b := m.items[i]
		isCursor := i == m.cursor

		prefix := "  "
		if i < 10 {
			prefix = fmt.Sprintf("%d ", i)
		}

		display := b.Path
		if b.Name != "" {
			display = b.Name + " → " + b.Path
		}
		if len(display) > innerW-4 {
			display = "…" + display[len(display)-innerW+5:]
		}

		line := prefix + display
		if isCursor {
			contentLines = append(contentLines, cursorStyle.Render(padStr(line, innerW)))
		} else {
			contentLines = append(contentLines, numStyle.Render(prefix)+bgStyle.Render(padStr(display, innerW-len(prefix))))
		}
	}

	if len(m.items) == 0 && !m.adding {
		empty := dimStyle.Render(" No bookmarks. Press 'a' to add.")
		emptyWidth := lipgloss.Width(empty)
		if emptyWidth < innerW {
			empty += dimStyle.Render(strings.Repeat(" ", innerW-emptyWidth))
		}
		contentLines = append(contentLines, empty)
	}

	// Footer with key hints
	keyStyle := lipgloss.NewStyle().Background(bg).Foreground(accent).Bold(true)
	sepStyle := dimStyle

	footer := keyStyle.Render(" a") + sepStyle.Render(":Add") +
		sepStyle.Render("  ") +
		keyStyle.Render("d") + sepStyle.Render(":Del") +
		sepStyle.Render("  ") +
		keyStyle.Render("f") + sepStyle.Render(":Filter") +
		sepStyle.Render("  ") +
		keyStyle.Render("0-9") + sepStyle.Render(":Jump") +
		sepStyle.Render("  ") +
		keyStyle.Render("Enter") + sepStyle.Render(":Go") +
		sepStyle.Render("  ") +
		keyStyle.Render("Esc") + sepStyle.Render(":Close")
	footerWidth := lipgloss.Width(footer)
	if footerWidth < innerW {
		footer += dimStyle.Render(strings.Repeat(" ", innerW-footerWidth))
	}

	return overlay.RenderBox("Bookmarks", contentLines, footer, boxW, boxH,
		accent, bg, highlight)
}

func padStr(s string, width int) string {
	if len(s) >= width {
		return s[:width]
	}
	return s + strings.Repeat(" ", width-len(s))
}

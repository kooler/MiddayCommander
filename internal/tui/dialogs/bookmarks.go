package dialogs

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	bookmarkstore "github.com/kooler/MiddayCommander/internal/bookmarks"
	midfs "github.com/kooler/MiddayCommander/internal/fs"
	"github.com/kooler/MiddayCommander/internal/ui/overlay"
	"github.com/kooler/MiddayCommander/internal/ui/theme"
)

type BookmarkSelectMsg struct {
	URI midfs.URI
}

type BookmarkDismissMsg struct{}

type BookmarksModel struct {
	store     *bookmarkstore.Store
	items     []bookmarkstore.Bookmark
	cursor    int
	offset    int
	width     int
	height    int
	filter    string
	filtering bool
	adding    bool
	addURI    midfs.URI
	addName   string
}

func NewBookmarks(store *bookmarkstore.Store, currentURI midfs.URI, width, height int) BookmarksModel {
	return BookmarksModel{
		store:  store,
		items:  store.Sorted(),
		width:  width,
		height: height,
		addURI: currentURI,
	}
}

func (m BookmarksModel) Update(msg tea.KeyMsg) (BookmarksModel, tea.Cmd) {
	if m.adding {
		return m.updateAdding(msg)
	}

	if m.filtering {
		switch msg.String() {
		case "esc":
			m.filtering = false
			m.filter = ""
			m.refilter()
		case "enter":
			if uri, ok := m.currentBookmarkURI(); ok {
				m.store.Touch(uri)
				_ = m.store.Save()
				return m, func() tea.Msg { return BookmarkSelectMsg{URI: uri} }
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

	switch msg.String() {
	case "esc", "ctrl+b":
		return m, func() tea.Msg { return BookmarkDismissMsg{} }
	case "enter":
		if uri, ok := m.currentBookmarkURI(); ok {
			m.store.Touch(uri)
			_ = m.store.Save()
			return m, func() tea.Msg { return BookmarkSelectMsg{URI: uri} }
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
		if uri, ok := m.currentBookmarkURI(); ok {
			m.store.Remove(uri)
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
		index := int(msg.String()[0] - '0')
		if index < len(m.items) {
			if uri, err := m.items[index].ParsedURI(); err == nil {
				m.store.Touch(uri)
				_ = m.store.Save()
				return m, func() tea.Msg { return BookmarkSelectMsg{URI: uri} }
			}
		}
	}

	return m, nil
}

func (m BookmarksModel) updateAdding(msg tea.KeyMsg) (BookmarksModel, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.adding = false
		return m, nil
	case "enter":
		m.store.Add(m.addURI, m.addName)
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

func (m *BookmarksModel) refilter() {
	all := m.store.Sorted()
	if m.filter == "" {
		m.items = all
	} else {
		query := strings.ToLower(m.filter)
		m.items = nil
		for _, bookmark := range all {
			target := strings.ToLower(bookmark.DisplayPath())
			if bookmark.Name != "" {
				target += " " + strings.ToLower(bookmark.Name)
			}
			if strings.Contains(target, query) {
				m.items = append(m.items, bookmark)
			}
		}
	}
	m.cursor = 0
	m.offset = 0
}

func (m BookmarksModel) BoxSize(screenWidth, screenHeight int) (int, int) {
	width := screenWidth * 2 / 3
	if width < 40 {
		width = min(40, screenWidth)
	}
	height := len(m.items) + 4
	if height < 8 {
		height = 8
	}
	maxHeight := screenHeight * 3 / 4
	if height > maxHeight {
		height = maxHeight
	}
	return width, height
}

func (m BookmarksModel) resultHeight() int {
	_, boxHeight := m.BoxSize(m.width, m.height)
	resultHeight := boxHeight - 4
	if resultHeight < 1 {
		resultHeight = 1
	}
	return resultHeight
}

func (m *BookmarksModel) clampOffset() {
	resultHeight := m.resultHeight()
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+resultHeight {
		m.offset = m.cursor - resultHeight + 1
	}
}

func (m BookmarksModel) View(th theme.Theme, screenWidth, screenHeight int) string {
	boxWidth, boxHeight := m.BoxSize(screenWidth, screenHeight)
	innerWidth := boxWidth - 2

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

	hasExtraLine := false
	if m.filtering {
		filterLine := promptStyle.Render(" Filter: ") + bgStyle.Render(m.filter+"_")
		filterWidth := lipgloss.Width(filterLine)
		if filterWidth < innerWidth {
			filterLine += bgStyle.Render(strings.Repeat(" ", innerWidth-filterWidth))
		}
		contentLines = append(contentLines, filterLine)
		hasExtraLine = true
	} else if m.adding {
		addLine := promptStyle.Render(" Name: ") + bgStyle.Render(m.addName+"_")
		addWidth := lipgloss.Width(addLine)
		if addWidth < innerWidth {
			addLine += bgStyle.Render(strings.Repeat(" ", innerWidth-addWidth))
		}
		contentLines = append(contentLines, addLine)
		hasExtraLine = true
	}

	resultHeight := m.resultHeight()
	if hasExtraLine {
		resultHeight--
	}
	end := m.offset + resultHeight
	if end > len(m.items) {
		end = len(m.items)
	}

	for index := m.offset; index < end; index++ {
		bookmark := m.items[index]
		isCursor := index == m.cursor

		prefix := "  "
		if index < 10 {
			prefix = fmt.Sprintf("%d ", index)
		}

		display := bookmark.DisplayPath()
		if bookmark.Name != "" {
			display = bookmark.Name + " -> " + display
		}
		if len(display) > innerWidth-4 {
			display = "..." + display[len(display)-innerWidth+7:]
		}

		line := prefix + display
		if isCursor {
			contentLines = append(contentLines, cursorStyle.Render(padStr(line, innerWidth)))
		} else {
			contentLines = append(contentLines, numStyle.Render(prefix)+bgStyle.Render(padStr(display, innerWidth-len(prefix))))
		}
	}

	if len(m.items) == 0 && !m.adding {
		empty := dimStyle.Render(" No bookmarks. Press 'a' to add.")
		emptyWidth := lipgloss.Width(empty)
		if emptyWidth < innerWidth {
			empty += dimStyle.Render(strings.Repeat(" ", innerWidth-emptyWidth))
		}
		contentLines = append(contentLines, empty)
	}

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
	if footerWidth < innerWidth {
		footer += dimStyle.Render(strings.Repeat(" ", innerWidth-footerWidth))
	}

	return overlay.RenderBox("Bookmarks", contentLines, footer, boxWidth, boxHeight, accent, bg, highlight)
}

func (m BookmarksModel) currentBookmarkURI() (midfs.URI, bool) {
	if m.cursor < 0 || m.cursor >= len(m.items) {
		return midfs.URI{}, false
	}
	uri, err := m.items[m.cursor].ParsedURI()
	if err != nil {
		return midfs.URI{}, false
	}
	return uri, true
}

func padStr(value string, width int) string {
	if len(value) >= width {
		return value[:width]
	}
	return value + strings.Repeat(" ", width-len(value))
}

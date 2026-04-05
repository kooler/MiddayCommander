package dialogs

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	midfs "github.com/kooler/MiddayCommander/internal/fs"
	sftpfs "github.com/kooler/MiddayCommander/internal/fs/sftp"
	profilesstore "github.com/kooler/MiddayCommander/internal/profiles"
	"github.com/kooler/MiddayCommander/internal/ui/overlay"
	"github.com/kooler/MiddayCommander/internal/ui/theme"
)

type ProfileSelectMsg struct {
	Profile profilesstore.Profile
	URI     midfs.URI
}

type ProfilesDismissMsg struct{}

type ProfilesModel struct {
	store     *profilesstore.Store
	items     []profilesstore.Profile
	cursor    int
	offset    int
	width     int
	height    int
	filter    string
	filtering bool
}

func NewProfiles(store *profilesstore.Store, width, height int) ProfilesModel {
	items := []profilesstore.Profile(nil)
	if store != nil {
		items = store.All()
	}

	return ProfilesModel{
		store:  store,
		items:  items,
		width:  width,
		height: height,
	}
}

func (m ProfilesModel) Update(msg tea.KeyMsg) (ProfilesModel, tea.Cmd) {
	if m.filtering {
		switch msg.String() {
		case "esc":
			m.filtering = false
			m.filter = ""
			m.refilter()
		case "enter":
			if profile, uri, ok := m.currentProfileURI(); ok {
				return m, func() tea.Msg { return ProfileSelectMsg{Profile: profile, URI: uri} }
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
	case "esc":
		return m, func() tea.Msg { return ProfilesDismissMsg{} }
	case "enter":
		if profile, uri, ok := m.currentProfileURI(); ok {
			return m, func() tea.Msg { return ProfileSelectMsg{Profile: profile, URI: uri} }
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
	case "f":
		m.filtering = true
		m.filter = ""
	case "n":
		return m, func() tea.Msg { return ConnectOpenMsg{} }
	case "0", "1", "2", "3", "4", "5", "6", "7", "8", "9":
		index := int(msg.String()[0] - '0')
		if index < len(m.items) {
			profile := m.items[index]
			opts, err := sftpfs.FromProfile(profile)
			if err == nil {
				return m, func() tea.Msg { return ProfileSelectMsg{Profile: profile, URI: opts.URI()} }
			}
		}
	}

	return m, nil
}

func (m *ProfilesModel) refilter() {
	all := []profilesstore.Profile(nil)
	if m.store != nil {
		all = m.store.All()
	}
	if m.filter == "" {
		m.items = all
	} else {
		query := strings.ToLower(m.filter)
		m.items = nil
		for _, profile := range all {
			target := strings.ToLower(profile.Name + " " + profile.User + " " + profile.Host + " " + profile.Path)
			if strings.Contains(target, query) {
				m.items = append(m.items, profile)
			}
		}
	}
	m.cursor = 0
	m.offset = 0
}

func (m ProfilesModel) BoxSize(screenWidth, screenHeight int) (int, int) {
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

func (m ProfilesModel) resultHeight() int {
	_, boxHeight := m.BoxSize(m.width, m.height)
	resultHeight := boxHeight - 4
	if resultHeight < 1 {
		resultHeight = 1
	}
	return resultHeight
}

func (m *ProfilesModel) clampOffset() {
	resultHeight := m.resultHeight()
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+resultHeight {
		m.offset = m.cursor - resultHeight + 1
	}
}

func (m ProfilesModel) View(th theme.Theme, screenWidth, screenHeight int) string {
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
	promptStyle := lipgloss.NewStyle().Background(bg).Foreground(accent).Bold(true)

	var contentLines []string
	hasExtraLine := false
	if m.filtering {
		filterLine := promptStyle.Render(" Filter: ") + bgStyle.Render(m.filter+"_")
		filterWidth := lipgloss.Width(filterLine)
		if filterWidth < innerWidth {
			filterLine += bgStyle.Render(strings.Repeat(" ", innerWidth-filterWidth))
		}
		contentLines = append(contentLines, filterLine)
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
		profile := m.items[index]
		line := profileLine(profile, innerWidth)
		prefix := "  "
		if index < 10 {
			prefix = fmt.Sprintf("%d ", index)
		}

		if index == m.cursor {
			contentLines = append(contentLines, cursorStyle.Render(padStr(prefix+line, innerWidth)))
		} else {
			contentLines = append(contentLines, numStyle.Render(prefix)+bgStyle.Render(padStr(line, innerWidth-len(prefix))))
		}
	}

	if len(m.items) == 0 {
		empty := dimStyle.Render(" No remote profiles. Add entries to profiles.toml.")
		emptyWidth := lipgloss.Width(empty)
		if emptyWidth < innerWidth {
			empty += dimStyle.Render(strings.Repeat(" ", innerWidth-emptyWidth))
		}
		contentLines = append(contentLines, empty)
	}

	keyStyle := lipgloss.NewStyle().Background(bg).Foreground(accent).Bold(true)
	footer := keyStyle.Render("f") + dimStyle.Render(":Filter") +
		dimStyle.Render("  ") +
		keyStyle.Render("n") + dimStyle.Render(":Manual") +
		dimStyle.Render("  ") +
		keyStyle.Render("0-9") + dimStyle.Render(":Jump") +
		dimStyle.Render("  ") +
		keyStyle.Render("Enter") + dimStyle.Render(":Connect") +
		dimStyle.Render("  ") +
		keyStyle.Render("Esc") + dimStyle.Render(":Close")
	footerWidth := lipgloss.Width(footer)
	if footerWidth < innerWidth {
		footer += dimStyle.Render(strings.Repeat(" ", innerWidth-footerWidth))
	}

	return overlay.RenderBox("Remote Profiles", contentLines, footer, boxWidth, boxHeight, accent, bg, highlight)
}

func (m ProfilesModel) currentProfileURI() (profilesstore.Profile, midfs.URI, bool) {
	if m.cursor < 0 || m.cursor >= len(m.items) {
		return profilesstore.Profile{}, midfs.URI{}, false
	}

	profile := m.items[m.cursor]
	opts, err := sftpfs.FromProfile(profile)
	if err != nil {
		return profilesstore.Profile{}, midfs.URI{}, false
	}
	return profile, opts.URI(), true
}

func profileLine(profile profilesstore.Profile, width int) string {
	opts, err := sftpfs.FromProfile(profile)
	display := profile.Name
	if err == nil {
		display = profile.Name + " -> " + opts.URI().Display()
	}
	if width > 4 && lipgloss.Width(display) > width {
		display = truncateWithEllipsis(display, width)
	}
	return display
}

func truncateWithEllipsis(value string, width int) string {
	if width <= 3 {
		return strings.Repeat(".", max(0, width))
	}
	runes := []rune(value)
	if len(runes) <= width {
		return value
	}
	return string(runes[:width-3]) + "..."
}

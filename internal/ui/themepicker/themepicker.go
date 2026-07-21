package themepicker

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/kooler/MiddayCommander/internal/ui/overlay"
	"github.com/kooler/MiddayCommander/internal/ui/theme"
)

// spinnerFrames animates the "fetching remote themes" indicator.
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// PreviewMsg is sent when the user navigates to a theme (for live preview).
type PreviewMsg struct{ Theme theme.Theme }

// SelectMsg is sent when the user confirms a theme selection.
type SelectMsg struct {
	Key     string // config key (filename stem)
	Source  string // theme.SourceDefault, SourceLocal, or SourceRemote
	RawTOML []byte // non-nil for remote themes (written to disk on select)
	Theme   theme.Theme
}

// DismissMsg is sent when the user closes the theme picker without selecting.
type DismissMsg struct{}

// RemoteThemesMsg delivers themes fetched from GitHub.
type RemoteThemesMsg struct {
	Themes []theme.AvailableTheme
}

// SpinnerTickMsg advances the "fetching remote themes" spinner.
type SpinnerTickMsg struct{}

// SpinnerTick returns a command that fires the next spinner frame.
func SpinnerTick() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(time.Time) tea.Msg {
		return SpinnerTickMsg{}
	})
}

// Model is the theme picker overlay.
type Model struct {
	entries   []theme.AvailableTheme
	activeKey string // config key of the currently applied theme
	cursor    int
	offset    int
	width     int
	height    int
	loading   bool // remote themes are still being fetched
	spinner   int  // current spinner frame
}

// New creates a new theme picker overlay. activeKey is the config key of the
// currently applied theme; the cursor opens on it so it's visible right away.
func New(available []theme.AvailableTheme, activeKey string, width, height int) Model {
	m := Model{
		entries:   available,
		activeKey: activeKey,
		width:     width,
		height:    height,
		loading:   true,
	}
	for i, e := range available {
		if e.Key == activeKey {
			m.cursor = i
			break
		}
	}
	m.clampOffset()
	return m
}

// FetchRemote returns a command that fetches remote themes from GitHub.
// localKeys are theme keys already present locally (to avoid duplicates).
func FetchRemote(localKeys map[string]bool) tea.Cmd {
	return func() tea.Msg {
		remote := theme.FetchRemoteThemes(localKeys)
		return RemoteThemesMsg{Themes: remote}
	}
}

// HandleRemote merges remote themes into the picker's entries and stops the
// loading indicator.
func (m *Model) HandleRemote(msg RemoteThemesMsg) {
	m.entries = append(m.entries, msg.Themes...)
	m.loading = false
}

// Loading reports whether remote themes are still being fetched.
func (m Model) Loading() bool { return m.loading }

// AdvanceSpinner moves to the next spinner frame.
func (m *Model) AdvanceSpinner() {
	m.spinner = (m.spinner + 1) % len(spinnerFrames)
}

// Update handles key events.
func (m Model) Update(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		return m, func() tea.Msg { return DismissMsg{} }
	case "enter":
		if m.cursor >= 0 && m.cursor < len(m.entries) {
			e := m.entries[m.cursor]
			return m, func() tea.Msg {
				return SelectMsg{Key: e.Key, Source: e.Source, RawTOML: e.RawTOML, Theme: e.Theme}
			}
		}
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
			m.clampOffset()
		}
		th := m.entries[m.cursor].Theme
		return m, func() tea.Msg { return PreviewMsg{Theme: th} }
	case "down", "j":
		if m.cursor < len(m.entries)-1 {
			m.cursor++
			m.clampOffset()
		}
		th := m.entries[m.cursor].Theme
		return m, func() tea.Msg { return PreviewMsg{Theme: th} }
	}
	return m, nil
}

// BoxSize returns desired box dimensions.
func (m Model) BoxSize(screenWidth, screenHeight int) (int, int) {
	w := screenWidth * 2 / 3
	if w < 30 {
		w = min(30, screenWidth)
	}
	if w > 50 {
		w = 50
	}
	h := len(m.entries) + 4 // borders(2) + header(1) + footer(1)
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

// View renders the theme picker as a floating box.
func (m Model) View(th theme.Theme, screenWidth, screenHeight int) string {
	boxW, boxH := m.BoxSize(screenWidth, screenHeight)
	innerW := boxW - 2

	bg := lipgloss.Color("#1e1e2e")
	fg := lipgloss.Color("#cdd6f4")
	subtle := lipgloss.Color("#a6adc8")
	accent := lipgloss.Color("#89b4fa")
	highlight := lipgloss.Color("#f9e2af")
	cursorBg := lipgloss.Color("#45475a")
	remoteFg := lipgloss.Color("#a6e3a1") // green for remote

	bgStyle := lipgloss.NewStyle().Background(bg).Foreground(fg)
	cursorStyle := lipgloss.NewStyle().Background(cursorBg).Foreground(fg)
	dimStyle := lipgloss.NewStyle().Background(bg).Foreground(subtle)
	tagLocalStyle := lipgloss.NewStyle().Background(bg).Foreground(subtle)
	tagRemoteStyle := lipgloss.NewStyle().Background(bg).Foreground(remoteFg)
	tagLocalCursorStyle := lipgloss.NewStyle().Background(cursorBg).Foreground(subtle)
	tagRemoteCursorStyle := lipgloss.NewStyle().Background(cursorBg).Foreground(remoteFg)

	var contentLines []string

	// Theme list
	rh := m.resultHeight()
	end := m.offset + rh
	if end > len(m.entries) {
		end = len(m.entries)
	}

	for i := m.offset; i < end; i++ {
		entry := m.entries[i]
		isCursor := i == m.cursor
		isActive := entry.Key == m.activeKey

		// Gutter: a space, then a dot marking the active theme, then a space
		// before the name — so the dot has an equal gap on each side. Rendered
		// with the row background so it matches the rest of the line.
		rowBg, gap := bg, bgStyle
		if isCursor {
			rowBg, gap = cursorBg, cursorStyle
		}
		markerCh := " "
		if isActive {
			markerCh = "●"
		}
		dot := lipgloss.NewStyle().Background(rowBg).Foreground(highlight).Render(markerCh)
		gutter := gap.Render(" ") + dot + gap.Render(" ")

		// Build prefix tag
		var tag string
		var tagStyle, tagCursorStyle lipgloss.Style
		switch entry.Source {
		case theme.SourceLocal:
			tag = "[local] "
			tagStyle = tagLocalStyle
			tagCursorStyle = tagLocalCursorStyle
		case theme.SourceRemote:
			tag = "[remote] "
			tagStyle = tagRemoteStyle
			tagCursorStyle = tagRemoteCursorStyle
		default:
			tag = ""
		}

		display := entry.Name
		maxNameW := innerW - 4 - len(tag) // 3-cell gutter + 1-col right margin
		if len(display) > maxNameW {
			display = display[:maxNameW-3] + "..."
		}

		if isCursor {
			var line string
			if tag != "" {
				line = gutter + tagCursorStyle.Render(tag) + cursorStyle.Render(display)
			} else {
				line = gutter + cursorStyle.Render(display)
			}
			lineW := lipgloss.Width(line)
			if lineW < innerW {
				line += cursorStyle.Render(strings.Repeat(" ", innerW-lineW))
			}
			contentLines = append(contentLines, line)
		} else {
			var line string
			if tag != "" {
				line = gutter + tagStyle.Render(tag) + bgStyle.Render(display)
			} else {
				line = gutter + bgStyle.Render(display)
			}
			lineW := lipgloss.Width(line)
			if lineW < innerW {
				line += bgStyle.Render(strings.Repeat(" ", innerW-lineW))
			}
			contentLines = append(contentLines, line)
		}
	}

	if len(m.entries) == 0 {
		empty := dimStyle.Render(" No themes found.")
		emptyWidth := lipgloss.Width(empty)
		if emptyWidth < innerW {
			empty += dimStyle.Render(strings.Repeat(" ", innerW-emptyWidth))
		}
		contentLines = append(contentLines, empty)
	}

	// Footer with key hints
	keyStyle := lipgloss.NewStyle().Background(bg).Foreground(accent).Bold(true)
	sepStyle := dimStyle

	footer := keyStyle.Render(" Enter") + sepStyle.Render(":Apply") +
		sepStyle.Render("  ") +
		keyStyle.Render("Esc") + sepStyle.Render(":Cancel") +
		sepStyle.Render("  ") +
		keyStyle.Render("↑↓") + sepStyle.Render(":Preview")
	footerWidth := lipgloss.Width(footer)
	if footerWidth < innerW {
		footer += dimStyle.Render(strings.Repeat(" ", innerW-footerWidth))
	}

	title := "Theme"
	if m.loading {
		title = "Theme  " + spinnerFrames[m.spinner] + " fetching remote…"
	}

	return overlay.RenderBox(title, contentLines, footer, boxW, boxH,
		accent, bg, highlight)
}

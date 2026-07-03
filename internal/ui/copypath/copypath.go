package copypath

import (
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/kooler/MiddayCommander/internal/platform"
	"github.com/kooler/MiddayCommander/internal/ui/overlay"
	"github.com/kooler/MiddayCommander/internal/ui/theme"
)

// DismissMsg is sent when the overlay closes.
type DismissMsg struct{}

// Model is the copy-path overlay: path variants the user can copy to the clipboard.
type Model struct {
	paths  []string
	cursor int
	width  int
	height int
}

// New builds the overlay for the given absolute file path.
func New(fullPath string, width, height int) Model {
	return Model{
		paths:  buildPaths(fullPath),
		width:  width,
		height: height,
	}
}

// buildPaths lists the file name, then each added parent directory (relative),
// and finally the full absolute path.
// e.g. /users/bob/data/1.txt -> [1.txt, data/1.txt, bob/data/1.txt, /users/bob/data/1.txt]
func buildPaths(fullPath string) []string {
	clean := filepath.Clean(fullPath)
	dir, file := filepath.Split(clean)
	parents := splitParents(dir)

	out := []string{file}
	rel := file
	for i := len(parents) - 1; i >= 0; i-- {
		rel = parents[i] + string(filepath.Separator) + rel
		// Skip the root-less full form; the absolute path below covers it.
		if i == 0 {
			break
		}
		out = append(out, rel)
	}
	out = append(out, clean)
	return dedup(out)
}

// splitParents returns a path's directory components, without separators.
func splitParents(dir string) []string {
	sep := string(filepath.Separator)
	dir = strings.Trim(dir, sep)
	if dir == "" {
		return nil
	}
	return strings.Split(dir, sep)
}

func dedup(in []string) []string {
	seen := make(map[string]bool, len(in))
	var out []string
	for _, s := range in {
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

// Update handles key events.
func (m Model) Update(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		return m, dismiss
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.paths)-1 {
			m.cursor++
		}
	case "enter":
		if m.cursor >= 0 && m.cursor < len(m.paths) {
			platform.CopyToClipboard(m.paths[m.cursor])
			return m, dismiss
		}
	}
	return m, nil
}

func dismiss() tea.Msg { return DismissMsg{} }

// BoxSize returns the desired box dimensions.
func (m Model) BoxSize(screenWidth, screenHeight int) (int, int) {
	maxLen := len(helpText)
	for _, p := range m.paths {
		if l := len(p) + 2; l > maxLen { // +2 for the cursor/selection prefix
			maxLen = l
		}
	}
	w := maxLen + 4 // borders + side padding
	if w > screenWidth {
		w = screenWidth
	}
	if w < 40 {
		w = min(40, screenWidth)
	}

	// borders(2) + help(1) + blank(1) + paths + footer(1)
	h := len(m.paths) + 5
	maxH := screenHeight * 3 / 4
	if h > maxH {
		h = maxH
	}
	return w, h
}

const helpText = "Select a path and press Enter to copy it to the clipboard"

// View renders the overlay as a floating box.
func (m Model) View(_ theme.Theme, screenWidth, screenHeight int) string {
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

	var contentLines []string
	contentLines = append(contentLines, dimStyle.Render(padStr(" "+helpText, innerW)))
	contentLines = append(contentLines, bgStyle.Render(strings.Repeat(" ", innerW)))

	for i, p := range m.paths {
		prefix := "  "
		if i == m.cursor {
			prefix = "> "
		}
		display := p
		if len(display) > innerW-len(prefix) {
			display = "…" + display[len(display)-(innerW-len(prefix))+1:]
		}
		line := padStr(prefix+display, innerW)
		if i == m.cursor {
			contentLines = append(contentLines, cursorStyle.Render(line))
		} else {
			contentLines = append(contentLines, bgStyle.Render(line))
		}
	}

	keyStyle := lipgloss.NewStyle().Background(bg).Foreground(accent).Bold(true)
	sepStyle := dimStyle
	footer := keyStyle.Render(" ↑/↓") + sepStyle.Render(":Navigate") +
		sepStyle.Render("  ") +
		keyStyle.Render("Enter") + sepStyle.Render(":Copy") +
		sepStyle.Render("  ") +
		keyStyle.Render("Esc") + sepStyle.Render(":Close")
	if fw := lipgloss.Width(footer); fw < innerW {
		footer += dimStyle.Render(strings.Repeat(" ", innerW-fw))
	}

	return overlay.RenderBox("Copy Path", contentLines, footer, boxW, boxH,
		accent, bg, highlight)
}

func padStr(s string, width int) string {
	if lipgloss.Width(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-lipgloss.Width(s))
}

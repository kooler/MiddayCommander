package help

import (
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/kooler/MiddayCommander/internal/config"
	"github.com/kooler/MiddayCommander/internal/ui/overlay"
	"github.com/kooler/MiddayCommander/internal/ui/theme"
)

// DismissMsg is sent when the user closes help.
type DismissMsg struct{}

// Model is the help overlay.
type Model struct {
	keys   config.KeyBindings
	offset int
	width  int
	height int
}

// New creates a new help overlay.
func New(keys config.KeyBindings, width, height int) Model {
	return Model{keys: keys, width: width, height: height}
}

// BoxSize returns desired box dimensions.
func (m Model) BoxSize(screenWidth, screenHeight int) (int, int) {
	w := 60
	if w > screenWidth-4 {
		w = screenWidth - 4
	}
	h := screenHeight * 3 / 4
	if h < 15 {
		h = min(15, screenHeight)
	}
	return w, h
}

// Update handles key events.
func (m Model) Update(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q", "f1", "enter":
		return m, func() tea.Msg { return DismissMsg{} }
	case "up", "k":
		if m.offset > 0 {
			m.offset--
		}
	case "down", "j":
		m.offset++
	}
	return m, nil
}

type entry struct {
	label string
	keys  string
}

func (m Model) buildEntries() []entry {
	k := m.keys
	return []entry{
		{"", ""},
		{"── Navigation ──", ""},
		{"Move up", fmtKeys(k.Up)},
		{"Move down", fmtKeys(k.Down)},
		{"Page up", fmtKeys(k.PageUp)},
		{"Page down", fmtKeys(k.PageDown)},
		{"Go to top", fmtKeys(k.Home)},
		{"Go to bottom", fmtKeys(k.End)},
		{"Go back", fmtKeys(k.GoBack)},
		{"Go to path", fmtKeys(k.GoTo)},
		{"Switch panel", fmtKeys(k.TogglePanel)},
		{"Swap panels", fmtKeys(k.SwapPanels)},
		{"", ""},
		{"── File Operations ──", ""},
		{"View file", fmtKeys(k.View)},
		{"Edit file", fmtKeys(k.Edit)},
		{"Copy", fmtKeys(k.Copy)},
		{"Move", fmtKeys(k.Move)},
		{"Delete", fmtKeys(k.Delete)},
		{"Make directory", fmtKeys(k.Mkdir)},
		{"Rename", fmtKeys(k.Rename)},
		{"", ""},
		{"── Selection ──", ""},
		{"Toggle select", fmtKeys(k.ToggleSelect)},
		{"Select up", fmtKeys(k.SelectUp)},
		{"Select down", fmtKeys(k.SelectDown)},
		{"", ""},
		{"── Tools ──", ""},
		{"Fuzzy find", fmtKeys(k.FuzzyFind)},
		{"Bookmarks", fmtKeys(k.Bookmarks)},
		{"Quick search", fmtKeys(k.QuickSearch)},
		{"Theme picker", fmtKeys(k.ThemePicker)},
		{"Run command", fmtKeys(k.CmdExec)},
		{"Help", fmtKeys(k.Help)},
		{"Quit", fmtKeys(k.Quit)},
	}
}

func fmtKeys(keys config.StringOrList) string {
	var parts []string
	for _, k := range keys {
		parts = append(parts, formatKey(k))
	}
	return strings.Join(parts, ", ")
}

func formatKey(k string) string {
	if len(k) > 1 && k[0] == 'f' && k[1] >= '0' && k[1] <= '9' {
		if n, err := strconv.Atoi(k[1:]); err == nil && n >= 13 && n <= 20 {
			return fmt.Sprintf("Shift-F%d", n-12)
		}
		return "F" + k[1:]
	}
	if strings.HasPrefix(k, "ctrl+") {
		return "Ctrl-" + strings.ToUpper(k[5:])
	}
	if strings.HasPrefix(k, "shift+") {
		return "Shift-" + strings.ToUpper(k[6:])
	}
	if strings.HasPrefix(k, "alt+") {
		return "Alt-" + strings.ToUpper(k[4:])
	}
	return k
}

// View renders the help overlay.
func (m Model) View(th theme.Theme, screenWidth, screenHeight int) string {
	boxW, boxH := m.BoxSize(screenWidth, screenHeight)
	innerW := boxW - 2

	bg := lipgloss.Color("#1e1e2e")
	fg := lipgloss.Color("#cdd6f4")
	subtle := lipgloss.Color("#a6adc8")
	accent := lipgloss.Color("#89b4fa")
	heading := lipgloss.Color("#f9e2af")

	bgStyle := lipgloss.NewStyle().Background(bg).Foreground(fg)
	headStyle := lipgloss.NewStyle().Background(bg).Foreground(heading).Bold(true)
	keyStyle := lipgloss.NewStyle().Background(bg).Foreground(accent)
	dimStyle := lipgloss.NewStyle().Background(bg).Foreground(subtle)

	entries := m.buildEntries()

	var contentLines []string

	// App info
	titleLine := bgStyle.Render(" Midday Commander (mdc)")
	titleWidth := lipgloss.Width(titleLine)
	if titleWidth < innerW {
		titleLine += bgStyle.Render(strings.Repeat(" ", innerW-titleWidth))
	}
	contentLines = append(contentLines, titleLine)

	verLine := dimStyle.Render(" A modern dual-panel file manager")
	verWidth := lipgloss.Width(verLine)
	if verWidth < innerW {
		verLine += dimStyle.Render(strings.Repeat(" ", innerW-verWidth))
	}
	contentLines = append(contentLines, verLine)

	// Keybindings
	maxVisible := boxH - 4 // borders(2) + footer(1) + 1 padding
	end := m.offset + maxVisible - len(contentLines)
	if end > len(entries) {
		end = len(entries)
	}
	if m.offset > len(entries) {
		m.offset = len(entries) - 1
	}

	for i := m.offset; i < end; i++ {
		e := entries[i]
		if e.label == "" && e.keys == "" {
			contentLines = append(contentLines, bgStyle.Render(strings.Repeat(" ", innerW)))
			continue
		}
		if e.keys == "" {
			// Section heading
			line := headStyle.Render(" " + e.label)
			lineW := lipgloss.Width(line)
			if lineW < innerW {
				line += bgStyle.Render(strings.Repeat(" ", innerW-lineW))
			}
			contentLines = append(contentLines, line)
			continue
		}

		// Key binding row: label right-padded, keys right-aligned
		keysStr := keyStyle.Render(e.keys)
		keysWidth := lipgloss.Width(keysStr)
		labelWidth := innerW - keysWidth - 2
		if labelWidth < 1 {
			labelWidth = 1
		}
		label := fmt.Sprintf(" %-*s", labelWidth, e.label)
		if len(label) > labelWidth+1 {
			label = label[:labelWidth+1]
		}
		line := dimStyle.Render(label) + keysStr + bgStyle.Render(" ")
		lineW := lipgloss.Width(line)
		if lineW < innerW {
			line += bgStyle.Render(strings.Repeat(" ", innerW-lineW))
		}
		contentLines = append(contentLines, line)
	}

	// Footer
	footerKeyStyle := lipgloss.NewStyle().Background(bg).Foreground(accent).Bold(true)
	footer := footerKeyStyle.Render(" Esc") + dimStyle.Render(":Close") +
		dimStyle.Render("  ") +
		footerKeyStyle.Render("↑↓") + dimStyle.Render(":Scroll")
	footerWidth := lipgloss.Width(footer)
	if footerWidth < innerW {
		footer += dimStyle.Render(strings.Repeat(" ", innerW-footerWidth))
	}

	return overlay.RenderBox("Help", contentLines, footer, boxW, boxH,
		accent, bg, accent)
}

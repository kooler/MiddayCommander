package menubar

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/kooler/MiddayCommander/internal/config"
	"github.com/kooler/MiddayCommander/internal/ui/theme"
)

// Item represents a single menu bar button.
type Item struct {
	Key      string // display label, e.g. "F5"
	Label    string // action label, e.g. "Copy"
	RawKey   string // actual key string for matching clicks, e.g. "f5"
}

// DefaultItems returns the default menu bar items.
func DefaultItems(cfg config.Config) []Item {
	return []Item{
		itemFromCfg(cfg.Keys.Help, "Help"),
		itemFromCfg(cfg.Keys.Bookmarks, "Bookm"),
		itemFromCfg(cfg.Keys.View, "View"),
		itemFromCfg(cfg.Keys.Edit, "Edit"),
		itemFromCfg(cfg.Keys.Copy, "Copy"),
		itemFromCfg(cfg.Keys.Move, "Move"),
		itemFromCfg(cfg.Keys.Mkdir, "Mkdir"),
		itemFromCfg(cfg.Keys.Delete, "Delete"),
		itemFromCfg(cfg.Keys.FuzzyFind, "FZF"),
		itemFromCfg(cfg.Keys.Quit, "Quit"),
	}
}

func itemFromCfg(keys config.StringOrList, label string) Item {
	if len(keys) == 0 {
		return Item{Label: label}
	}
	raw := keys[0]
	return Item{
		Key:    formatKeyDisplay(raw),
		Label:  label,
		RawKey: raw,
	}
}

// formatKeyDisplay turns "f5" into "F5", "ctrl+g" into "C-g", etc.
func formatKeyDisplay(k string) string {
	if len(k) > 1 && k[0] == 'f' && k[1] >= '0' && k[1] <= '9' {
		return "F" + k[1:]
	}
	if strings.HasPrefix(k, "ctrl+") {
		return "C-" + k[5:]
	}
	if strings.HasPrefix(k, "alt+") {
		return "A-" + k[4:]
	}
	if strings.HasPrefix(k, "shift+") {
		return "S-" + k[6:]
	}
	return k
}

// HandleClick checks if a mouse click at column x hits a menu item.
// Returns the raw key string, or "" if no hit.
func HandleClick(x, width int, items []Item) string {
	n := len(items)
	if n == 0 {
		return ""
	}
	colWidth := width / n
	idx := x / colWidth
	if idx >= n {
		idx = n - 1
	}
	return items[idx].RawKey
}

// View renders the F-key hints bar at the bottom, evenly spaced across width.
func View(th theme.Theme, width int, items []Item) string {
	n := len(items)
	if n == 0 {
		return th.MenuBar.Render(strings.Repeat(" ", width))
	}

	colWidth := width / n

	var b strings.Builder
	for i, itm := range items {
		w := colWidth
		if i == n-1 {
			w = width - colWidth*(n-1)
		}

		keyStr := th.FKeyHint.Render(itm.Key)
		keyWidth := lipgloss.Width(keyStr)
		labelWidth := w - keyWidth
		if labelWidth < 0 {
			labelWidth = 0
		}
		labelStr := th.StatusBar.Render(padOrTrunc(itm.Label, labelWidth))

		b.WriteString(keyStr)
		b.WriteString(labelStr)
	}

	return b.String()
}

func padOrTrunc(s string, width int) string {
	if len(s) > width {
		return s[:width]
	}
	return s + strings.Repeat(" ", width-len(s))
}

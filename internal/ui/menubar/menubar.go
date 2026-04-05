package menubar

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/kooler/MiddayCommander/internal/config"
	"github.com/kooler/MiddayCommander/internal/ui/theme"
)

// Item represents a single menu bar button.
type Item struct {
	Key    string // display label, e.g. "F5"
	Label  string // action label, e.g. "Copy"
	RawKey string // actual key string for matching clicks, e.g. "f5"
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

// ShiftItems returns menu bar items for the Shift+F1..F10 state.
// Only positions with a shift+F binding get a label; the rest are blank.
func ShiftItems(cfg config.Config) []Item {
	items := make([]Item, 10)
	// Always show F1-F10 key hints; label is empty unless a shift binding exists.
	for i := range items {
		items[i] = Item{Key: fmt.Sprintf("F%d", i+1)}
	}

	type entry struct {
		keys  config.StringOrList
		label string
	}
	bindings := []entry{
		{cfg.Keys.Help, "Help"},
		{cfg.Keys.Bookmarks, "Bookm"},
		{cfg.Keys.RemoteConnect, "Remote"},
		{cfg.Keys.View, "View"},
		{cfg.Keys.Edit, "Edit"},
		{cfg.Keys.Copy, "Copy"},
		{cfg.Keys.Move, "Move"},
		{cfg.Keys.Mkdir, "Mkdir"},
		{cfg.Keys.Delete, "Delete"},
		{cfg.Keys.FuzzyFind, "FZF"},
		{cfg.Keys.Quit, "Quit"},
		{cfg.Keys.Rename, "Rename"},
		{cfg.Keys.GoTo, "GoTo"},
		{cfg.Keys.TogglePanel, "Panel"},
		{cfg.Keys.SwapPanels, "Swap"},
		{cfg.Keys.ThemePicker, "Theme"},
	}

	for _, b := range bindings {
		for _, k := range b.keys {
			if pos := shiftFKeyPos(k); pos >= 0 {
				items[pos].Label = b.label
				items[pos].RawKey = k
			}
		}
	}
	return items
}

// shiftFKeyPos returns the 0-based menu position for a shift F-key string,
// or -1 if the key is not a shift F-key. "f13" -> 0, "f18" -> 5, etc.
func shiftFKeyPos(k string) int {
	if len(k) < 3 || k[0] != 'f' {
		return -1
	}
	n, err := strconv.Atoi(k[1:])
	if err != nil || n < 13 || n > 20 {
		return -1
	}
	return n - 13
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
// F13-F20 (BubbleTea's representation of Shift+F1..F8) display as "S-F1".."S-F8".
func formatKeyDisplay(k string) string {
	if len(k) > 1 && k[0] == 'f' && k[1] >= '0' && k[1] <= '9' {
		if n, err := strconv.Atoi(k[1:]); err == nil && n >= 13 && n <= 20 {
			return fmt.Sprintf("S-F%d", n-12)
		}
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

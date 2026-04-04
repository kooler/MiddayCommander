package panel

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/kooler/MiddayCommander/internal/ui/theme"
)

// iconPadding is the number of spaces between the icon and the filename.
const iconPadding = 1

// View renders the panel as a string.
func (m Model) View(th theme.Theme) string {
	if m.width <= 0 || m.height <= 0 {
		return ""
	}

	// Border style based on active state
	borderStyle := th.PanelBorder
	headerStyle := th.PanelHeader
	if m.active {
		borderStyle = th.PanelBorderActive
		headerStyle = th.PanelHeaderActive
	}

	innerWidth := m.width - 2 // account for left+right border chars

	// Header: current path (show archive name when inside one)
	header := m.path
	if m.inArchive {
		archName := filepath.Base(m.archiveFS.ArchivePath())
		if m.path == "." {
			header = archName + "://"
		} else {
			header = archName + "://" + m.path
		}
	}
	if len(header) > innerWidth-4 {
		header = "..." + header[len(header)-innerWidth+7:]
	}
	headerLine := borderStyle.Render("┌") +
		headerStyle.Render(" "+truncOrPad(header, innerWidth-2)+" ") +
		borderStyle.Render("┐")

	// File list rows
	var rows []string
	end := m.offset + m.height
	if end > len(m.entries) {
		end = len(m.entries)
	}

	for i := m.offset; i < end; i++ {
		row := m.renderRow(i, innerWidth, th)
		rows = append(rows, borderStyle.Render("│")+row+borderStyle.Render("│"))
	}

	// Fill remaining rows with empty space
	emptyRow := th.FileNormal.Render(strings.Repeat(" ", innerWidth))
	for len(rows) < m.height {
		rows = append(rows, borderStyle.Render("│")+emptyRow+borderStyle.Render("│"))
	}

	// Footer
	var footerText string
	if m.searching {
		footerText = fmt.Sprintf(" Search: %s_ ", m.searchQuery)
	} else {
		count := len(m.entries)
		if m.entries != nil && !isRootPath(m.path) {
			count-- // exclude ".."
		}
		footerText = fmt.Sprintf(" %d files ", count)
	}
	footerLine := borderStyle.Render("└") +
		headerStyle.Render(truncOrPad(footerText, innerWidth)) +
		borderStyle.Render("┘")

	// Assemble
	parts := []string{headerLine}
	parts = append(parts, rows...)
	parts = append(parts, footerLine)

	return strings.Join(parts, "\n")
}

func (m Model) renderRow(idx, width int, th theme.Theme) string {
	entry := m.entries[idx]
	info := m.infos[idx]

	name := entry.Name()
	isDir := entry.IsDir()
	isCursor := idx == m.cursor
	isSelected := m.selected[idx]

	// Resolve icon and its display width
	icon := m.iconResolver.ResolveIcon(name, isDir)
	iconWidth := 0
	if icon != "" {
		iconWidth = m.iconResolver.IconWidth(name, isDir)
	}

	// Determine columns: name, size, time
	sizeStr := ""
	timeStr := ""
	if info != nil {
		if isDir {
			sizeStr = "<DIR>"
		} else {
			sizeStr = FormatSize(info.Size())
		}
		timeStr = FormatTime(info.ModTime())
	} else if isDir {
		sizeStr = "<DIR>"
	}

	// Column widths: time=12, size=7, rest=name (minus icon width + padding)
	timeWidth := 12
	sizeWidth := 7
	iconSpace := iconWidth + iconPadding
	nameWidth := width - iconSpace - sizeWidth - timeWidth - 2 // 2 spaces between columns
	if nameWidth < 4 {
		nameWidth = 4
	}

	// Build the line: [icon][name] [size] [time]
	namePart := truncOrPad(name, nameWidth)
	if icon != "" {
		namePart = icon + strings.Repeat(" ", iconPadding) + namePart
	}
	sizePart := padLeft(sizeStr, sizeWidth)
	timePart := truncOrPad(timeStr, timeWidth)

	line := namePart + " " + sizePart + " " + timePart

	// Style based on state
	var style lipgloss.Style
	switch {
	case isCursor && isDir:
		style = th.FileCursorDir
	case isCursor:
		style = th.FileCursor
	case isSelected:
		style = th.FileSelected
	case isDir:
		style = th.FileDir
	case info != nil && isExecutable(info.Mode()):
		style = th.FileExec
	case entry.Type()&fs.ModeSymlink != 0:
		style = th.FileSymlink
	default:
		style = th.FileNormal
	}

	return style.Render(line)
}

func truncOrPad(s string, width int) string {
	if len(s) > width {
		if width > 3 {
			return s[:width-3] + "..."
		}
		return s[:width]
	}
	return s + strings.Repeat(" ", width-len(s))
}

func padLeft(s string, width int) string {
	if len(s) >= width {
		return s[:width]
	}
	return strings.Repeat(" ", width-len(s)) + s
}

func isExecutable(mode fs.FileMode) bool {
	return mode&0111 != 0
}

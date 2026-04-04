package panel

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	midfs "github.com/kooler/MiddayCommander/internal/fs"
	"github.com/kooler/MiddayCommander/internal/ui/theme"
)

func (m Model) View(th theme.Theme) string {
	if m.width <= 0 || m.height <= 0 {
		return ""
	}

	borderStyle := th.PanelBorder
	headerStyle := th.PanelHeader
	if m.active {
		borderStyle = th.PanelBorderActive
		headerStyle = th.PanelHeaderActive
	}

	innerWidth := m.width - 2
	header := m.dir.Display()
	if len(header) > innerWidth-4 {
		header = "..." + header[len(header)-innerWidth+7:]
	}

	headerLine := borderStyle.Render("┌") +
		headerStyle.Render(" "+truncOrPad(header, innerWidth-2)+" ") +
		borderStyle.Render("┐")

	var rows []string
	end := m.offset + m.height
	if end > len(m.entries) {
		end = len(m.entries)
	}
	for index := m.offset; index < end; index++ {
		row := m.renderRow(index, innerWidth, th)
		rows = append(rows, borderStyle.Render("│")+row+borderStyle.Render("│"))
	}

	emptyRow := th.FileNormal.Render(strings.Repeat(" ", innerWidth))
	for len(rows) < m.height {
		rows = append(rows, borderStyle.Render("│")+emptyRow+borderStyle.Render("│"))
	}

	var footerText string
	if m.searching {
		footerText = fmt.Sprintf(" Search: %s_ ", m.searchQuery)
	} else {
		count := len(m.entries)
		if parent := m.router.Parent(m.dir); parent.String() != m.dir.String() {
			count--
		}
		footerText = fmt.Sprintf(" %d files ", count)
	}
	footerLine := borderStyle.Render("└") +
		headerStyle.Render(truncOrPad(footerText, innerWidth)) +
		borderStyle.Render("┘")

	parts := []string{headerLine}
	parts = append(parts, rows...)
	parts = append(parts, footerLine)
	return strings.Join(parts, "\n")
}

func (m Model) renderRow(index, width int, th theme.Theme) string {
	entry := m.entries[index]
	name := entry.Name
	isDir := entry.IsDir()
	isCursor := index == m.cursor
	isSelected := m.selected[index]

	sizeStr := ""
	timeStr := ""
	if isDir {
		sizeStr = "<DIR>"
	} else {
		sizeStr = FormatSize(entry.Size)
	}
	if !entry.ModTime.IsZero() {
		timeStr = FormatTime(entry.ModTime)
	}

	timeWidth := 12
	sizeWidth := 7
	nameWidth := width - sizeWidth - timeWidth - 2
	if nameWidth < 4 {
		nameWidth = 4
	}

	namePart := truncOrPad(name, nameWidth)
	sizePart := padLeft(sizeStr, sizeWidth)
	timePart := truncOrPad(timeStr, timeWidth)
	line := namePart + " " + sizePart + " " + timePart

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
	case entry.IsSymlink():
		style = th.FileSymlink
	case entry.Mode&0o111 != 0:
		style = th.FileExec
	case entry.Type == midfs.EntryArchive:
		style = th.FileExec
	default:
		style = th.FileNormal
	}

	return style.Render(line)
}

func truncOrPad(value string, width int) string {
	if len(value) > width {
		if width > 3 {
			return value[:width-3] + "..."
		}
		return value[:width]
	}
	return value + strings.Repeat(" ", width-len(value))
}

func padLeft(value string, width int) string {
	if len(value) >= width {
		return value[:width]
	}
	return strings.Repeat(" ", width-len(value)) + value
}

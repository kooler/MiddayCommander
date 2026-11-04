package panel

import (
	"fmt"
	"io/fs"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/kooler/MiddayCommander/internal/ui/overlay"
	"github.com/kooler/MiddayCommander/internal/ui/theme"
)

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
	if innerWidth < 0 {
		innerWidth = 0
	}

	header := m.Location().Display()
	if ansi.StringWidth(header) > innerWidth-4 {
		header = overlay.TruncateLeftEllipsis(header, innerWidth-4)
	}
	headerLine := borderStyle.Render("┌") +
		headerStyle.Render(" "+overlay.PadOrTruncDots(header, innerWidth-2)+" ") +
		borderStyle.Render("┐")

	// File list rows. The vertical bar is styled once and reused: it is the
	// hottest allocation in the render path, twice per row per frame.
	vbar := borderStyle.Render("│")
	rows := make([]string, 0, m.height)
	end := m.offset + m.height
	if end > len(m.entries) {
		end = len(m.entries)
	}

	for i := m.offset; i < end; i++ {
		row := m.renderRow(i, innerWidth, th)
		rows = append(rows, vbar+row+vbar)
	}

	// Fill remaining rows with empty space
	emptyRow := th.FileNormal.Render(strings.Repeat(" ", innerWidth))
	for len(rows) < m.height {
		rows = append(rows, vbar+emptyRow+vbar)
	}

	// Footer
	var footerText string
	if m.searching {
		footerText = fmt.Sprintf(" Search: %s_ ", m.searchQuery)
	} else {
		count := len(m.entries)
		if m.entries != nil && !m.Location().IsRoot() {
			count-- // exclude ".."
		}
		if m.showHidden {
			footerText = fmt.Sprintf(" %d files ", count)
		} else {
			footerText = fmt.Sprintf(" %d files [.hidden] ", count)
		}
	}
	footerLine := borderStyle.Render("└") +
		headerStyle.Render(overlay.PadOrTruncDots(footerText, innerWidth)) +
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
	isDir := IsDirLike(&m, entry)
	isCursor := idx == m.cursor && m.active
	isSelected := m.selected[idx]

	// Determine columns: name, size, time
	sizeStr := ""
	timeStr := ""
	if info != nil {
		if size, ok := m.dirSizes[m.Location().Join(name)]; ok && isDir && size < 0 {
			sizeStr = "..."
		} else if ok && isDir {
			sizeStr = FormatSize(size)
		} else if isDir {
			sizeStr = "<DIR>"
		} else {
			sizeStr = FormatSize(info.Size())
		}
		timeStr = FormatTime(info.ModTime())
	} else if isDir {
		sizeStr = "<DIR>"
	}

	// Column widths: time=12, size=7, rest=name
	timeWidth := 12
	sizeWidth := 7
	nameWidth := width - sizeWidth - timeWidth - 2 // 2 spaces between columns
	if nameWidth < 4 {
		timeWidth = 0
		nameWidth = width - sizeWidth - 1
		if nameWidth < 4 {
			sizeWidth = 0
			nameWidth = width
		}
	}
	if nameWidth < 1 {
		return ""
	}

	line := overlay.PadOrTruncDots(name, nameWidth)
	if sizeWidth > 0 {
		line += " " + overlay.PadLeft(sizeStr, sizeWidth)
		if timeWidth > 0 {
			line += " " + overlay.PadOrTruncDots(timeStr, timeWidth)
		}
	}

	// Style based on state
	var style lipgloss.Style
	switch {
	case isCursor && isDir && isSelected:
		style = th.FileCursorDirSelected
	case isCursor && isDir:
		style = th.FileCursorDir
	case isCursor && isSelected:
		style = th.FileCursorSelected
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

func isExecutable(mode fs.FileMode) bool {
	return mode&0111 != 0
}

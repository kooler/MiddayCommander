package overlay

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// Place composites a box on top of a background string, centered.
// The background remains visible around the box.
func Place(bg string, boxContent string, bgWidth, bgHeight, boxWidth, boxHeight int) string {
	bgLines := strings.Split(bg, "\n")
	for len(bgLines) < bgHeight {
		bgLines = append(bgLines, strings.Repeat(" ", bgWidth))
	}

	fgLines := strings.Split(boxContent, "\n")

	xOff := (bgWidth - boxWidth) / 2
	yOff := (bgHeight - boxHeight) / 2
	if xOff < 0 {
		xOff = 0
	}
	if yOff < 0 {
		yOff = 0
	}

	for i, fgLine := range fgLines {
		bgIdx := yOff + i
		if bgIdx >= len(bgLines) {
			break
		}

		bgLine := bgLines[bgIdx]

		// Cell- and grapheme-aware slicing: keep the left and right portions of
		// the background around the foreground box without breaking ANSI escapes
		left := ansi.Truncate(bgLine, xOff, "")
		right := ansi.Cut(bgLine, xOff+boxWidth, bgWidth)

		bgLines[bgIdx] = left + fgLine + right
	}

	return strings.Join(bgLines[:bgHeight], "\n")
}

// RenderBox draws a bordered box with title, content lines, and optional footer.
func RenderBox(title string, contentLines []string, footer string, width, height int, borderColor, bgColor, titleColor lipgloss.Color) string {
	borderStyle := lipgloss.NewStyle().Foreground(borderColor).Background(bgColor)
	titleStyle := lipgloss.NewStyle().Foreground(titleColor).Background(bgColor).Bold(true)
	fillStyle := lipgloss.NewStyle().Background(bgColor)

	innerWidth := width - 2

	var lines []string

	// Top border with title (width-aware so multibyte titles align correctly)
	titleStr := " " + title + " "
	if lipgloss.Width(titleStr) > innerWidth {
		titleStr = ansi.Truncate(titleStr, innerWidth, "")
	}
	padLen := innerWidth - lipgloss.Width(titleStr)
	if padLen < 0 {
		padLen = 0
	}
	top := borderStyle.Render("┌") + titleStyle.Render(titleStr) + borderStyle.Render(strings.Repeat("─", padLen)+"┐")
	lines = append(lines, top)

	// Content rows
	contentHeight := height - 2
	if footer != "" {
		contentHeight--
	}

	for i := 0; i < contentHeight; i++ {
		var row string
		if i < len(contentLines) {
			row = contentLines[i]
		} else {
			row = fillStyle.Render(strings.Repeat(" ", innerWidth))
		}
		rowWidth := lipgloss.Width(row)
		if rowWidth < innerWidth {
			row += fillStyle.Render(strings.Repeat(" ", innerWidth-rowWidth))
		}
		lines = append(lines, borderStyle.Render("│")+row+borderStyle.Render("│"))
	}

	// Footer
	if footer != "" {
		footerWidth := lipgloss.Width(footer)
		if footerWidth < innerWidth {
			footer += fillStyle.Render(strings.Repeat(" ", innerWidth-footerWidth))
		}
		lines = append(lines, borderStyle.Render("│")+footer+borderStyle.Render("│"))
	}

	// Bottom border
	bottom := borderStyle.Render("└" + strings.Repeat("─", innerWidth) + "┘")
	lines = append(lines, bottom)

	return strings.Join(lines, "\n")
}

// TruncateLeftEllipsis keeps the right-most cells of s, prefixing with an
// ellipsis if clipped. It never returns wider than width cells.
func TruncateLeftEllipsis(s string, width int) string {
	const ellipsis = "…"
	if width < 1 {
		return ""
	}
	w := ansi.StringWidth(s)
	if w <= width {
		return s
	}
	if width == 1 {
		return ellipsis
	}

	needToStrip := w - (width - 1)
	strippedWidth := 0
	for i := 0; i < len(s); {
		cluster, gw := ansi.FirstGraphemeCluster(s[i:], ansi.GraphemeWidth)
		strippedWidth += gw
		if strippedWidth >= needToStrip {
			break
		}
		i += len(cluster)
	}
	tail := ansi.TruncateLeft(s, strippedWidth, "")
	if ansi.StringWidth(tail) > width-1 {
		tail = ""
	}
	return ellipsis + strings.Repeat(" ", width-1-ansi.StringWidth(tail)) + tail
}

// ClampScroll returns the offset that keeps cursor inside the visible window
// of the given height and caps it to max(0, count-height) so growing the
// window cannot leave blank rows at the bottom while top rows stay hidden.
// Every scrollable list shares this, so the window a list
// scrolls in and the window it draws in cannot drift apart.
func ClampScroll(cursor, offset, height, count int) int {
	if height < 1 {
		height = 1
	}
	if count < 0 {
		count = 0
	}
	maxOffset := max(0, count-height)
	if offset > maxOffset {
		offset = maxOffset
	}
	if cursor < offset {
		offset = cursor
	}
	if cursor >= offset+height {
		offset = cursor - height + 1
	}
	if offset > maxOffset {
		offset = maxOffset
	}
	if offset < 0 {
		offset = 0
	}
	return offset
}

// PadOrTrunc pads s with trailing spaces to exactly width cells, truncating
// from the right when s is wider. The result is exactly width cells when
// width >= 1 and empty otherwise
func PadOrTrunc(s string, width int) string {
	if width < 1 {
		return ""
	}
	w := ansi.StringWidth(s)
	if w > width {
		out := ansi.Truncate(s, width, "")
		return out + strings.Repeat(" ", width-ansi.StringWidth(out))
	}
	return s + strings.Repeat(" ", width-w)
}

// PadOrTruncDots is PadOrTrunc, but clipped values end with "..." so the
// cut stays visible. The result is exactly width cells when width >= 1
func PadOrTruncDots(s string, width int) string {
	if width < 1 {
		return ""
	}
	w := ansi.StringWidth(s)
	if w > width {
		if width > 3 {
			out := ansi.Truncate(s, width-3, "") + "..."
			return out + strings.Repeat(" ", width-ansi.StringWidth(out))
		}
		out := ansi.Truncate(s, width, "")
		return out + strings.Repeat(" ", width-ansi.StringWidth(out))
	}
	return s + strings.Repeat(" ", width-w)
}

// PadLeft right-aligns s in width cells, clipping on the left edge when s is
// wider. The result is exactly width cells when width >= 1
func PadLeft(s string, width int) string {
	if width < 1 {
		return ""
	}
	w := ansi.StringWidth(s)
	if w >= width {
		out := ansi.Truncate(s, width, "")
		return strings.Repeat(" ", width-ansi.StringWidth(out)) + out
	}
	return strings.Repeat(" ", width-w) + s
}

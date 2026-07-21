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

		// ANSI-aware slicing: left portion of bg, then the fg box line, then right portion of bg
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

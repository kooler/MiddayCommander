package cmdexec

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/kooler/MiddayCommander/internal/ui/overlay"
	"github.com/kooler/MiddayCommander/internal/ui/theme"
)

// DismissMsg is sent when the user closes the command overlay.
type DismissMsg struct{}

// CommandDoneMsg delivers the result of an executed command.
type CommandDoneMsg struct {
	Output string
	Err    error
}

// Model is the command execution overlay.
type Model struct {
	input        string
	inputPos     int
	output       string
	outputLines  []string
	outputOffset int
	running      bool
	dir          string
	suggestions  []string
	execOnly     bool
	width        int
	height       int
}

// New creates a new command execution overlay for the given directory.
func New(dir string, width, height int) Model {
	return Model{
		dir:    dir,
		width:  width,
		height: height,
	}
}

// Update handles messages.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case CommandDoneMsg:
		m.running = false
		if msg.Err != nil {
			if msg.Output != "" {
				m.output = msg.Output + "\n" + msg.Err.Error()
			} else {
				m.output = msg.Err.Error()
			}
		} else {
			m.output = msg.Output
		}
		if m.output == "" {
			m.output = "(no output)"
		}
		m.outputLines = strings.Split(strings.TrimRight(m.output, "\n"), "\n")
		m.outputOffset = 0

	case tea.KeyMsg:
		if m.running {
			if msg.String() == "esc" {
				return m, func() tea.Msg { return DismissMsg{} }
			}
			return m, nil
		}
		return m.handleKey(msg)
	}

	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		return m, func() tea.Msg { return DismissMsg{} }

	case "enter":
		if m.input != "" {
			m.running = true
			m.output = ""
			m.outputLines = nil
			m.outputOffset = 0
			m.suggestions = nil
			return m, runCommandCmd(m.dir, m.input)
		}

	case "tab":
		return m.completeCurrentWord(), nil

	case "ctrl+g":
		m.execOnly = true
		return m.updateSuggestions(), nil

	case "backspace":
		if m.inputPos > 0 {
			m.input = m.input[:m.inputPos-1] + m.input[m.inputPos:]
			m.inputPos--
		}
		m = m.updateSuggestions()

	case "delete":
		if m.inputPos < len(m.input) {
			m.input = m.input[:m.inputPos] + m.input[m.inputPos+1:]
		}
		m = m.updateSuggestions()

	case "left":
		if m.inputPos > 0 {
			m.inputPos--
		}
		m = m.updateSuggestions()

	case "right":
		if m.inputPos < len(m.input) {
			m.inputPos++
		}
		m = m.updateSuggestions()

	case "home":
		m.inputPos = 0
		m = m.updateSuggestions()

	case "end":
		m.inputPos = len(m.input)
		m = m.updateSuggestions()

	case "up":
		if m.outputOffset > 0 {
			m.outputOffset--
		}

	case "down":
		maxOffset := len(m.outputLines) - m.outputHeight()
		if maxOffset < 0 {
			maxOffset = 0
		}
		if m.outputOffset < maxOffset {
			m.outputOffset++
		}

	case "pgup":
		m.outputOffset -= m.outputHeight()
		if m.outputOffset < 0 {
			m.outputOffset = 0
		}

	case "pgdown":
		m.outputOffset += m.outputHeight()
		maxOffset := len(m.outputLines) - m.outputHeight()
		if maxOffset < 0 {
			maxOffset = 0
		}
		if m.outputOffset > maxOffset {
			m.outputOffset = maxOffset
		}

	default:
		s := msg.String()
		if len(s) == 1 && s[0] >= 32 {
			if m.inputPos < 0 {
				m.inputPos = 0
			} else if m.inputPos > len(m.input) {
				m.inputPos = len(m.input)
			}
			m.input = m.input[:m.inputPos] + s + m.input[m.inputPos:]
			m.inputPos++
			m = m.updateSuggestions()
		}
	}

	return m, nil
}

// BoxSize returns the desired box dimensions for the overlay.
func (m Model) BoxSize(screenWidth, screenHeight int) (int, int) {
	w := screenWidth * 4 / 5
	if w < 50 {
		w = min(50, screenWidth)
	}
	h := screenHeight * 3 / 5
	if h < 12 {
		h = min(12, screenHeight)
	}
	return w, h
}

func (m Model) outputHeight() int {
	_, boxH := m.BoxSize(m.width, m.height)
	h := boxH - 6 // borders(2) + dir(1) + input(1) + separator(1) + footer(1)
	if h < 1 {
		h = 1
	}
	return h
}

// View renders the command execution overlay.
func (m Model) View(th theme.Theme, screenWidth, screenHeight int) string {
	boxW, boxH := m.BoxSize(screenWidth, screenHeight)
	innerW := boxW - 2

	bg := lipgloss.Color("#1e1e2e")
	fg := lipgloss.Color("#cdd6f4")
	subtle := lipgloss.Color("#a6adc8")
	accent := lipgloss.Color("#89b4fa")

	bgStyle := lipgloss.NewStyle().Background(bg).Foreground(fg)
	promptStyle := lipgloss.NewStyle().Background(bg).Foreground(accent).Bold(true)
	dimStyle := lipgloss.NewStyle().Background(bg).Foreground(subtle)

	var contentLines []string

	// Directory line
	dir := m.dir
	if len(dir) > innerW-2 {
		dir = "..." + dir[len(dir)-innerW+5:]
	}
	dirLine := dimStyle.Render(" " + dir)
	dirWidth := lipgloss.Width(dirLine)
	if dirWidth < innerW {
		dirLine += dimStyle.Render(strings.Repeat(" ", innerW-dirWidth))
	}
	contentLines = append(contentLines, dirLine)

	// Input line with cursor
	var inputDisplay string
	if m.inputPos < len(m.input) {
		inputDisplay = m.input[:m.inputPos] + "█" + m.input[m.inputPos:]
	} else {
		inputDisplay = m.input + "█"
	}
	inputLine := promptStyle.Render(" $ ") + bgStyle.Render(inputDisplay)
	inputWidth := lipgloss.Width(inputLine)
	if inputWidth < innerW {
		inputLine += bgStyle.Render(strings.Repeat(" ", innerW-inputWidth))
	}
	contentLines = append(contentLines, inputLine)

	// Separator
	sepLine := dimStyle.Render(strings.Repeat("─", innerW))
	contentLines = append(contentLines, sepLine)

	// Output area
	oh := boxH - 6 // borders(2) + dir(1) + input(1) + separator(1) + footer(1)
	if oh < 1 {
		oh = 1
	}

	if m.running {
		runLine := dimStyle.Render(" Running...")
		runWidth := lipgloss.Width(runLine)
		if runWidth < innerW {
			runLine += dimStyle.Render(strings.Repeat(" ", innerW-runWidth))
		}
		contentLines = append(contentLines, runLine)
	} else if len(m.outputLines) > 0 {
		end := m.outputOffset + oh
		if end > len(m.outputLines) {
			end = len(m.outputLines)
		}
		for i := m.outputOffset; i < end; i++ {
			line := " " + m.outputLines[i]
			if lipgloss.Width(line) > innerW {
				line = line[:innerW]
			}
			rendered := bgStyle.Render(line)
			renderedWidth := lipgloss.Width(rendered)
			if renderedWidth < innerW {
				rendered += bgStyle.Render(strings.Repeat(" ", innerW-renderedWidth))
			}
			contentLines = append(contentLines, rendered)
		}
	} else if len(m.suggestions) > 0 {
		oh := boxH - 6
		if oh < 1 {
			oh = 1
		}
		suggestionLines := formatSuggestions(m.suggestions, innerW, oh)
		for _, line := range suggestionLines {
			contentLines = append(contentLines, dimStyle.Render(line))
		}
	} else {
		hint := dimStyle.Render(" Type a command and press Enter")
		hintWidth := lipgloss.Width(hint)
		if hintWidth < innerW {
			hint += dimStyle.Render(strings.Repeat(" ", innerW-hintWidth))
		}
		contentLines = append(contentLines, hint)
	}

	// Footer
	footerKeyStyle := lipgloss.NewStyle().Background(bg).Foreground(accent).Bold(true)
	footer := footerKeyStyle.Render(" Enter") + dimStyle.Render(":Run  ") +
		footerKeyStyle.Render("Esc") + dimStyle.Render(":Close  ") +
		footerKeyStyle.Render("↑↓") + dimStyle.Render(":Scroll")

	if len(m.outputLines) > oh {
		scrollInfo := fmt.Sprintf("  [%d-%d/%d]", m.outputOffset+1,
			min(m.outputOffset+oh, len(m.outputLines)), len(m.outputLines))
		footer += dimStyle.Render(scrollInfo)
	}

	footerWidth := lipgloss.Width(footer)
	if footerWidth < innerW {
		footer += dimStyle.Render(strings.Repeat(" ", innerW-footerWidth))
	}

	return overlay.RenderBox("Run Command", contentLines, footer, boxW, boxH,
		accent, bg, accent)
}

func runCommandCmd(dir, command string) tea.Cmd {
	return func() tea.Msg {
		cmd := exec.Command("sh", "-c", command)
		cmd.Dir = dir
		var buf bytes.Buffer
		cmd.Stdout = &buf
		cmd.Stderr = &buf
		err := cmd.Run()
		return CommandDoneMsg{Output: buf.String(), Err: err}
	}
}

func (m Model) completeCurrentWord() Model {
	start, end, prefix := currentWord(m.input, m.inputPos)
	if prefix == "" {
		return m
	}

	candidates := completeCandidates(prefix, m.dir, m.execOnly)
	m.suggestions = candidates
	if len(candidates) == 0 {
		return m
	}

	didComplete := false
	common := commonPrefix(candidates)
	if len(common) > len(prefix) {
		m.input = m.input[:start] + mergeCompletion(common, m.input[end:])
		m.inputPos = clamp(start+len(common)-(end-m.inputPos), len(m.input))
		didComplete = true
	}

	if len(candidates) == 1 {
		m.input = m.input[:start] + mergeCompletion(candidates[0], m.input[end:])
		m.inputPos = clamp(start+len(candidates[0])-(end-m.inputPos), len(m.input))
		didComplete = true
	}

	if didComplete {
		m.suggestions = nil
	}
	return m
}

func clamp(pos, length int) int {
	if pos < 0 {
		return 0
	}
	if pos > length {
		return length
	}
	return pos
}

func (m Model) updateSuggestions() Model {
	_, _, prefix := currentWord(m.input, m.inputPos)
	if prefix == "" {
		if m.execOnly {
			m.suggestions = completeCandidates(prefix, m.dir, m.execOnly)
		} else {
			m.suggestions = nil
		}
		return m
	}
	m.suggestions = completeCandidates(prefix, m.dir, m.execOnly)
	return m
}

func mergeCompletion(candidate, suffix string) string {
	for i := len(candidate); i > 0; i-- {
		if strings.HasPrefix(suffix, candidate[len(candidate)-i:]) {
			return candidate + suffix[i:]
		}
	}
	return candidate + suffix
}

func currentWord(input string, pos int) (int, int, string) {
	if pos > len(input) {
		pos = len(input)
	}
	start := strings.LastIndexFunc(input[:pos], unicode.IsSpace)
	if start == -1 {
		start = 0
	} else {
		start++
	}
	end := pos
	for end < len(input) && !unicode.IsSpace(rune(input[end])) {
		end++
	}
	return start, end, input[start:end]
}

func completeCandidates(prefix, dir string, execOnly bool) []string {
	if execOnly {
		return completeExecCandidates(prefix)
	}

	pathCandidates := completePathCandidates(prefix, dir)
	execCandidates := []string{}
	if len(pathCandidates) == 0 && !strings.Contains(prefix, "/") {
		execCandidates = completeExecCandidates(prefix)
	}
	candidates := make(map[string]struct{})
	for _, c := range pathCandidates {
		candidates[c] = struct{}{}
	}
	for _, c := range execCandidates {
		candidates[c] = struct{}{}
	}

	var out []string
	for c := range candidates {
		out = append(out, c)
	}
	sort.Strings(out)
	return out
}

func expandTilde(path string) string {
	if !strings.HasPrefix(path, "~") {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	if path == "~" {
		return home
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(home, path[2:])
	}
	return path
}

func completePathCandidates(prefix, dir string) []string {
	if prefix == "~" {
		return []string{"~/"}
	}

	rawDir, base := filepath.Split(prefix)
	if rawDir == "" {
		rawDir = "."
	}

	// Expand ~ in both rawDir and display
	expandedRawDir := rawDir
	if strings.Contains(rawDir, "~") {
		expandedRawDir = expandTilde(rawDir)
	}

	scanDir := expandedRawDir
	if !filepath.IsAbs(expandedRawDir) && expandedRawDir != "." {
		scanDir = filepath.Join(dir, expandedRawDir)
	}
	entries, err := os.ReadDir(scanDir)
	if err != nil {
		return nil
	}

	var candidates []string
	for _, entry := range entries {
		if base != "" && !strings.HasPrefix(entry.Name(), base) {
			continue
		}
		candidate := filepath.Join(rawDir, entry.Name())
		if rawDir == "." {
			candidate = entry.Name()
		}
		if entry.IsDir() {
			candidate += string(os.PathSeparator)
		}
		candidates = append(candidates, candidate)
	}
	sort.Strings(candidates)
	return candidates
}

func completeExecCandidates(prefix string) []string {
	pathEnv := os.Getenv("PATH")
	paths := filepath.SplitList(pathEnv)
	seen := make(map[string]struct{})
	var candidates []string

	for _, p := range paths {
		entries, err := os.ReadDir(p)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			name := entry.Name()
			if !strings.HasPrefix(name, prefix) {
				continue
			}
			if strings.Contains(name, "/") {
				continue
			}
			if _, ok := seen[name]; ok {
				continue
			}
			path := filepath.Join(p, name)
			info, err := os.Stat(path)
			if err != nil {
				continue
			}
			if info.Mode().IsRegular() && info.Mode().Perm()&0111 != 0 {
				seen[name] = struct{}{}
				candidates = append(candidates, name)
			}
		}
	}
	sort.Strings(candidates)
	return candidates
}

func commonPrefix(strs []string) string {
	if len(strs) == 0 {
		return ""
	}
	prefix := strs[0]
	for _, s := range strs[1:] {
		for !strings.HasPrefix(s, prefix) {
			if prefix == "" {
				return ""
			}
			prefix = prefix[:len(prefix)-1]
		}
	}
	return prefix
}

func truncOrPad(s string, width int) string {
	if lipgloss.Width(s) > width {
		if width > 3 {
			return s[:width-3] + "..."
		}
		return s[:width]
	}
	return s + strings.Repeat(" ", width-lipgloss.Width(s))
}

func formatSuggestions(suggestions []string, width, maxLines int) []string {
	if len(suggestions) == 0 {
		return nil
	}

	var lines []string
	currentLine := " "
	currentWidth := 1

	// Estimate padding needed (2 spaces between items)
	itemSpacing := 2

	for i, sug := range suggestions {
		// Display basename for readability, but use full path for completion
		displayName := filepath.Base(strings.TrimRight(sug, "/"))
		sugWidth := lipgloss.Width(displayName)
		neededWidth := sugWidth + itemSpacing
		if i == 0 {
			neededWidth = sugWidth + 1 // Just space after first item
		}

		// If adding this suggestion would exceed width, start a new line
		if currentWidth+neededWidth > width {
			// Pad the current line to width
			if lipgloss.Width(currentLine) < width {
				currentLine += strings.Repeat(" ", width-lipgloss.Width(currentLine))
			}
			lines = append(lines, currentLine)
			if len(lines) >= maxLines {
				break
			}
			currentLine = " "
			currentWidth = 1
		}

		if len(currentLine) > 1 {
			currentLine += "  "
			currentWidth += 2
		}
		currentLine += displayName
		currentWidth += sugWidth

		if len(lines) >= maxLines {
			// Truncate current suggestion if we're at max lines
			break
		}
	}

	// Add the last line if there's content and we haven't hit max lines
	if len(lines) < maxLines && currentWidth > 1 {
		if lipgloss.Width(currentLine) < width {
			currentLine += strings.Repeat(" ", width-lipgloss.Width(currentLine))
		}
		lines = append(lines, currentLine)
	}

	return lines
}

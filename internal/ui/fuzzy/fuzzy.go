package fuzzy

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/kooler/MiddayCommander/internal/ui/overlay"
	"github.com/kooler/MiddayCommander/internal/ui/theme"
)

// ResultMsg is sent when the user selects a result.
type ResultMsg struct {
	Path string // full path of the selected file/directory
}

// DismissMsg is sent when the user cancels the fuzzy finder.
type DismissMsg struct{}

// FileWalkMsg delivers a batch of discovered paths.
type FileWalkMsg struct {
	Paths []string
	Done  bool
}

// Model is the fuzzy finder overlay.
type Model struct {
	query     string
	allPaths  []string     // all discovered paths (accumulated)
	matches   []match      // filtered + scored results
	cursor    int          // selected result index
	offset    int          // scroll offset
	rootDir   string       // directory being searched
	walking   bool         // true while background walker is running
	width     int
	height    int
}

type match struct {
	path       string
	score      int
	matchIdxs  []int // character indices that matched in the display name
}

// New creates a new fuzzy finder searching from rootDir.
func New(rootDir string, width, height int) Model {
	return Model{
		rootDir: rootDir,
		walking: true,
		width:   width,
		height:  height,
	}
}

// Init starts the background file walker.
func (m Model) Init() tea.Cmd {
	return walkFilesCmd(m.rootDir)
}

// Update handles messages.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case FileWalkMsg:
		m.allPaths = append(m.allPaths, msg.Paths...)
		m.walking = !msg.Done
		m.refilter()

	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return m, func() tea.Msg { return DismissMsg{} }
		case "enter":
			if m.cursor >= 0 && m.cursor < len(m.matches) {
				path := m.matches[m.cursor].path
				return m, func() tea.Msg { return ResultMsg{Path: path} }
			}
			return m, func() tea.Msg { return DismissMsg{} }
		case "up", "ctrl+p":
			if m.cursor > 0 {
				m.cursor--
				m.clampOffset()
			}
		case "down", "ctrl+n":
			if m.cursor < len(m.matches)-1 {
				m.cursor++
				m.clampOffset()
			}
		case "pgup":
			m.cursor -= m.resultHeight()
			if m.cursor < 0 {
				m.cursor = 0
			}
			m.clampOffset()
		case "pgdown":
			m.cursor += m.resultHeight()
			if m.cursor >= len(m.matches) {
				m.cursor = len(m.matches) - 1
			}
			if m.cursor < 0 {
				m.cursor = 0
			}
			m.clampOffset()
		case "backspace":
			if len(m.query) > 0 {
				m.query = m.query[:len(m.query)-1]
				m.refilter()
			}
		default:
			s := msg.String()
			if len(s) == 1 && s[0] >= 32 {
				m.query += s
				m.refilter()
			}
		}
	}

	return m, nil
}

// Done returns true when the finder should be closed (never — closed via messages).
func (m Model) Done() bool {
	return false
}

// BoxSize returns the desired box dimensions for the overlay.
func (m Model) BoxSize(screenWidth, screenHeight int) (int, int) {
	w := screenWidth * 3 / 4
	if w < 40 {
		w = min(40, screenWidth)
	}
	h := screenHeight * 3 / 4
	if h < 10 {
		h = min(10, screenHeight)
	}
	return w, h
}

// View renders the fuzzy finder as a bordered floating box content.
func (m Model) View(th theme.Theme, screenWidth, screenHeight int) string {
	boxW, boxH := m.BoxSize(screenWidth, screenHeight)
	innerW := boxW - 2 // borders

	bg := lipgloss.Color("#1e1e2e")
	fg := lipgloss.Color("#cdd6f4")
	subtle := lipgloss.Color("#a6adc8")
	accent := lipgloss.Color("#89b4fa")
	matchColor := lipgloss.Color("#f9e2af")
	cursorBg := lipgloss.Color("#45475a")

	bgStyle := lipgloss.NewStyle().Background(bg).Foreground(fg)
	promptStyle := lipgloss.NewStyle().Background(bg).Foreground(accent).Bold(true)
	matchHLStyle := lipgloss.NewStyle().Background(bg).Foreground(matchColor).Bold(true)
	cursorStyle := lipgloss.NewStyle().Background(cursorBg).Foreground(fg)
	normalStyle := bgStyle
	dimStyle := lipgloss.NewStyle().Background(bg).Foreground(subtle)

	var contentLines []string

	// Search input line
	status := ""
	if m.walking {
		status = " (scanning...)"
	}
	inputLine := promptStyle.Render("❯ ") + bgStyle.Render(m.query+"_") + dimStyle.Render(status)
	inputWidth := lipgloss.Width(inputLine)
	if inputWidth < innerW {
		inputLine += bgStyle.Render(strings.Repeat(" ", innerW-inputWidth))
	}
	contentLines = append(contentLines, inputLine)

	// Results
	rh := boxH - 4 // borders(2) + input(1) + footer(1)
	if rh < 1 {
		rh = 1
	}

	end := m.offset + rh
	if end > len(m.matches) {
		end = len(m.matches)
	}

	for i := m.offset; i < end; i++ {
		mt := m.matches[i]
		isCursor := i == m.cursor

		rel, _ := filepath.Rel(m.rootDir, mt.path)
		if rel == "" {
			rel = mt.path
		}
		display := rel
		if len(display) > innerW-1 {
			display = "…" + display[len(display)-innerW+2:]
		}

		var line string
		if isCursor {
			line = cursorStyle.Render(padStr(" "+display, innerW))
		} else {
			line = renderWithHighlights(" "+display, shiftIdxs(mt.matchIdxs, 1), normalStyle, matchHLStyle, innerW)
		}
		contentLines = append(contentLines, line)
	}

	// Footer
	countStr := fmt.Sprintf(" %d/%d ", len(m.matches), len(m.allPaths))
	footer := dimStyle.Render(countStr)
	footerWidth := lipgloss.Width(footer)
	if footerWidth < innerW {
		footer += dimStyle.Render(strings.Repeat(" ", innerW-footerWidth))
	}

	return overlay.RenderBox("Find File", contentLines, footer, boxW, boxH,
		accent, bg, accent)
}

func (m Model) resultHeight() int {
	_, boxH := m.BoxSize(m.width, m.height)
	h := boxH - 4
	if h < 1 {
		h = 1
	}
	return h
}

func shiftIdxs(idxs []int, offset int) []int {
	out := make([]int, len(idxs))
	for i, idx := range idxs {
		out[i] = idx + offset
	}
	return out
}

func (m *Model) clampOffset() {
	rh := m.resultHeight()
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+rh {
		m.offset = m.cursor - rh + 1
	}
}

func (m *Model) refilter() {
	m.matches = fuzzyFilter(m.allPaths, m.query, m.rootDir)
	m.cursor = 0
	m.offset = 0
}

// --- Fuzzy matching ---

func fuzzyFilter(paths []string, query, rootDir string) []match {
	if query == "" {
		// Show all paths (up to a limit), no scoring needed
		var results []match
		limit := 1000
		for _, p := range paths {
			rel, _ := filepath.Rel(rootDir, p)
			if rel == "" {
				rel = p
			}
			results = append(results, match{path: p, score: 0})
			if len(results) >= limit {
				break
			}
		}
		return results
	}

	queryLower := strings.ToLower(query)
	var results []match

	for _, p := range paths {
		rel, _ := filepath.Rel(rootDir, p)
		if rel == "" {
			rel = p
		}
		score, idxs := fuzzyMatch(rel, queryLower)
		if score > 0 {
			results = append(results, match{path: p, score: score, matchIdxs: idxs})
		}
	}

	// Sort by score descending (simple insertion sort, fast enough for interactive use)
	for i := 1; i < len(results); i++ {
		for j := i; j > 0 && results[j].score > results[j-1].score; j-- {
			results[j], results[j-1] = results[j-1], results[j]
		}
	}

	if len(results) > 1000 {
		results = results[:1000]
	}

	return results
}

// fuzzyMatch scores how well target matches the query (case-insensitive).
// Returns 0 if no match.
func fuzzyMatch(target, queryLower string) (int, []int) {
	targetLower := strings.ToLower(target)
	qi := 0
	score := 0
	var idxs []int
	prevMatch := false

	for ti := 0; ti < len(targetLower) && qi < len(queryLower); ti++ {
		if targetLower[ti] == queryLower[qi] {
			idxs = append(idxs, ti)
			score += 10
			// Bonus for consecutive matches
			if prevMatch {
				score += 5
			}
			// Bonus for matching at word boundary
			if ti == 0 || target[ti-1] == '/' || target[ti-1] == '_' || target[ti-1] == '-' || target[ti-1] == '.' {
				score += 10
			}
			// Bonus for exact case match
			if target[ti] == queryLower[qi] || (qi < len(queryLower) && unicode.ToUpper(rune(target[ti])) == unicode.ToUpper(rune(queryLower[qi]))) {
				score++
			}
			qi++
			prevMatch = true
		} else {
			prevMatch = false
		}
	}

	if qi < len(queryLower) {
		return 0, nil // not all query chars matched
	}

	// Prefer shorter paths (basename matches)
	score -= len(target) / 5

	return score, idxs
}

// --- File walker ---

func walkFilesCmd(rootDir string) tea.Cmd {
	return func() tea.Msg {
		var paths []string
		_ = filepath.WalkDir(rootDir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil // skip errors
			}
			name := d.Name()
			// Skip hidden dirs and common large directories
			if d.IsDir() && (strings.HasPrefix(name, ".") || name == "node_modules" || name == "vendor" || name == "__pycache__") {
				return filepath.SkipDir
			}
			paths = append(paths, path)
			if len(paths) >= 50000 {
				return filepath.SkipAll
			}
			return nil
		})
		return FileWalkMsg{Paths: paths, Done: true}
	}
}

// --- Render helpers ---

func renderWithHighlights(s string, matchIdxs []int, normal, highlight lipgloss.Style, width int) string {
	matchSet := make(map[int]bool, len(matchIdxs))
	for _, idx := range matchIdxs {
		matchSet[idx] = true
	}

	var b strings.Builder
	for i, ch := range s {
		if matchSet[i] {
			b.WriteString(highlight.Render(string(ch)))
		} else {
			b.WriteString(normal.Render(string(ch)))
		}
	}
	// Pad to width
	rendered := b.String()
	visWidth := lipgloss.Width(rendered)
	if visWidth < width {
		rendered += normal.Render(strings.Repeat(" ", width-visWidth))
	}
	return rendered
}

func padStr(s string, width int) string {
	if len(s) >= width {
		return s[:width]
	}
	return s + strings.Repeat(" ", width-len(s))
}

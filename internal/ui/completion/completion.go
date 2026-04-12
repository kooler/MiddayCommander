package completion

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/charmbracelet/lipgloss"
)

var (
	execCacheMu        sync.Mutex
	execPathEnv        string
	execCandidateCache []string
)

func ExpandTilde(path string) string {
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

func CommonPrefix(strs []string) string {
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

func PadOrTrim(s string, width int) string {
	if lipgloss.Width(s) > width {
		if width > 3 {
			return s[:width-3] + "..."
		}
		return s[:width]
	}
	return s + strings.Repeat(" ", width-lipgloss.Width(s))
}

func FormatSuggestions(suggestions []string, width, maxLines int, basename bool) []string {
	if len(suggestions) == 0 {
		return nil
	}

	var lines []string
	currentLine := " "
	currentWidth := 1
	itemSpacing := 2

	for i, sug := range suggestions {
		displayName := sug
		if basename {
			displayName = filepath.Base(strings.TrimRight(sug, string(os.PathSeparator)))
		}
		sugWidth := lipgloss.Width(displayName)
		neededWidth := sugWidth + itemSpacing
		if i == 0 {
			neededWidth = sugWidth + 1
		}

		if currentWidth+neededWidth > width {
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
			currentLine += strings.Repeat(" ", itemSpacing)
			currentWidth += itemSpacing
		}
		currentLine += displayName
		currentWidth += sugWidth

		if len(lines) >= maxLines {
			break
		}
	}

	if len(lines) < maxLines && currentWidth > 1 {
		if lipgloss.Width(currentLine) < width {
			currentLine += strings.Repeat(" ", width-lipgloss.Width(currentLine))
		}
		lines = append(lines, currentLine)
	}

	return lines
}

func CurrentWord(input string, pos int) (int, int, string) {
	if pos > len(input) {
		pos = len(input)
	}
	start := strings.LastIndexFunc(input[:pos], func(r rune) bool {
		return strings.ContainsRune(" \t\n\r", r)
	})
	if start == -1 {
		start = 0
	} else {
		start++
	}
	end := pos
	for end < len(input) && !strings.ContainsRune(" \t\n\r", rune(input[end])) {
		end++
	}
	return start, end, input[start:end]
}

func CompletePathCandidates(prefix, dir string, dirsOnly bool) []string {
	if prefix == "~" {
		return []string{"~/"}
	}

	rawDir, base := filepath.Split(prefix)
	if rawDir == "" {
		rawDir = "."
	}

	expandedRawDir := rawDir
	if strings.Contains(rawDir, "~") {
		expandedRawDir = ExpandTilde(rawDir)
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
		if dirsOnly && !entry.IsDir() {
			continue
		}
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

func CompleteExecCandidates(prefix string) []string {
	if strings.Contains(prefix, string(filepath.Separator)) {
		rawDir, base := filepath.Split(prefix)
		if rawDir == "" {
			rawDir = "."
		}

		expandedRawDir := rawDir
		if strings.Contains(rawDir, "~") {
			expandedRawDir = ExpandTilde(rawDir)
		}

		scanDir := expandedRawDir
		if !filepath.IsAbs(expandedRawDir) && expandedRawDir != "." {
			scanDir = filepath.Join(".", expandedRawDir)
		}

		entries, err := os.ReadDir(scanDir)
		if err != nil {
			return nil
		}

		var out []string
		for _, entry := range entries {
			if base != "" && !strings.HasPrefix(entry.Name(), base) {
				continue
			}
			if entry.IsDir() {
				continue
			}
			info, err := entry.Info()
			if err != nil {
				continue
			}
			if !info.Mode().IsRegular() || info.Mode().Perm()&0111 == 0 {
				continue
			}

			candidate := filepath.Join(rawDir, entry.Name())
			out = append(out, candidate)
		}
		sort.Strings(out)
		return out
	}

	pathEnv := os.Getenv("PATH")
	execCacheMu.Lock()
	if execCandidateCache == nil || pathEnv != execPathEnv {
		execPathEnv = pathEnv
		execCandidateCache = buildExecCache(pathEnv)
	}
	candidates := execCandidateCache
	execCacheMu.Unlock()

	if prefix == "" {
		return append([]string(nil), candidates...)
	}

	var out []string
	for _, candidate := range candidates {
		if strings.HasPrefix(candidate, prefix) {
			out = append(out, candidate)
		}
	}
	return out
}

func buildExecCache(pathEnv string) []string {
	paths := filepath.SplitList(pathEnv)
	seen := make(map[string]struct{})
	var candidates []string
	for _, p := range paths {
		if p == "" {
			p = "."
		}
		entries, err := os.ReadDir(p)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			name := entry.Name()
			if strings.Contains(name, "/") {
				continue
			}
			if _, ok := seen[name]; ok {
				continue
			}
			info, err := entry.Info()
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

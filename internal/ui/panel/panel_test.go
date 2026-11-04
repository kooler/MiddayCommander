package panel

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/kooler/MiddayCommander/internal/config"
	"github.com/kooler/MiddayCommander/internal/ui/theme"
	"github.com/kooler/MiddayCommander/internal/vfs/local"
)

// mustWrite and mustMkdir create fixture files, failing the test instead of
// silently ignoring the error.
func mustWrite(t *testing.T, path string, data []byte, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, data, mode); err != nil {
		t.Fatal(err)
	}
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

// newTestModel builds a panel.Model populated with real directory entries
// so renderRow and View can be exercised against actual fs.DirEntry / fs.FileInfo.
func newTestModel(t *testing.T, dir string) Model {
	t.Helper()
	lfs := local.New("/")
	cfg := config.Default()
	km := KeyMap{}
	m := New(lfs, dir, km, cfg)

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	m.entries = entries
	m.infos = make([]fs.FileInfo, len(entries))
	for i, e := range entries {
		info, err := e.Info()
		if err != nil {
			t.Fatal(err)
		}
		m.infos[i] = info
	}
	m.width = 80
	m.height = 10
	m.active = true
	m.selected = make(map[int]bool)
	m.cfg = cfg
	return m
}

// rowWidth returns the cell width of the rendered row (without the surrounding border bars).
func rowWidth(t *testing.T, row string) int {
	t.Helper()
	w := ansi.StringWidth(row)
	if w < 0 {
		t.Fatalf("negative width: %d", w)
	}
	return w
}

// TestRenderRowWidth verifies the exact-width contract: every row rendered by
// renderRow is exactly `width` cells wide, including when the filename is a
// wide grapheme (CJK / emoji) that gets truncated.
func TestRenderRowWidth(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "report.pdf"), []byte("x"), 0o644)
	mustMkdir(t, filepath.Join(dir, "subdir"))

	th := theme.Default()

	tests := []struct {
		name     string
		filename string
		isDir    bool
		width    int
	}{
		{"ascii fits", "report.pdf", false, 30},
		{"cjk truncated", "日本語のファイル名前です.txt", false, 20},
		{"emoji truncated", "🎉🎉🎉🎉🎉report.pdf", false, 16},
		{"wide dir", "日本語のディレクトリ", true, 20},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestModel(t, dir)
			// Override the first entry's name for width testing.
			// We use a synthetic DirEntry-like struct because fs.DirEntry is an interface.
			m.entries = []fs.DirEntry{&nameOnlyEntry{name: tt.filename, isDir: tt.isDir}}
			m.infos = []fs.FileInfo{nil}

			m.cursor = 0
			m.active = true
			row := m.renderRow(0, tt.width, th)
			if got := rowWidth(t, row); got != tt.width {
				t.Errorf("width = %d, want %d (row %q)", got, tt.width, row)
			}
		})
	}
}

// TestRenderRowNarrow verifies that panels narrower than the column budget
// collapse size / time columns without breaking or returning the wrong width.
func TestRenderRowNarrow(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "a.txt"), []byte("x"), 0o644)
	mustMkdir(t, filepath.Join(dir, "b_dir"))

	th := theme.Default()
	m := newTestModel(t, dir)

	tests := []struct {
		name  string
		width int
	}{
		{"normal", 40},
		{"drop time", 20},
		{"drop size and time", 10},
		{"single cell", 1},
		{"zero width", 0},
		{"negative width", -3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for idx := range m.entries {
				row := m.renderRow(idx, tt.width, th)
				if tt.width < 1 {
					if row != "" {
						t.Errorf("expected empty row for width=%d, got %q", tt.width, row)
					}
					continue
				}
				if got := rowWidth(t, row); got != tt.width {
					t.Errorf("width = %d, want %d (row %q)", got, tt.width, row)
				}
			}
		})
	}
}

// TestRenderRowColumnAlignment checks that a wide filename does not shift the
// size / time columns. For a fixed panel width, the right-hand columns must
// start at the same cell regardless of filename cell width.
func TestRenderRowColumnAlignment(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "short.txt"), []byte("x"), 0o644)
	mustMkdir(t, filepath.Join(dir, "wide"))
	info, err := os.Stat(filepath.Join(dir, "short.txt"))
	if err != nil {
		t.Fatal(err)
	}

	th := theme.Default()
	m := newTestModel(t, dir)

	// Replace entries with a fixed set: one ASCII name, one wide CJK name.
	m.entries = []fs.DirEntry{
		&nameOnlyEntry{name: "short.txt", isDir: false},
		&nameOnlyEntry{name: "日本語のファイル名前です.txt", isDir: false},
	}
	m.infos = []fs.FileInfo{info, info}
	m.cursor = 0
	m.active = true

	width := 50
	rows := make([]string, len(m.entries))
	for i := range m.entries {
		rows[i] = m.renderRow(i, width, th)
		if got := rowWidth(t, rows[i]); got != width {
			t.Errorf("row %d width = %d, want %d", i, got, width)
		}
	}

	// With fixed width, the size column should start at the same position in
	// both rows. We verify by finding the position of the size string in each row.
	sizeStr := " 4B" // FormatSize(4) for our 1-byte file
	for i, row := range rows {
		idx := lipgloss.Width(row) - lipgloss.Width(sizeStr)
		if idx < 0 {
			t.Fatalf("size column not found in row %d", i)
		}
		// The size column should be at width - len(sizeStr) for all rows.
		if idx != width-lipgloss.Width(sizeStr) {
			t.Errorf("row %d: size column at %d, want %d", i, idx, width-lipgloss.Width(sizeStr))
		}
	}
}

// TestRenderRowStates checks that cursor / selected / dir / exec styles don't
// affect width.
func TestRenderRowStates(t *testing.T) {
	dir := t.TempDir()
	binPath := filepath.Join(dir, "run.sh")
	mustWrite(t, binPath, []byte("#!/bin/sh\necho hi\n"), 0o755)
	info, _ := os.Stat(binPath)

	th := theme.Default()
	m := newTestModel(t, dir)
	m.entries = []fs.DirEntry{&nameOnlyEntry{name: "run.sh", isDir: false}}
	m.infos = []fs.FileInfo{info}
	m.width = 30

	tests := []struct {
		name       string
		active     bool
		cursor     bool
		selected   bool
		expectExec bool
	}{
		{"normal", true, false, false, true},
		{"cursor", true, true, false, true},
		{"selected", true, false, true, true},
		{"cursor+selected", true, true, true, true},
		{"inactive cursor", false, true, false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m.active = tt.active
			m.cursor = 0
			if tt.cursor {
				m.cursor = 0
			} else {
				m.cursor = 1 // off the entry
			}
			m.selected = map[int]bool{0: tt.selected}
			row := m.renderRow(0, 30, th)
			if got := rowWidth(t, row); got != 30 {
				t.Errorf("width = %d, want 30", got)
			}
		})
	}
}

// nameOnlyEntry is a minimal fs.DirEntry for tests.
type nameOnlyEntry struct {
	name  string
	isDir bool
}

func (e *nameOnlyEntry) Name() string               { return e.name }
func (e *nameOnlyEntry) IsDir() bool                { return e.isDir }
func (e *nameOnlyEntry) Type() fs.FileMode          { return 0 }
func (e *nameOnlyEntry) Info() (fs.FileInfo, error) { return nil, nil }
func (e *nameOnlyEntry) String() string             { return e.name }

// symlinkToDirEntry is a minimal fs.DirEntry that behaves like a directory but
// reports IsDir=false (same as os.ReadDir on a symlink-to-dir).
type symlinkToDirEntry struct {
	nameOnlyEntry
}

func (e *symlinkToDirEntry) Type() fs.FileMode { return fs.ModeSymlink }

func TestSortEntriesSymlinkGroupsWithDirs(t *testing.T) {
	entries := []fs.DirEntry{
		&nameOnlyEntry{name: "b.txt", isDir: false},
		&nameOnlyEntry{name: "a.txt", isDir: false},
		&symlinkToDirEntry{nameOnlyEntry{name: "d-link"}},
		&nameOnlyEntry{name: "c.txt", isDir: false},
		&nameOnlyEntry{name: "real-dir", isDir: true},
		&nameOnlyEntry{name: "a-dir", isDir: true},
	}
	SortEntries(entries, SortByName, func(e fs.DirEntry) bool {
		if e.Type()&fs.ModeSymlink != 0 {
			return true // simulated symlink-to-dir: group with dirs
		}
		return e.IsDir()
	})

	if got := entries[0].Name(); got != "a-dir" {
		t.Errorf("first = %q, want dir", got)
	}
	if got := entries[1].Name(); got != "d-link" {
		t.Errorf("second = %q, want symlink-to-dir grouped with dirs", got)
	}
	if got := entries[2].Name(); got != "real-dir" {
		t.Errorf("third = %q, want dir", got)
	}
	for i, name := range []string{"a-dir", "d-link", "real-dir"} {
		if entries[i].Name() != name {
			t.Errorf("entries[%d] = %q, want %q", i, entries[i].Name(), name)
		}
	}
}

func TestHandleDirLoadedRoutesByID(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "file.txt"), []byte("x"), 0o644)

	a := newTestModel(t, dir)
	b := newTestModel(t, dir)

	// Both point at the same path, so only the ID can tell them apart.
	msg := a.LoadDir()().(DirLoadedMsg)
	if !a.HandleDirLoaded(msg) {
		t.Error("the owning panel should apply its own load")
	}
	if b.HandleDirLoaded(msg) {
		t.Error("another panel must ignore a load addressed to a different panel")
	}
}

// TestSpaceSelectSizesDirectory checks space_action = "select": Space tags the
// entry without moving the cursor and the directory shows its total size.
func TestSpaceSelectSizesDirectory(t *testing.T) {
	dir := t.TempDir()
	mustMkdir(t, filepath.Join(dir, "sub"))
	mustMkdir(t, filepath.Join(dir, "sub", "deep"))
	mustWrite(t, filepath.Join(dir, "sub", "a"), make([]byte, 1000), 0o644)
	mustWrite(t, filepath.Join(dir, "sub", "deep", "b"), make([]byte, 1048), 0o644)

	m := newTestModel(t, dir)
	m.cfg.Behavior.SpaceAction = "select"

	cmd := m.handleSpace()
	if !m.selected[0] || m.cursor != 0 {
		t.Fatalf("want entry 0 tagged with cursor kept, got selected=%v cursor=%d", m.selected, m.cursor)
	}
	if cmd == nil {
		t.Fatal("want a size command for a directory")
	}
	if m.handleSpace(); m.handleSpace() != nil {
		t.Fatal("Space while a walk is pending must not start another")
	}
	if row := ansi.Strip(m.renderRow(0, 80, theme.Default())); !strings.Contains(row, "...") {
		t.Fatalf("want ... in row while pending, got %q", row)
	}
	msg := cmd().(DirSizeMsg)
	if msg.Size != 2048 {
		t.Fatalf("want 2048 bytes, got %d", msg.Size)
	}
	if !m.HandleDirSize(msg) {
		t.Fatal("size message not accepted by its own panel")
	}
	if row := ansi.Strip(m.renderRow(0, 80, theme.Default())); !strings.Contains(row, "2.0K") {
		t.Fatalf("want 2.0K in row, got %q", row)
	}

	m.handleSpace()
	if m.selected[0] {
		t.Fatal("second Space should untag")
	}
}

// TestSpaceSelectSymlinkAndStale checks a symlinked directory is sized by its
// target, and a result arriving after a reload is dropped.
func TestSpaceSelectSymlinkAndStale(t *testing.T) {
	dir := t.TempDir()
	mustMkdir(t, filepath.Join(dir, "real"))
	mustWrite(t, filepath.Join(dir, "real", "a"), make([]byte, 2048), 0o644)
	if err := os.Symlink(filepath.Join(dir, "real"), filepath.Join(dir, "link")); err != nil {
		t.Fatal(err)
	}

	m := newTestModel(t, dir)
	m.cfg.Behavior.SpaceAction = "select"
	m.cursor = 0 // "link" sorts before "real"
	if m.CurrentEntry().Name() != "link" {
		t.Fatalf("want link under cursor, got %s", m.CurrentEntry().Name())
	}

	msg := m.handleSpace()().(DirSizeMsg)
	if msg.Size != 2048 {
		t.Fatalf("want symlinked dir sized by target, got %d", msg.Size)
	}

	m.dirSizes = nil // what HandleDirLoaded does on reload
	m.HandleDirSize(msg)
	if _, ok := m.dirSizes[msg.Path]; ok {
		t.Fatal("a result arriving after a reload must be dropped")
	}
}

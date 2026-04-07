package panel

import (
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/kooler/MiddayCommander/internal/vfs"
	"github.com/kooler/MiddayCommander/internal/vfs/archive"
)

// KeyMap defines configurable panel keybindings.
type KeyMap struct {
	Up           key.Binding
	Down         key.Binding
	PageUp       key.Binding
	PageDown     key.Binding
	Home         key.Binding
	End          key.Binding
	GoBack       key.Binding
	ToggleSelect key.Binding
	SelectUp     key.Binding
	SelectDown   key.Binding
	QuickSearch  key.Binding
}

// Model represents a single file panel.
type Model struct {
	fs         vfs.FS
	path       string        // absolute path of current directory
	entries    []fs.DirEntry // directory contents (sorted)
	infos      []fs.FileInfo // cached FileInfo for each entry
	cursor     int           // highlighted entry index
	offset     int           // scroll offset for viewport
	selected   map[int]bool  // tagged/selected entries
	sortMode   SortMode
	showHidden bool // whether to show dotfiles
	width      int
	height     int // height available for file list rows
	active     bool
	err        error

	// Quick search state
	searching   bool
	searchQuery string

	// Archive browsing state
	inArchive   bool        // true when browsing inside an archive
	archiveFS   *archive.FS // the archive VFS (nil when not in archive)
	archivePath string      // path within the archive
	realFS      vfs.FS      // the original filesystem (to restore when leaving archive)
	realPath    string      // the directory containing the archive file

	keyMap KeyMap
}

// New creates a new panel browsing the given directory.
func New(filesystem vfs.FS, path string, km KeyMap) Model {
	return Model{
		fs:         filesystem,
		path:       path,
		selected:   make(map[int]bool),
		sortMode:   SortByName,
		showHidden: true,
		keyMap:     km,
	}
}

// ToggleHidden flips whether dotfiles are shown.
func (m *Model) ToggleHidden() {
	m.showHidden = !m.showHidden
}

// ShowHidden returns the current hidden-file visibility state.
func (m Model) ShowHidden() bool {
	return m.showHidden
}

// Path returns the current directory path.
func (m Model) Path() string {
	return m.path
}

// SetPath changes the directory path (call LoadDir after).
func (m *Model) SetPath(path string) {
	// If currently in archive, leave it
	if m.inArchive {
		m.leaveArchive()
	}
	m.path = path
	m.cursor = 0
	m.offset = 0
}

// InArchive returns whether this panel is browsing inside an archive.
func (m Model) InArchive() bool {
	return m.inArchive
}

// ArchiveLabel returns a display string for the archive being browsed, or "".
func (m Model) ArchiveLabel() string {
	if !m.inArchive {
		return ""
	}
	return m.archiveFS.ArchivePath()
}

// SetSize sets the panel dimensions.
func (m *Model) SetSize(w, h int) {
	m.width = w
	m.height = h
}

// SetActive marks this panel as focused/unfocused.
func (m *Model) SetActive(active bool) {
	m.active = active
}

// Active returns whether this panel has focus.
func (m Model) Active() bool {
	return m.active
}

// CurrentEntry returns the entry under the cursor, or nil.
func (m Model) CurrentEntry() fs.DirEntry {
	if m.cursor >= 0 && m.cursor < len(m.entries) {
		return m.entries[m.cursor]
	}
	return nil
}

// CurrentInfo returns the FileInfo of the entry under the cursor, or nil.
func (m Model) CurrentInfo() fs.FileInfo {
	if m.cursor >= 0 && m.cursor < len(m.infos) {
		return m.infos[m.cursor]
	}
	return nil
}

// CurrentPath returns the full path of the entry under the cursor.
// For archive browsing this returns the path within the archive, not a real filesystem path.
func (m Model) CurrentPath() string {
	e := m.CurrentEntry()
	if e == nil {
		return m.path
	}
	if m.inArchive {
		if m.path == "." {
			return e.Name()
		}
		return m.path + "/" + e.Name()
	}
	return filepath.Join(m.path, e.Name())
}

// SelectedPaths returns full paths of all tagged files. If none are tagged, returns the current entry.
func (m Model) SelectedPaths() []string {
	var paths []string
	for i, sel := range m.selected {
		if sel && i < len(m.entries) {
			paths = append(paths, filepath.Join(m.path, m.entries[i].Name()))
		}
	}
	if len(paths) == 0 {
		if e := m.CurrentEntry(); e != nil && e.Name() != ".." {
			paths = append(paths, m.CurrentPath())
		}
	}
	return paths
}

// LoadDir reads the current directory and populates entries.
func (m *Model) LoadDir() tea.Cmd {
	path := m.path
	filesystem := m.fs
	return func() tea.Msg {
		entries, err := readDir(filesystem, path)
		return DirLoadedMsg{Path: path, Entries: entries, Err: err}
	}
}

func readDir(filesystem vfs.FS, path string) ([]fs.DirEntry, error) {
	entries, err := filesystem.ReadDir(path)
	if err != nil {
		return nil, err
	}
	return entries, nil
}

// DirLoadedMsg is sent when a directory listing completes.
type DirLoadedMsg struct {
	Path    string
	Entries []fs.DirEntry
	Err     error
}

// HandleDirLoaded processes a completed directory load.
func (m *Model) HandleDirLoaded(msg DirLoadedMsg) {
	if msg.Err != nil {
		m.err = msg.Err
		return
	}
	if msg.Path != m.path {
		return // stale load
	}

	m.err = nil

	// Prepend ".." entry unless at root
	var all []fs.DirEntry
	if !isRootPath(m.path) {
		all = append(all, parentEntry{})
	}
	for _, e := range msg.Entries {
		if !m.showHidden && strings.HasPrefix(e.Name(), ".") {
			continue
		}
		all = append(all, e)
	}

	SortEntries(all, m.sortMode)
	m.entries = all

	// Cache FileInfo
	m.infos = make([]fs.FileInfo, len(all))
	for i, e := range all {
		info, _ := e.Info()
		m.infos[i] = info
	}

	m.selected = make(map[int]bool)
	if m.cursor >= len(m.entries) {
		m.cursor = max(0, len(m.entries)-1)
	}
	m.clampOffset()
}

// Searching returns whether quick search is active and the current query.
func (m Model) Searching() (bool, string) {
	return m.searching, m.searchQuery
}

// Update handles key events for this panel. Only called when the panel is active.
func (m *Model) Update(msg tea.KeyMsg) tea.Cmd {
	// Quick search mode intercepts keys
	if m.searching {
		return m.updateSearch(msg)
	}

	km := m.keyMap
	switch {
	case key.Matches(msg, km.Up):
		m.moveUp(1)
	case key.Matches(msg, km.Down):
		m.moveDown(1)
	case key.Matches(msg, km.SelectUp):
		m.selectAt(m.cursor)
		m.moveUp(1)
	case key.Matches(msg, km.SelectDown):
		m.selectAt(m.cursor)
		m.moveDown(1)
	case key.Matches(msg, km.PageUp):
		m.moveUp(m.height)
	case key.Matches(msg, km.PageDown):
		m.moveDown(m.height)
	case key.Matches(msg, km.Home):
		m.cursor = 0
		m.offset = 0
	case key.Matches(msg, km.End):
		m.cursor = max(0, len(m.entries)-1)
		m.clampOffset()
	case msg.String() == "enter":
		return m.handleEnter()
	case msg.String() == " ":
		return m.handleSpace()
	case key.Matches(msg, km.GoBack):
		return m.goUp()
	case msg.String() == "insert":
		m.toggleSelect()
		m.moveDown(1)
	case key.Matches(msg, km.ToggleSelect):
		m.toggleSelect()
	case key.Matches(msg, km.QuickSearch):
		m.searching = true
		m.searchQuery = ""
	default:
		// Auto-start search on any printable letter/digit
		s := msg.String()
		if len(s) == 1 && ((s[0] >= 'a' && s[0] <= 'z') || (s[0] >= 'A' && s[0] <= 'Z') || (s[0] >= '0' && s[0] <= '9') || s[0] == '.' || s[0] == '_' || s[0] == '-') {
			m.searching = true
			m.searchQuery = s
			m.jumpToMatch()
		}
	}
	return nil
}

func (m *Model) updateSearch(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "esc":
		m.searching = false
		m.searchQuery = ""
		return nil
	case "backspace":
		if len(m.searchQuery) > 0 {
			m.searchQuery = m.searchQuery[:len(m.searchQuery)-1]
			if m.searchQuery == "" {
				m.searching = false
			} else {
				m.jumpToMatch()
			}
		} else {
			m.searching = false
		}
		return nil
	default:
		s := msg.String()
		if len(s) == 1 && ((s[0] >= 'a' && s[0] <= 'z') || (s[0] >= 'A' && s[0] <= 'Z') || (s[0] >= '0' && s[0] <= '9') || s[0] == '.' || s[0] == '_' || s[0] == '-') {
			m.searchQuery += s
			m.jumpToMatch()
			return nil
		}
		// Any other key: clear search and pass through to normal handling
		m.searching = false
		m.searchQuery = ""
		return m.Update(msg)
	}
}

func (m *Model) jumpToMatch() {
	if m.searchQuery == "" {
		return
	}
	query := strings.ToLower(m.searchQuery)
	// Search forward from cursor
	for i := m.cursor; i < len(m.entries); i++ {
		if strings.HasPrefix(strings.ToLower(m.entries[i].Name()), query) {
			m.cursor = i
			m.clampOffset()
			return
		}
	}
	// Wrap around from beginning
	for i := 0; i < m.cursor; i++ {
		if strings.HasPrefix(strings.ToLower(m.entries[i].Name()), query) {
			m.cursor = i
			m.clampOffset()
			return
		}
	}
}

func (m *Model) moveUp(n int) {
	m.cursor -= n
	if m.cursor < 0 {
		m.cursor = 0
	}
	m.clampOffset()
}

func (m *Model) moveDown(n int) {
	m.cursor += n
	if m.cursor >= len(m.entries) {
		m.cursor = max(0, len(m.entries)-1)
	}
	m.clampOffset()
}

func (m *Model) clampOffset() {
	if m.height <= 0 {
		return
	}
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+m.height {
		m.offset = m.cursor - m.height + 1
	}
}

func (m *Model) handleEnter() tea.Cmd {
	e := m.CurrentEntry()
	if e == nil {
		return nil
	}
	if e.IsDir() {
		return m.enterDir()
	}

	// Check if it's an archive we can browse into (only from real FS, not nested)
	if !m.inArchive {
		fullPath := m.CurrentPath()
		if archive.IsArchive(fullPath) {
			return m.enterArchive(fullPath)
		}
	}

	// Enter on file = open for edit
	path := m.CurrentPath()
	return func() tea.Msg { return OpenFileMsg{Path: path} }
}

func (m *Model) handleSpace() tea.Cmd {
	e := m.CurrentEntry()
	if e == nil || e.IsDir() || m.inArchive {
		return nil
	}
	// Space on file = preview
	path := m.CurrentPath()
	return func() tea.Msg { return PreviewFileMsg{Path: path} }
}

func (m *Model) enterArchive(archivePath string) tea.Cmd {
	afs, err := archive.New(archivePath)
	if err != nil {
		m.err = err
		return nil
	}

	m.realFS = m.fs
	m.realPath = m.path
	m.archiveFS = afs
	m.inArchive = true
	m.fs = afs
	m.path = "."
	m.archivePath = "."
	m.cursor = 0
	m.offset = 0
	return m.LoadDir()
}

func (m *Model) leaveArchive() {
	m.fs = m.realFS
	m.path = m.realPath
	m.inArchive = false
	m.archiveFS = nil
	m.archivePath = ""
	m.realFS = nil
	m.realPath = ""
}

func (m *Model) enterDir() tea.Cmd {
	e := m.CurrentEntry()
	if e == nil {
		return nil
	}
	if !e.IsDir() {
		return nil
	}
	if e.Name() == ".." {
		return m.goUp()
	}

	if m.inArchive {
		if m.path == "." {
			m.path = e.Name()
		} else {
			m.path = m.path + "/" + e.Name()
		}
	} else {
		m.path = filepath.Join(m.path, e.Name())
	}
	m.cursor = 0
	m.offset = 0
	return m.LoadDir()
}

func (m *Model) goUp() tea.Cmd {
	if m.inArchive {
		// Going up within archive
		if m.path == "." || m.path == "" {
			// Leave the archive entirely
			archiveName := filepath.Base(m.archiveFS.ArchivePath())
			m.leaveArchive()
			m.cursor = 0
			m.offset = 0
			return tea.Sequence(m.LoadDir(), func() tea.Msg {
				return RestoreCursorMsg{Name: archiveName}
			})
		}
		// Go up one level within the archive
		oldDir := filepath.Base(m.path)
		parent := filepath.Dir(m.path)
		if parent == "." || parent == "/" {
			m.path = "."
		} else {
			m.path = parent
		}
		m.cursor = 0
		m.offset = 0
		return tea.Sequence(m.LoadDir(), func() tea.Msg {
			return RestoreCursorMsg{Name: oldDir}
		})
	}

	if isRootPath(m.path) {
		return nil
	}
	oldDir := filepath.Base(m.path)
	m.path = filepath.Dir(m.path)
	m.cursor = 0
	m.offset = 0

	return tea.Sequence(m.LoadDir(), func() tea.Msg {
		return RestoreCursorMsg{Name: oldDir}
	})
}

// RestoreCursorMsg requests placing the cursor on a named entry after navigation.
type RestoreCursorMsg struct {
	Name string
}

// OpenFileMsg is sent when the user wants to open a file (Enter on file).
type OpenFileMsg struct {
	Path string
}

// PreviewFileMsg is sent when the user wants to preview a file (Space on file).
type PreviewFileMsg struct {
	Path string
}

// RestoreCursor places cursor on the named entry (used after going up).
func (m *Model) RestoreCursor(name string) {
	for i, e := range m.entries {
		if e.Name() == name {
			m.cursor = i
			m.clampOffset()
			return
		}
	}
}

func (m *Model) toggleSelect() {
	if m.cursor >= 0 && m.cursor < len(m.entries) && m.entries[m.cursor].Name() != ".." {
		m.selected[m.cursor] = !m.selected[m.cursor]
	}
}

func (m *Model) selectAt(idx int) {
	if idx >= 0 && idx < len(m.entries) && m.entries[idx].Name() != ".." {
		m.selected[idx] = true
	}
}

// ChangeSortMode cycles to the next sort mode and re-sorts.
func (m *Model) ChangeSortMode() {
	m.sortMode = (m.sortMode + 1) % 4
	SortEntries(m.entries, m.sortMode)
}

func isRootPath(path string) bool {
	if path == string(filepath.Separator) {
		return true
	}
	if len(path) == 3 && path[1] == ':' {
		return true
	}
	return false
}

// parentEntry is a synthetic ".." directory entry.
type parentEntry struct{}

func (parentEntry) Name() string               { return ".." }
func (parentEntry) IsDir() bool                { return true }
func (parentEntry) Type() fs.FileMode          { return fs.ModeDir }
func (parentEntry) Info() (fs.FileInfo, error) { return nil, nil }

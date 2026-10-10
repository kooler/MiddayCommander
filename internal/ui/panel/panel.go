package panel

import (
	"io/fs"
	"path/filepath"
	"strings"
	"sync/atomic"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/kooler/MiddayCommander/internal/config"
	"github.com/kooler/MiddayCommander/internal/ui/overlay"
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

// nextSeq numbers panel instances so a load can be routed back to the panel
// that requested it.
var nextSeq atomic.Uint64

// Model represents a single file panel.
type Model struct {
	// id travels with LoadDir so the app can route the result back here.
	id uint64

	// stack is the chain of locations descended into, the last being on
	// screen. An archive or server pushes; ".." at a root pops.
	stack      []vfs.Location
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

	cfg config.Config // application config

	// Quick search state
	searching   bool
	searchQuery string

	keyMap KeyMap
}

// New creates a new panel browsing the given directory.
func New(filesystem vfs.FS, path string, km KeyMap, cfg config.Config) Model {
	return Model{
		id:         nextSeq.Add(1),
		stack:      []vfs.Location{vfs.NewLocation(filesystem, path, vfs.KindLocal, "")},
		selected:   make(map[int]bool),
		sortMode:   SortByName,
		showHidden: cfg.Behavior.ShowHidden == nil || *cfg.Behavior.ShowHidden,
		keyMap:     km,
		cfg:        cfg,
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

func (m Model) Location() vfs.Location {
	return m.stack[len(m.stack)-1]
}

func (m *Model) setLocation(loc vfs.Location) {
	m.stack[len(m.stack)-1] = loc
}

func (m *Model) push(loc vfs.Location) {
	m.stack = append(m.stack, loc)
	m.cursor = 0
	m.offset = 0
}

// pop leaves the innermost location and names the entry to put the cursor
// back on. No-op at the outermost.
func (m *Model) pop() string {
	if len(m.stack) < 2 {
		return ""
	}
	inner := m.stack[len(m.stack)-1]
	m.stack = m.stack[:len(m.stack)-1]
	m.cursor = 0
	m.offset = 0
	return vfs.BasePath(m.Location().Kind, inner.Origin)
}

func (m Model) Path() string {
	return m.Location().Path
}

func (m Model) Ref() vfs.FileRef {
	return m.Location().Ref()
}

// UsesFS tells the app whether this panel still needs a shared connection.
func (m Model) UsesFS(fsys vfs.FS) bool {
	for _, loc := range m.stack {
		if loc.FS == fsys {
			return true
		}
	}
	return false
}

// LocalPath is where the panel sits on this machine, even while showing a
// server.
func (m Model) LocalPath() string {
	for i := len(m.stack) - 1; i >= 0; i-- {
		if m.stack[i].IsLocal() {
			return m.stack[i].Path
		}
	}
	return m.stack[0].Path
}

func (m Model) IsLocal() bool {
	return m.Location().IsLocal()
}

// SetPath leaves any archive or server first.
func (m *Model) SetPath(path string) {
	m.stack = m.stack[:1]
	m.setLocation(m.stack[0].WithPath(path))
	m.cursor = 0
	m.offset = 0
}

// SetLocation descends from the outermost level, for opening a server.
func (m *Model) SetLocation(loc vfs.Location) {
	m.stack = m.stack[:1]
	m.push(loc)
}

// InArchive returns whether this panel is browsing inside an archive.
func (m Model) InArchive() bool {
	return m.Location().Kind == vfs.KindArchive
}

// IsSymlinkToDir returns whether e is a symlink that resolves to a directory.
func (m Model) IsSymlinkToDir(e fs.DirEntry) bool {
	if m.InArchive() || e.Type()&fs.ModeSymlink == 0 {
		return false
	}
	info, err := m.Location().FS.Stat(m.CurrentPath())
	return err == nil && info.IsDir()
}

// IsDirLike reports whether e behaves like a directory (real dir or symlink-to-dir).
func IsDirLike(m *Model, e fs.DirEntry) bool {
	return e.IsDir() || m.IsSymlinkToDir(e)
}

// LocationLabel is the display string for a nested location, or "".
func (m Model) LocationLabel() string {
	return m.Location().Label
}

// SetSize sets the panel dimensions.
func (m *Model) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.clampOffset()
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

// CurrentPath is within the panel's own filesystem, not necessarily this
// machine.
func (m Model) CurrentPath() string {
	e := m.CurrentEntry()
	if e == nil {
		return m.Path()
	}
	return m.Location().Join(e.Name())
}

func (m Model) CurrentRef() vfs.FileRef {
	e := m.CurrentEntry()
	if e == nil {
		return m.Ref()
	}
	return m.Location().Child(e.Name())
}

// SelectedRefs returns the tagged entries, or the one under the cursor.
func (m Model) SelectedRefs() []vfs.FileRef {
	loc := m.Location()
	var out []vfs.FileRef
	for i, sel := range m.selected {
		if sel && i < len(m.entries) && m.entries[i].Name() != ".." {
			out = append(out, loc.Child(m.entries[i].Name()))
		}
	}
	if len(out) == 0 {
		if e := m.CurrentEntry(); e != nil && e.Name() != ".." {
			out = append(out, m.CurrentRef())
		}
	}
	return out
}

// LoadDir reads the current directory and populates entries.
func (m *Model) LoadDir() tea.Cmd {
	path := m.Path()
	filesystem := m.Location().FS
	id := m.id
	return func() tea.Msg {
		entries, err := readDir(filesystem, path)
		return DirLoadedMsg{ID: id, Path: path, Entries: entries, Err: err}
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
	ID      uint64 // the panel that asked for the load
	Path    string
	Entries []fs.DirEntry
	Err     error
}

// HandleDirLoaded applies a completed load, reporting whether it belonged to
// this panel.
func (m *Model) HandleDirLoaded(msg DirLoadedMsg) bool {
	if msg.ID != m.id {
		return false // belongs to another panel
	}
	if msg.Err != nil {
		m.err = msg.Err
		return true
	}
	if msg.Path != m.Path() {
		return true // stale load
	}

	m.err = nil

	// Prepend ".." unless there is nowhere to go: at the root of a nested
	// location it leaves that location rather than the filesystem.
	var all []fs.DirEntry
	if !m.Location().IsRoot() || len(m.stack) > 1 {
		all = append(all, parentEntry{})
	}
	for _, e := range msg.Entries {
		if !m.showHidden && strings.HasPrefix(e.Name(), ".") {
			continue
		}
		all = append(all, e)
	}

	SortEntries(all, m.sortMode, func(e fs.DirEntry) bool { return IsDirLike(m, e) })
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
	return true
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
		m.toggleSelect()
		m.moveUp(1)
	case key.Matches(msg, km.SelectDown):
		m.toggleSelect()
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
	m.offset = overlay.ClampScroll(m.cursor, m.offset, m.height, len(m.entries))
}

func (m *Model) handleEnter() tea.Cmd {
	e := m.CurrentEntry()
	if e == nil {
		return nil
	}
	if e.IsDir() || m.IsSymlinkToDir(e) {
		return m.enterDir()
	}

	// Only a real file on this machine can be browsed as an archive; a
	// remote one would have to be downloaded first.
	if m.IsLocal() {
		fullPath := m.CurrentPath()
		if archive.IsArchive(fullPath) {
			return m.enterArchive(fullPath)
		}
	}

	// Enter on file
	path := m.CurrentPath()
	info := m.CurrentInfo()
	if info != nil && isExecutable(info.Mode()) && m.cfg.Behavior.EnterAction == "execute" {
		return func() tea.Msg { return ExecuteFileMsg{Path: path} }
	}
	return func() tea.Msg { return OpenFileMsg{Path: path} }
}

func (m *Model) handleSpace() tea.Cmd {
	e := m.CurrentEntry()
	if e == nil || IsDirLike(m, e) || !m.IsLocal() {
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

	m.push(vfs.Location{
		FS:     afs,
		Path:   vfs.KindArchive.RootPath(),
		Kind:   vfs.KindArchive,
		Label:  filepath.Base(archivePath) + "://",
		Origin: archivePath,
	})
	return m.LoadDir()
}

func (m *Model) enterDir() tea.Cmd {
	e := m.CurrentEntry()
	if e == nil {
		return nil
	}
	if !e.IsDir() && !m.IsSymlinkToDir(e) {
		return nil
	}
	if e.Name() == ".." {
		return m.goUp()
	}

	loc := m.Location()
	m.setLocation(loc.WithPath(loc.Join(e.Name())))
	m.cursor = 0
	m.offset = 0
	return m.LoadDir()
}

func (m *Model) goUp() tea.Cmd {
	loc := m.Location()

	// At the top of a nested location, ".." leaves it.
	if loc.IsRoot() {
		name := m.pop()
		if name == "" {
			return nil // already at the outermost root
		}
		return tea.Sequence(m.LoadDir(), func() tea.Msg {
			return RestoreCursorMsg{ID: m.id, Name: name}
		})
	}

	oldDir := loc.Base()
	m.setLocation(loc.WithPath(loc.Parent()))
	m.cursor = 0
	m.offset = 0

	return tea.Sequence(m.LoadDir(), func() tea.Msg {
		return RestoreCursorMsg{ID: m.id, Name: oldDir}
	})
}

// RestoreCursorMsg places the cursor on a named entry after navigation.
type RestoreCursorMsg struct {
	ID   uint64
	Name string
}

// OpenFileMsg is sent when the user wants to open a file (Enter on file).
type OpenFileMsg struct {
	Path string
}

// ExecuteFileMsg is sent when the user wants to execute a file (Enter on executable file).
type ExecuteFileMsg struct {
	Path string
}

// PreviewFileMsg is sent when the user wants to preview a file (Space on file).
type PreviewFileMsg struct {
	Path string
}

func (m Model) Owns(id uint64) bool {
	return id != 0 && id == m.id
}

func (m Model) KeyMap() KeyMap {
	return m.keyMap
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

// SelectByPattern marks every non-".." entry whose Name() matches the
// shell glob (filepath.Match). Returns an error if the pattern is invalid.
func (m *Model) SelectByPattern(pattern string) error {
	for i, e := range m.entries {
		if e.Name() == ".." {
			continue
		}
		ok, err := filepath.Match(pattern, e.Name())
		if err != nil {
			return err
		}
		if ok {
			m.selected[i] = true
		}
	}
	return nil
}

// DeselectByPattern clears selection on every non-".." entry whose Name()
// matches the shell glob.
func (m *Model) DeselectByPattern(pattern string) error {
	for i, e := range m.entries {
		if e.Name() == ".." {
			continue
		}
		ok, err := filepath.Match(pattern, e.Name())
		if err != nil {
			return err
		}
		if ok {
			delete(m.selected, i)
		}
	}
	return nil
}

// InvertSelection flips selection state for every non-".." entry.
func (m *Model) InvertSelection() {
	for i, e := range m.entries {
		if e.Name() == ".." {
			continue
		}
		if m.selected[i] {
			delete(m.selected, i)
		} else {
			m.selected[i] = true
		}
	}
}

// ChangeSortMode cycles to the next sort mode and re-sorts.
func (m *Model) ChangeSortMode() {
	m.sortMode = (m.sortMode + 1) % 4
	SortEntries(m.entries, m.sortMode, func(e fs.DirEntry) bool { return IsDirLike(m, e) })
}

// parentEntry is a synthetic ".." directory entry.
type parentEntry struct{}

func (parentEntry) Name() string               { return ".." }
func (parentEntry) IsDir() bool                { return true }
func (parentEntry) Type() fs.FileMode          { return fs.ModeDir }
func (parentEntry) Info() (fs.FileInfo, error) { return nil, nil }

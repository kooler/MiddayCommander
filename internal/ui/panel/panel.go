package panel

import (
	"context"
	"path"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	midfs "github.com/kooler/MiddayCommander/internal/fs"
	archivefs "github.com/kooler/MiddayCommander/internal/fs/archive"
)

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

type Model struct {
	router   *midfs.Router
	dir      midfs.URI
	entries  []midfs.Entry
	cursor   int
	offset   int
	selected map[int]bool
	sortMode SortMode
	width    int
	height   int
	active   bool
	err      error

	searching   bool
	searchQuery string

	keyMap KeyMap
}

func New(router *midfs.Router, dir midfs.URI, km KeyMap) Model {
	return Model{
		router:   router,
		dir:      router.Clean(dir),
		selected: make(map[int]bool),
		sortMode: SortByName,
		keyMap:   km,
	}
}

func (m Model) URI() midfs.URI {
	return m.dir
}

func (m Model) DisplayPath() string {
	return m.dir.Display()
}

func (m Model) LocalPath() (string, bool) {
	if m.dir.Scheme != midfs.SchemeFile {
		return "", false
	}
	return m.dir.Path, true
}

func (m *Model) SetURI(uri midfs.URI) {
	m.dir = m.router.Clean(uri)
	m.cursor = 0
	m.offset = 0
}

func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func (m *Model) SetActive(active bool) {
	m.active = active
}

func (m Model) Active() bool {
	return m.active
}

func (m Model) CurrentEntry() *midfs.Entry {
	if m.cursor >= 0 && m.cursor < len(m.entries) {
		entry := m.entries[m.cursor]
		return &entry
	}
	return nil
}

func (m Model) CurrentURI() midfs.URI {
	entry := m.CurrentEntry()
	if entry == nil {
		return m.dir
	}
	return entry.URI
}

func (m Model) SelectedURIs() []midfs.URI {
	var uris []midfs.URI
	for index, selected := range m.selected {
		if !selected || index >= len(m.entries) {
			continue
		}
		if m.entries[index].Name == ".." {
			continue
		}
		uris = append(uris, m.entries[index].URI)
	}
	if len(uris) == 0 {
		entry := m.CurrentEntry()
		if entry != nil && entry.Name != ".." {
			uris = append(uris, entry.URI)
		}
	}
	return uris
}

func (m *Model) LoadDir() tea.Cmd {
	dir := m.dir
	router := m.router
	return func() tea.Msg {
		entries, err := router.List(context.Background(), dir)
		return DirLoadedMsg{URI: dir, Entries: entries, Err: err}
	}
}

type DirLoadedMsg struct {
	URI     midfs.URI
	Entries []midfs.Entry
	Err     error
}

func (m *Model) HandleDirLoaded(msg DirLoadedMsg) {
	if msg.Err != nil {
		m.err = msg.Err
		return
	}
	if msg.URI.String() != m.dir.String() {
		return
	}

	m.err = nil
	all := make([]midfs.Entry, 0, len(msg.Entries)+1)
	parent := m.router.Parent(m.dir)
	if parent.String() != m.dir.String() {
		all = append(all, midfs.Entry{
			Name:     "..",
			Path:     parent.Path,
			URI:      parent,
			Type:     midfs.EntryDir,
			Readable: true,
			Writable: false,
		})
	}
	all = append(all, msg.Entries...)

	SortEntries(all, m.sortMode)
	m.entries = all
	m.selected = make(map[int]bool)
	if m.cursor >= len(m.entries) {
		m.cursor = max(0, len(m.entries)-1)
	}
	m.clampOffset()
}

func (m Model) Searching() (bool, string) {
	return m.searching, m.searchQuery
}

func (m *Model) Update(msg tea.KeyMsg) tea.Cmd {
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
	for index := m.cursor; index < len(m.entries); index++ {
		if strings.HasPrefix(strings.ToLower(m.entries[index].Name), query) {
			m.cursor = index
			m.clampOffset()
			return
		}
	}
	for index := 0; index < m.cursor; index++ {
		if strings.HasPrefix(strings.ToLower(m.entries[index].Name), query) {
			m.cursor = index
			m.clampOffset()
			return
		}
	}
}

func (m *Model) moveUp(lines int) {
	m.cursor -= lines
	if m.cursor < 0 {
		m.cursor = 0
	}
	m.clampOffset()
}

func (m *Model) moveDown(lines int) {
	m.cursor += lines
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
	entry := m.CurrentEntry()
	if entry == nil {
		return nil
	}
	if entry.Name == ".." {
		return m.goUp()
	}
	if entry.IsDir() {
		m.dir = entry.URI
		m.cursor = 0
		m.offset = 0
		return m.LoadDir()
	}
	if entry.URI.Scheme == midfs.SchemeFile && (entry.IsArchive || archivefs.IsArchive(entry.URI.Path)) {
		m.dir = midfs.NewArchiveURI(entry.URI.Path, ".")
		m.cursor = 0
		m.offset = 0
		return m.LoadDir()
	}
	return func() tea.Msg { return OpenFileMsg{URI: entry.URI} }
}

func (m *Model) handleSpace() tea.Cmd {
	entry := m.CurrentEntry()
	if entry == nil || entry.IsDir() || entry.Name == ".." {
		return nil
	}
	return func() tea.Msg { return PreviewFileMsg{URI: entry.URI} }
}

func (m *Model) goUp() tea.Cmd {
	parent := m.router.Parent(m.dir)
	if parent.String() == m.dir.String() {
		return nil
	}

	restoreName := currentLocationName(m.dir)
	m.dir = parent
	m.cursor = 0
	m.offset = 0
	return tea.Sequence(m.LoadDir(), func() tea.Msg {
		return RestoreCursorMsg{Name: restoreName}
	})
}

type RestoreCursorMsg struct {
	Name string
}

type OpenFileMsg struct {
	URI midfs.URI
}

type PreviewFileMsg struct {
	URI midfs.URI
}

func (m *Model) RestoreCursor(name string) {
	for index, entry := range m.entries {
		if entry.Name == name {
			m.cursor = index
			m.clampOffset()
			return
		}
	}
}

func (m *Model) toggleSelect() {
	if m.cursor >= 0 && m.cursor < len(m.entries) && m.entries[m.cursor].Name != ".." {
		m.selected[m.cursor] = !m.selected[m.cursor]
	}
}

func (m *Model) selectAt(index int) {
	if index >= 0 && index < len(m.entries) && m.entries[index].Name != ".." {
		m.selected[index] = true
	}
}

func (m *Model) ChangeSortMode() {
	m.sortMode = (m.sortMode + 1) % 4
	SortEntries(m.entries, m.sortMode)
}

func currentLocationName(uri midfs.URI) string {
	switch uri.Scheme {
	case midfs.SchemeArchive:
		entry := uri.QueryValue("entry")
		if entry == "" || entry == "." {
			return filepath.Base(uri.Path)
		}
		return path.Base(entry)
	case midfs.SchemeFile:
		return filepath.Base(uri.Path)
	default:
		return midfs.Base(uri)
	}
}

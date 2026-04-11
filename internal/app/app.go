package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/kooler/MiddayCommander/internal/bookmark"
	"github.com/kooler/MiddayCommander/internal/config"
	"github.com/kooler/MiddayCommander/internal/ui/bookmarks"
	"github.com/kooler/MiddayCommander/internal/ui/cmdexec"
	"github.com/kooler/MiddayCommander/internal/ui/dialog"
	"github.com/kooler/MiddayCommander/internal/ui/fuzzy"
	"github.com/kooler/MiddayCommander/internal/ui/help"
	"github.com/kooler/MiddayCommander/internal/ui/menubar"
	"github.com/kooler/MiddayCommander/internal/ui/overlay"
	"github.com/kooler/MiddayCommander/internal/ui/panel"
	"github.com/kooler/MiddayCommander/internal/ui/theme"
	"github.com/kooler/MiddayCommander/internal/ui/themepicker"
	"github.com/kooler/MiddayCommander/internal/vfs/local"
)

// FocusTarget tracks which panel has focus.
type FocusTarget int

const (
	FocusLeft FocusTarget = iota
	FocusRight
)

// Dialog tags identify which operation triggered the dialog.
const (
	tagCopy   = "copy"
	tagMove   = "move"
	tagDelete = "delete"
	tagMkdir  = "mkdir"
	tagRename = "rename"
	tagGoTo   = "goto"
)

// Model is the root application model.
type Model struct {
	leftPanel  panel.Model
	rightPanel panel.Model
	focus      FocusTarget
	keyMap     KeyMap
	theme      theme.Theme
	cfg        config.Config
	menuItems  []menubar.Item
	width      int
	height     int

	// Overlays
	dialog      *dialog.Model
	fuzzy       *fuzzy.Model
	bookmarks   *bookmarks.Model
	help        *help.Model
	themePicker *themepicker.Model
	cmdExec     *cmdexec.Model

	// Saved theme for reverting on Esc in theme picker
	themeBeforePick theme.Theme

	// Bookmark store
	bookmarkStore *bookmark.Store

	// Pending operation state (saved while dialog is open)
	pendingSources []string
	pendingDest    string

	// Double-Esc to quit
	lastEsc time.Time

	// Shift F-key menu bar
	shiftMenuItems []menubar.Item
	shiftHeld      bool
}

// New creates a new application model.
func New() Model {
	cfg := config.Load()

	home, err := os.UserHomeDir()
	if err != nil {
		home = string(filepath.Separator)
	}

	cwd, err := os.Getwd()
	if err != nil {
		cwd = home
	}

	lfs := local.New(string(filepath.Separator))

	panelKM := panelKeyMapFromConfig(cfg.Keys)

	left := panel.New(lfs, cwd, panelKM)
	left.SetActive(true)

	right := panel.New(lfs, home, panelKM)

	th := theme.Default()
	if cfg.Theme != "" {
		if loaded, err := theme.LoadByName(cfg.Theme); err == nil {
			th = loaded
		}
	}

	return Model{
		leftPanel:      left,
		rightPanel:     right,
		focus:          FocusLeft,
		keyMap:         KeyMapFromConfig(cfg.Keys),
		theme:          th,
		cfg:            cfg,
		menuItems:      menubar.DefaultItems(cfg),
		shiftMenuItems: menubar.ShiftItems(cfg),
		bookmarkStore:  bookmark.LoadStore(),
	}
}

func panelKeyMapFromConfig(keys config.KeyBindings) panel.KeyMap {
	return panel.KeyMap{
		Up:           binding(keys.Up, "up"),
		Down:         binding(keys.Down, "down"),
		PageUp:       binding(keys.PageUp, "page up"),
		PageDown:     binding(keys.PageDown, "page down"),
		Home:         binding(keys.Home, "home"),
		End:          binding(keys.End, "end"),
		GoBack:       binding(keys.GoBack, "go back"),
		ToggleSelect: binding(keys.ToggleSelect, "toggle select"),
		SelectUp:     binding(keys.SelectUp, "select up"),
		SelectDown:   binding(keys.SelectDown, "select down"),
		QuickSearch:  binding(keys.QuickSearch, "quick search"),
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.leftPanel.LoadDir(),
		m.rightPanel.LoadDir(),
	)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.recalcLayout()
		return m, nil

	case panel.DirLoadedMsg:
		m.leftPanel.HandleDirLoaded(msg)
		m.rightPanel.HandleDirLoaded(msg)
		return m, nil

	case panel.RestoreCursorMsg:
		m.activePanel().RestoreCursor(msg.Name)
		return m, nil

	// Help messages
	case help.DismissMsg:
		m.help = nil
		return m, nil

	// Theme picker messages
	case themepicker.RemoteThemesMsg:
		if m.themePicker != nil {
			m.themePicker.HandleRemote(msg)
		}
		return m, nil

	case themepicker.PreviewMsg:
		m.theme = msg.Theme
		return m, nil

	case themepicker.SelectMsg:
		m.theme = msg.Theme
		m.themePicker = nil
		// Save remote theme file locally so it's available on next startup.
		if msg.Source == theme.SourceRemote && len(msg.RawTOML) > 0 {
			dir := theme.ThemesDir()
			_ = os.MkdirAll(dir, 0o755)
			_ = os.WriteFile(filepath.Join(dir, msg.Key+".toml"), msg.RawTOML, 0o644)
		}
		_ = config.SaveTheme(msg.Key)
		return m, nil

	case themepicker.DismissMsg:
		m.theme = m.themeBeforePick
		m.themePicker = nil
		return m, nil

	// Bookmark messages
	case bookmarks.SelectMsg:
		m.bookmarks = nil
		m.activePanel().SetPath(msg.Path)
		return m, m.activePanel().LoadDir()

	case bookmarks.DismissMsg:
		m.bookmarks = nil
		return m, nil

	// Fuzzy finder internal messages — route to fuzzy model
	case fuzzy.FileWalkMsg:
		if m.fuzzy != nil {
			newFuzzy, cmd := m.fuzzy.Update(msg)
			m.fuzzy = &newFuzzy
			return m, cmd
		}
		return m, nil

	// Fuzzy finder result messages
	case fuzzy.ResultMsg:
		m.fuzzy = nil
		// Navigate to the selected path
		info, err := os.Stat(msg.Path)
		if err != nil {
			return m, nil
		}
		if info.IsDir() {
			m.activePanel().SetPath(msg.Path)
		} else {
			m.activePanel().SetPath(filepath.Dir(msg.Path))
		}
		return m, m.activePanel().LoadDir()

	case fuzzy.DismissMsg:
		m.fuzzy = nil
		return m, nil

	// Command execution messages
	case cmdexec.CommandDoneMsg:
		if m.cmdExec != nil {
			newCE, cmd := m.cmdExec.Update(msg)
			m.cmdExec = &newCE
			return m, cmd
		}
		return m, nil

	case cmdexec.DismissMsg:
		m.cmdExec = nil
		return m, m.refreshBothPanels()

	// File action messages from panel (configurable behavior)
	case panel.OpenFileMsg:
		return m, m.fileActionCmd(msg.Path, m.cfg.Behavior.EnterAction)

	case panel.PreviewFileMsg:
		return m, m.fileActionCmd(msg.Path, m.cfg.Behavior.SpaceAction)

	// File operation results
	case copyDoneMsg:
		m.dialog = nil
		if msg.err != nil {
			return m.showError("Copy Error", msg.err)
		}
		return m, m.refreshBothPanels()

	case moveDoneMsg:
		m.dialog = nil
		if msg.err != nil {
			return m.showError("Move Error", msg.err)
		}
		return m, m.refreshBothPanels()

	case deleteDoneMsg:
		m.dialog = nil
		if msg.err != nil {
			return m.showError("Delete Error", msg.err)
		}
		return m, m.refreshBothPanels()

	case mkdirDoneMsg:
		if msg.err != nil {
			return m.showError("Mkdir Error", msg.err)
		}
		return m, m.activePanel().LoadDir()

	case renameDoneMsg:
		if msg.err != nil {
			return m.showError("Rename Error", msg.err)
		}
		return m, m.activePanel().LoadDir()

	case externalDoneMsg:
		return m, m.refreshBothPanels()

	case dialog.Result:
		return m.handleDialogResult(msg)

	case ShiftPressMsg:
		m.shiftHeld = true
		return m, nil

	case ShiftReleaseMsg:
		m.shiftHeld = false
		return m, nil

	case tea.MouseMsg:
		// Track shift modifier from mouse events (primary shift detection).
		m.shiftHeld = msg.Shift

		if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
			// Click on menu bar (last row)
			if msg.Y == m.height-1 {
				items := m.menuItems
				if m.shiftHeld {
					items = m.shiftMenuItems
				}
				raw := menubar.HandleClick(msg.X, m.width, items)
				if raw != "" {
					return m.dispatchKey(raw)
				}
			}
		}
		return m, nil

	case tea.KeyMsg:
		// Track shift state for menu bar display.
		// Detect shift from F13-F20 (shift+F1..F8) or any "shift+…" key name.
		m.shiftHeld = hasShiftModifier(msg)

		// Help overlay gets priority
		if m.help != nil {
			newHelp, cmd := m.help.Update(msg)
			m.help = &newHelp
			return m, cmd
		}

		// Bookmarks overlay gets priority
		if m.bookmarks != nil {
			newBM, cmd := m.bookmarks.Update(msg)
			m.bookmarks = &newBM
			return m, cmd
		}

		// Theme picker gets priority when active
		if m.themePicker != nil {
			newTP, cmd := m.themePicker.Update(msg)
			m.themePicker = &newTP
			return m, cmd
		}

		// Fuzzy finder gets priority when active
		if m.fuzzy != nil {
			newFuzzy, cmd := m.fuzzy.Update(msg)
			m.fuzzy = &newFuzzy
			return m, cmd
		}

		// Command execution gets priority when active
		if m.cmdExec != nil {
			newCE, cmd := m.cmdExec.Update(msg)
			m.cmdExec = &newCE
			return m, cmd
		}

		// Dialog gets priority
		if m.dialog != nil {
			m.dialog.Update(msg)
			if m.dialog.Done() {
				result := m.dialog.GetResult()
				m.dialog = nil
				return m.handleDialogResult(result)
			}
			return m, nil
		}

		// Double-Esc to quit
		if msg.String() == "esc" {
			now := time.Now()
			if now.Sub(m.lastEsc) < 400*time.Millisecond {
				return m, tea.Quit
			}
			m.lastEsc = now
			return m, nil
		}

		// Global keybindings
		switch {
		case key.Matches(msg, m.keyMap.Quit):
			return m, tea.Quit

		case key.Matches(msg, m.keyMap.TogglePanel):
			m.toggleFocus()
			return m, nil

		case key.Matches(msg, m.keyMap.SwapPanels):
			m.leftPanel, m.rightPanel = m.rightPanel, m.leftPanel
			m.recalcLayout()
			return m, nil

		case key.Matches(msg, m.keyMap.Copy):
			return m.startCopy()

		case key.Matches(msg, m.keyMap.Move):
			return m.startMove()

		case key.Matches(msg, m.keyMap.Delete):
			return m.startDelete()

		case key.Matches(msg, m.keyMap.Mkdir):
			return m.startMkdir()

		case key.Matches(msg, m.keyMap.Rename):
			return m.startRename()

		case key.Matches(msg, m.keyMap.View):
			return m.startView()

		case key.Matches(msg, m.keyMap.Edit):
			return m.startEdit()

		case key.Matches(msg, m.keyMap.GoTo):
			return m.startGoTo()

		case key.Matches(msg, m.keyMap.FuzzyFind):
			return m.startFuzzyFind()

		case key.Matches(msg, m.keyMap.Bookmarks):
			return m.startBookmarks()

		case key.Matches(msg, m.keyMap.Help):
			return m.startHelp()

		case key.Matches(msg, m.keyMap.ThemePicker):
			return m.startThemePicker()

		case key.Matches(msg, m.keyMap.CmdExec):
			return m.startCmdExec()
		}

		// Delegate to active panel
		cmd := m.activePanel().Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}

	leftView := m.leftPanel.View(m.theme)
	rightView := m.rightPanel.View(m.theme)
	panels := lipgloss.JoinHorizontal(lipgloss.Top, leftView, rightView)

	items := m.menuItems
	if m.shiftHeld {
		items = m.shiftMenuItems
	}
	fkeyView := menubar.View(m.theme, m.width, items)

	screen := lipgloss.JoinVertical(lipgloss.Left, panels, fkeyView)

	if m.help != nil {
		box := m.help.View(m.theme, m.width, m.height)
		bw, bh := m.help.BoxSize(m.width, m.height)
		screen = overlay.Place(screen, box, m.width, m.height, bw, bh)
	} else if m.bookmarks != nil {
		box := m.bookmarks.View(m.theme, m.width, m.height)
		bw, bh := m.bookmarks.BoxSize(m.width, m.height)
		screen = overlay.Place(screen, box, m.width, m.height, bw, bh)
	} else if m.themePicker != nil {
		box := m.themePicker.View(m.theme, m.width, m.height)
		bw, bh := m.themePicker.BoxSize(m.width, m.height)
		screen = overlay.Place(screen, box, m.width, m.height, bw, bh)
	} else if m.fuzzy != nil {
		box := m.fuzzy.View(m.theme, m.width, m.height)
		bw, bh := m.fuzzy.BoxSize(m.width, m.height)
		screen = overlay.Place(screen, box, m.width, m.height, bw, bh)
	} else if m.cmdExec != nil {
		box := m.cmdExec.View(m.theme, m.width, m.height)
		bw, bh := m.cmdExec.BoxSize(m.width, m.height)
		screen = overlay.Place(screen, box, m.width, m.height, bw, bh)
	} else if m.dialog != nil {
		box := m.dialog.View(m.theme, m.width, m.height)
		bw, bh := m.dialog.BoxSize(m.width, m.height)
		screen = overlay.Place(screen, box, m.width, m.height, bw, bh)
	}

	return screen
}

// dispatchKey executes the action bound to a raw key string (used for menu bar clicks).
func (m Model) dispatchKey(raw string) (tea.Model, tea.Cmd) {
	cfg := m.cfg.Keys
	switch {
	case contains(cfg.Quit, raw):
		return m, tea.Quit
	case contains(cfg.Copy, raw):
		return m.startCopy()
	case contains(cfg.Move, raw):
		return m.startMove()
	case contains(cfg.Delete, raw):
		return m.startDelete()
	case contains(cfg.Mkdir, raw):
		return m.startMkdir()
	case contains(cfg.Rename, raw):
		return m.startRename()
	case contains(cfg.View, raw):
		return m.startView()
	case contains(cfg.Edit, raw):
		return m.startEdit()
	case contains(cfg.GoTo, raw):
		return m.startGoTo()
	case contains(cfg.Help, raw):
		return m.startHelp()
	case contains(cfg.Bookmarks, raw):
		return m.startBookmarks()
	case contains(cfg.FuzzyFind, raw):
		return m.startFuzzyFind()
	case contains(cfg.ThemePicker, raw):
		return m.startThemePicker()
	case contains(cfg.CmdExec, raw):
		return m.startCmdExec()
	}
	return m, nil
}

func isShiftFKey(msg tea.KeyMsg) bool {
	// KeyType uses negative iota: KeyF13 (-46) > KeyF20 (-53).
	return msg.Type <= tea.KeyF13 && msg.Type >= tea.KeyF20
}

func hasShiftModifier(msg tea.KeyMsg) bool {
	return isShiftFKey(msg) || strings.Contains(msg.String(), "shift+")
}

func contains(keys config.StringOrList, val string) bool {
	for _, k := range keys {
		if k == val {
			return true
		}
	}
	return false
}

// fileActionCmd maps a configurable action name to the appropriate command.
func (m *Model) fileActionCmd(path string, action string) tea.Cmd {
	switch action {
	case "edit":
		return editFileCmd(path)
	case "preview":
		return viewFileCmd(path)
	default:
		return editFileCmd(path)
	}
}

// --- File operation starters ---

func (m Model) startCopy() (tea.Model, tea.Cmd) {
	sources := m.selectedOrCurrent()
	if len(sources) == 0 {
		return m, nil
	}
	dest := m.inactivePanel()
	m.pendingSources = sources
	m.pendingDest = dest

	msg := fmt.Sprintf("Copy %d item(s) to %s?", len(sources), dest)
	d := dialog.NewConfirm("Copy", msg, tagCopy)
	m.dialog = &d
	return m, nil
}

func (m Model) startMove() (tea.Model, tea.Cmd) {
	sources := m.selectedOrCurrent()
	if len(sources) == 0 {
		return m, nil
	}
	dest := m.inactivePanel()
	m.pendingSources = sources
	m.pendingDest = dest

	msg := fmt.Sprintf("Move %d item(s) to %s?", len(sources), dest)
	d := dialog.NewConfirm("Move", msg, tagMove)
	m.dialog = &d
	return m, nil
}

func (m Model) startDelete() (tea.Model, tea.Cmd) {
	sources := m.selectedOrCurrent()
	if len(sources) == 0 {
		return m, nil
	}
	m.pendingSources = sources

	msg := fmt.Sprintf("Delete %d item(s)?", len(sources))
	d := dialog.NewConfirm("Delete", msg, tagDelete)
	m.dialog = &d
	return m, nil
}

func (m Model) startMkdir() (tea.Model, tea.Cmd) {
	d := dialog.NewInput("Create Directory", "Directory name:", "", tagMkdir)
	m.dialog = &d
	return m, nil
}

func (m Model) startRename() (tea.Model, tea.Cmd) {
	name := m.currentFileName()
	if name == "" || name == ".." {
		return m, nil
	}
	d := dialog.NewInput("Rename", "New name:", name, tagRename)
	m.dialog = &d
	return m, nil
}

func (m Model) startGoTo() (tea.Model, tea.Cmd) {
	d := dialog.NewInputWithBase("Go To", "Path:", m.activePanel().Path(), tagGoTo, m.activePanel().Path())
	m.dialog = &d
	return m, nil
}

func (m Model) startHelp() (tea.Model, tea.Cmd) {
	h := help.New(m.cfg.Keys, m.width, m.height)
	m.help = &h
	return m, nil
}

func (m Model) startBookmarks() (tea.Model, tea.Cmd) {
	bm := bookmarks.New(m.bookmarkStore, m.activePanel().Path(), m.width, m.height)
	m.bookmarks = &bm
	return m, nil
}

func (m Model) startThemePicker() (tea.Model, tea.Cmd) {
	m.themeBeforePick = m.theme
	available := theme.ListAvailable()
	tp := themepicker.New(available, m.width, m.height)
	m.themePicker = &tp

	// Build set of local keys so remote fetch skips duplicates.
	localKeys := make(map[string]bool)
	for _, a := range available {
		if a.Key != "" {
			localKeys[a.Key] = true
		}
	}
	return m, themepicker.FetchRemote(localKeys)
}

func (m Model) startCmdExec() (tea.Model, tea.Cmd) {
	ce := cmdexec.New(m.activePanel().Path(), m.width, m.height)
	m.cmdExec = &ce
	return m, nil
}

func (m Model) startFuzzyFind() (tea.Model, tea.Cmd) {
	f := fuzzy.New(m.activePanel().Path(), m.width, m.height)
	m.fuzzy = &f
	return m, f.Init()
}

func (m Model) startView() (tea.Model, tea.Cmd) {
	e := m.activePanel().CurrentEntry()
	if e == nil || e.IsDir() {
		return m, nil
	}
	return m, viewFileCmd(m.currentFilePath())
}

func (m Model) startEdit() (tea.Model, tea.Cmd) {
	e := m.activePanel().CurrentEntry()
	if e == nil || e.IsDir() {
		return m, nil
	}
	return m, editFileCmd(m.currentFilePath())
}

func (m Model) showError(title string, err error) (tea.Model, tea.Cmd) {
	d := dialog.NewError(title, err.Error())
	m.dialog = &d
	return m, nil
}

func (m Model) handleDialogResult(result dialog.Result) (tea.Model, tea.Cmd) {
	switch result.Tag {
	case tagCopy:
		if result.Confirmed {
			return m, copyCmd(m.pendingSources, m.pendingDest)
		}
	case tagMove:
		if result.Confirmed {
			return m, moveCmd(m.pendingSources, m.pendingDest)
		}
	case tagDelete:
		if result.Confirmed {
			return m, deleteCmd(m.pendingSources)
		}
	case tagMkdir:
		if result.Confirmed && result.Text != "" {
			return m, mkdirCmd(m.activePanelMkdir(result.Text))
		}
	case tagRename:
		if result.Confirmed && result.Text != "" {
			return m, renameCmd(m.currentFilePath(), result.Text)
		}
	case tagGoTo:
		if result.Confirmed && result.Text != "" {
			path := result.Text
			// Expand ~ to home directory
			if len(path) > 0 && path[0] == '~' {
				if home, err := os.UserHomeDir(); err == nil {
					path = home + path[1:]
				}
			}
			m.activePanel().SetPath(path)
			return m, m.activePanel().LoadDir()
		}
	}
	return m, nil
}

// --- Layout helpers ---

func (m *Model) activePanel() *panel.Model {
	if m.focus == FocusLeft {
		return &m.leftPanel
	}
	return &m.rightPanel
}

func (m *Model) toggleFocus() {
	if m.focus == FocusLeft {
		m.focus = FocusRight
		m.leftPanel.SetActive(false)
		m.rightPanel.SetActive(true)
	} else {
		m.focus = FocusLeft
		m.leftPanel.SetActive(true)
		m.rightPanel.SetActive(false)
	}
}

func (m *Model) recalcLayout() {
	panelHeight := m.height - 3 // 2 for panel borders (top+bottom), 1 for fkey bar
	if panelHeight < 1 {
		panelHeight = 1
	}
	panelWidth := m.width / 2
	rightWidth := m.width - panelWidth

	m.leftPanel.SetSize(panelWidth, panelHeight)
	m.rightPanel.SetSize(rightWidth, panelHeight)
}

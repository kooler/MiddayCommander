// Package servers is the saved SSH server list overlay.
package servers

import (
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/kooler/MiddayCommander/internal/remote"
	"github.com/kooler/MiddayCommander/internal/ui/overlay"
	"github.com/kooler/MiddayCommander/internal/ui/theme"
)

type ConnectMsg struct {
	Server remote.Server
}

type DismissMsg struct{}

type mode int

const (
	modeList mode = iota
	modeFilter
	modeForm
)

// field indexes into the add/edit form.
const (
	fieldName = iota
	fieldHost
	fieldPort
	fieldUser
	fieldKey
	fieldDir
	fieldCount
)

var fieldLabels = [fieldCount]string{
	fieldName: "Name",
	fieldHost: "Host",
	fieldPort: "Port",
	fieldUser: "User",
	fieldKey:  "Key file",
	fieldDir:  "Directory",
}

var fieldHints = [fieldCount]string{
	fieldName: "shown in this list; defaults to the host",
	fieldHost: "required",
	fieldPort: "blank for 22",
	fieldUser: "blank for your login name",
	fieldKey:  "blank to use your ssh-agent",
	fieldDir:  "where the panel opens; blank for your home directory",
}

type Model struct {
	store *remote.Store
	items []remote.SavedServer

	mode   mode
	cursor int
	offset int
	width  int
	height int

	filter string

	// Form state
	values     [fieldCount]string
	focused    int
	editingOld string // name of the server being edited, "" when adding
	formErr    string
}

func New(store *remote.Store, width, height int) Model {
	return Model{
		store:  store,
		items:  store.Sorted(),
		width:  width,
		height: height,
	}
}

// SetSize updates the screen size the box is laid out against, so a resized
// terminal does not leave the cursor outside the visible window.
func (m *Model) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.clampOffset()
}

func (m Model) Update(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch m.mode {
	case modeForm:
		return m.updateForm(msg)
	case modeFilter:
		return m.updateFilter(msg)
	default:
		return m.updateList(msg)
	}
}

func (m Model) updateList(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		return m, func() tea.Msg { return DismissMsg{} }
	case "enter":
		return m.connectAt(m.cursor)
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
			m.clampOffset()
		}
	case "down", "j":
		if m.cursor < len(m.items)-1 {
			m.cursor++
			m.clampOffset()
		}
	case "a":
		m.startAdd()
	case "e":
		m.startEdit()
	case "d", "delete":
		if m.cursor >= 0 && m.cursor < len(m.items) {
			m.store.Remove(m.items[m.cursor].Name)
			_ = m.store.Save()
			m.refilter()
		}
	case "f":
		m.mode = modeFilter
		m.filter = ""
		m.clampOffset() // the input line shrinks the window
	case "0", "1", "2", "3", "4", "5", "6", "7", "8", "9":
		return m.connectAt(int(msg.String()[0] - '0'))
	}
	return m, nil
}

func (m Model) connectAt(idx int) (Model, tea.Cmd) {
	if idx < 0 || idx >= len(m.items) {
		return m, nil
	}
	srv := m.items[idx].Server
	m.store.Touch(srv.Name)
	_ = m.store.Save()
	return m, func() tea.Msg { return ConnectMsg{Server: srv} }
}

func (m Model) updateFilter(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeList
		m.filter = ""
		m.refilter()
	case "enter":
		m.mode = modeList
		if len(m.items) == 0 {
			m.filter = ""
			m.refilter()
			return m, nil
		}
		return m.connectAt(m.cursor)
	case "backspace":
		if len(m.filter) > 0 {
			m.filter = m.filter[:len(m.filter)-1]
			m.refilter()
		} else {
			m.mode = modeList
		}
	case "up":
		if m.cursor > 0 {
			m.cursor--
			m.clampOffset()
		}
	case "down":
		if m.cursor < len(m.items)-1 {
			m.cursor++
			m.clampOffset()
		}
	default:
		if text := insertableText(msg); text != "" {
			m.filter += text
			m.refilter()
		}
	}
	return m, nil
}

func (m *Model) startAdd() {
	m.mode = modeForm
	m.values = [fieldCount]string{}
	m.focused = fieldName
	m.editingOld = ""
	m.formErr = ""
}

func (m *Model) startEdit() {
	if m.cursor < 0 || m.cursor >= len(m.items) {
		return
	}
	srv := m.items[m.cursor].Server

	m.mode = modeForm
	m.focused = fieldName
	m.editingOld = srv.Name
	m.formErr = ""
	m.values = [fieldCount]string{
		fieldName: srv.Name,
		fieldHost: srv.Host,
		fieldUser: srv.User,
		fieldKey:  srv.KeyPath,
		fieldDir:  srv.Dir,
	}
	if srv.Port != 0 {
		m.values[fieldPort] = strconv.Itoa(srv.Port)
	}
}

func (m Model) updateForm(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeList
		m.formErr = ""
		return m, nil

	case "tab", "down":
		m.focused = (m.focused + 1) % fieldCount
		return m, nil

	case "shift+tab", "up":
		m.focused = (m.focused - 1 + fieldCount) % fieldCount
		return m, nil

	case "enter":
		srv, err := m.buildServer()
		if err != nil {
			m.formErr = err.Error()
			return m, nil
		}
		// A rename replaces the old entry instead of duplicating it.
		if m.editingOld != "" && m.editingOld != srv.Name {
			m.store.Remove(m.editingOld)
		}
		m.store.Add(srv)
		_ = m.store.Save()
		m.mode = modeList
		m.formErr = ""
		m.refilter()
		return m, nil

	case "backspace":
		v := m.values[m.focused]
		if len(v) > 0 {
			m.values[m.focused] = v[:len(v)-1]
		}
		return m, nil

	default:
		m.values[m.focused] += insertableText(msg)
		return m, nil
	}
}

// insertableText returns the characters a key carries, or "" if it is not
// text. A paste arrives as one message of many runes.
func insertableText(msg tea.KeyMsg) string {
	if msg.Alt {
		return ""
	}
	switch msg.Type {
	case tea.KeyRunes:
		return string(msg.Runes)
	case tea.KeySpace:
		return " "
	}
	return ""
}

// buildServer validates the form.
func (m Model) buildServer() (remote.Server, error) {
	host := strings.TrimSpace(m.values[fieldHost])
	if host == "" {
		return remote.Server{}, fmt.Errorf("Host is required")
	}

	srv := remote.Server{
		Name:    strings.TrimSpace(m.values[fieldName]),
		Host:    host,
		User:    strings.TrimSpace(m.values[fieldUser]),
		KeyPath: strings.TrimSpace(m.values[fieldKey]),
		Dir:     strings.TrimSpace(m.values[fieldDir]),
	}
	if srv.Name == "" {
		srv.Name = host
	}

	if portStr := strings.TrimSpace(m.values[fieldPort]); portStr != "" {
		port, err := strconv.Atoi(portStr)
		if err != nil || port <= 0 || port > 65535 {
			return remote.Server{}, fmt.Errorf("Port must be a number between 1 and 65535")
		}
		srv.Port = port
	}

	return srv, nil
}

func (m *Model) refilter() {
	all := m.store.Sorted()
	if m.filter == "" {
		m.items = all
	} else {
		query := strings.ToLower(m.filter)
		m.items = nil
		for _, s := range all {
			target := strings.ToLower(s.Name + " " + s.Host + " " + s.User)
			if strings.Contains(target, query) {
				m.items = append(m.items, s)
			}
		}
	}
	if m.cursor >= len(m.items) {
		m.cursor = max(0, len(m.items)-1)
	}
	m.offset = 0
}

func (m Model) BoxSize(screenWidth, screenHeight int) (int, int) {
	w := screenWidth * 2 / 3
	if w < 48 {
		w = min(48, screenWidth)
	}

	h := len(m.items) + 4
	if m.mode == modeForm {
		h = fieldCount + 5 // fields, borders, title, footer, error line
	}
	if h < 9 {
		h = 9
	}
	maxH := screenHeight * 3 / 4
	if h > maxH {
		h = maxH
	}
	return w, h
}

// resultHeight is the window both clampOffset and View must agree on. The
// filter input line takes one of the rows.
func (m Model) resultHeight() int {
	_, boxH := m.BoxSize(m.width, m.height)
	h := boxH - 4
	if m.mode == modeFilter {
		h--
	}
	return max(h, 1)
}

func (m *Model) clampOffset() {
	m.offset = overlay.ClampScroll(m.cursor, m.offset, m.resultHeight(), len(m.items))
}

func (m Model) View(_ theme.Theme, screenWidth, screenHeight int) string {
	boxW, boxH := m.BoxSize(screenWidth, screenHeight)
	innerW := boxW - 2

	bg := lipgloss.Color("#1e1e2e")
	fg := lipgloss.Color("#cdd6f4")
	subtle := lipgloss.Color("#a6adc8")
	accent := lipgloss.Color("#89b4fa")
	highlight := lipgloss.Color("#f9e2af")
	danger := lipgloss.Color("#f38ba8")
	cursorBg := lipgloss.Color("#45475a")

	st := styles{
		base:   lipgloss.NewStyle().Background(bg).Foreground(fg),
		cursor: lipgloss.NewStyle().Background(cursorBg).Foreground(fg),
		dim:    lipgloss.NewStyle().Background(bg).Foreground(subtle),
		num:    lipgloss.NewStyle().Background(bg).Foreground(highlight),
		key:    lipgloss.NewStyle().Background(bg).Foreground(accent).Bold(true),
		err:    lipgloss.NewStyle().Background(bg).Foreground(danger),
		innerW: innerW,
	}

	var contentLines []string
	var footer string
	title := "SSH Servers"

	if m.mode == modeForm {
		title = "Add Server"
		if m.editingOld != "" {
			title = "Edit Server"
		}
		contentLines = m.formLines(st)
		footer = st.hints(
			"Tab", "Next field",
			"Enter", "Save",
			"Esc", "Cancel",
		)
	} else {
		contentLines = m.listLines(st)
		footer = st.hints(
			"a", "Add", "e", "Edit", "d", "Delete",
			"f", "Filter", "Enter", "Open", "Esc", "Close",
		)
	}

	return overlay.RenderBox(title, contentLines, footer, boxW, boxH, accent, bg, highlight)
}

func (m Model) formLines(st styles) []string {
	var lines []string

	labelW := 0
	for _, l := range fieldLabels {
		if len(l) > labelW {
			labelW = len(l)
		}
	}

	for i := 0; i < fieldCount; i++ {
		label := pad(fieldLabels[i]+":", labelW+2)
		value := m.values[i]

		var row string
		if i == m.focused {
			row = st.key.Render(" "+label) + st.cursor.Render(pad(value+"_", st.innerW-labelW-3))
		} else {
			shown := value
			if shown == "" {
				shown = fieldHints[i]
				row = st.dim.Render(" "+label) + st.dim.Render(pad(shown, st.innerW-labelW-3))
			} else {
				row = st.dim.Render(" "+label) + st.base.Render(pad(shown, st.innerW-labelW-3))
			}
		}
		lines = append(lines, row)
	}

	if m.formErr != "" {
		lines = append(lines, st.err.Render(pad(" "+m.formErr, st.innerW)))
	}

	return lines
}

func (m Model) listLines(st styles) []string {
	var lines []string

	rh := m.resultHeight()
	if m.mode == modeFilter {
		lines = append(lines, st.key.Render(" Filter: ")+st.base.Render(pad(m.filter+"_", st.innerW-9)))
	}

	if len(m.items) == 0 {
		lines = append(lines, st.dim.Render(pad(" No servers yet. Press 'a' to add one.", st.innerW)))
		return lines
	}

	end := min(m.offset+rh, len(m.items))
	for i := m.offset; i < end; i++ {
		srv := m.items[i]

		prefix := "  "
		if i < 10 {
			prefix = fmt.Sprintf("%d ", i)
		}

		detail := srv.Label()
		if srv.Dir != "" {
			detail += " " + srv.Dir
		}
		text := srv.DisplayName()
		if srv.Name != "" && srv.Name != srv.Host {
			text += "  " + detail
		} else {
			text = detail
		}
		text = truncate(text, st.innerW-len(prefix))

		if i == m.cursor {
			lines = append(lines, st.cursor.Render(pad(prefix+text, st.innerW)))
		} else {
			lines = append(lines, st.num.Render(prefix)+st.base.Render(pad(text, st.innerW-len(prefix))))
		}
	}

	return lines
}

// styles keeps the render helpers short.
type styles struct {
	base   lipgloss.Style
	cursor lipgloss.Style
	dim    lipgloss.Style
	num    lipgloss.Style
	key    lipgloss.Style
	err    lipgloss.Style
	innerW int
}

// hints renders alternating key/label pairs.
func (s styles) hints(pairs ...string) string {
	var b strings.Builder
	for i := 0; i+1 < len(pairs); i += 2 {
		if i > 0 {
			b.WriteString(s.dim.Render("  "))
		}
		b.WriteString(s.key.Render(" " + pairs[i]))
		b.WriteString(s.dim.Render(":" + pairs[i+1]))
	}
	return b.String()
}

func pad(s string, width int) string {
	if width < 0 {
		return ""
	}
	if len(s) >= width {
		return s[:width]
	}
	return s + strings.Repeat(" ", width-len(s))
}

func truncate(s string, width int) string {
	if width <= 1 || len(s) <= width {
		return s
	}
	return s[:width-1] + "…"
}

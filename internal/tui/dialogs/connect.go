package dialogs

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	midfs "github.com/kooler/MiddayCommander/internal/fs"
	sftpfs "github.com/kooler/MiddayCommander/internal/fs/sftp"
	profilesstore "github.com/kooler/MiddayCommander/internal/profiles"
	"github.com/kooler/MiddayCommander/internal/ui/overlay"
	"github.com/kooler/MiddayCommander/internal/ui/theme"
)

type ConnectOpenMsg struct{}

type ConnectSubmitMsg struct {
	Profile profilesstore.Profile
	URI     midfs.URI
}

type ConnectDismissMsg struct{}

type connectField int

const (
	connectFieldHost connectField = iota
	connectFieldPort
	connectFieldUser
	connectFieldPath
	connectFieldAuth
	connectFieldIdentityFile
	connectFieldKnownHostsFile
)

type connectFieldState struct {
	label  string
	value  string
	cursor int
}

type ConnectModel struct {
	fields      []connectFieldState
	activeField int
	width       int
	height      int
	errText     string
}

func NewConnect(currentURI midfs.URI, width, height int) ConnectModel {
	model := ConnectModel{
		fields: []connectFieldState{
			{label: "Host", value: "", cursor: 0},
			{label: "Port", value: "22", cursor: 2},
			{label: "User", value: defaultConnectUser(), cursor: len(defaultConnectUser())},
			{label: "Path", value: "/", cursor: 1},
			{label: "Auth", value: profilesstore.AuthAgent, cursor: len(profilesstore.AuthAgent)},
			{label: "Identity", value: "", cursor: 0},
			{label: "Known Hosts", value: "", cursor: 0},
		},
		width:  width,
		height: height,
	}

	if currentURI.Scheme == midfs.SchemeSFTP {
		if opts, err := sftpfs.FromURI(currentURI); err == nil {
			model.setFieldValue(connectFieldHost, opts.Host)
			model.setFieldValue(connectFieldPort, strconv.Itoa(opts.Port))
			model.setFieldValue(connectFieldUser, opts.User)
			model.setFieldValue(connectFieldPath, opts.Path)
			model.setFieldValue(connectFieldAuth, opts.Auth)
			model.setFieldValue(connectFieldIdentityFile, opts.IdentityFile)
			model.setFieldValue(connectFieldKnownHostsFile, opts.KnownHostsFile)
		}
	}

	return model
}

func (m ConnectModel) Update(msg tea.KeyMsg) (ConnectModel, tea.Cmd) {
	switch msg.String() {
	case "esc":
		return m, func() tea.Msg { return ConnectDismissMsg{} }
	case "tab", "down":
		m.moveField(1)
		return m, nil
	case "shift+tab", "up":
		m.moveField(-1)
		return m, nil
	case "enter":
		profile, uri, err := m.buildProfileURI()
		if err != nil {
			m.errText = err.Error()
			return m, nil
		}
		return m, func() tea.Msg { return ConnectSubmitMsg{Profile: profile, URI: uri} }
	case "left":
		if m.currentFieldID() == connectFieldAuth {
			m.toggleAuth(-1)
		} else {
			m.moveCursor(-1)
		}
		return m, nil
	case "right":
		if m.currentFieldID() == connectFieldAuth {
			m.toggleAuth(1)
		} else {
			m.moveCursor(1)
		}
		return m, nil
	case "home":
		m.activeState().cursor = 0
		return m, nil
	case "end":
		m.activeState().cursor = len(m.activeState().value)
		return m, nil
	case "backspace":
		if m.currentFieldID() == connectFieldAuth {
			return m, nil
		}
		if m.activeState().cursor > 0 {
			state := m.activeState()
			state.value = state.value[:state.cursor-1] + state.value[state.cursor:]
			state.cursor--
		}
		m.errText = ""
		return m, nil
	case "delete":
		if m.currentFieldID() == connectFieldAuth {
			return m, nil
		}
		state := m.activeState()
		if state.cursor < len(state.value) {
			state.value = state.value[:state.cursor] + state.value[state.cursor+1:]
		}
		m.errText = ""
		return m, nil
	}

	s := msg.String()
	if len(s) == 1 && s[0] >= 32 {
		if m.currentFieldID() == connectFieldAuth {
			switch strings.ToLower(s) {
			case "a":
				m.setFieldValue(connectFieldAuth, profilesstore.AuthAgent)
			case "k":
				m.setFieldValue(connectFieldAuth, profilesstore.AuthKey)
			}
			m.errText = ""
			return m, nil
		}

		state := m.activeState()
		state.value = state.value[:state.cursor] + s + state.value[state.cursor:]
		state.cursor++
		m.errText = ""
	}

	return m, nil
}

func (m ConnectModel) BoxSize(screenWidth, screenHeight int) (int, int) {
	width := screenWidth * 3 / 4
	if width < 64 {
		width = min(64, screenWidth)
	}
	height := len(m.fields) + 5
	if m.errText != "" {
		height++
	}
	maxHeight := screenHeight * 3 / 4
	if height > maxHeight {
		height = maxHeight
	}
	return width, height
}

func (m ConnectModel) View(th theme.Theme, screenWidth, screenHeight int) string {
	boxWidth, boxHeight := m.BoxSize(screenWidth, screenHeight)
	innerWidth := boxWidth - 2

	bg := lipgloss.Color("#1e1e2e")
	fg := lipgloss.Color("#cdd6f4")
	subtle := lipgloss.Color("#a6adc8")
	accent := lipgloss.Color("#89b4fa")
	highlight := lipgloss.Color("#f9e2af")
	cursorBg := lipgloss.Color("#45475a")
	errorColor := lipgloss.Color("#f38ba8")

	bgStyle := lipgloss.NewStyle().Background(bg).Foreground(fg)
	dimStyle := lipgloss.NewStyle().Background(bg).Foreground(subtle)
	activeStyle := lipgloss.NewStyle().Background(cursorBg).Foreground(fg)
	labelStyle := lipgloss.NewStyle().Background(bg).Foreground(accent).Bold(true)
	valueStyle := lipgloss.NewStyle().Background(bg).Foreground(highlight)
	errorStyle := lipgloss.NewStyle().Background(bg).Foreground(errorColor)

	contentLines := make([]string, 0, len(m.fields)+1)
	for index, field := range m.fields {
		isActive := index == m.activeField
		line := renderConnectFieldLine(field, isActive, innerWidth, labelStyle, dimStyle, activeStyle, valueStyle, bgStyle)
		contentLines = append(contentLines, line)
	}

	if m.errText != "" {
		errLine := errorStyle.Render(" " + padConnectRight(m.errText, innerWidth-1))
		contentLines = append(contentLines, errLine)
	}

	keyStyle := lipgloss.NewStyle().Background(bg).Foreground(accent).Bold(true)
	footer := keyStyle.Render(" Tab") + dimStyle.Render(":Field") +
		dimStyle.Render("  ") +
		keyStyle.Render("Enter") + dimStyle.Render(":Connect") +
		dimStyle.Render("  ") +
		keyStyle.Render("←→") + dimStyle.Render(":Cursor/Auth") +
		dimStyle.Render("  ") +
		keyStyle.Render("Esc") + dimStyle.Render(":Close")
	footerWidth := lipgloss.Width(footer)
	if footerWidth < innerWidth {
		footer += dimStyle.Render(strings.Repeat(" ", innerWidth-footerWidth))
	}

	return overlay.RenderBox("Remote Connect", contentLines, footer, boxWidth, boxHeight, accent, bg, highlight)
}

func (m ConnectModel) buildProfileURI() (profilesstore.Profile, midfs.URI, error) {
	portValue := strings.TrimSpace(m.fieldValue(connectFieldPort))
	port := 0
	if portValue != "" {
		parsedPort, err := strconv.Atoi(portValue)
		if err != nil {
			return profilesstore.Profile{}, midfs.URI{}, fmt.Errorf("port must be a number")
		}
		port = parsedPort
	}

	profile, err := profilesstore.Normalize(profilesstore.Profile{
		Name:           "manual",
		Host:           strings.TrimSpace(m.fieldValue(connectFieldHost)),
		Port:           port,
		User:           strings.TrimSpace(m.fieldValue(connectFieldUser)),
		Path:           strings.TrimSpace(m.fieldValue(connectFieldPath)),
		Auth:           strings.TrimSpace(m.fieldValue(connectFieldAuth)),
		IdentityFile:   strings.TrimSpace(m.fieldValue(connectFieldIdentityFile)),
		KnownHostsFile: strings.TrimSpace(m.fieldValue(connectFieldKnownHostsFile)),
	})
	if err != nil {
		return profilesstore.Profile{}, midfs.URI{}, err
	}

	opts, err := sftpfs.FromProfile(profile)
	if err != nil {
		return profilesstore.Profile{}, midfs.URI{}, err
	}
	return profile, opts.URI(), nil
}

func (m *ConnectModel) moveField(delta int) {
	m.activeField += delta
	if m.activeField < 0 {
		m.activeField = len(m.fields) - 1
	}
	if m.activeField >= len(m.fields) {
		m.activeField = 0
	}
}

func (m *ConnectModel) moveCursor(delta int) {
	state := m.activeState()
	state.cursor += delta
	if state.cursor < 0 {
		state.cursor = 0
	}
	if state.cursor > len(state.value) {
		state.cursor = len(state.value)
	}
}

func (m *ConnectModel) toggleAuth(delta int) {
	auths := []string{profilesstore.AuthAgent, profilesstore.AuthKey}
	current := m.fieldValue(connectFieldAuth)
	index := 0
	for i, auth := range auths {
		if auth == current {
			index = i
			break
		}
	}
	index += delta
	if index < 0 {
		index = len(auths) - 1
	}
	if index >= len(auths) {
		index = 0
	}
	m.setFieldValue(connectFieldAuth, auths[index])
	m.errText = ""
}

func (m *ConnectModel) currentFieldID() connectField {
	return connectField(m.activeField)
}

func (m *ConnectModel) activeState() *connectFieldState {
	return &m.fields[m.activeField]
}

func (m *ConnectModel) fieldValue(field connectField) string {
	return m.fields[int(field)].value
}

func (m *ConnectModel) setFieldValue(field connectField, value string) {
	m.fields[int(field)].value = value
	m.fields[int(field)].cursor = len(value)
}

func renderConnectFieldLine(
	field connectFieldState,
	active bool,
	innerWidth int,
	labelStyle, dimStyle, activeStyle, valueStyle, bgStyle lipgloss.Style,
) string {
	labelText := padConnectRight(field.label+":", 13)
	label := labelStyle.Render(" " + labelText)

	valueWidth := innerWidth - lipgloss.Width(label) - 1
	if valueWidth < 1 {
		valueWidth = 1
	}

	value := field.value
	if strings.TrimSpace(field.label) == "Auth" {
		value = "[" + value + "]"
	}

	var renderedValue string
	if active {
		renderedValue = renderEditableValue(value, field.cursor, valueWidth, activeStyle, valueStyle)
	} else {
		renderedValue = dimStyle.Render(padConnectRight(value, valueWidth))
	}

	line := label + renderedValue
	lineWidth := lipgloss.Width(line)
	if lineWidth < innerWidth {
		line += bgStyle.Render(strings.Repeat(" ", innerWidth-lineWidth))
	}
	return line
}

func renderEditableValue(value string, cursorPos, width int, activeStyle, valueStyle lipgloss.Style) string {
	runes := []rune(value)
	if cursorPos < 0 {
		cursorPos = 0
	}
	if cursorPos > len(runes) {
		cursorPos = len(runes)
	}

	if len(runes) > width {
		start := cursorPos - width/2
		if start < 0 {
			start = 0
		}
		end := start + width
		if end > len(runes) {
			end = len(runes)
			start = max(0, end-width)
		}
		runes = runes[start:end]
		cursorPos -= start
	}

	before := string(runes[:cursorPos])
	cursorChar := " "
	after := ""
	if cursorPos < len(runes) {
		cursorChar = string(runes[cursorPos])
		after = string(runes[cursorPos+1:])
	}

	rendered := valueStyle.Render(before) + activeStyle.Render(cursorChar) + valueStyle.Render(after)
	widthRemaining := width - lipgloss.Width(rendered)
	if widthRemaining > 0 {
		rendered += valueStyle.Render(strings.Repeat(" ", widthRemaining))
	}
	return rendered
}

func padConnectRight(value string, width int) string {
	runes := []rune(value)
	if len(runes) >= width {
		return string(runes[:width])
	}
	return value + strings.Repeat(" ", width-len(runes))
}

func defaultConnectUser() string {
	for _, key := range []string{"USER", "LOGNAME", "USERNAME"} {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	return ""
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

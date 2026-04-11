package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/kooler/MiddayCommander/internal/actions"
)

// File operation result messages.

type copyDoneMsg struct{ err error }
type moveDoneMsg struct{ err error }
type deleteDoneMsg struct{ err error }
type mkdirDoneMsg struct{ err error }
type renameDoneMsg struct{ err error }

// externalDoneMsg is sent when an external viewer/editor returns.
type externalDoneMsg struct{ err error }

func copyCmd(sources []string, dest string) tea.Cmd {
	return func() tea.Msg {
		err := actions.Copy(sources, dest, nil)
		return copyDoneMsg{err: err}
	}
}

func moveCmd(sources []string, dest string) tea.Cmd {
	return func() tea.Msg {
		err := actions.Move(sources, dest, nil)
		return moveDoneMsg{err: err}
	}
}

func deleteCmd(paths []string) tea.Cmd {
	return func() tea.Msg {
		err := actions.Delete(paths, nil)
		return deleteDoneMsg{err: err}
	}
}

func mkdirCmd(path string) tea.Cmd {
	return func() tea.Msg {
		err := actions.Mkdir(path)
		return mkdirDoneMsg{err: err}
	}
}

func renameCmd(oldPath, newName string) tea.Cmd {
	return func() tea.Msg {
		err := actions.Rename(oldPath, newName)
		return renameDoneMsg{err: err}
	}
}

func viewFileCmd(path string) tea.Cmd {
	return externalCmd("PAGER", "less", path)
}

func editFileCmd(path string) tea.Cmd {
	return externalCmd("EDITOR", "vi", path)
}

func externalCmd(envVar, fallback, path string) tea.Cmd {
	cmd := strings.TrimSpace(os.Getenv(envVar))
	if cmd == "" {
		cmd = fallback
	}
	parts := strings.Fields(cmd)
	c := exec.Command(parts[0], append(parts[1:], path)...)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return externalDoneMsg{err: err}
	})
}

// refreshBothPanels returns commands to reload both panels.
func (m *Model) refreshBothPanels() tea.Cmd {
	return tea.Batch(m.leftPanel.LoadDir(), m.rightPanel.LoadDir())
}

// inactivePanel returns the panel that does NOT have focus.
func (m *Model) inactivePanel() string {
	if m.focus == FocusLeft {
		return m.rightPanel.Path()
	}
	return m.leftPanel.Path()
}

// selectedOrCurrent returns the currently selected/tagged paths from the active panel.
func (m *Model) selectedOrCurrent() []string {
	return m.activePanel().SelectedPaths()
}

// currentFileName returns just the base name of the file under cursor.
func (m *Model) currentFileName() string {
	e := m.activePanel().CurrentEntry()
	if e == nil {
		return ""
	}
	return e.Name()
}

// currentFilePath returns the full path of the file under cursor.
func (m *Model) currentFilePath() string {
	return m.activePanel().CurrentPath()
}

// activePanelMkdir returns the full path for a new directory in the active panel.
func (m *Model) activePanelMkdir(name string) string {
	return filepath.Join(m.activePanel().Path(), name)
}

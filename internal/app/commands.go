package app

import (
	"os"
	"os/exec"
	"path/filepath"

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
	pager := os.Getenv("PAGER")
	if pager == "" {
		pager = "less"
	}
	c := exec.Command(pager, path)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return externalDoneMsg{err: err}
	})
}

func editFileCmd(path string) tea.Cmd {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}
	c := exec.Command(editor, path)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return externalDoneMsg{err: err}
	})
}

func executeFileCmd(path string, dir string) tea.Cmd {
	c := exec.Command(path)
	c.Dir = dir
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return externalDoneMsg{err: err}
	})
}

func startTerminalCmd(dir string) tea.Cmd {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "bash"
	}
	c := interactiveShellCmd(shell)
	c.Dir = dir
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return externalDoneMsg{err: err}
	})
}

func interactiveShellCmd(shellPath string) *exec.Cmd {
	name := filepath.Base(shellPath)
	switch name {
	case "bash":
		if rcFile, err := writeBashRc(); err == nil {
			return exec.Command(shellPath, "--rcfile", rcFile, "-i")
		}
	case "zsh":
		if dir, err := writeZshDir(); err == nil {
			cmd := exec.Command(shellPath, "-i")
			cmd.Env = append(os.Environ(), "ZDOTDIR="+dir)
			return cmd
		}
	}
	return exec.Command(shellPath, "-i")
}

func writeBashRc() (string, error) {
	f, err := os.CreateTemp("", "mdc-terminal-*.bashrc")
	if err != nil {
		return "", err
	}
	defer f.Close()
	_, err = f.WriteString(`bind '"\C-o": "\C-d"'` + "\n")
	if err != nil {
		return "", err
	}
	return f.Name(), nil
}

func writeZshDir() (string, error) {
	dir, err := os.MkdirTemp("", "mdc-terminal-*")
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, ".zshrc")
	if err := os.WriteFile(path, []byte("bindkey '^O' exit\n"), 0o600); err != nil {
		return "", err
	}
	return dir, nil
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

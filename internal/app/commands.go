package app

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/kooler/MiddayCommander/internal/actions"
	midfs "github.com/kooler/MiddayCommander/internal/fs"
	"github.com/kooler/MiddayCommander/internal/transfer"
)

type copyDoneMsg struct{ err error }
type moveDoneMsg struct{ err error }
type deleteDoneMsg struct{ err error }
type mkdirDoneMsg struct{ err error }
type renameDoneMsg struct{ err error }
type externalDoneMsg struct{ err error }

func copyCmd(router *midfs.Router, sources []midfs.URI, dest midfs.URI) tea.Cmd {
	return func() tea.Msg {
		err := actions.Copy(context.Background(), router, sources, dest, nil)
		return copyDoneMsg{err: err}
	}
}

func moveCmd(router *midfs.Router, sources []midfs.URI, dest midfs.URI) tea.Cmd {
	return func() tea.Msg {
		err := actions.Move(context.Background(), router, sources, dest, nil)
		return moveDoneMsg{err: err}
	}
}

func deleteCmd(router *midfs.Router, paths []midfs.URI) tea.Cmd {
	return func() tea.Msg {
		err := actions.Delete(context.Background(), router, paths, nil)
		return deleteDoneMsg{err: err}
	}
}

func mkdirCmd(router *midfs.Router, uri midfs.URI) tea.Cmd {
	return func() tea.Msg {
		err := actions.Mkdir(context.Background(), router, uri)
		return mkdirDoneMsg{err: err}
	}
}

func renameCmd(router *midfs.Router, oldURI midfs.URI, newName string) tea.Cmd {
	return func() tea.Msg {
		err := actions.Rename(context.Background(), router, oldURI, newName)
		return renameDoneMsg{err: err}
	}
}

func waitTransferEventCmd(manager *transfer.Manager) tea.Cmd {
	if manager == nil {
		return nil
	}
	return func() tea.Msg {
		event, ok := <-manager.Events()
		if !ok {
			return nil
		}
		return event
	}
}

func viewFileCmd(uri midfs.URI) tea.Cmd {
	pager := os.Getenv("PAGER")
	if pager == "" {
		pager = "less"
	}
	command := exec.Command(pager, uri.Path)
	return tea.ExecProcess(command, func(err error) tea.Msg {
		return externalDoneMsg{err: err}
	})
}

func editFileCmd(uri midfs.URI) tea.Cmd {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}
	command := exec.Command(editor, uri.Path)
	return tea.ExecProcess(command, func(err error) tea.Msg {
		return externalDoneMsg{err: err}
	})
}

func (m *Model) refreshBothPanels() tea.Cmd {
	return tea.Batch(m.leftPanel.LoadDir(), m.rightPanel.LoadDir())
}

func (m *Model) inactivePanel() midfs.URI {
	if m.focus == FocusLeft {
		return m.rightPanel.URI()
	}
	return m.leftPanel.URI()
}

func (m *Model) selectedOrCurrent() []midfs.URI {
	return m.activePanel().SelectedURIs()
}

func (m *Model) currentFileName() string {
	entry := m.activePanel().CurrentEntry()
	if entry == nil {
		return ""
	}
	return entry.Name
}

func (m *Model) currentFileURI() midfs.URI {
	return m.activePanel().CurrentURI()
}

func (m *Model) activePanelMkdir(name string) midfs.URI {
	return m.router.Join(m.activePanel().URI(), name)
}

func (m *Model) activePanelLocalPath() (string, bool) {
	return m.activePanel().LocalPath()
}

func parseGoToURI(input string) (midfs.URI, error) {
	value := strings.TrimSpace(input)
	if value == "" {
		return midfs.URI{}, fmt.Errorf("path is empty")
	}
	if strings.Contains(value, "://") {
		return midfs.ParseURI(value)
	}
	if strings.HasPrefix(value, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return midfs.URI{}, err
		}
		value = home + value[1:]
	}
	if !filepath.IsAbs(value) {
		absValue, err := filepath.Abs(value)
		if err != nil {
			return midfs.URI{}, err
		}
		value = absValue
	}
	return midfs.NewFileURI(value), nil
}

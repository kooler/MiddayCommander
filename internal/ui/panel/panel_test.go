package panel

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"testing"

	midfs "github.com/kooler/MiddayCommander/internal/fs"
	archivefs "github.com/kooler/MiddayCommander/internal/fs/archive"
	localfs "github.com/kooler/MiddayCommander/internal/fs/local"
)

func TestPanelNavigatesIntoAndOutOfArchive(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	archivePath := filepath.Join(root, "sample.zip")
	writePanelZip(t, archivePath, map[string]string{
		"folder/note.txt": "inside",
	})

	router := midfs.NewRouter(localfs.New(), archivefs.New())
	model := New(router, midfs.NewFileURI(root), KeyMap{})
	loadPanelDir(t, &model)

	model.RestoreCursor("sample.zip")
	cmd := model.handleEnter()
	msg, ok := cmd().(DirLoadedMsg)
	if !ok {
		t.Fatalf("handleEnter() returned unexpected msg type %T", cmd())
	}
	model.HandleDirLoaded(msg)

	if model.dir.Scheme != midfs.SchemeArchive {
		t.Fatalf("dir.Scheme = %q, want archive", model.dir.Scheme)
	}
	if model.dir.QueryValue("entry") != "." {
		t.Fatalf("archive root entry = %q, want .", model.dir.QueryValue("entry"))
	}

	model.goUp()
	if model.dir.Scheme != midfs.SchemeFile || model.dir.Path != root {
		t.Fatalf("goUp() dir = %#v, want file dir %q", model.dir, root)
	}
}

func TestPanelSelectedURIsUsesCurrentEntryWhenNothingTagged(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	filePath := filepath.Join(root, "alpha.txt")
	if err := os.WriteFile(filePath, []byte("alpha"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	router := midfs.NewRouter(localfs.New(), archivefs.New())
	model := New(router, midfs.NewFileURI(root), KeyMap{})
	loadPanelDir(t, &model)

	model.RestoreCursor("alpha.txt")
	selected := model.SelectedURIs()
	if len(selected) != 1 {
		t.Fatalf("len(SelectedURIs()) = %d, want 1", len(selected))
	}
	if selected[0].Path != filePath {
		t.Fatalf("SelectedURIs()[0].Path = %q, want %q", selected[0].Path, filePath)
	}
}

func TestPanelOpenFileAndPreviewMessagesCarryURI(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	filePath := filepath.Join(root, "alpha.txt")
	if err := os.WriteFile(filePath, []byte("alpha"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	router := midfs.NewRouter(localfs.New(), archivefs.New())
	model := New(router, midfs.NewFileURI(root), KeyMap{})
	loadPanelDir(t, &model)
	model.RestoreCursor("alpha.txt")

	openMsg, ok := model.handleEnter()().(OpenFileMsg)
	if !ok {
		t.Fatalf("handleEnter() message type = %T, want OpenFileMsg", model.handleEnter()())
	}
	if openMsg.URI.Path != filePath {
		t.Fatalf("OpenFileMsg.URI.Path = %q, want %q", openMsg.URI.Path, filePath)
	}

	previewMsg, ok := model.handleSpace()().(PreviewFileMsg)
	if !ok {
		t.Fatalf("handleSpace() message type = %T, want PreviewFileMsg", model.handleSpace()())
	}
	if previewMsg.URI.Path != filePath {
		t.Fatalf("PreviewFileMsg.URI.Path = %q, want %q", previewMsg.URI.Path, filePath)
	}
}

func loadPanelDir(t *testing.T, model *Model) {
	t.Helper()

	msg, ok := model.LoadDir()().(DirLoadedMsg)
	if !ok {
		t.Fatalf("LoadDir() returned unexpected msg type")
	}
	model.HandleDirLoaded(msg)
}

func writePanelZip(t *testing.T, archivePath string, files map[string]string) {
	t.Helper()

	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("Create(%q) error = %v", archivePath, err)
	}
	defer file.Close()

	writer := zip.NewWriter(file)
	for name, contents := range files {
		entryWriter, err := writer.Create(name)
		if err != nil {
			t.Fatalf("Create(%q) error = %v", name, err)
		}
		if _, err := io.WriteString(entryWriter, contents); err != nil {
			t.Fatalf("write zip entry error = %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("writer.Close() error = %v", err)
	}
}

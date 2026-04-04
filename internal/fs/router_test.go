package fs_test

import (
	"archive/zip"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	midfs "github.com/kooler/MiddayCommander/internal/fs"
	archivefs "github.com/kooler/MiddayCommander/internal/fs/archive"
	localfs "github.com/kooler/MiddayCommander/internal/fs/local"
)

func TestRouterJoinParentAndDispatch(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	root := t.TempDir()
	archivePath := filepath.Join(root, "tree.zip")
	createArchive(t, archivePath, "folder/file.txt", "hello")

	router := midfs.NewRouter(localfs.New(), archivefs.New())

	localRoot := midfs.NewFileURI(root)
	localChild := router.Join(localRoot, "child")
	if localChild.Path != filepath.Join(root, "child") {
		t.Fatalf("Join(file) path = %q", localChild.Path)
	}

	archiveRoot := midfs.NewArchiveURI(archivePath, ".")
	archiveChild := router.Join(archiveRoot, "folder", "file.txt")
	if archiveChild.QueryValue("entry") != "folder/file.txt" {
		t.Fatalf("Join(archive) entry = %q", archiveChild.QueryValue("entry"))
	}

	parent := router.Parent(archiveRoot)
	if parent.Scheme != midfs.SchemeFile || parent.Path != root {
		t.Fatalf("Parent(archive root) = %#v, want file uri %q", parent, root)
	}

	entry, err := router.Stat(ctx, archiveChild)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if entry.Name != "file.txt" {
		t.Fatalf("Stat().Name = %q, want file.txt", entry.Name)
	}

	reader, err := router.OpenReader(ctx, archiveChild, midfs.OpenReadOptions{})
	if err != nil {
		t.Fatalf("OpenReader() error = %v", err)
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if string(data) != "hello" {
		t.Fatalf("read content = %q, want hello", string(data))
	}
}

func createArchive(t *testing.T, archivePath, entryName, contents string) {
	t.Helper()

	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("Create(%q) error = %v", archivePath, err)
	}
	defer file.Close()

	writer := zip.NewWriter(file)
	entryWriter, err := writer.Create(entryName)
	if err != nil {
		t.Fatalf("Create(%q) error = %v", entryName, err)
	}
	if _, err := io.WriteString(entryWriter, contents); err != nil {
		t.Fatalf("write archive entry error = %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

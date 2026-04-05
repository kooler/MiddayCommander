package actions_test

import (
	"archive/zip"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/kooler/MiddayCommander/internal/actions"
	midfs "github.com/kooler/MiddayCommander/internal/fs"
	archivefs "github.com/kooler/MiddayCommander/internal/fs/archive"
	localfs "github.com/kooler/MiddayCommander/internal/fs/local"
	sftpfs "github.com/kooler/MiddayCommander/internal/fs/sftp"
)

func TestFileActionsSmoke(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	root := t.TempDir()
	srcDir := filepath.Join(root, "src")
	dstDir := filepath.Join(root, "dst")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(src) error = %v", err)
	}
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(dst) error = %v", err)
	}

	sourceFile := filepath.Join(srcDir, "notes.txt")
	if err := os.WriteFile(sourceFile, []byte("hello"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	router := midfs.NewRouter(localfs.New(), archivefs.New())
	if err := actions.Copy(ctx, router, []midfs.URI{midfs.NewFileURI(sourceFile)}, midfs.NewFileURI(dstDir), nil); err != nil {
		t.Fatalf("Copy() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(dstDir, "notes.txt")); err != nil {
		t.Fatalf("copied file missing: %v", err)
	}

	if err := actions.Rename(ctx, router, midfs.NewFileURI(filepath.Join(dstDir, "notes.txt")), "renamed.txt"); err != nil {
		t.Fatalf("Rename() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(dstDir, "renamed.txt")); err != nil {
		t.Fatalf("renamed file missing: %v", err)
	}

	moveDest := filepath.Join(root, "moved")
	if err := os.MkdirAll(moveDest, 0o755); err != nil {
		t.Fatalf("MkdirAll(moveDest) error = %v", err)
	}
	if err := actions.Move(ctx, router, []midfs.URI{midfs.NewFileURI(filepath.Join(dstDir, "renamed.txt"))}, midfs.NewFileURI(moveDest), nil); err != nil {
		t.Fatalf("Move() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(moveDest, "renamed.txt")); err != nil {
		t.Fatalf("moved file missing: %v", err)
	}

	newDir := midfs.NewFileURI(filepath.Join(root, "created"))
	if err := actions.Mkdir(ctx, router, newDir); err != nil {
		t.Fatalf("Mkdir() error = %v", err)
	}
	if _, err := os.Stat(newDir.Path); err != nil {
		t.Fatalf("created dir missing: %v", err)
	}

	if err := actions.Delete(ctx, router, []midfs.URI{newDir}, nil); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err := os.Stat(newDir.Path); !os.IsNotExist(err) {
		t.Fatalf("Delete() did not remove %q", newDir.Path)
	}
}

func TestMoveFromReadOnlyArchiveFailsBeforeCopy(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	root := t.TempDir()
	archivePath := filepath.Join(root, "sample.zip")
	writeArchive(t, archivePath, "inside.txt", "hello")

	destDir := filepath.Join(root, "dest")
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(dest) error = %v", err)
	}

	router := midfs.NewRouter(localfs.New(), archivefs.New())
	err := actions.Move(ctx, router, []midfs.URI{midfs.NewArchiveURI(archivePath, "inside.txt")}, midfs.NewFileURI(destDir), nil)
	if err == nil {
		t.Fatalf("Move() error = nil, want failure for read-only archive source")
	}
	if _, statErr := os.Stat(filepath.Join(destDir, "inside.txt")); !os.IsNotExist(statErr) {
		t.Fatalf("Move() unexpectedly copied file into destination")
	}
}

func TestCopyWithSFTPSourceIsDeferredToPhase3(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	root := t.TempDir()
	destDir := filepath.Join(root, "dest")
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(dest) error = %v", err)
	}

	router := midfs.NewRouter(localfs.New(), archivefs.New(), sftpfs.New())
	err := actions.Copy(ctx, router, []midfs.URI{{
		Scheme: midfs.SchemeSFTP,
		Host:   "files.example.test",
		User:   "demo",
		Path:   "/remote/inside.txt",
	}}, midfs.NewFileURI(destDir), nil)
	if !errors.Is(err, actions.ErrSFTPTransfersDeferred) {
		t.Fatalf("Copy() error = %v, want ErrSFTPTransfersDeferred", err)
	}
	if _, statErr := os.Stat(filepath.Join(destDir, "inside.txt")); !os.IsNotExist(statErr) {
		t.Fatalf("Copy() unexpectedly wrote a local file")
	}
}

func TestMoveWithSFTPSourceIsDeferredToPhase3(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	root := t.TempDir()
	destDir := filepath.Join(root, "dest")
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(dest) error = %v", err)
	}

	router := midfs.NewRouter(localfs.New(), archivefs.New(), sftpfs.New())
	err := actions.Move(ctx, router, []midfs.URI{{
		Scheme: midfs.SchemeSFTP,
		Host:   "files.example.test",
		User:   "demo",
		Path:   "/remote/inside.txt",
	}}, midfs.NewFileURI(destDir), nil)
	if !errors.Is(err, actions.ErrSFTPTransfersDeferred) {
		t.Fatalf("Move() error = %v, want ErrSFTPTransfersDeferred", err)
	}
	if _, statErr := os.Stat(filepath.Join(destDir, "inside.txt")); !os.IsNotExist(statErr) {
		t.Fatalf("Move() unexpectedly wrote a local file")
	}
}

func writeArchive(t *testing.T, archivePath, entryName, contents string) {
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

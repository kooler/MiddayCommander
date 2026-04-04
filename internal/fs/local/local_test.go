package local_test

import (
	"context"
	"io"
	"path/filepath"
	"testing"

	midfs "github.com/kooler/MiddayCommander/internal/fs"
	"github.com/kooler/MiddayCommander/internal/fs/local"
)

func TestLocalFileSystemCRUD(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	fsys := local.New()
	root := t.TempDir()
	fileURI := midfs.NewFileURI(filepath.Join(root, "notes.txt"))

	writer, err := fsys.OpenWriter(ctx, fileURI, midfs.OpenWriteOptions{
		Overwrite: true,
		Perm:      0o644,
	})
	if err != nil {
		t.Fatalf("OpenWriter() error = %v", err)
	}
	if _, err := io.WriteString(writer, "hello world"); err != nil {
		t.Fatalf("write error = %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	entry, err := fsys.Stat(ctx, fileURI)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if entry.Name != "notes.txt" {
		t.Fatalf("Stat().Name = %q, want notes.txt", entry.Name)
	}
	if entry.Size != int64(len("hello world")) {
		t.Fatalf("Stat().Size = %d, want %d", entry.Size, len("hello world"))
	}

	entries, err := fsys.List(ctx, midfs.NewFileURI(root))
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(entries) != 1 || entries[0].Name != "notes.txt" {
		t.Fatalf("List() = %#v, want one notes.txt entry", entries)
	}

	renamed := midfs.NewFileURI(filepath.Join(root, "renamed.txt"))
	if err := fsys.Rename(ctx, fileURI, renamed); err != nil {
		t.Fatalf("Rename() error = %v", err)
	}

	reader, err := fsys.OpenReader(ctx, renamed, midfs.OpenReadOptions{})
	if err != nil {
		t.Fatalf("OpenReader() error = %v", err)
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if string(data) != "hello world" {
		t.Fatalf("read content = %q, want hello world", string(data))
	}

	if err := fsys.Remove(ctx, renamed, false); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}
	if _, err := fsys.Stat(ctx, renamed); err == nil {
		t.Fatalf("Stat() after Remove() = nil error, want failure")
	}
}

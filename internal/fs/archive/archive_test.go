package archive_test

import (
	"archive/zip"
	"context"
	"io"
	"os"
	"path/filepath"
	"slices"
	"testing"

	midfs "github.com/kooler/MiddayCommander/internal/fs"
	archivefs "github.com/kooler/MiddayCommander/internal/fs/archive"
)

func TestArchiveListReadAndParent(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	root := t.TempDir()
	archivePath := filepath.Join(root, "sample.zip")
	writeZipArchive(t, archivePath, map[string]string{
		"root.txt":        "root",
		"folder/note.txt": "nested",
	})

	fsys := archivefs.New()
	archiveRoot := midfs.NewArchiveURI(archivePath, ".")
	entries, err := fsys.List(ctx, archiveRoot)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("len(entries) = %d, want 2", len(entries))
	}

	names := []string{entries[0].Name, entries[1].Name}
	slices.Sort(names)
	if !slices.Equal(names, []string{"folder", "root.txt"}) {
		t.Fatalf("archive root names = %v, want [folder root.txt]", names)
	}

	nestedFile := fsys.Join(archiveRoot, "folder", "note.txt")
	reader, err := fsys.OpenReader(ctx, nestedFile, midfs.OpenReadOptions{})
	if err != nil {
		t.Fatalf("OpenReader() error = %v", err)
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if string(data) != "nested" {
		t.Fatalf("read content = %q, want nested", string(data))
	}

	parent := fsys.Parent(archiveRoot)
	if parent.Scheme != midfs.SchemeFile || parent.Path != root {
		t.Fatalf("Parent(root) = %#v, want file URI for %q", parent, root)
	}

	nestedParent := fsys.Parent(nestedFile)
	if nestedParent.QueryValue("entry") != "folder" {
		t.Fatalf("Parent(nested) entry = %q, want folder", nestedParent.QueryValue("entry"))
	}
}

func writeZipArchive(t *testing.T, archivePath string, files map[string]string) {
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
			t.Fatalf("Create(%q) in zip error = %v", name, err)
		}
		if _, err := io.WriteString(entryWriter, contents); err != nil {
			t.Fatalf("write zip entry %q error = %v", name, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("zip Close() error = %v", err)
	}
}

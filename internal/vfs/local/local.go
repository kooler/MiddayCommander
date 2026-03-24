package local

import (
	"io/fs"
	"os"
	"path/filepath"

	"github.com/kooler/MiddayCommander/internal/vfs"
)

// FS implements vfs.WritableFS for the local filesystem.
type FS struct {
	root string
}

// New creates a new local filesystem rooted at root.
func New(root string) *FS {
	return &FS{root: root}
}

func (f *FS) resolve(name string) string {
	return filepath.Join(f.root, name)
}

func (f *FS) Open(name string) (fs.File, error) {
	return os.Open(f.resolve(name))
}

func (f *FS) ReadDir(name string) ([]fs.DirEntry, error) {
	return os.ReadDir(f.resolve(name))
}

func (f *FS) Stat(name string) (fs.FileInfo, error) {
	return os.Stat(f.resolve(name))
}

func (f *FS) Create(name string) (vfs.WriteFile, error) {
	file, err := os.Create(f.resolve(name))
	if err != nil {
		return nil, err
	}
	return file, nil
}

func (f *FS) Mkdir(name string, perm fs.FileMode) error {
	return os.Mkdir(f.resolve(name), perm)
}

func (f *FS) MkdirAll(name string, perm fs.FileMode) error {
	return os.MkdirAll(f.resolve(name), perm)
}

func (f *FS) Remove(name string) error {
	return os.Remove(f.resolve(name))
}

func (f *FS) RemoveAll(name string) error {
	return os.RemoveAll(f.resolve(name))
}

func (f *FS) Rename(oldname, newname string) error {
	return os.Rename(f.resolve(oldname), f.resolve(newname))
}

// Verify interface compliance at compile time.
var _ vfs.WritableFS = (*FS)(nil)

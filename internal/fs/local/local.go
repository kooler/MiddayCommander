package local

import (
	"context"
	"io"
	iofs "io/fs"
	"os"
	"path/filepath"
	"strings"

	midfs "github.com/kooler/MiddayCommander/internal/fs"
)

type FS struct{}

func New() *FS {
	return &FS{}
}

func (f *FS) ID() string {
	return "local"
}

func (f *FS) Scheme() midfs.Scheme {
	return midfs.SchemeFile
}

func (f *FS) Capabilities() uint64 {
	return midfs.CapList | midfs.CapRead | midfs.CapWrite | midfs.CapMkdir | midfs.CapRename | midfs.CapRemove
}

func (f *FS) List(ctx context.Context, dir midfs.URI) ([]midfs.Entry, error) {
	dirEntries, err := os.ReadDir(dir.Path)
	if err != nil {
		return nil, err
	}

	entries := make([]midfs.Entry, 0, len(dirEntries))
	for _, dirEntry := range dirEntries {
		info, err := dirEntry.Info()
		if err != nil {
			return nil, err
		}
		entryURI := midfs.NewFileURI(filepath.Join(dir.Path, dirEntry.Name()))
		entries = append(entries, entryFromInfo(dirEntry.Name(), entryURI, info))
	}
	return entries, nil
}

func (f *FS) Stat(ctx context.Context, uri midfs.URI) (midfs.Entry, error) {
	info, err := os.Lstat(uri.Path)
	if err != nil {
		return midfs.Entry{}, err
	}
	return entryFromInfo(filepath.Base(uri.Path), uri, info), nil
}

func (f *FS) Mkdir(ctx context.Context, uri midfs.URI, perm os.FileMode) error {
	return os.Mkdir(uri.Path, perm)
}

func (f *FS) Rename(ctx context.Context, from midfs.URI, to midfs.URI) error {
	return os.Rename(from.Path, to.Path)
}

func (f *FS) Remove(ctx context.Context, uri midfs.URI, recursive bool) error {
	if recursive {
		return os.RemoveAll(uri.Path)
	}
	return os.Remove(uri.Path)
}

func (f *FS) OpenReader(ctx context.Context, uri midfs.URI, opts midfs.OpenReadOptions) (io.ReadCloser, error) {
	file, err := os.Open(uri.Path)
	if err != nil {
		return nil, err
	}
	if opts.Offset <= 0 {
		return file, nil
	}
	if _, err := file.Seek(opts.Offset, io.SeekStart); err != nil {
		file.Close()
		return nil, err
	}
	return file, nil
}

func (f *FS) OpenWriter(ctx context.Context, uri midfs.URI, opts midfs.OpenWriteOptions) (io.WriteCloser, error) {
	perm := opts.Perm
	if perm == 0 {
		perm = 0o644
	}
	if opts.Atomic {
		tempPath := uri.Path + opts.TempExtension
		if opts.TempExtension == "" {
			tempPath = uri.Path + ".tmp"
		}
		file, err := os.OpenFile(tempPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
		if err != nil {
			return nil, err
		}
		if opts.Offset > 0 {
			if _, err := file.Seek(opts.Offset, io.SeekStart); err != nil {
				file.Close()
				_ = os.Remove(tempPath)
				return nil, err
			}
		}
		return &atomicWriteCloser{
			File:      file,
			target:    uri.Path,
			tempPath:  tempPath,
			overwrite: opts.Overwrite,
		}, nil
	}

	flags := os.O_CREATE | os.O_WRONLY
	switch {
	case opts.Offset > 0:
	case opts.Overwrite:
		flags |= os.O_TRUNC
	default:
		flags |= os.O_EXCL
	}

	file, err := os.OpenFile(uri.Path, flags, perm)
	if err != nil {
		return nil, err
	}
	if opts.Offset > 0 {
		if _, err := file.Seek(opts.Offset, io.SeekStart); err != nil {
			file.Close()
			return nil, err
		}
	}
	return file, nil
}

func (f *FS) Join(base midfs.URI, elems ...string) midfs.URI {
	parts := []string{base.Path}
	parts = append(parts, elems...)
	return midfs.NewFileURI(filepath.Join(parts...))
}

func (f *FS) Parent(uri midfs.URI) midfs.URI {
	cleanPath := filepath.Clean(uri.Path)
	if midfs.IsRootFilePath(cleanPath) {
		return midfs.NewFileURI(cleanPath)
	}
	return midfs.NewFileURI(filepath.Dir(cleanPath))
}

func (f *FS) Clean(uri midfs.URI) midfs.URI {
	cleanURI := uri.Clone()
	cleanURI.Scheme = midfs.SchemeFile
	cleanURI.Path = filepath.Clean(uri.Path)
	cleanURI.Host = ""
	cleanURI.Port = 0
	cleanURI.User = ""
	cleanURI.Query = nil
	return cleanURI
}

func (f *FS) Close() error {
	return nil
}

type atomicWriteCloser struct {
	*os.File
	target    string
	tempPath  string
	overwrite bool
}

func (w *atomicWriteCloser) Discard() error {
	if w.File != nil {
		_ = w.File.Close()
	}
	return os.Remove(w.tempPath)
}

func (w *atomicWriteCloser) Close() error {
	if err := w.File.Close(); err != nil {
		_ = os.Remove(w.tempPath)
		return err
	}
	if !w.overwrite {
		if _, err := os.Stat(w.target); err == nil {
			_ = os.Remove(w.tempPath)
			return iofs.ErrExist
		}
	}
	return os.Rename(w.tempPath, w.target)
}

func entryFromInfo(name string, uri midfs.URI, info iofs.FileInfo) midfs.Entry {
	entryType := midfs.EntryFile
	target := ""
	switch {
	case info.IsDir():
		entryType = midfs.EntryDir
	case info.Mode()&os.ModeSymlink != 0:
		entryType = midfs.EntrySymlink
		if linkTarget, err := os.Readlink(uri.Path); err == nil {
			target = linkTarget
		}
	}

	return midfs.Entry{
		Name:      name,
		Path:      uri.Path,
		URI:       uri,
		Type:      entryType,
		Size:      info.Size(),
		Mode:      info.Mode(),
		ModTime:   info.ModTime(),
		Target:    target,
		Readable:  info.Mode().Perm()&0o400 != 0,
		Writable:  info.Mode().Perm()&0o200 != 0,
		Hidden:    strings.HasPrefix(name, "."),
		IsArchive: false,
	}
}

var _ midfs.FileSystem = (*FS)(nil)

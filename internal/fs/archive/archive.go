package archive

import (
	"context"
	"fmt"
	"io"
	iofs "io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/mholt/archiver/v4"

	midfs "github.com/kooler/MiddayCommander/internal/fs"
)

type FS struct{}

func New() *FS {
	return &FS{}
}

func (f *FS) ID() string {
	return "archive"
}

func (f *FS) Scheme() midfs.Scheme {
	return midfs.SchemeArchive
}

func (f *FS) Capabilities() uint64 {
	return midfs.CapList | midfs.CapRead
}

func (f *FS) List(ctx context.Context, dir midfs.URI) ([]midfs.Entry, error) {
	archiveFS, err := openArchiveFS(ctx, dir.Path)
	if err != nil {
		return nil, err
	}

	currentEntry := entryPath(dir)
	dirEntries, err := archiveFS.ReadDir(currentEntry)
	if err != nil {
		return nil, err
	}

	entries := make([]midfs.Entry, 0, len(dirEntries))
	for _, dirEntry := range dirEntries {
		info, err := dirEntry.Info()
		if err != nil {
			return nil, err
		}
		childEntry := joinEntryPath(currentEntry, dirEntry.Name())
		entryURI := midfs.NewArchiveURI(dir.Path, childEntry)
		entries = append(entries, entryFromInfo(dirEntry.Name(), childEntry, entryURI, info, ""))
	}
	return entries, nil
}

func (f *FS) Stat(ctx context.Context, uri midfs.URI) (midfs.Entry, error) {
	archiveFS, err := openArchiveFS(ctx, uri.Path)
	if err != nil {
		return midfs.Entry{}, err
	}

	currentEntry := entryPath(uri)
	info, err := archiveFS.Stat(currentEntry)
	if err != nil {
		return midfs.Entry{}, err
	}

	name := path.Base(currentEntry)
	if currentEntry == "." {
		name = filepath.Base(uri.Path)
	}
	return entryFromInfo(name, currentEntry, uri, info, ""), nil
}

func (f *FS) Mkdir(ctx context.Context, uri midfs.URI, perm os.FileMode) error {
	return midfs.CapabilityError(uri, midfs.CapMkdir)
}

func (f *FS) Rename(ctx context.Context, from midfs.URI, to midfs.URI) error {
	return midfs.CapabilityError(from, midfs.CapRename)
}

func (f *FS) Remove(ctx context.Context, uri midfs.URI, recursive bool) error {
	return midfs.CapabilityError(uri, midfs.CapRemove)
}

func (f *FS) OpenReader(ctx context.Context, uri midfs.URI, opts midfs.OpenReadOptions) (io.ReadCloser, error) {
	archiveFS, err := openArchiveFS(ctx, uri.Path)
	if err != nil {
		return nil, err
	}

	reader, err := archiveFS.Open(entryPath(uri))
	if err != nil {
		return nil, err
	}
	if opts.Offset <= 0 {
		return reader, nil
	}

	seeker, ok := reader.(io.Seeker)
	if !ok {
		reader.Close()
		return nil, fmt.Errorf("archive reader for %s does not support seeking", uri.String())
	}
	if _, err := seeker.Seek(opts.Offset, io.SeekStart); err != nil {
		reader.Close()
		return nil, err
	}
	return reader, nil
}

func (f *FS) OpenWriter(ctx context.Context, uri midfs.URI, opts midfs.OpenWriteOptions) (io.WriteCloser, error) {
	return nil, midfs.CapabilityError(uri, midfs.CapWrite)
}

func (f *FS) Join(base midfs.URI, elems ...string) midfs.URI {
	joined := entryPath(base)
	for _, elem := range elems {
		joined = joinEntryPath(joined, elem)
	}
	return midfs.NewArchiveURI(base.Path, joined)
}

func (f *FS) Parent(uri midfs.URI) midfs.URI {
	currentEntry := entryPath(uri)
	if currentEntry == "." {
		return midfs.NewFileURI(filepath.Dir(uri.Path))
	}

	parent := path.Dir(currentEntry)
	if parent == "." || parent == "/" {
		parent = "."
	}
	return midfs.NewArchiveURI(uri.Path, parent)
}

func (f *FS) Clean(uri midfs.URI) midfs.URI {
	cleanURI := uri.Clone()
	cleanURI.Scheme = midfs.SchemeArchive
	cleanURI.Path = filepath.Clean(uri.Path)
	cleanURI.Host = ""
	cleanURI.Port = 0
	cleanURI.User = ""
	cleanURI.Query = map[string]string{
		"entry": entryPath(uri),
	}
	return cleanURI
}

func (f *FS) Close() error {
	return nil
}

func IsArchive(path string) bool {
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()

	format, _, err := archiver.Identify(context.Background(), path, file)
	if err != nil {
		return false
	}
	_, ok := format.(archiver.Extraction)
	return ok
}

func openArchiveFS(ctx context.Context, archivePath string) (archiver.ArchiveFS, error) {
	file, err := os.Open(archivePath)
	if err != nil {
		return archiver.ArchiveFS{}, err
	}
	defer file.Close()

	format, _, err := archiver.Identify(ctx, archivePath, file)
	if err != nil {
		return archiver.ArchiveFS{}, err
	}
	format = normalizeExtractor(format)

	extractor, ok := format.(archiver.Extraction)
	if !ok {
		return archiver.ArchiveFS{}, &iofs.PathError{Op: "open", Path: archivePath, Err: iofs.ErrInvalid}
	}

	return archiver.ArchiveFS{
		Path:    archivePath,
		Format:  extractor,
		Context: ctx,
	}, nil
}

func normalizeExtractor(format archiver.Format) archiver.Format {
	switch typed := format.(type) {
	case archiver.Archive:
		if typed.Compression == nil && typed.Extraction != nil {
			return typed.Extraction
		}
	case *archiver.Archive:
		if typed != nil && typed.Compression == nil && typed.Extraction != nil {
			return typed.Extraction
		}
	}
	return format
}

func entryPath(uri midfs.URI) string {
	entry := uri.QueryValue("entry")
	if entry == "" {
		return "."
	}
	entry = strings.TrimPrefix(strings.ReplaceAll(entry, "\\", "/"), "/")
	if entry == "" || entry == "." {
		return "."
	}
	return path.Clean(entry)
}

func joinEntryPath(base, elem string) string {
	base = entryPath(midfs.NewArchiveURI("", base))
	elem = strings.TrimPrefix(strings.ReplaceAll(elem, "\\", "/"), "/")
	if base == "." {
		return path.Clean(elem)
	}
	return path.Clean(path.Join(base, elem))
}

func entryFromInfo(name, entryPath string, uri midfs.URI, info iofs.FileInfo, target string) midfs.Entry {
	entryType := midfs.EntryFile
	switch {
	case info.IsDir():
		entryType = midfs.EntryDir
	case info.Mode()&os.ModeSymlink != 0:
		entryType = midfs.EntrySymlink
	}
	return midfs.Entry{
		Name:      name,
		Path:      entryPath,
		URI:       uri,
		Type:      entryType,
		Size:      info.Size(),
		Mode:      info.Mode(),
		ModTime:   info.ModTime(),
		Target:    target,
		Readable:  true,
		Writable:  false,
		Hidden:    strings.HasPrefix(name, "."),
		IsArchive: false,
	}
}

var _ midfs.FileSystem = (*FS)(nil)

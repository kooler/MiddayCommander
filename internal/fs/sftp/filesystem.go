package sftp

import (
	"context"
	"fmt"
	"io"
	iofs "io/fs"
	"os"
	"path"
	"strings"

	midfs "github.com/kooler/MiddayCommander/internal/fs"
	pkgsftp "github.com/pkg/sftp"
)

// FS exposes an SFTP-backed filesystem through the shared MiddayCommander
// filesystem abstraction.
type FS struct {
	pool *Pool
}

// New builds an SFTP filesystem backed by its own connection pool.
func New() *FS {
	return NewWithPool(nil)
}

// NewWithPool lets callers share a pool across multiple SFTP filesystem users.
func NewWithPool(pool *Pool) *FS {
	if pool == nil {
		pool = NewPool()
	}
	return &FS{pool: pool}
}

func (f *FS) ID() string {
	return "sftp"
}

func (f *FS) Scheme() midfs.Scheme {
	return midfs.SchemeSFTP
}

func (f *FS) Capabilities() uint64 {
	return midfs.CapList | midfs.CapRead | midfs.CapWrite | midfs.CapMkdir | midfs.CapRename | midfs.CapRemove
}

func (f *FS) List(ctx context.Context, dir midfs.URI) ([]midfs.Entry, error) {
	client, cleanURI, err := f.clientForURI(ctx, dir)
	if err != nil {
		return nil, err
	}

	infos, err := client.SFTP().ReadDir(cleanURI.Path)
	if err != nil {
		return nil, err
	}

	entries := make([]midfs.Entry, 0, len(infos))
	for _, info := range infos {
		childURI := f.Join(cleanURI, info.Name())
		target, err := linkTarget(client, childURI.Path, info)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entryFromInfo(info.Name(), childURI, info, target))
	}
	return entries, nil
}

func (f *FS) Stat(ctx context.Context, uri midfs.URI) (midfs.Entry, error) {
	client, cleanURI, err := f.clientForURI(ctx, uri)
	if err != nil {
		return midfs.Entry{}, err
	}

	info, err := client.SFTP().Lstat(cleanURI.Path)
	if err != nil {
		return midfs.Entry{}, err
	}

	target, err := linkTarget(client, cleanURI.Path, info)
	if err != nil {
		return midfs.Entry{}, err
	}
	return entryFromInfo(midfs.Base(cleanURI), cleanURI, info, target), nil
}

func (f *FS) Mkdir(ctx context.Context, uri midfs.URI, perm os.FileMode) error {
	client, cleanURI, err := f.clientForURI(ctx, uri)
	if err != nil {
		return err
	}
	if err := client.SFTP().Mkdir(cleanURI.Path); err != nil {
		return err
	}
	if perm != 0 {
		if err := client.SFTP().Chmod(cleanURI.Path, perm); err != nil {
			return err
		}
	}
	return nil
}

func (f *FS) Rename(ctx context.Context, from midfs.URI, to midfs.URI) error {
	client, cleanFrom, cleanTo, err := f.renameClientAndPaths(ctx, from, to)
	if err != nil {
		return err
	}
	return client.SFTP().Rename(cleanFrom.Path, cleanTo.Path)
}

func (f *FS) Remove(ctx context.Context, uri midfs.URI, recursive bool) error {
	client, cleanURI, err := f.clientForURI(ctx, uri)
	if err != nil {
		return err
	}
	return removePath(client.SFTP(), cleanURI.Path, recursive)
}

func (f *FS) OpenReader(ctx context.Context, uri midfs.URI, opts midfs.OpenReadOptions) (io.ReadCloser, error) {
	client, cleanURI, err := f.clientForURI(ctx, uri)
	if err != nil {
		return nil, err
	}

	file, err := client.SFTP().Open(cleanURI.Path)
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
	client, cleanURI, err := f.clientForURI(ctx, uri)
	if err != nil {
		return nil, err
	}

	perm := opts.Perm
	if perm == 0 {
		perm = 0o644
	}

	if opts.Atomic {
		return openAtomicWriter(client.SFTP(), cleanURI.Path, opts, perm)
	}

	flags := os.O_CREATE | os.O_WRONLY
	switch {
	case opts.Offset > 0:
	case opts.Overwrite:
		flags |= os.O_TRUNC
	default:
		flags |= os.O_EXCL
	}

	file, err := client.SFTP().OpenFile(cleanURI.Path, flags)
	if err != nil {
		return nil, err
	}
	if err := client.SFTP().Chmod(cleanURI.Path, perm); err != nil && !os.IsPermission(err) {
		_ = file.Close()
		return nil, err
	}
	if opts.Offset > 0 {
		if _, err := file.Seek(opts.Offset, io.SeekStart); err != nil {
			_ = file.Close()
			return nil, err
		}
	}
	return file, nil
}

func (f *FS) Join(base midfs.URI, elems ...string) midfs.URI {
	joined := cleanPath(base.Path)
	for _, elem := range elems {
		joined = path.Join(joined, strings.TrimSpace(strings.ReplaceAll(elem, "\\", "/")))
	}
	clone := f.Clean(base)
	clone.Path = cleanPath(joined)
	return clone
}

func (f *FS) Parent(uri midfs.URI) midfs.URI {
	cleanURI := f.Clean(uri)
	if cleanURI.Path == "/" {
		return cleanURI
	}

	cleanURI.Path = cleanPath(path.Dir(cleanURI.Path))
	return cleanURI
}

func (f *FS) Clean(uri midfs.URI) midfs.URI {
	cleanURI := uri.Clone()
	cleanURI.Scheme = midfs.SchemeSFTP
	cleanURI.Host = strings.TrimSpace(uri.Host)
	cleanURI.User = strings.TrimSpace(uri.User)
	cleanURI.Path = cleanPath(uri.Path)
	if cleanURI.Port < 0 {
		cleanURI.Port = 0
	}

	auth := strings.TrimSpace(cleanURI.QueryValue(QueryAuth))
	identityFile := strings.TrimSpace(cleanURI.QueryValue(QueryIdentityFile))
	knownHostsFile := strings.TrimSpace(cleanURI.QueryValue(QueryKnownHostsFile))

	if auth == "" && cleanURI.Query == nil {
		cleanURI.Query = nil
		return cleanURI
	}

	cleanURI.Query = map[string]string{}
	if auth != "" {
		cleanURI.Query[QueryAuth] = auth
	}
	if identityFile != "" {
		cleanURI.Query[QueryIdentityFile] = identityFile
	}
	if knownHostsFile != "" {
		cleanURI.Query[QueryKnownHostsFile] = knownHostsFile
	}
	if len(cleanURI.Query) == 0 {
		cleanURI.Query = nil
	}
	return cleanURI
}

func (f *FS) Close() error {
	if f == nil || f.pool == nil {
		return nil
	}
	return f.pool.Close()
}

func (f *FS) clientForURI(ctx context.Context, uri midfs.URI) (*Client, midfs.URI, error) {
	if f == nil || f.pool == nil {
		return nil, midfs.URI{}, fmt.Errorf("sftp filesystem is not initialized")
	}

	cleanURI := f.Clean(uri)
	opts, err := FromURI(cleanURI)
	if err != nil {
		return nil, cleanURI, err
	}

	client, err := f.pool.Client(ctx, opts)
	if err != nil {
		return nil, cleanURI, err
	}
	return client, cleanURI, nil
}

func (f *FS) renameClientAndPaths(ctx context.Context, from midfs.URI, to midfs.URI) (*Client, midfs.URI, midfs.URI, error) {
	cleanFrom := f.Clean(from)
	cleanTo := f.Clean(to)

	sameEndpoint, err := sameConnectionEndpoint(cleanFrom, cleanTo)
	if err != nil {
		return nil, midfs.URI{}, midfs.URI{}, err
	}
	if !sameEndpoint {
		return nil, midfs.URI{}, midfs.URI{}, fmt.Errorf("rename across sftp endpoints is not supported")
	}

	client, _, err := f.clientForURI(ctx, cleanFrom)
	if err != nil {
		return nil, midfs.URI{}, midfs.URI{}, err
	}
	return client, cleanFrom, cleanTo, nil
}

func linkTarget(client *Client, path string, info os.FileInfo) (string, error) {
	if client == nil || info == nil || info.Mode()&os.ModeSymlink == 0 {
		return "", nil
	}

	target, err := client.SFTP().ReadLink(path)
	if err != nil {
		return "", err
	}
	return target, nil
}

func entryFromInfo(name string, uri midfs.URI, info os.FileInfo, target string) midfs.Entry {
	entryType := midfs.EntryFile
	switch {
	case info.IsDir():
		entryType = midfs.EntryDir
	case info.Mode()&os.ModeSymlink != 0:
		entryType = midfs.EntrySymlink
	}

	perm := info.Mode().Perm()
	return midfs.Entry{
		Name:      name,
		Path:      uri.Path,
		URI:       uri,
		Type:      entryType,
		Size:      info.Size(),
		Mode:      info.Mode(),
		ModTime:   info.ModTime(),
		Target:    target,
		Readable:  perm&0o444 != 0,
		Writable:  perm&0o222 != 0,
		Hidden:    strings.HasPrefix(name, "."),
		IsArchive: false,
	}
}

func sameConnectionEndpoint(from midfs.URI, to midfs.URI) (bool, error) {
	fromOpts, err := FromURI(from)
	if err != nil {
		return false, err
	}
	toOpts, err := FromURI(to)
	if err != nil {
		return false, err
	}

	normalizedFrom, err := normalizeOptions(fromOpts)
	if err != nil {
		return false, err
	}
	normalizedTo, err := normalizeOptions(toOpts)
	if err != nil {
		return false, err
	}

	return connectionKey(normalizedFrom) == connectionKey(normalizedTo), nil
}

func removePath(client *pkgsftp.Client, target string, recursive bool) error {
	info, err := client.Lstat(target)
	if err != nil {
		return err
	}

	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return client.Remove(target)
	}
	if !recursive {
		return client.RemoveDirectory(target)
	}

	children, err := client.ReadDir(target)
	if err != nil {
		return err
	}
	for _, child := range children {
		childPath := path.Join(target, child.Name())
		if err := removePath(client, childPath, true); err != nil {
			return err
		}
	}
	return client.RemoveDirectory(target)
}

func openAtomicWriter(client *pkgsftp.Client, target string, opts midfs.OpenWriteOptions, perm os.FileMode) (io.WriteCloser, error) {
	tempPath := target + opts.TempExtension
	if opts.TempExtension == "" {
		tempPath = target + ".tmp"
	}

	file, err := client.OpenFile(tempPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC)
	if err != nil {
		return nil, err
	}
	if err := client.Chmod(tempPath, perm); err != nil && !os.IsPermission(err) {
		_ = file.Close()
		_ = client.Remove(tempPath)
		return nil, err
	}
	if opts.Offset > 0 {
		if _, err := file.Seek(opts.Offset, io.SeekStart); err != nil {
			_ = file.Close()
			_ = client.Remove(tempPath)
			return nil, err
		}
	}

	return &atomicWriteCloser{
		File:      file,
		client:    client,
		target:    target,
		tempPath:  tempPath,
		overwrite: opts.Overwrite,
	}, nil
}

type atomicWriteCloser struct {
	*pkgsftp.File
	client    *pkgsftp.Client
	target    string
	tempPath  string
	overwrite bool
}

func (w *atomicWriteCloser) Close() error {
	if err := w.File.Close(); err != nil {
		_ = w.client.Remove(w.tempPath)
		return err
	}

	if !w.overwrite {
		if _, err := w.client.Lstat(w.target); err == nil {
			_ = w.client.Remove(w.tempPath)
			return iofs.ErrExist
		} else if err != nil && !os.IsNotExist(err) {
			_ = w.client.Remove(w.tempPath)
			return err
		}
		return w.client.Rename(w.tempPath, w.target)
	}

	if err := w.client.PosixRename(w.tempPath, w.target); err == nil {
		return nil
	}

	if info, err := w.client.Lstat(w.target); err == nil {
		if info.IsDir() {
			_ = w.client.Remove(w.tempPath)
			return fmt.Errorf("%s already exists and is a directory", w.target)
		}
		if err := w.client.Remove(w.target); err != nil {
			_ = w.client.Remove(w.tempPath)
			return err
		}
	} else if err != nil && !os.IsNotExist(err) {
		_ = w.client.Remove(w.tempPath)
		return err
	}

	if err := w.client.Rename(w.tempPath, w.target); err != nil {
		_ = w.client.Remove(w.tempPath)
		return err
	}
	return nil
}

var (
	_ midfs.FileSystem = (*FS)(nil)
	_ io.WriteCloser   = (*atomicWriteCloser)(nil)
)

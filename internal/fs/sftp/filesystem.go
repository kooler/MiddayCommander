package sftp

import (
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"strings"

	midfs "github.com/kooler/MiddayCommander/internal/fs"
)

// FS exposes an SFTP-backed filesystem through the shared MiddayCommander
// filesystem abstraction.
type FS struct {
	pool *Pool
}

// New builds a read-only SFTP filesystem backed by its own connection pool.
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
	return midfs.CapList | midfs.CapRead
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
	return midfs.CapabilityError(f.Clean(uri), midfs.CapMkdir)
}

func (f *FS) Rename(ctx context.Context, from midfs.URI, to midfs.URI) error {
	return midfs.CapabilityError(f.Clean(from), midfs.CapRename)
}

func (f *FS) Remove(ctx context.Context, uri midfs.URI, recursive bool) error {
	return midfs.CapabilityError(f.Clean(uri), midfs.CapRemove)
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
	return nil, midfs.CapabilityError(f.Clean(uri), midfs.CapWrite)
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

var _ midfs.FileSystem = (*FS)(nil)

package fs

import (
	"context"
	"fmt"
	"io"
	"os"
)

type Router struct {
	filesystems map[Scheme]FileSystem
}

func NewRouter(filesystems ...FileSystem) *Router {
	router := &Router{
		filesystems: make(map[Scheme]FileSystem, len(filesystems)),
	}
	for _, filesystem := range filesystems {
		if filesystem == nil {
			continue
		}
		router.filesystems[filesystem.Scheme()] = filesystem
	}
	return router
}

func (r *Router) Resolve(uri URI) (FileSystem, URI, error) {
	filesystem, ok := r.filesystems[uri.Scheme]
	if !ok {
		return nil, URI{}, fmt.Errorf("no filesystem registered for scheme %q", uri.Scheme)
	}
	return filesystem, filesystem.Clean(uri), nil
}

func (r *Router) List(ctx context.Context, dir URI) ([]Entry, error) {
	filesystem, cleanURI, err := r.Resolve(dir)
	if err != nil {
		return nil, err
	}
	if !HasCapability(filesystem, CapList) {
		return nil, CapabilityError(cleanURI, CapList)
	}
	return filesystem.List(ctx, cleanURI)
}

func (r *Router) Stat(ctx context.Context, uri URI) (Entry, error) {
	filesystem, cleanURI, err := r.Resolve(uri)
	if err != nil {
		return Entry{}, err
	}
	return filesystem.Stat(ctx, cleanURI)
}

func (r *Router) Mkdir(ctx context.Context, uri URI, perm os.FileMode) error {
	filesystem, cleanURI, err := r.Resolve(uri)
	if err != nil {
		return err
	}
	if !HasCapability(filesystem, CapMkdir) {
		return CapabilityError(cleanURI, CapMkdir)
	}
	return filesystem.Mkdir(ctx, cleanURI, perm)
}

func (r *Router) Rename(ctx context.Context, from URI, to URI) error {
	filesystem, cleanFrom, err := r.Resolve(from)
	if err != nil {
		return err
	}
	if !HasCapability(filesystem, CapRename) {
		return CapabilityError(cleanFrom, CapRename)
	}

	destinationFS, cleanTo, err := r.Resolve(to)
	if err != nil {
		return err
	}
	if filesystem.ID() != destinationFS.ID() {
		return fmt.Errorf("rename across filesystems is not supported")
	}
	return filesystem.Rename(ctx, cleanFrom, cleanTo)
}

func (r *Router) Remove(ctx context.Context, uri URI, recursive bool) error {
	filesystem, cleanURI, err := r.Resolve(uri)
	if err != nil {
		return err
	}
	if !HasCapability(filesystem, CapRemove) {
		return CapabilityError(cleanURI, CapRemove)
	}
	return filesystem.Remove(ctx, cleanURI, recursive)
}

func (r *Router) OpenReader(ctx context.Context, uri URI, opts OpenReadOptions) (io.ReadCloser, error) {
	filesystem, cleanURI, err := r.Resolve(uri)
	if err != nil {
		return nil, err
	}
	if !HasCapability(filesystem, CapRead) {
		return nil, CapabilityError(cleanURI, CapRead)
	}
	return filesystem.OpenReader(ctx, cleanURI, opts)
}

func (r *Router) OpenWriter(ctx context.Context, uri URI, opts OpenWriteOptions) (io.WriteCloser, error) {
	filesystem, cleanURI, err := r.Resolve(uri)
	if err != nil {
		return nil, err
	}
	if !HasCapability(filesystem, CapWrite) {
		return nil, CapabilityError(cleanURI, CapWrite)
	}
	return filesystem.OpenWriter(ctx, cleanURI, opts)
}

func (r *Router) Join(base URI, elems ...string) URI {
	filesystem, cleanURI, err := r.Resolve(base)
	if err != nil {
		return base
	}
	return filesystem.Join(cleanURI, elems...)
}

func (r *Router) Parent(uri URI) URI {
	filesystem, cleanURI, err := r.Resolve(uri)
	if err != nil {
		return uri
	}
	return filesystem.Parent(cleanURI)
}

func (r *Router) Clean(uri URI) URI {
	filesystem, cleanURI, err := r.Resolve(uri)
	if err != nil {
		return uri
	}
	return filesystem.Clean(cleanURI)
}

func (r *Router) Close() error {
	for _, filesystem := range r.filesystems {
		if err := filesystem.Close(); err != nil {
			return err
		}
	}
	return nil
}

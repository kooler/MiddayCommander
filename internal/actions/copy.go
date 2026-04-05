package actions

import (
	"context"
	"fmt"
	"io"
	"os"

	midfs "github.com/kooler/MiddayCommander/internal/fs"
)

func Copy(ctx context.Context, router *midfs.Router, sources []midfs.URI, destDir midfs.URI, progressFn func(Progress)) error {
	if InvolvesSFTPTransfer(sources, destDir) {
		return UnsupportedSFTPTransferError("copy")
	}

	totalFiles, totalBytes, err := countFilesAndBytes(ctx, router, sources)
	if err != nil {
		return err
	}

	progress := Progress{
		Op:         OpCopy,
		TotalFiles: totalFiles,
		TotalBytes: totalBytes,
	}

	for _, source := range sources {
		entry, err := router.Stat(ctx, source)
		if err != nil {
			return fmt.Errorf("stat %s: %w", source.String(), err)
		}
		dest := router.Join(destDir, entry.Name)
		if err := copyEntry(ctx, router, entry, dest, &progress, progressFn); err != nil {
			return err
		}
	}

	return nil
}

func copyEntry(ctx context.Context, router *midfs.Router, source midfs.Entry, dest midfs.URI, progress *Progress, progressFn func(Progress)) error {
	if source.IsDir() {
		return copyDir(ctx, router, source, dest, progress, progressFn)
	}
	return copyFile(ctx, router, source, dest, progress, progressFn)
}

func copyFile(ctx context.Context, router *midfs.Router, source midfs.Entry, dest midfs.URI, progress *Progress, progressFn func(Progress)) error {
	progress.Current = source.Name
	if progressFn != nil {
		progressFn(*progress)
	}

	reader, err := router.OpenReader(ctx, source.URI, midfs.OpenReadOptions{})
	if err != nil {
		return fmt.Errorf("open %s: %w", source.URI.String(), err)
	}
	defer reader.Close()

	writer, err := router.OpenWriter(ctx, dest, midfs.OpenWriteOptions{
		Overwrite: true,
		Perm:      source.Mode.Perm(),
	})
	if err != nil {
		return fmt.Errorf("open %s: %w", dest.String(), err)
	}

	written, err := io.Copy(writer, reader)
	closeErr := writer.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return fmt.Errorf("copy %s -> %s: %w", source.URI.String(), dest.String(), err)
	}

	progress.DoneBytes += written
	progress.DoneFiles++
	if progressFn != nil {
		progressFn(*progress)
	}
	return nil
}

func copyDir(ctx context.Context, router *midfs.Router, source midfs.Entry, dest midfs.URI, progress *Progress, progressFn func(Progress)) error {
	if err := ensureDir(ctx, router, dest, source.Mode.Perm()); err != nil {
		return err
	}

	children, err := router.List(ctx, source.URI)
	if err != nil {
		return fmt.Errorf("list %s: %w", source.URI.String(), err)
	}
	for _, child := range children {
		target := router.Join(dest, child.Name)
		if err := copyEntry(ctx, router, child, target, progress, progressFn); err != nil {
			return err
		}
	}
	return nil
}

func ensureDir(ctx context.Context, router *midfs.Router, dir midfs.URI, perm os.FileMode) error {
	entry, err := router.Stat(ctx, dir)
	if err == nil {
		if entry.IsDir() {
			return nil
		}
		return fmt.Errorf("%s already exists and is not a directory", dir.String())
	}
	return router.Mkdir(ctx, dir, perm)
}

func countFilesAndBytes(ctx context.Context, router *midfs.Router, paths []midfs.URI) (int, int64, error) {
	var totalFiles int
	var totalBytes int64

	var walk func(uri midfs.URI) error
	walk = func(uri midfs.URI) error {
		entry, err := router.Stat(ctx, uri)
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			totalFiles++
			totalBytes += entry.Size
			return nil
		}

		children, err := router.List(ctx, uri)
		if err != nil {
			return err
		}
		for _, child := range children {
			if err := walk(child.URI); err != nil {
				return err
			}
		}
		return nil
	}

	for _, uri := range paths {
		if err := walk(uri); err != nil {
			return 0, 0, err
		}
	}

	return totalFiles, totalBytes, nil
}

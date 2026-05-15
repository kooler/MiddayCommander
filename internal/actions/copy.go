package actions

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// copyChunk is the max bytes copied between progress reports / cancel checks.
const copyChunk = 256 * 1024

// progressInterval throttles per-file byte progress reports.
const progressInterval = 80 * time.Millisecond

// Copy recursively copies sources to destDir, reporting progress via progressFn.
// The operation can be cancelled via ctx.
func Copy(ctx context.Context, sources []string, destDir string, progressFn func(Progress)) error {
	totalFiles, totalBytes := countFilesAndBytes(sources)
	p := Progress{
		Op:         OpCopy,
		TotalFiles: totalFiles,
		TotalBytes: totalBytes,
	}

	for _, src := range sources {
		if err := ctx.Err(); err != nil {
			return ErrCancelled
		}

		info, err := os.Lstat(src)
		if err != nil {
			return fmt.Errorf("stat %s: %w", src, err)
		}

		destPath := filepath.Join(destDir, filepath.Base(src))

		if info.IsDir() {
			if err := copyDir(ctx, src, destPath, &p, progressFn); err != nil {
				return err
			}
		} else {
			if err := copyFile(ctx, src, destPath, info, &p, progressFn); err != nil {
				return err
			}
		}
	}

	return nil
}

func copyFile(ctx context.Context, src, dst string, info fs.FileInfo, p *Progress, progressFn func(Progress)) error {
	p.Current = filepath.Base(src)
	p.FileTotalBytes = info.Size()
	p.FileDoneBytes = 0
	if progressFn != nil {
		progressFn(*p)
	}

	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open %s: %w", src, err)
	}
	defer srcFile.Close()

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
	if err != nil {
		return fmt.Errorf("create %s: %w", dst, err)
	}
	defer dstFile.Close()

	buf := make([]byte, copyChunk)
	lastReport := time.Now()

	for {
		if err := ctx.Err(); err != nil {
			return ErrCancelled
		}

		n, rerr := srcFile.Read(buf)
		if n > 0 {
			if _, werr := dstFile.Write(buf[:n]); werr != nil {
				return fmt.Errorf("write %s: %w", dst, werr)
			}
			p.FileDoneBytes += int64(n)
			p.DoneBytes += int64(n)
			if progressFn != nil && time.Since(lastReport) >= progressInterval {
				progressFn(*p)
				lastReport = time.Now()
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return fmt.Errorf("read %s: %w", src, rerr)
		}
	}

	p.DoneFiles++
	if progressFn != nil {
		progressFn(*p)
	}

	return nil
}

func copyDir(ctx context.Context, src, dst string, p *Progress, progressFn func(Progress)) error {
	if err := ctx.Err(); err != nil {
		return ErrCancelled
	}

	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return fmt.Errorf("mkdir %s: %w", dst, err)
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("readdir %s: %w", src, err)
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		info, err := entry.Info()
		if err != nil {
			return err
		}

		if entry.IsDir() {
			if err := copyDir(ctx, srcPath, dstPath, p, progressFn); err != nil {
				return err
			}
		} else {
			if err := copyFile(ctx, srcPath, dstPath, info, p, progressFn); err != nil {
				return err
			}
		}
	}

	return nil
}

func countFilesAndBytes(paths []string) (int, int64) {
	var files int
	var bytes int64

	for _, path := range paths {
		_ = filepath.WalkDir(path, func(_ string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			files++
			if info, err := d.Info(); err == nil {
				bytes += info.Size()
			}
			return nil
		})
	}

	return files, bytes
}

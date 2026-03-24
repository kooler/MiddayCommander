package actions

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// Copy recursively copies sources to destDir, reporting progress via progressFn.
func Copy(sources []string, destDir string, progressFn func(Progress)) error {
	totalFiles, totalBytes := countFilesAndBytes(sources)
	p := Progress{
		Op:         OpCopy,
		TotalFiles: totalFiles,
		TotalBytes: totalBytes,
	}

	for _, src := range sources {
		info, err := os.Lstat(src)
		if err != nil {
			return fmt.Errorf("stat %s: %w", src, err)
		}

		destPath := filepath.Join(destDir, filepath.Base(src))

		if info.IsDir() {
			if err := copyDir(src, destPath, &p, progressFn); err != nil {
				return err
			}
		} else {
			if err := copyFile(src, destPath, info, &p, progressFn); err != nil {
				return err
			}
		}
	}

	return nil
}

func copyFile(src, dst string, info fs.FileInfo, p *Progress, progressFn func(Progress)) error {
	p.Current = filepath.Base(src)
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

	written, err := io.Copy(dstFile, srcFile)
	if err != nil {
		return fmt.Errorf("copy %s -> %s: %w", src, dst, err)
	}

	p.DoneBytes += written
	p.DoneFiles++
	if progressFn != nil {
		progressFn(*p)
	}

	return nil
}

func copyDir(src, dst string, p *Progress, progressFn func(Progress)) error {
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
			if err := copyDir(srcPath, dstPath, p, progressFn); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath, info, p, progressFn); err != nil {
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

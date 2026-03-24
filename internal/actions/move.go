package actions

import (
	"fmt"
	"os"
	"path/filepath"
)

// Move moves sources to destDir. Tries os.Rename first (fast, same device),
// falls back to copy+delete for cross-device moves.
func Move(sources []string, destDir string, progressFn func(Progress)) error {
	for _, src := range sources {
		destPath := filepath.Join(destDir, filepath.Base(src))

		// Try rename first (instant if same filesystem)
		err := os.Rename(src, destPath)
		if err == nil {
			if progressFn != nil {
				progressFn(Progress{Op: OpMove, Current: filepath.Base(src), DoneFiles: 1, TotalFiles: 1})
			}
			continue
		}

		// Cross-device: copy then delete
		if err := Copy([]string{src}, destDir, progressFn); err != nil {
			return fmt.Errorf("move (copy phase) %s: %w", src, err)
		}
		if err := os.RemoveAll(src); err != nil {
			return fmt.Errorf("move (delete phase) %s: %w", src, err)
		}
	}

	return nil
}

// Rename renames a single file or directory.
func Rename(oldPath, newName string) error {
	dir := filepath.Dir(oldPath)
	newPath := filepath.Join(dir, newName)
	return os.Rename(oldPath, newPath)
}

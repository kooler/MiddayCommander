package actions

import (
	"os"
	"path/filepath"
)

// Delete removes all specified paths.
func Delete(paths []string, progressFn func(Progress)) error {
	p := Progress{
		Op:         OpDelete,
		TotalFiles: len(paths),
	}

	for _, path := range paths {
		p.Current = filepath.Base(path)
		if progressFn != nil {
			progressFn(p)
		}

		if err := os.RemoveAll(path); err != nil {
			return err
		}

		p.DoneFiles++
		if progressFn != nil {
			progressFn(p)
		}
	}

	return nil
}

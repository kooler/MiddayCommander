package actions

import (
	"context"
	"os"
	"path/filepath"
)

// Delete removes all specified paths.
func Delete(ctx context.Context, paths []string, progressFn func(Progress)) error {
	p := Progress{
		Op:         OpDelete,
		TotalFiles: len(paths),
	}

	for _, path := range paths {
		if err := ctx.Err(); err != nil {
			return ErrCancelled
		}
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

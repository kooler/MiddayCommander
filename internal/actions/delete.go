package actions

import (
	"context"

	midfs "github.com/kooler/MiddayCommander/internal/fs"
)

func Delete(ctx context.Context, router *midfs.Router, paths []midfs.URI, progressFn func(Progress)) error {
	progress := Progress{
		Op:         OpDelete,
		TotalFiles: len(paths),
	}

	for _, uri := range paths {
		progress.Current = midfs.Base(uri)
		if progressFn != nil {
			progressFn(progress)
		}
		if err := router.Remove(ctx, uri, true); err != nil {
			return err
		}
		progress.DoneFiles++
		if progressFn != nil {
			progressFn(progress)
		}
	}

	return nil
}

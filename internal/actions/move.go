package actions

import (
	"context"
	"fmt"

	midfs "github.com/kooler/MiddayCommander/internal/fs"
)

func Move(ctx context.Context, router *midfs.Router, sources []midfs.URI, destDir midfs.URI, progressFn func(Progress)) error {
	for _, source := range sources {
		entry, err := router.Stat(ctx, source)
		if err != nil {
			return fmt.Errorf("stat %s: %w", source.String(), err)
		}

		dest := router.Join(destDir, entry.Name)
		sourceFS, _, err := router.Resolve(source)
		if err != nil {
			return err
		}
		destFS, _, err := router.Resolve(dest)
		if err != nil {
			return err
		}

		if sourceFS.ID() == destFS.ID() && midfs.HasCapability(sourceFS, midfs.CapRename) {
			err := router.Rename(ctx, source, dest)
			if err == nil {
				if progressFn != nil {
					progressFn(Progress{Op: OpMove, Current: entry.Name, DoneFiles: 1, TotalFiles: 1})
				}
				continue
			}
		}
		if !midfs.HasCapability(sourceFS, midfs.CapRemove) {
			return midfs.CapabilityError(source, midfs.CapRemove)
		}

		if err := Copy(ctx, router, []midfs.URI{source}, destDir, progressFn); err != nil {
			return fmt.Errorf("move (copy phase) %s: %w", source.String(), err)
		}
		if err := router.Remove(ctx, source, true); err != nil {
			return fmt.Errorf("move (delete phase) %s: %w", source.String(), err)
		}
	}

	return nil
}

func Rename(ctx context.Context, router *midfs.Router, oldURI midfs.URI, newName string) error {
	parent := router.Parent(oldURI)
	newURI := router.Join(parent, newName)
	return router.Rename(ctx, oldURI, newURI)
}

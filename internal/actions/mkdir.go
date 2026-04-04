package actions

import (
	"context"

	midfs "github.com/kooler/MiddayCommander/internal/fs"
)

func Mkdir(ctx context.Context, router *midfs.Router, uri midfs.URI) error {
	return router.Mkdir(ctx, uri, 0o755)
}

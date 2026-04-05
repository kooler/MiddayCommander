package actions

import (
	"errors"
	"fmt"

	midfs "github.com/kooler/MiddayCommander/internal/fs"
)

// ErrSFTPTransfersDeferred marks copy/move flows that are intentionally held
// for the Phase 3 transfer engine.
var ErrSFTPTransfersDeferred = errors.New("remote transfers involving sftp land in Phase 3")

// InvolvesSFTPTransfer reports whether a copy/move operation touches an SFTP
// endpoint on either the source or destination side.
func InvolvesSFTPTransfer(sources []midfs.URI, dest midfs.URI) bool {
	if dest.Scheme == midfs.SchemeSFTP {
		return true
	}
	for _, source := range sources {
		if source.Scheme == midfs.SchemeSFTP {
			return true
		}
	}
	return false
}

// UnsupportedSFTPTransferError returns the user-facing Phase 2 boundary error
// for copy/move operations that involve SFTP.
func UnsupportedSFTPTransferError(op string) error {
	if op == "" {
		op = "transfer"
	}
	return fmt.Errorf("%s involving sftp is not available yet: %w", op, ErrSFTPTransfersDeferred)
}

package actions

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// Move moves sources to destDir. Tries os.Rename first (fast, same device),
// falls back to copy+delete for cross-device moves.
func Move(ctx context.Context, sources []string, destDir string, progressFn func(Progress)) error {
	// Precompute totals so the progress dialog has stable denominators even
	// when some sources are renamed and others fall back to copy.
	totalFiles, totalBytes := countFilesAndBytes(sources)
	agg := Progress{
		Op:         OpMove,
		TotalFiles: totalFiles,
		TotalBytes: totalBytes,
	}

	for _, src := range sources {
		if err := ctx.Err(); err != nil {
			return ErrCancelled
		}

		destPath := filepath.Join(destDir, filepath.Base(src))

		// Try rename first (instant if same filesystem).
		if err := os.Rename(src, destPath); err == nil {
			files, bytes := countFilesAndBytes([]string{destPath})
			agg.DoneFiles += files
			agg.DoneBytes += bytes
			agg.Current = filepath.Base(src)
			agg.FileTotalBytes = 0
			agg.FileDoneBytes = 0
			if progressFn != nil {
				progressFn(agg)
			}
			continue
		}

		// Cross-device: copy then delete. Run the copy with a nested Progress
		// but forward updates into the aggregate totals.
		srcFiles, srcBytes := countFilesAndBytes([]string{src})
		startDoneFiles := agg.DoneFiles
		startDoneBytes := agg.DoneBytes

		forward := func(p Progress) {
			agg.Current = p.Current
			agg.FileTotalBytes = p.FileTotalBytes
			agg.FileDoneBytes = p.FileDoneBytes
			agg.DoneFiles = startDoneFiles + p.DoneFiles
			agg.DoneBytes = startDoneBytes + p.DoneBytes
			if progressFn != nil {
				progressFn(agg)
			}
		}

		if err := Copy(ctx, []string{src}, destDir, forward); err != nil {
			return fmt.Errorf("move (copy phase) %s: %w", src, err)
		}
		if err := os.RemoveAll(src); err != nil {
			return fmt.Errorf("move (delete phase) %s: %w", src, err)
		}
		// Ensure aggregate reflects completion of this source even if Copy
		// finished without a final progress tick at 100%.
		agg.DoneFiles = startDoneFiles + srcFiles
		agg.DoneBytes = startDoneBytes + srcBytes
	}

	return nil
}

// MoveAs moves a single source to destPath (a full path, not a directory).
// Used for single-item move where the user may have renamed the target.
// Tries os.Rename first (fast, same device), falls back to copy+delete for
// cross-device moves.
func MoveAs(ctx context.Context, source, destPath string, progressFn func(Progress)) error {
	if err := ctx.Err(); err != nil {
		return ErrCancelled
	}

	if absEq(source, destPath) {
		return fmt.Errorf("source and destination are the same: %s", source)
	}

	totalFiles, totalBytes := countFilesAndBytes([]string{source})
	agg := Progress{
		Op:         OpMove,
		TotalFiles: totalFiles,
		TotalBytes: totalBytes,
	}

	// Try rename first (instant if same filesystem).
	if err := os.Rename(source, destPath); err == nil {
		agg.DoneFiles = totalFiles
		agg.DoneBytes = totalBytes
		agg.Current = filepath.Base(destPath)
		if progressFn != nil {
			progressFn(agg)
		}
		return nil
	}

	// Cross-device: copy then delete. Forward copy progress as a move op so
	// the dialog keeps showing "Moving".
	forward := func(p Progress) {
		agg.Current = p.Current
		agg.FileTotalBytes = p.FileTotalBytes
		agg.FileDoneBytes = p.FileDoneBytes
		agg.DoneFiles = p.DoneFiles
		agg.DoneBytes = p.DoneBytes
		if progressFn != nil {
			progressFn(agg)
		}
	}
	if err := CopyAs(ctx, source, destPath, forward); err != nil {
		return fmt.Errorf("move (copy phase) %s: %w", source, err)
	}
	if err := os.RemoveAll(source); err != nil {
		return fmt.Errorf("move (delete phase) %s: %w", source, err)
	}
	return nil
}

// Rename renames a single file or directory.
func Rename(oldPath, newName string) error {
	dir := filepath.Dir(oldPath)
	newPath := filepath.Join(dir, newName)
	return os.Rename(oldPath, newPath)
}

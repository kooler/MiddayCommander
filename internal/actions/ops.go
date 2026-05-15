package actions

import "errors"

// OpType identifies a file operation.
type OpType int

const (
	OpCopy OpType = iota
	OpMove
	OpDelete
	OpMkdir
	OpRename
)

// ErrCancelled is returned when an operation is aborted by the user.
var ErrCancelled = errors.New("operation cancelled")

// Progress reports the state of an ongoing file operation.
type Progress struct {
	Op         OpType
	TotalFiles int
	DoneFiles  int
	TotalBytes int64
	DoneBytes  int64

	// Current file being processed, and its byte-level progress.
	Current        string
	FileTotalBytes int64
	FileDoneBytes  int64

	Err error
}

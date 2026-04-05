package transfer

import (
	"time"

	"github.com/kooler/MiddayCommander/internal/actions"
	midfs "github.com/kooler/MiddayCommander/internal/fs"
)

type Operation string

const (
	OperationCopy Operation = "copy"
	OperationMove Operation = "move"
)

type ConflictPolicy string

const (
	ConflictAsk       ConflictPolicy = "ask"
	ConflictOverwrite ConflictPolicy = "overwrite"
	ConflictSkip      ConflictPolicy = "skip"
	ConflictRename    ConflictPolicy = "rename"
)

type VerifyMode string

const (
	VerifyNone   VerifyMode = "none"
	VerifySize   VerifyMode = "size"
	VerifySHA256 VerifyMode = "sha256"
)

type State string

const (
	StateQueued    State = "queued"
	StateRunning   State = "running"
	StateCompleted State = "completed"
	StateFailed    State = "failed"
)

type EventType string

const (
	EventQueued    EventType = "queued"
	EventStarted   EventType = "started"
	EventProgress  EventType = "progress"
	EventRetried   EventType = "retried"
	EventCompleted EventType = "completed"
	EventFailed    EventType = "failed"
)

// Request describes a transfer to enqueue.
type Request struct {
	Operation Operation
	Sources   []midfs.URI
	DestDir   midfs.URI
	Conflict  ConflictPolicy
	Verify    VerifyMode
	Retries   int
}

// Job is a normalized queued transfer request.
type Job struct {
	ID        string
	Operation Operation
	Sources   []midfs.URI
	DestDir   midfs.URI
	Conflict  ConflictPolicy
	Verify    VerifyMode
	Retries   int
}

type JobStatus struct {
	Job         Job
	State       State
	Progress    actions.Progress
	Error       string
	Attempt     int
	EnqueuedAt  time.Time
	StartedAt   time.Time
	CompletedAt time.Time
}

type Snapshot struct {
	Current *JobStatus
	Queue   []JobStatus
	Recent  []JobStatus
}

type Event struct {
	Type     EventType
	Job      JobStatus
	Snapshot Snapshot
}

func (j JobStatus) Percent() float64 {
	if j.Progress.TotalBytes > 0 {
		return clamp01(float64(j.Progress.DoneBytes) / float64(j.Progress.TotalBytes))
	}
	if j.Progress.TotalFiles > 0 {
		return clamp01(float64(j.Progress.DoneFiles) / float64(j.Progress.TotalFiles))
	}
	if j.State == StateCompleted {
		return 1
	}
	return 0
}

func (j JobStatus) TotalAttempts() int {
	if j.Job.Retries < 0 {
		return 1
	}
	return j.Job.Retries + 1
}

func clamp01(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}

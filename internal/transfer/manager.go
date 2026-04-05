package transfer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	iofs "io/fs"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kooler/MiddayCommander/internal/actions"
	"github.com/kooler/MiddayCommander/internal/audit"
	midfs "github.com/kooler/MiddayCommander/internal/fs"
)

var (
	ErrClosed                  = errors.New("transfer manager is closed")
	ErrConflictNeedsResolution = errors.New("transfer conflict requires interactive resolution")
	ErrVerificationFailed      = errors.New("transfer verification failed")
)

const retryBackoff = 350 * time.Millisecond

type discardWriteCloser interface {
	io.WriteCloser
	Discard() error
}

type Manager struct {
	router *midfs.Router
	audit  audit.Logger

	ctx    context.Context
	cancel context.CancelFunc

	mu            sync.Mutex
	pauseCond     *sync.Cond
	closed        bool
	paused        bool
	nextID        uint64
	status        map[string]*JobStatus
	queue         []string
	recent        []string
	current       string
	canceled      map[string]struct{}
	currentCancel context.CancelFunc

	jobs   chan Job
	events chan Event
	wg     sync.WaitGroup
}

func NewManager(router *midfs.Router, logger audit.Logger) *Manager {
	if logger == nil {
		logger = audit.NopLogger{}
	}
	ctx, cancel := context.WithCancel(context.Background())
	m := &Manager{
		router:   router,
		audit:    logger,
		ctx:      ctx,
		cancel:   cancel,
		status:   make(map[string]*JobStatus),
		canceled: make(map[string]struct{}),
		jobs:     make(chan Job, 32),
		events:   make(chan Event, 128),
	}
	m.pauseCond = sync.NewCond(&m.mu)
	m.wg.Add(1)
	go m.worker()
	return m
}

func (m *Manager) Events() <-chan Event {
	if m == nil {
		return nil
	}
	return m.events
}

func (m *Manager) Submit(req Request) (Job, error) {
	if m == nil {
		return Job{}, ErrClosed
	}
	if m.router == nil {
		return Job{}, fmt.Errorf("transfer manager router is nil")
	}
	job, err := normalizeRequest(req, m.nextJobID())
	if err != nil {
		return Job{}, err
	}

	status := &JobStatus{
		Job:        job,
		State:      StateQueued,
		EnqueuedAt: time.Now(),
	}

	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return Job{}, ErrClosed
	}
	m.status[job.ID] = status
	m.queue = append(m.queue, job.ID)
	snapshot := m.snapshotLocked()
	statusCopy := cloneStatus(status)
	m.mu.Unlock()

	select {
	case m.jobs <- job:
	case <-m.ctx.Done():
		return Job{}, ErrClosed
	}
	m.emit(Event{
		Type:     EventQueued,
		Job:      statusCopy,
		Snapshot: snapshot,
	})
	_ = m.audit.Record(context.Background(), audit.Event{
		Kind:      "transfer",
		JobID:     job.ID,
		Operation: string(job.Operation),
		Status:    string(StateQueued),
		Dest:      job.DestDir.String(),
		Message:   fmt.Sprintf("queued %d source(s)", len(job.Sources)),
	})
	return job, nil
}

func (m *Manager) Snapshot() Snapshot {
	if m == nil {
		return Snapshot{}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.snapshotLocked()
}

func (m *Manager) Close() error {
	if m == nil {
		return nil
	}

	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return nil
	}
	m.closed = true
	m.pauseCond.Broadcast()
	m.mu.Unlock()

	m.cancel()
	m.wg.Wait()
	close(m.events)
	return m.audit.Close()
}

func (m *Manager) worker() {
	defer m.wg.Done()

	for {
		if !m.waitIfResumed() {
			return
		}

		select {
		case <-m.ctx.Done():
			return
		case job, ok := <-m.jobs:
			if !ok {
				return
			}
			if m.consumeCanceled(job.ID) {
				continue
			}
			jobCtx, jobCancel := context.WithCancel(m.ctx)
			m.setCurrentCancel(job.ID, jobCancel)
			m.markStarted(job.ID)

			var finalErr error
			completed := false
			maxAttempts := job.Retries + 1
			for attempt := 1; attempt <= maxAttempts; attempt++ {
				finalErr = m.executeJob(jobCtx, job)
				if finalErr == nil {
					completed = true
					break
				}
				if attempt >= maxAttempts || !shouldRetry(finalErr) {
					break
				}

				if !m.markRetry(job.ID, attempt, maxAttempts, finalErr) {
					finalErr = m.ctx.Err()
					break
				}
			}
			jobCancel()
			m.clearCurrentCancel(job.ID)

			if completed {
				m.markFinished(job.ID, StateCompleted, nil)
				continue
			}
			if errors.Is(finalErr, context.Canceled) {
				m.markFinished(job.ID, StateCanceled, finalErr)
				continue
			}
			m.markFinished(job.ID, StateFailed, finalErr)
		}
	}
}

func (m *Manager) waitIfResumed() bool {
	if m == nil {
		return false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for m.paused && !m.closed {
		m.pauseCond.Wait()
	}
	return !m.closed
}

func (m *Manager) markStarted(jobID string) {
	m.mu.Lock()
	status := m.status[jobID]
	if status == nil {
		m.mu.Unlock()
		return
	}
	m.removeQueuedLocked(jobID)
	m.current = jobID
	status.State = StateRunning
	status.Error = ""
	status.Attempt = 1
	status.StartedAt = time.Now()
	snapshot := m.snapshotLocked()
	statusCopy := cloneStatus(status)
	m.mu.Unlock()

	m.emit(Event{Type: EventStarted, Job: statusCopy, Snapshot: snapshot})
	_ = m.audit.Record(context.Background(), audit.Event{
		Kind:      "transfer",
		JobID:     status.Job.ID,
		Operation: string(status.Job.Operation),
		Status:    string(StateRunning),
		Dest:      status.Job.DestDir.String(),
		Message:   "started",
	})
}

func (m *Manager) markRetry(jobID string, attempt int, maxAttempts int, err error) bool {
	m.mu.Lock()
	status := m.status[jobID]
	if status == nil {
		m.mu.Unlock()
		return false
	}

	status.Attempt = attempt + 1
	status.Error = fmt.Sprintf("attempt %d/%d failed: %v", attempt, maxAttempts, err)
	status.Progress.DoneFiles = 0
	status.Progress.DoneBytes = 0
	status.Progress.Err = nil
	status.Progress.Current = fmt.Sprintf("retrying %d/%d", status.Attempt, maxAttempts)
	snapshot := m.snapshotLocked()
	statusCopy := cloneStatus(status)
	m.mu.Unlock()

	m.emit(Event{Type: EventRetried, Job: statusCopy, Snapshot: snapshot})
	_ = m.audit.Record(context.Background(), audit.Event{
		Kind:      "transfer",
		JobID:     status.Job.ID,
		Operation: string(status.Job.Operation),
		Status:    "retrying",
		Dest:      status.Job.DestDir.String(),
		Message:   status.Error,
	})

	timer := time.NewTimer(time.Duration(attempt) * retryBackoff)
	defer timer.Stop()

	select {
	case <-m.ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func (m *Manager) markProgress(jobID string, progress actions.Progress) {
	m.mu.Lock()
	status := m.status[jobID]
	if status == nil {
		m.mu.Unlock()
		return
	}
	status.Progress = progress
	snapshot := m.snapshotLocked()
	statusCopy := cloneStatus(status)
	m.mu.Unlock()

	m.emit(Event{Type: EventProgress, Job: statusCopy, Snapshot: snapshot})
}

func (m *Manager) markFinished(jobID string, state State, err error) {
	m.mu.Lock()
	status := m.status[jobID]
	if status == nil {
		m.mu.Unlock()
		return
	}
	status.State = state
	status.CompletedAt = time.Now()
	if err != nil {
		if state == StateCanceled {
			status.Error = "canceled"
		} else {
			status.Error = err.Error()
		}
		status.Progress.Err = err
	} else {
		status.Error = ""
		status.Progress.Err = nil
	}
	if state == StateCanceled {
		status.Progress.Current = "canceled"
	}
	m.current = ""
	m.currentCancel = nil
	m.recent = append([]string{jobID}, m.recent...)
	if len(m.recent) > 8 {
		m.recent = m.recent[:8]
	}
	snapshot := m.snapshotLocked()
	statusCopy := cloneStatus(status)
	m.mu.Unlock()

	eventType := EventCompleted
	if state == StateFailed {
		eventType = EventFailed
	} else if state == StateCanceled {
		eventType = EventCanceled
	}
	m.emit(Event{Type: eventType, Job: statusCopy, Snapshot: snapshot})

	record := audit.Event{
		Kind:      "transfer",
		JobID:     status.Job.ID,
		Operation: string(status.Job.Operation),
		Status:    string(state),
		Dest:      status.Job.DestDir.String(),
	}
	if err != nil {
		if state == StateCanceled {
			record.Error = ""
			record.Message = "canceled"
		} else {
			record.Error = err.Error()
			record.Message = "failed"
		}
	} else {
		record.Message = "completed"
	}
	_ = m.audit.Record(context.Background(), record)
}

func (m *Manager) emit(event Event) {
	if m == nil {
		return
	}
	select {
	case m.events <- event:
	case <-m.ctx.Done():
	}
}

func (m *Manager) snapshotLocked() Snapshot {
	snapshot := Snapshot{Paused: m.paused}
	if current := m.status[m.current]; current != nil {
		currentCopy := cloneStatus(current)
		snapshot.Current = &currentCopy
	}
	for _, jobID := range m.queue {
		if status := m.status[jobID]; status != nil {
			snapshot.Queue = append(snapshot.Queue, cloneStatus(status))
		}
	}
	for _, jobID := range m.recent {
		if status := m.status[jobID]; status != nil {
			snapshot.Recent = append(snapshot.Recent, cloneStatus(status))
		}
	}
	return snapshot
}

func (m *Manager) removeQueuedLocked(jobID string) {
	for i, queuedID := range m.queue {
		if queuedID == jobID {
			m.queue = append(m.queue[:i], m.queue[i+1:]...)
			return
		}
	}
}

func cloneStatus(status *JobStatus) JobStatus {
	if status == nil {
		return JobStatus{}
	}
	clone := *status
	clone.Job.Sources = append([]midfs.URI(nil), status.Job.Sources...)
	return clone
}

func (m *Manager) Pause() Snapshot {
	if m == nil {
		return Snapshot{}
	}
	m.mu.Lock()
	if !m.closed {
		m.paused = true
	}
	snapshot := m.snapshotLocked()
	m.mu.Unlock()
	m.emit(Event{Type: EventPaused, Snapshot: snapshot})
	_ = m.audit.Record(context.Background(), audit.Event{
		Kind:    "transfer_queue",
		Status:  "paused",
		Message: "queue paused",
	})
	return snapshot
}

func (m *Manager) Resume() Snapshot {
	if m == nil {
		return Snapshot{}
	}
	m.mu.Lock()
	if !m.closed {
		m.paused = false
		m.pauseCond.Broadcast()
	}
	snapshot := m.snapshotLocked()
	m.mu.Unlock()
	m.emit(Event{Type: EventResumed, Snapshot: snapshot})
	_ = m.audit.Record(context.Background(), audit.Event{
		Kind:    "transfer_queue",
		Status:  "resumed",
		Message: "queue resumed",
	})
	return snapshot
}

func (m *Manager) TogglePause() Snapshot {
	if m == nil {
		return Snapshot{}
	}
	m.mu.Lock()
	paused := m.paused
	m.mu.Unlock()
	if paused {
		return m.Resume()
	}
	return m.Pause()
}

func (m *Manager) CancelCurrent() Snapshot {
	if m == nil {
		return Snapshot{}
	}
	m.mu.Lock()
	cancel := m.currentCancel
	if status := m.status[m.current]; status != nil {
		status.Error = "cancel requested"
		status.Progress.Current = "canceling..."
	}
	snapshot := m.snapshotLocked()
	currentID := m.current
	m.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if currentID != "" {
		_ = m.audit.Record(context.Background(), audit.Event{
			Kind:    "transfer_queue",
			JobID:   currentID,
			Status:  "cancel_requested",
			Message: "cancel current transfer",
		})
	}
	return snapshot
}

func (m *Manager) CancelQueued() Snapshot {
	if m == nil {
		return Snapshot{}
	}
	now := time.Now()

	m.mu.Lock()
	queuedIDs := append([]string(nil), m.queue...)
	m.queue = nil
	for _, jobID := range queuedIDs {
		status := m.status[jobID]
		if status == nil {
			continue
		}
		status.State = StateCanceled
		status.Error = "canceled before start"
		status.CompletedAt = now
		m.canceled[jobID] = struct{}{}
		m.recent = append([]string{jobID}, m.recent...)
		if len(m.recent) > 8 {
			m.recent = m.recent[:8]
		}
	}
	snapshot := m.snapshotLocked()
	canceledStatuses := make([]JobStatus, 0, len(queuedIDs))
	for _, jobID := range queuedIDs {
		if status := m.status[jobID]; status != nil {
			canceledStatuses = append(canceledStatuses, cloneStatus(status))
		}
	}
	m.mu.Unlock()

	for _, status := range canceledStatuses {
		m.emit(Event{Type: EventCanceled, Job: status, Snapshot: snapshot})
		_ = m.audit.Record(context.Background(), audit.Event{
			Kind:      "transfer",
			JobID:     status.Job.ID,
			Operation: string(status.Job.Operation),
			Status:    string(StateCanceled),
			Dest:      status.Job.DestDir.String(),
			Message:   "canceled before start",
		})
	}
	return snapshot
}

func (m *Manager) setCurrentCancel(jobID string, cancel context.CancelFunc) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.current == jobID || m.current == "" {
		m.currentCancel = cancel
	}
}

func (m *Manager) clearCurrentCancel(jobID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.current == jobID || m.current == "" {
		m.currentCancel = nil
	}
}

func (m *Manager) consumeCanceled(jobID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.canceled[jobID]; ok {
		delete(m.canceled, jobID)
		return true
	}
	return false
}

func normalizeRequest(req Request, id string) (Job, error) {
	if len(req.Sources) == 0 {
		return Job{}, fmt.Errorf("transfer requires at least one source")
	}
	if req.Operation != OperationCopy && req.Operation != OperationMove {
		return Job{}, fmt.Errorf("unsupported transfer operation %q", req.Operation)
	}
	if req.Retries < 0 {
		return Job{}, fmt.Errorf("retries must be zero or greater")
	}
	conflict := req.Conflict
	if conflict == "" {
		conflict = ConflictOverwrite
	}
	switch conflict {
	case ConflictAsk, ConflictOverwrite, ConflictSkip, ConflictRename:
	default:
		return Job{}, fmt.Errorf("unsupported conflict policy %q", req.Conflict)
	}

	verify := req.Verify
	if verify == "" {
		verify = VerifySize
	}
	switch verify {
	case VerifyNone, VerifySize, VerifySHA256:
	default:
		return Job{}, fmt.Errorf("unsupported verify mode %q", req.Verify)
	}

	job := Job{
		ID:        id,
		Operation: req.Operation,
		Sources:   append([]midfs.URI(nil), req.Sources...),
		DestDir:   req.DestDir,
		Conflict:  conflict,
		Verify:    verify,
		Retries:   req.Retries,
	}
	return job, nil
}

func (m *Manager) nextJobID() string {
	seq := atomic.AddUint64(&m.nextID, 1)
	return fmt.Sprintf("tr-%06d", seq)
}

func (m *Manager) executeJob(ctx context.Context, job Job) error {
	totalFiles, totalBytes, err := m.countFilesAndBytes(ctx, job.Sources)
	if err != nil {
		return err
	}

	progress := actions.Progress{
		TotalFiles: totalFiles,
		TotalBytes: totalBytes,
	}
	switch job.Operation {
	case OperationCopy:
		progress.Op = actions.OpCopy
	case OperationMove:
		progress.Op = actions.OpMove
	}

	for _, sourceURI := range job.Sources {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		entry, err := m.router.Stat(ctx, sourceURI)
		if err != nil {
			return fmt.Errorf("stat %s: %w", sourceURI.String(), err)
		}
		dest := m.router.Join(job.DestDir, entry.Name)

		switch job.Operation {
		case OperationCopy:
			if err := m.copyEntry(ctx, entry, dest, job, &progress); err != nil {
				return err
			}
		case OperationMove:
			if err := m.moveEntry(ctx, entry, dest, job, &progress); err != nil {
				return err
			}
		}
	}

	return nil
}

func (m *Manager) moveEntry(ctx context.Context, source midfs.Entry, dest midfs.URI, job Job, progress *actions.Progress) error {
	sourceFS, _, err := m.router.Resolve(source.URI)
	if err != nil {
		return err
	}
	destFS, _, err := m.router.Resolve(dest)
	if err != nil {
		return err
	}

	resolvedDest, skip, err := m.resolveConflict(ctx, dest, source, job.Conflict)
	if err != nil {
		return err
	}
	if skip {
		progress.Current = source.Name + " (skipped)"
		progress.DoneFiles++
		progress.DoneBytes += source.Size
		m.markProgress(job.ID, *progress)
		return nil
	}

	if sourceFS.ID() == destFS.ID() && midfs.HasCapability(sourceFS, midfs.CapRename) {
		if err := m.router.Rename(ctx, source.URI, resolvedDest); err == nil {
			progress.Current = source.Name
			progress.DoneFiles++
			progress.DoneBytes += source.Size
			m.markProgress(job.ID, *progress)
			return nil
		}
	}

	if err := m.copyEntry(ctx, source, resolvedDest, job, progress); err != nil {
		return err
	}
	if err := m.router.Remove(ctx, source.URI, true); err != nil {
		return fmt.Errorf("delete %s after move: %w", source.URI.String(), err)
	}
	return nil
}

func (m *Manager) copyEntry(ctx context.Context, source midfs.Entry, dest midfs.URI, job Job, progress *actions.Progress) error {
	if source.IsDir() {
		return m.copyDir(ctx, source, dest, job, progress)
	}
	return m.copyFile(ctx, source, dest, job, progress)
}

func (m *Manager) copyDir(ctx context.Context, source midfs.Entry, dest midfs.URI, job Job, progress *actions.Progress) error {
	destEntry, err := m.router.Stat(ctx, dest)
	if err == nil {
		if destEntry.IsDir() {
			dest = destEntry.URI
		} else {
			resolvedDest, skip, err := m.resolveConflict(ctx, dest, source, job.Conflict)
			if err != nil {
				return err
			}
			if skip {
				return nil
			}
			dest = resolvedDest
			if err := m.ensureDir(ctx, dest, source.Mode.Perm()); err != nil {
				return err
			}
		}
	} else if !osIsNotExist(err) {
		return err
	} else {
		if err := m.ensureDir(ctx, dest, source.Mode.Perm()); err != nil {
			return err
		}
	}

	children, err := m.router.List(ctx, source.URI)
	if err != nil {
		return fmt.Errorf("list %s: %w", source.URI.String(), err)
	}
	for _, child := range children {
		if err := m.copyEntry(ctx, child, m.router.Join(dest, child.Name), job, progress); err != nil {
			return err
		}
	}
	return nil
}

func (m *Manager) copyFile(ctx context.Context, source midfs.Entry, dest midfs.URI, job Job, progress *actions.Progress) error {
	resolvedDest, skip, err := m.resolveConflict(ctx, dest, source, job.Conflict)
	if err != nil {
		return err
	}
	if skip {
		progress.Current = source.Name + " (skipped)"
		progress.DoneFiles++
		progress.DoneBytes += source.Size
		m.markProgress(job.ID, *progress)
		return nil
	}

	progress.Current = source.Name
	m.markProgress(job.ID, *progress)

	reader, err := m.router.OpenReader(ctx, source.URI, midfs.OpenReadOptions{})
	if err != nil {
		return fmt.Errorf("open %s: %w", source.URI.String(), err)
	}
	defer reader.Close()

	writer, err := m.router.OpenWriter(ctx, resolvedDest, midfs.OpenWriteOptions{
		Atomic:        true,
		Overwrite:     job.Conflict == ConflictOverwrite,
		TempExtension: ".mdc.part",
		Perm:          source.Mode.Perm(),
	})
	if err != nil {
		return fmt.Errorf("open %s: %w", resolvedDest.String(), err)
	}

	written, copyErr := m.copyWithContext(ctx, writer, reader, job.ID, source.Name, progress)
	if copyErr != nil {
		if discardWriter, ok := writer.(discardWriteCloser); ok {
			_ = discardWriter.Discard()
		} else {
			_ = writer.Close()
		}
	} else {
		copyErr = writer.Close()
	}
	if copyErr != nil {
		return fmt.Errorf("copy %s -> %s: %w", source.URI.String(), resolvedDest.String(), copyErr)
	}

	if err := m.verifyFile(ctx, source, resolvedDest, written, job.Verify); err != nil {
		return err
	}

	progress.DoneBytes += written
	progress.DoneFiles++
	m.markProgress(job.ID, *progress)
	return nil
}

func (m *Manager) copyWithContext(ctx context.Context, writer io.Writer, reader io.Reader, jobID string, name string, progress *actions.Progress) (int64, error) {
	buf := make([]byte, 64*1024)
	baseDone := progress.DoneBytes
	var written int64

	for {
		select {
		case <-ctx.Done():
			return written, ctx.Err()
		default:
		}

		nr, readErr := reader.Read(buf)
		if nr > 0 {
			nw, writeErr := writer.Write(buf[:nr])
			written += int64(nw)
			progress.Current = name
			progress.DoneBytes = baseDone + written
			m.markProgress(jobID, *progress)
			if writeErr != nil {
				return written, writeErr
			}
			if nw != nr {
				return written, io.ErrShortWrite
			}
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				break
			}
			return written, readErr
		}
	}

	progress.DoneBytes = baseDone
	return written, nil
}

func (m *Manager) resolveConflict(ctx context.Context, dest midfs.URI, source midfs.Entry, policy ConflictPolicy) (midfs.URI, bool, error) {
	entry, err := m.router.Stat(ctx, dest)
	if err != nil {
		if osIsNotExist(err) {
			return dest, false, nil
		}
		return midfs.URI{}, false, err
	}

	switch policy {
	case ConflictOverwrite:
		if err := m.router.Remove(ctx, entry.URI, true); err != nil {
			return midfs.URI{}, false, err
		}
		return dest, false, nil
	case ConflictSkip:
		return dest, true, nil
	case ConflictRename:
		return m.availableName(ctx, dest, source.Name)
	case ConflictAsk:
		return midfs.URI{}, false, ErrConflictNeedsResolution
	default:
		return midfs.URI{}, false, fmt.Errorf("unsupported conflict policy %q", policy)
	}
}

func (m *Manager) availableName(ctx context.Context, dest midfs.URI, originalName string) (midfs.URI, bool, error) {
	parent := m.router.Parent(dest)
	base := originalName
	ext := ""
	if dot := strings.LastIndex(originalName, "."); dot > 0 {
		base = originalName[:dot]
		ext = originalName[dot:]
	}

	for i := 1; i < 1000; i++ {
		candidateName := fmt.Sprintf("%s (copy %d)%s", base, i, ext)
		candidate := m.router.Join(parent, candidateName)
		if _, err := m.router.Stat(ctx, candidate); err != nil {
			if osIsNotExist(err) {
				return candidate, false, nil
			}
			return midfs.URI{}, false, err
		}
	}
	return midfs.URI{}, false, fmt.Errorf("unable to find free destination name for %q", originalName)
}

func (m *Manager) verifyFile(ctx context.Context, source midfs.Entry, dest midfs.URI, written int64, mode VerifyMode) error {
	switch mode {
	case VerifyNone:
		return nil
	case VerifySize:
		destEntry, err := m.router.Stat(ctx, dest)
		if err != nil {
			return fmt.Errorf("verify %s: %w", dest.String(), err)
		}
		if destEntry.Size != source.Size || written != source.Size {
			return fmt.Errorf("%w: size mismatch for %s", ErrVerificationFailed, dest.String())
		}
		return nil
	case VerifySHA256:
		if err := m.verifyFile(ctx, source, dest, written, VerifySize); err != nil {
			return err
		}
		sourceDigest, err := m.digest(ctx, source.URI)
		if err != nil {
			return err
		}
		destDigest, err := m.digest(ctx, dest)
		if err != nil {
			return err
		}
		if sourceDigest != destDigest {
			return fmt.Errorf("%w: sha256 mismatch for %s", ErrVerificationFailed, dest.String())
		}
		return nil
	default:
		return fmt.Errorf("unsupported verify mode %q", mode)
	}
}

func (m *Manager) digest(ctx context.Context, uri midfs.URI) (string, error) {
	reader, err := m.router.OpenReader(ctx, uri, midfs.OpenReadOptions{})
	if err != nil {
		return "", err
	}
	defer reader.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, reader); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func (m *Manager) countFilesAndBytes(ctx context.Context, sources []midfs.URI) (int, int64, error) {
	var totalFiles int
	var totalBytes int64

	var walk func(uri midfs.URI) error
	walk = func(uri midfs.URI) error {
		entry, err := m.router.Stat(ctx, uri)
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			totalFiles++
			totalBytes += entry.Size
			return nil
		}

		children, err := m.router.List(ctx, uri)
		if err != nil {
			return err
		}
		for _, child := range children {
			if err := walk(child.URI); err != nil {
				return err
			}
		}
		return nil
	}

	for _, source := range sources {
		if err := walk(source); err != nil {
			return 0, 0, err
		}
	}
	return totalFiles, totalBytes, nil
}

func (m *Manager) ensureDir(ctx context.Context, dir midfs.URI, perm iofs.FileMode) error {
	entry, err := m.router.Stat(ctx, dir)
	if err == nil {
		if entry.IsDir() {
			return nil
		}
		return fmt.Errorf("%s already exists and is not a directory", dir.String())
	}
	if !osIsNotExist(err) {
		return err
	}
	if perm == 0 {
		perm = 0o755
	}
	return m.router.Mkdir(ctx, dir, perm)
}

func osIsNotExist(err error) bool {
	return err != nil && (errors.Is(err, os.ErrNotExist) || errors.Is(err, iofs.ErrNotExist))
}

func shouldRetry(err error) bool {
	switch {
	case err == nil:
		return false
	case errors.Is(err, context.Canceled), errors.Is(err, ErrClosed), errors.Is(err, ErrConflictNeedsResolution):
		return false
	default:
		return true
	}
}

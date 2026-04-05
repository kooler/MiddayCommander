package audit

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Logger records transfer and remote-operation audit events.
type Logger interface {
	Record(context.Context, Event) error
	Close() error
}

// Event is a structured audit record written as JSON lines.
type Event struct {
	Time      time.Time         `json:"time"`
	Kind      string            `json:"kind"`
	JobID     string            `json:"job_id,omitempty"`
	Operation string            `json:"operation,omitempty"`
	Status    string            `json:"status,omitempty"`
	Source    string            `json:"source,omitempty"`
	Dest      string            `json:"dest,omitempty"`
	Message   string            `json:"message,omitempty"`
	Error     string            `json:"error,omitempty"`
	Fields    map[string]string `json:"fields,omitempty"`
}

// NopLogger drops every audit record.
type NopLogger struct{}

func (NopLogger) Record(context.Context, Event) error { return nil }
func (NopLogger) Close() error                        { return nil }

// FileLogger appends audit events as JSON lines.
type FileLogger struct {
	mu   sync.Mutex
	file *os.File
}

// NewFileLogger opens or creates a JSONL audit log file.
func NewFileLogger(path string) (*FileLogger, error) {
	if path == "" {
		return nil, os.ErrInvalid
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	return &FileLogger{file: file}, nil
}

func (l *FileLogger) Record(_ context.Context, event Event) error {
	if l == nil || l.file == nil {
		return nil
	}
	if event.Time.IsZero() {
		event.Time = time.Now().UTC()
	}

	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	if _, err := l.file.Write(append(data, '\n')); err != nil {
		return err
	}
	return l.file.Sync()
}

func (l *FileLogger) Close() error {
	if l == nil || l.file == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	err := l.file.Close()
	l.file = nil
	return err
}

var (
	_ Logger = (*FileLogger)(nil)
	_ Logger = NopLogger{}
)

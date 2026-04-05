package audit_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/kooler/MiddayCommander/internal/audit"
)

func TestFileLoggerWritesJSONLines(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "audit.log")
	logger, err := audit.NewFileLogger(path)
	if err != nil {
		t.Fatalf("NewFileLogger() error = %v", err)
	}
	defer logger.Close()

	event := audit.Event{
		Kind:      "transfer",
		JobID:     "job-1",
		Operation: "copy",
		Status:    "completed",
		Source:    "file:///tmp/source.txt",
		Dest:      "sftp://demo@example.test/tmp",
	}
	if err := logger.Record(context.Background(), event); err != nil {
		t.Fatalf("Record() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if len(data) == 0 {
		t.Fatal("audit log is empty")
	}

	var decoded audit.Event
	if err := json.Unmarshal(data[:len(data)-1], &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if decoded.JobID != event.JobID {
		t.Fatalf("decoded.JobID = %q, want %q", decoded.JobID, event.JobID)
	}
	if decoded.Status != event.Status {
		t.Fatalf("decoded.Status = %q, want %q", decoded.Status, event.Status)
	}
	if decoded.Time.IsZero() {
		t.Fatal("decoded.Time = zero, want non-zero")
	}
}

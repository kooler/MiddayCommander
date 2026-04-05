package audit_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/kooler/MiddayCommander/internal/audit"
)

func TestReadRecentReturnsNewestFirst(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.log")
	logger, err := audit.NewFileLogger(path)
	if err != nil {
		t.Fatalf("NewFileLogger() error = %v", err)
	}
	defer logger.Close()

	for _, event := range []audit.Event{
		{Kind: "transfer", JobID: "1", Status: "queued", Message: "first"},
		{Kind: "transfer", JobID: "2", Status: "running", Message: "second"},
		{Kind: "transfer", JobID: "3", Status: "completed", Message: "third"},
	} {
		if err := logger.Record(context.Background(), event); err != nil {
			t.Fatalf("Record() error = %v", err)
		}
	}

	events, err := audit.ReadRecent(path, 2)
	if err != nil {
		t.Fatalf("ReadRecent() error = %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("len(events) = %d, want 2", len(events))
	}
	if events[0].JobID != "3" || events[1].JobID != "2" {
		t.Fatalf("job ids = [%s %s], want [3 2]", events[0].JobID, events[1].JobID)
	}
}

func TestReadRecentMissingFileReturnsEmpty(t *testing.T) {
	events, err := audit.ReadRecent(filepath.Join(t.TempDir(), "missing.log"), 10)
	if err != nil {
		t.Fatalf("ReadRecent(missing) error = %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("len(events) = %d, want 0", len(events))
	}
}

package actions

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
)

func TestCopyReportsProgress(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	for _, name := range []string{"a", "b"} {
		if err := os.WriteFile(filepath.Join(src, name), make([]byte, 1<<16), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	var lastBytes int64
	var calls int32
	progressFn := func(p Progress) {
		atomic.AddInt32(&calls, 1)
		if p.DoneBytes > 0 {
			lastBytes = p.DoneBytes
		}
		if p.TotalFiles != 2 {
			t.Errorf("want TotalFiles=2, got %d", p.TotalFiles)
		}
	}

	if err := Copy(context.Background(), []string{src}, dst, progressFn); err != nil {
		t.Fatalf("copy: %v", err)
	}
	if lastBytes != 2*(1<<16) {
		t.Errorf("want %d bytes reported, got %d", 2*(1<<16), lastBytes)
	}
	if atomic.LoadInt32(&calls) == 0 {
		t.Error("progressFn was never called")
	}
}

func TestCopyCancel(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	// One large file so cancellation has time to kick in.
	if err := os.WriteFile(filepath.Join(src, "big"), make([]byte, 8<<20), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before starting

	err := Copy(ctx, []string{src}, dst, nil)
	if !errors.Is(err, ErrCancelled) {
		t.Fatalf("want ErrCancelled, got %v", err)
	}
}

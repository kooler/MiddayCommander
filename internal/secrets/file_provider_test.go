package secrets_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/kooler/MiddayCommander/internal/secrets"
)

func TestFileProviderStoreLoadDelete(t *testing.T) {
	t.Parallel()

	provider, err := secrets.NewFileProvider(filepath.Join(t.TempDir(), "secrets.json"), []byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatalf("NewFileProvider() error = %v", err)
	}

	if err := provider.Store(context.Background(), "demo", []byte("hunter2")); err != nil {
		t.Fatalf("Store() error = %v", err)
	}

	value, err := provider.Load(context.Background(), "demo")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if string(value) != "hunter2" {
		t.Fatalf("Load() value = %q, want %q", string(value), "hunter2")
	}

	if err := provider.Delete(context.Background(), "demo"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err := provider.Load(context.Background(), "demo"); !os.IsNotExist(err) {
		t.Fatalf("Load() after Delete error = %v, want not-exist", err)
	}
}

func TestFileProviderRejectsInvalidKeyLength(t *testing.T) {
	t.Parallel()

	if _, err := secrets.NewFileProvider(filepath.Join(t.TempDir(), "secrets.json"), []byte("short")); err == nil {
		t.Fatal("NewFileProvider() error = nil, want invalid key length failure")
	}
}

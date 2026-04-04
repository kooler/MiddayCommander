package bookmarks_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kooler/MiddayCommander/internal/bookmarks"
	midfs "github.com/kooler/MiddayCommander/internal/fs"
)

func TestLoadStoreMigratesLegacyPathBookmarks(t *testing.T) {
	configRoot := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configRoot)

	legacy := map[string]any{
		"bookmarks": []map[string]any{
			{
				"path":      "/tmp/example",
				"name":      "Example",
				"count":     3,
				"last_used": time.Now().UTC().Format(time.RFC3339Nano),
			},
		},
	}
	data, err := json.Marshal(legacy)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	storeFile := filepath.Join(configRoot, "mdc", "bookmarks.json")
	if err := os.MkdirAll(filepath.Dir(storeFile), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(storeFile, data, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	store := bookmarks.LoadStore()
	if len(store.Bookmarks) != 1 {
		t.Fatalf("len(Bookmarks) = %d, want 1", len(store.Bookmarks))
	}

	wantURI := midfs.NewFileURI("/tmp/example").String()
	if store.Bookmarks[0].URI != wantURI {
		t.Fatalf("Bookmark.URI = %q, want %q", store.Bookmarks[0].URI, wantURI)
	}

	if err := store.Save(); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	persisted, err := os.ReadFile(storeFile)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !strings.Contains(string(persisted), "\"uri\"") {
		t.Fatalf("saved bookmarks.json does not contain uri field: %s", string(persisted))
	}
	if strings.Contains(string(persisted), "\"path\"") {
		t.Fatalf("saved bookmarks.json still contains legacy path field: %s", string(persisted))
	}
}

package dialogs

import (
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	profilesstore "github.com/kooler/MiddayCommander/internal/profiles"
)

func TestProfilesModelSelectsProfileAsSFTPUri(t *testing.T) {
	store := loadProfilesStoreForTest(t, `
[[profiles]]
name = "alpha"
host = "alpha.example.test"
user = "alice"
path = "/srv/data"
`)

	model := NewProfiles(store, 80, 24)
	_, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	msg, ok := cmd().(ProfileSelectMsg)
	if !ok {
		t.Fatalf("Update(enter) message = %T, want ProfileSelectMsg", cmd())
	}

	if msg.Profile.Name != "alpha" {
		t.Fatalf("Profile.Name = %q, want alpha", msg.Profile.Name)
	}
	if msg.URI.Scheme != "sftp" {
		t.Fatalf("URI.Scheme = %q, want sftp", msg.URI.Scheme)
	}
	if msg.URI.Host != "alpha.example.test" || msg.URI.User != "alice" || msg.URI.Path != "/srv/data" {
		t.Fatalf("URI = %#v", msg.URI)
	}
}

func TestProfilesModelFiltersItems(t *testing.T) {
	store := loadProfilesStoreForTest(t, `
[[profiles]]
name = "alpha"
host = "alpha.example.test"
user = "alice"

[[profiles]]
name = "backup"
host = "backup.example.test"
user = "bob"
`)

	model := NewProfiles(store, 80, 24)
	model, _ = model.Update(keyRunes("f"))
	model, _ = model.Update(keyRunes("b"))

	if len(model.items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(model.items))
	}
	if model.items[0].Name != "backup" {
		t.Fatalf("items[0].Name = %q, want backup", model.items[0].Name)
	}
}

func keyRunes(value string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(value)}
}

func loadProfilesStoreForTest(t *testing.T, data string) *profilesstore.Store {
	t.Helper()

	path := filepath.Join(t.TempDir(), "profiles.toml")
	if err := osWriteFile(path, data); err != nil {
		t.Fatalf("write profiles.toml: %v", err)
	}

	store, err := profilesstore.LoadPath(path)
	if err != nil {
		t.Fatalf("LoadPath() error = %v", err)
	}
	return store
}

func osWriteFile(path string, data string) error {
	return os.WriteFile(path, []byte(data), 0o644)
}

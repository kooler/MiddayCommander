package profiles_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kooler/MiddayCommander/internal/profiles"
)

func TestLoadStoreAppliesDefaults(t *testing.T) {
	configRoot := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configRoot)

	profilesFile := filepath.Join(configRoot, "mdc", "profiles.toml")
	if err := os.MkdirAll(filepath.Dir(profilesFile), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	data := []byte(`
[[profiles]]
name = "lab"
host = "files.example.test"
user = "demo"
`)
	if err := os.WriteFile(profilesFile, data, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	store, err := profiles.LoadStore()
	if err != nil {
		t.Fatalf("LoadStore() error = %v", err)
	}

	all := store.All()
	if len(all) != 1 {
		t.Fatalf("len(All()) = %d, want 1", len(all))
	}

	profile := all[0]
	if profile.Port != profiles.DefaultPort {
		t.Fatalf("Port = %d, want %d", profile.Port, profiles.DefaultPort)
	}
	if profile.Path != "/" {
		t.Fatalf("Path = %q, want /", profile.Path)
	}
	if profile.Auth != profiles.AuthAgent {
		t.Fatalf("Auth = %q, want %q", profile.Auth, profiles.AuthAgent)
	}
}

func TestLoadPathRejectsMissingRequiredFields(t *testing.T) {
	profilesFile := filepath.Join(t.TempDir(), "profiles.toml")
	data := []byte(`
[[profiles]]
name = "broken"
user = "demo"
`)
	if err := os.WriteFile(profilesFile, data, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, err := profiles.LoadPath(profilesFile)
	if err == nil {
		t.Fatal("LoadPath() error = nil, want missing host error")
	}
	if !strings.Contains(err.Error(), "host is required") {
		t.Fatalf("LoadPath() error = %v, want missing host", err)
	}
}

func TestLoadPathRejectsDuplicateNames(t *testing.T) {
	profilesFile := filepath.Join(t.TempDir(), "profiles.toml")
	data := []byte(`
[[profiles]]
name = "lab"
host = "one.example.test"
user = "demo"

[[profiles]]
name = "lab"
host = "two.example.test"
user = "demo"
`)
	if err := os.WriteFile(profilesFile, data, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, err := profiles.LoadPath(profilesFile)
	if err == nil {
		t.Fatal("LoadPath() error = nil, want duplicate name error")
	}
	if !strings.Contains(err.Error(), "duplicate profile name") {
		t.Fatalf("LoadPath() error = %v, want duplicate name", err)
	}
}

func TestLoadPathPreservesKnownHostsOverride(t *testing.T) {
	profilesFile := filepath.Join(t.TempDir(), "profiles.toml")
	data := []byte(`
[[profiles]]
name = "deploy"
host = "deploy.example.test"
user = "ops"
auth = "key"
identity_file = "~/.ssh/id_ed25519"
known_hosts_file = "/tmp/mdc-known-hosts"
path = "/srv/releases"
`)
	if err := os.WriteFile(profilesFile, data, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	store, err := profiles.LoadPath(profilesFile)
	if err != nil {
		t.Fatalf("LoadPath() error = %v", err)
	}

	profile, ok := store.Find("deploy")
	if !ok {
		t.Fatal("Find(deploy) = false, want true")
	}
	if profile.KnownHostsFile != "/tmp/mdc-known-hosts" {
		t.Fatalf("KnownHostsFile = %q, want /tmp/mdc-known-hosts", profile.KnownHostsFile)
	}
	if profile.Auth != profiles.AuthKey {
		t.Fatalf("Auth = %q, want %q", profile.Auth, profiles.AuthKey)
	}
}

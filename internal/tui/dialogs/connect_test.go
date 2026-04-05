package dialogs

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	midfs "github.com/kooler/MiddayCommander/internal/fs"
	profilesstore "github.com/kooler/MiddayCommander/internal/profiles"
)

func TestConnectModelBuildsSFTPUri(t *testing.T) {
	model := NewConnect(midfs.URI{}, 80, 24)
	model.setFieldValue(connectFieldHost, "files.example.test")
	model.setFieldValue(connectFieldPort, "2222")
	model.setFieldValue(connectFieldUser, "alice")
	model.setFieldValue(connectFieldPath, "/srv/data")
	model.setFieldValue(connectFieldAuth, profilesstore.AuthAgent)

	profile, uri, err := model.buildProfileURI()
	if err != nil {
		t.Fatalf("buildProfileURI() error = %v", err)
	}
	if profile.Host != "files.example.test" || profile.Port != 2222 || profile.User != "alice" {
		t.Fatalf("profile = %#v", profile)
	}
	if uri.Scheme != midfs.SchemeSFTP || uri.Host != "files.example.test" || uri.Path != "/srv/data" {
		t.Fatalf("uri = %#v", uri)
	}
}

func TestConnectModelValidatesKeyAuthIdentity(t *testing.T) {
	model := NewConnect(midfs.URI{}, 80, 24)
	model.setFieldValue(connectFieldHost, "files.example.test")
	model.setFieldValue(connectFieldUser, "alice")
	model.setFieldValue(connectFieldAuth, profilesstore.AuthKey)

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Fatalf("Update(enter) cmd = %v, want nil on validation failure", cmd)
	}
	if !strings.Contains(updated.errText, "identity_file") {
		t.Fatalf("errText = %q, want identity_file guidance", updated.errText)
	}
}

func TestProfilesModelCanOpenManualConnect(t *testing.T) {
	store := loadProfilesStoreForTest(t, `
[[profiles]]
name = "alpha"
host = "alpha.example.test"
user = "alice"
`)

	model := NewProfiles(store, 80, 24)
	_, cmd := model.Update(keyRunes("n"))
	msg, ok := cmd().(ConnectOpenMsg)
	if !ok {
		t.Fatalf("Update(n) message = %T, want ConnectOpenMsg", cmd())
	}
	_ = msg
}

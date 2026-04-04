package sftp_test

import (
	"testing"

	midfs "github.com/kooler/MiddayCommander/internal/fs"
	sftpfs "github.com/kooler/MiddayCommander/internal/fs/sftp"
	"github.com/kooler/MiddayCommander/internal/profiles"
)

func TestFromProfileBuildsCanonicalURI(t *testing.T) {
	profile := profiles.Profile{
		Name:           "lab",
		Host:           "files.example.test",
		User:           "demo",
		Path:           "home/demo",
		Auth:           profiles.AuthAgent,
		KnownHostsFile: "~/.ssh/known_hosts",
	}

	options, err := sftpfs.FromProfile(profile)
	if err != nil {
		t.Fatalf("FromProfile() error = %v", err)
	}

	uri := options.URI()
	if got := uri.String(); got != "sftp://demo@files.example.test:22/home/demo?auth=agent&known_hosts_file=~%2F.ssh%2Fknown_hosts" {
		t.Fatalf("URI().String() = %q", got)
	}
}

func TestFromURIAppliesDefaultAuthAndPort(t *testing.T) {
	uri := midfs.MustParseURI("sftp://demo@files.example.test/var/data")

	options, err := sftpfs.FromURI(uri)
	if err != nil {
		t.Fatalf("FromURI() error = %v", err)
	}

	if options.Port != profiles.DefaultPort {
		t.Fatalf("Port = %d, want %d", options.Port, profiles.DefaultPort)
	}
	if options.Auth != profiles.AuthAgent {
		t.Fatalf("Auth = %q, want %q", options.Auth, profiles.AuthAgent)
	}
	if options.Path != "/var/data" {
		t.Fatalf("Path = %q, want /var/data", options.Path)
	}
}

func TestFromURIRoundTripsKeyAuthOptions(t *testing.T) {
	uri := midfs.MustParseURI("sftp://deploy@files.example.test:2222/srv/releases?auth=key&identity_file=%7E%2F.ssh%2Fid_ed25519&known_hosts_file=%2Ftmp%2Fmdc-known-hosts")

	options, err := sftpfs.FromURI(uri)
	if err != nil {
		t.Fatalf("FromURI() error = %v", err)
	}

	if options.Auth != profiles.AuthKey {
		t.Fatalf("Auth = %q, want %q", options.Auth, profiles.AuthKey)
	}
	if options.IdentityFile != "~/.ssh/id_ed25519" {
		t.Fatalf("IdentityFile = %q, want ~/.ssh/id_ed25519", options.IdentityFile)
	}
	if options.KnownHostsFile != "/tmp/mdc-known-hosts" {
		t.Fatalf("KnownHostsFile = %q, want /tmp/mdc-known-hosts", options.KnownHostsFile)
	}

	roundTrip := options.URI()
	if got := roundTrip.String(); got != "sftp://deploy@files.example.test:2222/srv/releases?auth=key&identity_file=~%2F.ssh%2Fid_ed25519&known_hosts_file=%2Ftmp%2Fmdc-known-hosts" {
		t.Fatalf("roundTrip URI = %q", got)
	}
}

func TestFromURIRejectsNonSFTPURIs(t *testing.T) {
	_, err := sftpfs.FromURI(midfs.NewFileURI("/tmp"))
	if err == nil {
		t.Fatal("FromURI() error = nil, want unsupported scheme")
	}
}

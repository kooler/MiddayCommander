package sftp_test

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"net"
	"os"
	"path/filepath"
	"testing"

	sftpfs "github.com/kooler/MiddayCommander/internal/fs/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

func TestResolveKnownHostsPathUsesOverride(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	path, err := sftpfs.ResolveKnownHostsPath(sftpfs.Options{
		KnownHostsFile: "~/.ssh/custom_known_hosts",
	})
	if err != nil {
		t.Fatalf("ResolveKnownHostsPath() error = %v", err)
	}

	want := filepath.Join(home, ".ssh", "custom_known_hosts")
	if path != want {
		t.Fatalf("ResolveKnownHostsPath() = %q, want %q", path, want)
	}
}

func TestStrictHostKeyCallbackUsesDefaultKnownHosts(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	key := newPublicKey(t)
	writeKnownHosts(t, filepath.Join(home, ".ssh", "known_hosts"), "files.example.test", key)

	callback, err := sftpfs.StrictHostKeyCallback(sftpfs.Options{})
	if err != nil {
		t.Fatalf("StrictHostKeyCallback() error = %v", err)
	}

	if err := callback("files.example.test:22", &net.TCPAddr{IP: net.ParseIP("192.0.2.10"), Port: 22}, key); err != nil {
		t.Fatalf("callback() error = %v", err)
	}
}

func TestStrictHostKeyCallbackFailsWhenKnownHostsIsMissing(t *testing.T) {
	_, err := sftpfs.StrictHostKeyCallback(sftpfs.Options{
		KnownHostsFile: filepath.Join(t.TempDir(), "missing_known_hosts"),
	})
	if err == nil {
		t.Fatal("StrictHostKeyCallback() error = nil, want missing known_hosts failure")
	}
}

func TestStrictHostKeyCallbackRejectsUnknownHost(t *testing.T) {
	knownKey := newPublicKey(t)
	callback := newCallbackWithKnownHost(t, "files.example.test", knownKey)

	err := callback("other.example.test:22", &net.TCPAddr{IP: net.ParseIP("192.0.2.11"), Port: 22}, knownKey)
	if err == nil {
		t.Fatal("callback() error = nil, want unknown host failure")
	}

	var keyErr *knownhosts.KeyError
	if !errors.As(err, &keyErr) {
		t.Fatalf("callback() error = %T, want *knownhosts.KeyError", err)
	}
	if len(keyErr.Want) != 0 {
		t.Fatalf("unknown host Want len = %d, want 0", len(keyErr.Want))
	}
}

func TestStrictHostKeyCallbackRejectsMismatchedKey(t *testing.T) {
	knownKey := newPublicKey(t)
	callback := newCallbackWithKnownHost(t, "files.example.test", knownKey)

	mismatchedKey := newPublicKey(t)
	err := callback("files.example.test:22", &net.TCPAddr{IP: net.ParseIP("192.0.2.12"), Port: 22}, mismatchedKey)
	if err == nil {
		t.Fatal("callback() error = nil, want mismatch failure")
	}

	var keyErr *knownhosts.KeyError
	if !errors.As(err, &keyErr) {
		t.Fatalf("callback() error = %T, want *knownhosts.KeyError", err)
	}
	if len(keyErr.Want) == 0 {
		t.Fatal("mismatch Want len = 0, want existing known key entries")
	}
}

func newCallbackWithKnownHost(t *testing.T, host string, key ssh.PublicKey) ssh.HostKeyCallback {
	t.Helper()

	path := filepath.Join(t.TempDir(), "known_hosts")
	writeKnownHosts(t, path, host, key)

	callback, err := sftpfs.StrictHostKeyCallback(sftpfs.Options{KnownHostsFile: path})
	if err != nil {
		t.Fatalf("StrictHostKeyCallback() error = %v", err)
	}
	return callback
}

func writeKnownHosts(t *testing.T, path, host string, key ssh.PublicKey) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", path, err)
	}

	line := knownhosts.Line([]string{host}, key) + "\n"
	if err := os.WriteFile(path, []byte(line), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}

func newPublicKey(t *testing.T) ssh.PublicKey {
	t.Helper()

	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}

	signer, err := ssh.NewSignerFromKey(privateKey)
	if err != nil {
		t.Fatalf("NewSignerFromKey() error = %v", err)
	}

	return signer.PublicKey()
}

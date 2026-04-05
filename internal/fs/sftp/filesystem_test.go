package sftp_test

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	midfs "github.com/kooler/MiddayCommander/internal/fs"
	sftpfs "github.com/kooler/MiddayCommander/internal/fs/sftp"
	"github.com/kooler/MiddayCommander/internal/profiles"
)

func TestFilesystemListStatAndOpenReader(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("loopback sftp tests rely on unix-flavored filesystem paths")
	}

	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "docs"), 0o755); err != nil {
		t.Fatalf("Mkdir() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "hello.txt"), []byte("hello remote"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".hidden.txt"), []byte("secret"), 0o600); err != nil {
		t.Fatalf("WriteFile(hidden) error = %v", err)
	}

	identityFile := writePrivateKey(t, false)
	clientPublicKey := readPublicKey(t, identityFile)
	addr, knownHostsPath, cleanup := startTestSFTPServer(t, clientPublicKey)
	defer cleanup()

	host, port, err := splitPort(addr)
	if err != nil {
		t.Fatalf("splitPort() error = %v", err)
	}

	fsys := sftpfs.New()
	defer fsys.Close()

	rootURI := midfs.URI{
		Scheme: midfs.SchemeSFTP,
		Host:   host,
		Port:   port,
		User:   "tester",
		Path:   root,
		Query: map[string]string{
			sftpfs.QueryAuth:           profiles.AuthKey,
			sftpfs.QueryIdentityFile:   identityFile,
			sftpfs.QueryKnownHostsFile: knownHostsPath,
		},
	}

	entries, err := fsys.List(context.Background(), rootURI)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	foundFile := false
	foundDir := false
	foundHidden := false
	for _, entry := range entries {
		switch entry.Name {
		case "hello.txt":
			foundFile = true
			if entry.Type != midfs.EntryFile {
				t.Fatalf("hello.txt Type = %q, want file", entry.Type)
			}
			if entry.URI.Path != filepath.ToSlash(filepath.Join(root, "hello.txt")) && entry.URI.Path != filepath.Join(root, "hello.txt") {
				t.Fatalf("hello.txt URI.Path = %q", entry.URI.Path)
			}
		case "docs":
			foundDir = true
			if entry.Type != midfs.EntryDir {
				t.Fatalf("docs Type = %q, want dir", entry.Type)
			}
		case ".hidden.txt":
			foundHidden = true
			if !entry.Hidden {
				t.Fatal(".hidden.txt Hidden = false, want true")
			}
		}
	}
	if !foundFile || !foundDir || !foundHidden {
		t.Fatalf("List() missing expected entries: file=%v dir=%v hidden=%v", foundFile, foundDir, foundHidden)
	}

	fileURI := fsys.Join(rootURI, "hello.txt")
	stat, err := fsys.Stat(context.Background(), fileURI)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if stat.Type != midfs.EntryFile {
		t.Fatalf("Stat().Type = %q, want file", stat.Type)
	}
	if stat.Size != int64(len("hello remote")) {
		t.Fatalf("Stat().Size = %d, want %d", stat.Size, len("hello remote"))
	}

	reader, err := fsys.OpenReader(context.Background(), fileURI, midfs.OpenReadOptions{Offset: 6})
	if err != nil {
		t.Fatalf("OpenReader() error = %v", err)
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if string(data) != "remote" {
		t.Fatalf("OpenReader() data = %q, want %q", string(data), "remote")
	}
}

func TestFilesystemMutationsAndOpenWriter(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("loopback sftp tests rely on unix-flavored filesystem paths")
	}

	ctx := context.Background()
	root := t.TempDir()
	identityFile := writePrivateKey(t, false)
	clientPublicKey := readPublicKey(t, identityFile)
	addr, knownHostsPath, cleanup := startTestSFTPServer(t, clientPublicKey)
	defer cleanup()

	host, port, err := splitPort(addr)
	if err != nil {
		t.Fatalf("splitPort() error = %v", err)
	}

	fsys := sftpfs.New()
	defer fsys.Close()

	rootURI := midfs.URI{
		Scheme: midfs.SchemeSFTP,
		Host:   host,
		Port:   port,
		User:   "tester",
		Path:   root,
		Query: map[string]string{
			sftpfs.QueryAuth:           profiles.AuthKey,
			sftpfs.QueryIdentityFile:   identityFile,
			sftpfs.QueryKnownHostsFile: knownHostsPath,
		},
	}

	remoteDir := fsys.Join(rootURI, "uploads")
	if err := fsys.Mkdir(ctx, remoteDir, 0o755); err != nil {
		t.Fatalf("Mkdir() error = %v", err)
	}
	if info, err := os.Stat(filepath.Join(root, "uploads")); err != nil || !info.IsDir() {
		t.Fatalf("remote directory not created: info=%v err=%v", info, err)
	}

	fileURI := fsys.Join(remoteDir, "draft.txt")
	writer, err := fsys.OpenWriter(ctx, fileURI, midfs.OpenWriteOptions{
		Overwrite: false,
		Perm:      0o640,
	})
	if err != nil {
		t.Fatalf("OpenWriter(create) error = %v", err)
	}
	if _, err := io.WriteString(writer, "draft contents"); err != nil {
		t.Fatalf("WriteString(create) error = %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close(create) error = %v", err)
	}

	data, err := os.ReadFile(filepath.Join(root, "uploads", "draft.txt"))
	if err != nil {
		t.Fatalf("ReadFile(created) error = %v", err)
	}
	if string(data) != "draft contents" {
		t.Fatalf("created file contents = %q, want %q", string(data), "draft contents")
	}

	writer, err = fsys.OpenWriter(ctx, fileURI, midfs.OpenWriteOptions{
		Atomic:    true,
		Overwrite: true,
		Perm:      0o600,
	})
	if err != nil {
		t.Fatalf("OpenWriter(atomic overwrite) error = %v", err)
	}
	if _, err := io.WriteString(writer, "updated"); err != nil {
		t.Fatalf("WriteString(atomic overwrite) error = %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close(atomic overwrite) error = %v", err)
	}

	data, err = os.ReadFile(filepath.Join(root, "uploads", "draft.txt"))
	if err != nil {
		t.Fatalf("ReadFile(updated) error = %v", err)
	}
	if string(data) != "updated" {
		t.Fatalf("updated file contents = %q, want %q", string(data), "updated")
	}

	renamedURI := fsys.Join(remoteDir, "final.txt")
	if err := fsys.Rename(ctx, fileURI, renamedURI); err != nil {
		t.Fatalf("Rename() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "uploads", "draft.txt")); !os.IsNotExist(err) {
		t.Fatalf("old remote file still exists after rename: %v", err)
	}
	if data, err = os.ReadFile(filepath.Join(root, "uploads", "final.txt")); err != nil {
		t.Fatalf("ReadFile(renamed) error = %v", err)
	}
	if string(data) != "updated" {
		t.Fatalf("renamed file contents = %q, want %q", string(data), "updated")
	}

	if err := fsys.Remove(ctx, remoteDir, true); err != nil {
		t.Fatalf("Remove(recursive) error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "uploads")); !os.IsNotExist(err) {
		t.Fatalf("remote directory still exists after recursive remove: %v", err)
	}
}

func TestFilesystemRenameRejectsCrossEndpoint(t *testing.T) {
	fsys := sftpfs.New()
	defer fsys.Close()

	from := midfs.URI{
		Scheme: midfs.SchemeSFTP,
		Host:   "files-1.example.test",
		User:   "demo",
		Path:   "/alpha.txt",
		Query: map[string]string{
			sftpfs.QueryAuth: "agent",
		},
	}
	to := midfs.URI{
		Scheme: midfs.SchemeSFTP,
		Host:   "files-2.example.test",
		User:   "demo",
		Path:   "/beta.txt",
		Query: map[string]string{
			sftpfs.QueryAuth: "agent",
		},
	}

	err := fsys.Rename(context.Background(), from, to)
	if err == nil || !strings.Contains(err.Error(), "across sftp endpoints") {
		t.Fatalf("Rename() error = %v, want cross-endpoint failure", err)
	}
}

func TestFilesystemPathHelpersAndClose(t *testing.T) {
	fsys := sftpfs.New()

	base := midfs.URI{
		Scheme: midfs.SchemeSFTP,
		Host:   "files.example.test",
		User:   "demo",
		Path:   "/alpha/../beta",
		Query: map[string]string{
			sftpfs.QueryAuth: "key",
		},
	}

	clean := fsys.Clean(base)
	if clean.Path != "/beta" {
		t.Fatalf("Clean().Path = %q, want /beta", clean.Path)
	}

	joined := fsys.Join(clean, "docs", "readme.txt")
	if joined.Path != "/beta/docs/readme.txt" {
		t.Fatalf("Join().Path = %q, want /beta/docs/readme.txt", joined.Path)
	}
	if joined.QueryValue(sftpfs.QueryAuth) != "key" {
		t.Fatalf("Join().Query(auth) = %q, want key", joined.QueryValue(sftpfs.QueryAuth))
	}

	parent := fsys.Parent(joined)
	if parent.Path != "/beta/docs" {
		t.Fatalf("Parent().Path = %q, want /beta/docs", parent.Path)
	}

	rootParent := fsys.Parent(midfs.URI{
		Scheme: midfs.SchemeSFTP,
		Host:   "files.example.test",
		User:   "demo",
		Path:   "/",
	})
	if rootParent.Path != "/" {
		t.Fatalf("Parent(root).Path = %q, want /", rootParent.Path)
	}

	if err := fsys.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	_, err := fsys.List(context.Background(), midfs.URI{
		Scheme: midfs.SchemeSFTP,
		Host:   "files.example.test",
		User:   "demo",
		Path:   "/",
	})
	if err == nil || !strings.Contains(err.Error(), "closed") {
		t.Fatalf("List() after Close error = %v, want closed error", err)
	}
}

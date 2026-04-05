package transfer_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
	"time"

	"github.com/kooler/MiddayCommander/internal/audit"
	midfs "github.com/kooler/MiddayCommander/internal/fs"
	archivefs "github.com/kooler/MiddayCommander/internal/fs/archive"
	localfs "github.com/kooler/MiddayCommander/internal/fs/local"
	sftpfs "github.com/kooler/MiddayCommander/internal/fs/sftp"
	"github.com/kooler/MiddayCommander/internal/profiles"
	"github.com/kooler/MiddayCommander/internal/transfer"
	pkgsftp "github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

const flakyScheme midfs.Scheme = "flaky"

func TestManagerCopiesLocalFileToSFTP(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("loopback sftp tests rely on unix-flavored filesystem paths")
	}

	localRoot := t.TempDir()
	sourcePath := filepath.Join(localRoot, "notes.txt")
	if err := os.WriteFile(sourcePath, []byte("hello over lan"), 0o644); err != nil {
		t.Fatalf("WriteFile(source) error = %v", err)
	}

	remoteRoot := t.TempDir()
	identityFile := writePrivateKey(t)
	clientPublicKey := readPublicKey(t, identityFile)
	addr, knownHostsPath, cleanup := startTestSFTPServer(t, clientPublicKey)
	defer cleanup()

	host, port, err := splitPort(addr)
	if err != nil {
		t.Fatalf("splitPort() error = %v", err)
	}

	router := midfs.NewRouter(localfs.New(), archivefs.New(), sftpfs.New())
	defer router.Close()

	manager := transfer.NewManager(router, audit.NopLogger{})
	defer manager.Close()

	job, err := manager.Submit(transfer.Request{
		Operation: transfer.OperationCopy,
		Sources:   []midfs.URI{midfs.NewFileURI(sourcePath)},
		DestDir: midfs.URI{
			Scheme: midfs.SchemeSFTP,
			Host:   host,
			Port:   port,
			User:   "tester",
			Path:   remoteRoot,
			Query: map[string]string{
				sftpfs.QueryAuth:           profiles.AuthKey,
				sftpfs.QueryIdentityFile:   identityFile,
				sftpfs.QueryKnownHostsFile: knownHostsPath,
			},
		},
		Conflict: transfer.ConflictOverwrite,
		Verify:   transfer.VerifySHA256,
	})
	if err != nil {
		t.Fatalf("Submit(copy) error = %v", err)
	}

	event := waitForTerminalEvent(t, manager.Events(), job.ID)
	if event.Type != transfer.EventCompleted {
		t.Fatalf("event.Type = %q, want completed (err=%q)", event.Type, event.Job.Error)
	}

	data, err := os.ReadFile(filepath.Join(remoteRoot, "notes.txt"))
	if err != nil {
		t.Fatalf("ReadFile(remote) error = %v", err)
	}
	if string(data) != "hello over lan" {
		t.Fatalf("remote data = %q, want %q", string(data), "hello over lan")
	}
}

func TestManagerMovesSFTPFileToLocalDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("loopback sftp tests rely on unix-flavored filesystem paths")
	}

	remoteRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(remoteRoot, "report.txt"), []byte("move me"), 0o644); err != nil {
		t.Fatalf("WriteFile(remote) error = %v", err)
	}
	localDest := t.TempDir()

	identityFile := writePrivateKey(t)
	clientPublicKey := readPublicKey(t, identityFile)
	addr, knownHostsPath, cleanup := startTestSFTPServer(t, clientPublicKey)
	defer cleanup()

	host, port, err := splitPort(addr)
	if err != nil {
		t.Fatalf("splitPort() error = %v", err)
	}

	router := midfs.NewRouter(localfs.New(), archivefs.New(), sftpfs.New())
	defer router.Close()

	manager := transfer.NewManager(router, audit.NopLogger{})
	defer manager.Close()

	job, err := manager.Submit(transfer.Request{
		Operation: transfer.OperationMove,
		Sources: []midfs.URI{{
			Scheme: midfs.SchemeSFTP,
			Host:   host,
			Port:   port,
			User:   "tester",
			Path:   filepath.ToSlash(filepath.Join(remoteRoot, "report.txt")),
			Query: map[string]string{
				sftpfs.QueryAuth:           profiles.AuthKey,
				sftpfs.QueryIdentityFile:   identityFile,
				sftpfs.QueryKnownHostsFile: knownHostsPath,
			},
		}},
		DestDir:  midfs.NewFileURI(localDest),
		Conflict: transfer.ConflictOverwrite,
		Verify:   transfer.VerifySize,
	})
	if err != nil {
		t.Fatalf("Submit(move) error = %v", err)
	}

	event := waitForTerminalEvent(t, manager.Events(), job.ID)
	if event.Type != transfer.EventCompleted {
		t.Fatalf("event.Type = %q, want completed (err=%q)", event.Type, event.Job.Error)
	}

	data, err := os.ReadFile(filepath.Join(localDest, "report.txt"))
	if err != nil {
		t.Fatalf("ReadFile(local) error = %v", err)
	}
	if string(data) != "move me" {
		t.Fatalf("local data = %q, want %q", string(data), "move me")
	}
	if _, err := os.Stat(filepath.Join(remoteRoot, "report.txt")); !os.IsNotExist(err) {
		t.Fatalf("remote source still exists after move: %v", err)
	}
}

func TestManagerConflictRenameKeepsExistingDestination(t *testing.T) {
	root := t.TempDir()
	sourcePath := filepath.Join(root, "report.txt")
	destDir := filepath.Join(root, "dest")
	if err := os.Mkdir(destDir, 0o755); err != nil {
		t.Fatalf("Mkdir(dest) error = %v", err)
	}
	if err := os.WriteFile(sourcePath, []byte("fresh"), 0o644); err != nil {
		t.Fatalf("WriteFile(source) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(destDir, "report.txt"), []byte("existing"), 0o644); err != nil {
		t.Fatalf("WriteFile(existing) error = %v", err)
	}

	router := midfs.NewRouter(localfs.New(), archivefs.New(), sftpfs.New())
	defer router.Close()

	manager := transfer.NewManager(router, audit.NopLogger{})
	defer manager.Close()

	job, err := manager.Submit(transfer.Request{
		Operation: transfer.OperationCopy,
		Sources:   []midfs.URI{midfs.NewFileURI(sourcePath)},
		DestDir:   midfs.NewFileURI(destDir),
		Conflict:  transfer.ConflictRename,
		Verify:    transfer.VerifySize,
	})
	if err != nil {
		t.Fatalf("Submit(rename conflict) error = %v", err)
	}

	event := waitForTerminalEvent(t, manager.Events(), job.ID)
	if event.Type != transfer.EventCompleted {
		t.Fatalf("event.Type = %q, want completed (err=%q)", event.Type, event.Job.Error)
	}

	data, err := os.ReadFile(filepath.Join(destDir, "report.txt"))
	if err != nil {
		t.Fatalf("ReadFile(existing) error = %v", err)
	}
	if string(data) != "existing" {
		t.Fatalf("existing destination = %q, want %q", string(data), "existing")
	}

	renamed, err := os.ReadFile(filepath.Join(destDir, "report (copy 1).txt"))
	if err != nil {
		t.Fatalf("ReadFile(renamed copy) error = %v", err)
	}
	if string(renamed) != "fresh" {
		t.Fatalf("renamed copy = %q, want %q", string(renamed), "fresh")
	}
}

func TestManagerRetriesAfterOpenWriterFailure(t *testing.T) {
	root := t.TempDir()
	sourcePath := filepath.Join(root, "retry.txt")
	destDir := filepath.Join(root, "dest")
	if err := os.Mkdir(destDir, 0o755); err != nil {
		t.Fatalf("Mkdir(dest) error = %v", err)
	}
	if err := os.WriteFile(sourcePath, []byte("retry me"), 0o644); err != nil {
		t.Fatalf("WriteFile(source) error = %v", err)
	}

	flaky := newFlakyFS(1)
	router := midfs.NewRouter(flaky)
	defer router.Close()

	manager := transfer.NewManager(router, audit.NopLogger{})
	defer manager.Close()

	job, err := manager.Submit(transfer.Request{
		Operation: transfer.OperationCopy,
		Sources: []midfs.URI{{
			Scheme: flakyScheme,
			Path:   sourcePath,
		}},
		DestDir: midfs.URI{
			Scheme: flakyScheme,
			Path:   destDir,
		},
		Conflict: transfer.ConflictOverwrite,
		Verify:   transfer.VerifySize,
		Retries:  1,
	})
	if err != nil {
		t.Fatalf("Submit(retry) error = %v", err)
	}

	var sawRetry bool
	timeout := time.After(10 * time.Second)
	for {
		select {
		case event, ok := <-manager.Events():
			if !ok {
				t.Fatal("events channel closed before completion")
			}
			if event.Job.Job.ID != job.ID {
				continue
			}
			if event.Type == transfer.EventRetried {
				sawRetry = true
			}
			if event.Type == transfer.EventCompleted {
				if !sawRetry {
					t.Fatal("expected retry event before completion")
				}
				if event.Job.Attempt != 2 {
					t.Fatalf("completed attempt = %d, want 2", event.Job.Attempt)
				}
				data, err := os.ReadFile(filepath.Join(destDir, "retry.txt"))
				if err != nil {
					t.Fatalf("ReadFile(dest) error = %v", err)
				}
				if string(data) != "retry me" {
					t.Fatalf("dest data = %q, want %q", string(data), "retry me")
				}
				return
			}
			if event.Type == transfer.EventFailed {
				t.Fatalf("job failed after retry: %s", event.Job.Error)
			}
		case <-timeout:
			t.Fatal("timeout waiting for retry completion")
		}
	}
}

func waitForTerminalEvent(t *testing.T, events <-chan transfer.Event, jobID string) transfer.Event {
	t.Helper()

	timeout := time.After(10 * time.Second)
	for {
		select {
		case event, ok := <-events:
			if !ok {
				t.Fatal("events channel closed before completion")
			}
			if event.Job.Job.ID != jobID {
				continue
			}
			if event.Type == transfer.EventCompleted || event.Type == transfer.EventFailed {
				return event
			}
		case <-timeout:
			t.Fatalf("timeout waiting for terminal event for %s", jobID)
		}
	}
}

func startTestSFTPServer(t *testing.T, allowedPubKey ssh.PublicKey) (string, string, func()) {
	t.Helper()

	hostSigner := newSigner(t)
	serverConfig := &ssh.ServerConfig{
		PublicKeyCallback: func(_ ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			if bytes.Equal(key.Marshal(), allowedPubKey.Marshal()) {
				return nil, nil
			}
			return nil, fmt.Errorf("unexpected public key")
		},
	}
	serverConfig.AddHostKey(hostSigner)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}

	done := make(chan struct{})
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				select {
				case <-done:
					return
				default:
					return
				}
			}
			go serveTestSFTPConn(conn, serverConfig)
		}
	}()

	knownHostsPath := writeKnownHostsForAddr(t, listener.Addr().String(), hostSigner.PublicKey())
	cleanup := func() {
		close(done)
		_ = listener.Close()
	}
	return listener.Addr().String(), knownHostsPath, cleanup
}

func serveTestSFTPConn(conn net.Conn, config *ssh.ServerConfig) {
	defer conn.Close()

	_, chans, reqs, err := ssh.NewServerConn(conn, config)
	if err != nil {
		return
	}
	go ssh.DiscardRequests(reqs)

	for newChannel := range chans {
		if newChannel.ChannelType() != "session" {
			_ = newChannel.Reject(ssh.UnknownChannelType, "unsupported channel type")
			continue
		}

		channel, requests, err := newChannel.Accept()
		if err != nil {
			continue
		}
		go handleSFTPSubsystem(channel, requests)
	}
}

func handleSFTPSubsystem(channel ssh.Channel, requests <-chan *ssh.Request) {
	defer channel.Close()

	for req := range requests {
		switch req.Type {
		case "subsystem":
			var payload struct{ Name string }
			if err := ssh.Unmarshal(req.Payload, &payload); err != nil || payload.Name != "sftp" {
				_ = req.Reply(false, nil)
				continue
			}
			_ = req.Reply(true, nil)
			server, err := pkgsftp.NewServer(channel)
			if err != nil {
				return
			}
			if err := server.Serve(); err != nil && err != io.EOF {
				_ = server.Close()
				return
			}
			_ = server.Close()
			return
		default:
			_ = req.Reply(false, nil)
		}
	}
}

type flakyFS struct {
	delegate      midfs.FileSystem
	failOpenWrite int
	openWrites    int
}

func newFlakyFS(failOpenWrite int) *flakyFS {
	return &flakyFS{
		delegate:      localfs.New(),
		failOpenWrite: failOpenWrite,
	}
}

func (f *flakyFS) ID() string { return "flaky-local" }

func (f *flakyFS) Scheme() midfs.Scheme { return flakyScheme }

func (f *flakyFS) Capabilities() uint64 { return f.delegate.Capabilities() }

func (f *flakyFS) List(ctx context.Context, dir midfs.URI) ([]midfs.Entry, error) {
	entries, err := f.delegate.List(ctx, f.toFileURI(dir))
	if err != nil {
		return nil, err
	}
	return f.withSchemeEntries(entries), nil
}

func (f *flakyFS) Stat(ctx context.Context, uri midfs.URI) (midfs.Entry, error) {
	entry, err := f.delegate.Stat(ctx, f.toFileURI(uri))
	if err != nil {
		return midfs.Entry{}, err
	}
	entry.URI.Scheme = flakyScheme
	return entry, nil
}

func (f *flakyFS) Mkdir(ctx context.Context, uri midfs.URI, perm os.FileMode) error {
	return f.delegate.Mkdir(ctx, f.toFileURI(uri), perm)
}

func (f *flakyFS) Rename(ctx context.Context, from midfs.URI, to midfs.URI) error {
	return f.delegate.Rename(ctx, f.toFileURI(from), f.toFileURI(to))
}

func (f *flakyFS) Remove(ctx context.Context, uri midfs.URI, recursive bool) error {
	return f.delegate.Remove(ctx, f.toFileURI(uri), recursive)
}

func (f *flakyFS) OpenReader(ctx context.Context, uri midfs.URI, opts midfs.OpenReadOptions) (io.ReadCloser, error) {
	return f.delegate.OpenReader(ctx, f.toFileURI(uri), opts)
}

func (f *flakyFS) OpenWriter(ctx context.Context, uri midfs.URI, opts midfs.OpenWriteOptions) (io.WriteCloser, error) {
	f.openWrites++
	if f.openWrites <= f.failOpenWrite {
		return nil, fmt.Errorf("simulated open writer failure")
	}
	return f.delegate.OpenWriter(ctx, f.toFileURI(uri), opts)
}

func (f *flakyFS) Join(base midfs.URI, elems ...string) midfs.URI {
	joined := f.delegate.Join(f.toFileURI(base), elems...)
	joined.Scheme = flakyScheme
	return joined
}

func (f *flakyFS) Parent(uri midfs.URI) midfs.URI {
	parent := f.delegate.Parent(f.toFileURI(uri))
	parent.Scheme = flakyScheme
	return parent
}

func (f *flakyFS) Clean(uri midfs.URI) midfs.URI {
	clean := f.delegate.Clean(f.toFileURI(uri))
	clean.Scheme = flakyScheme
	return clean
}

func (f *flakyFS) Close() error { return f.delegate.Close() }

func (f *flakyFS) toFileURI(uri midfs.URI) midfs.URI {
	uri.Scheme = midfs.SchemeFile
	return uri
}

func (f *flakyFS) withSchemeEntries(entries []midfs.Entry) []midfs.Entry {
	result := make([]midfs.Entry, 0, len(entries))
	for _, entry := range entries {
		entry.URI.Scheme = flakyScheme
		result = append(result, entry)
	}
	return result
}

func writeKnownHostsForAddr(t *testing.T, addr string, hostKey ssh.PublicKey) string {
	t.Helper()

	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("SplitHostPort() error = %v", err)
	}

	line := knownhosts.Line([]string{fmt.Sprintf("[%s]:%s", host, port)}, hostKey)
	path := filepath.Join(t.TempDir(), "known_hosts")
	if err := os.WriteFile(path, []byte(line+"\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(known_hosts) error = %v", err)
	}
	return path
}

func writePrivateKey(t *testing.T) string {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}

	privateKeyBytes := x509.MarshalPKCS1PrivateKey(privateKey)
	block := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: privateKeyBytes}
	path := filepath.Join(t.TempDir(), "id_rsa")
	if err := os.WriteFile(path, pem.EncodeToMemory(block), 0o600); err != nil {
		t.Fatalf("WriteFile(id_rsa) error = %v", err)
	}
	return path
}

func readPublicKey(t *testing.T, identityFile string) ssh.PublicKey {
	t.Helper()

	data, err := os.ReadFile(identityFile)
	if err != nil {
		t.Fatalf("ReadFile(identity) error = %v", err)
	}

	signer, err := ssh.ParsePrivateKey(data)
	if err != nil {
		t.Fatalf("ParsePrivateKey() error = %v", err)
	}
	return signer.PublicKey()
}

func newSigner(t *testing.T) ssh.Signer {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}

	signer, err := ssh.NewSignerFromKey(privateKey)
	if err != nil {
		t.Fatalf("NewSignerFromKey() error = %v", err)
	}
	return signer
}

func splitPort(addr string) (string, int, error) {
	host, portValue, err := net.SplitHostPort(addr)
	if err != nil {
		return "", 0, err
	}
	port, err := strconv.Atoi(portValue)
	if err != nil {
		return "", 0, err
	}
	return host, port, nil
}

package app

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

	midfs "github.com/kooler/MiddayCommander/internal/fs"
	archivefs "github.com/kooler/MiddayCommander/internal/fs/archive"
	localfs "github.com/kooler/MiddayCommander/internal/fs/local"
	sftpfs "github.com/kooler/MiddayCommander/internal/fs/sftp"
	"github.com/kooler/MiddayCommander/internal/profiles"
	"github.com/kooler/MiddayCommander/internal/tui/dialogs"
	"github.com/kooler/MiddayCommander/internal/ui/panel"
	pkgsftp "github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

func TestHandleDialogResultGoToLoadsSFTPPanel(t *testing.T) {
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

	identityFile := writePrivateKeyForAppTest(t)
	clientPublicKey := readPublicKeyForAppTest(t, identityFile)
	addr, knownHostsPath, cleanup := startGoToTestSFTPServer(t, clientPublicKey)
	defer cleanup()

	host, port, err := splitPortForAppTest(addr)
	if err != nil {
		t.Fatalf("splitPortForAppTest() error = %v", err)
	}

	router := midfs.NewRouter(localfs.New(), archivefs.New(), sftpfs.New())
	defer router.Close()

	model := Model{
		router:     router,
		leftPanel:  panel.New(router, midfs.NewFileURI(root), panel.KeyMap{}),
		rightPanel: panel.New(router, midfs.NewFileURI(root), panel.KeyMap{}),
		focus:      FocusLeft,
	}
	model.leftPanel.SetActive(true)

	rawURI := midfs.URI{
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
	}.String()

	updatedModel, cmd := model.handleDialogResult(dialogs.Result{
		Kind:      dialogs.KindInput,
		Confirmed: true,
		Text:      rawURI,
		Tag:       tagGoTo,
	})

	updated, ok := updatedModel.(Model)
	if !ok {
		t.Fatalf("handleDialogResult() model type = %T, want app.Model", updatedModel)
	}
	if updated.leftPanel.URI().Scheme != midfs.SchemeSFTP {
		t.Fatalf("leftPanel.URI().Scheme = %q, want sftp", updated.leftPanel.URI().Scheme)
	}

	msg, ok := cmd().(panel.DirLoadedMsg)
	if !ok {
		t.Fatalf("Go To command message = %T, want panel.DirLoadedMsg", cmd())
	}
	if msg.Err != nil {
		t.Fatalf("Go To load error = %v", msg.Err)
	}

	updated.leftPanel.HandleDirLoaded(msg)
	updated.leftPanel.RestoreCursor("hello.txt")
	entry := updated.leftPanel.CurrentEntry()
	if entry == nil || entry.Name != "hello.txt" {
		t.Fatalf("CurrentEntry() = %#v, want hello.txt", entry)
	}
	if entry.URI.Scheme != midfs.SchemeSFTP {
		t.Fatalf("entry.URI.Scheme = %q, want sftp", entry.URI.Scheme)
	}
}

func TestModelCloseClosesRouter(t *testing.T) {
	filesystem := &closeTrackingFS{}
	model := Model{
		router: midfs.NewRouter(filesystem),
	}

	if err := model.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if !filesystem.closed {
		t.Fatal("Close() did not close the router filesystem")
	}
}

func TestProfileSelectMsgLoadsActivePanel(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("loopback sftp tests rely on unix-flavored filesystem paths")
	}

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "hello.txt"), []byte("hello remote"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	identityFile := writePrivateKeyForAppTest(t)
	clientPublicKey := readPublicKeyForAppTest(t, identityFile)
	addr, knownHostsPath, cleanup := startGoToTestSFTPServer(t, clientPublicKey)
	defer cleanup()

	host, port, err := splitPortForAppTest(addr)
	if err != nil {
		t.Fatalf("splitPortForAppTest() error = %v", err)
	}

	router := midfs.NewRouter(localfs.New(), archivefs.New(), sftpfs.New())
	defer router.Close()

	model := Model{
		router:     router,
		leftPanel:  panel.New(router, midfs.NewFileURI(root), panel.KeyMap{}),
		rightPanel: panel.New(router, midfs.NewFileURI(root), panel.KeyMap{}),
		focus:      FocusLeft,
	}
	model.leftPanel.SetActive(true)

	msgModel, cmd := model.Update(dialogs.ProfileSelectMsg{
		Profile: profiles.Profile{
			Name: "alpha",
			Host: host,
			Port: port,
			User: "tester",
			Path: root,
			Auth: profiles.AuthKey,
		},
		URI: midfs.URI{
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
		},
	})

	updated := msgModel.(Model)
	loadMsg, ok := cmd().(panel.DirLoadedMsg)
	if !ok {
		t.Fatalf("Update(ProfileSelectMsg) msg = %T, want panel.DirLoadedMsg", cmd())
	}
	if loadMsg.Err != nil {
		t.Fatalf("loadMsg.Err = %v", loadMsg.Err)
	}

	updated.leftPanel.HandleDirLoaded(loadMsg)
	updated.leftPanel.RestoreCursor("hello.txt")
	entry := updated.leftPanel.CurrentEntry()
	if entry == nil || entry.URI.Scheme != midfs.SchemeSFTP {
		t.Fatalf("CurrentEntry() = %#v, want sftp entry", entry)
	}
}

type closeTrackingFS struct {
	closed bool
}

func (f *closeTrackingFS) ID() string { return "close-tracking" }

func (f *closeTrackingFS) Scheme() midfs.Scheme { return midfs.SchemeSFTP }

func (f *closeTrackingFS) Capabilities() uint64 { return 0 }

func (f *closeTrackingFS) List(context.Context, midfs.URI) ([]midfs.Entry, error) {
	return nil, nil
}

func (f *closeTrackingFS) Stat(context.Context, midfs.URI) (midfs.Entry, error) {
	return midfs.Entry{}, nil
}

func (f *closeTrackingFS) Mkdir(context.Context, midfs.URI, os.FileMode) error { return nil }

func (f *closeTrackingFS) Rename(context.Context, midfs.URI, midfs.URI) error { return nil }

func (f *closeTrackingFS) Remove(context.Context, midfs.URI, bool) error { return nil }

func (f *closeTrackingFS) OpenReader(context.Context, midfs.URI, midfs.OpenReadOptions) (io.ReadCloser, error) {
	return nil, nil
}

func (f *closeTrackingFS) OpenWriter(context.Context, midfs.URI, midfs.OpenWriteOptions) (io.WriteCloser, error) {
	return nil, nil
}

func (f *closeTrackingFS) Join(base midfs.URI, elems ...string) midfs.URI { return base }

func (f *closeTrackingFS) Parent(uri midfs.URI) midfs.URI { return uri }

func (f *closeTrackingFS) Clean(uri midfs.URI) midfs.URI { return uri }

func (f *closeTrackingFS) Close() error {
	f.closed = true
	return nil
}

func startGoToTestSFTPServer(t *testing.T, allowedPubKey ssh.PublicKey) (string, string, func()) {
	t.Helper()

	hostSigner := newSignerForAppTest(t)
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

			go serveGoToTestSFTPConn(conn, serverConfig)
		}
	}()

	knownHostsPath := writeKnownHostsForAppTest(t, listener.Addr().String(), hostSigner.PublicKey())
	cleanup := func() {
		close(done)
		_ = listener.Close()
	}

	return listener.Addr().String(), knownHostsPath, cleanup
}

func serveGoToTestSFTPConn(conn net.Conn, config *ssh.ServerConfig) {
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

		go handleGoToSFTPSubsystem(channel, requests)
	}
}

func handleGoToSFTPSubsystem(channel ssh.Channel, requests <-chan *ssh.Request) {
	defer channel.Close()

	for req := range requests {
		switch req.Type {
		case "subsystem":
			var payload struct {
				Name string
			}
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

func writeKnownHostsForAppTest(t *testing.T, addr string, hostKey ssh.PublicKey) string {
	t.Helper()

	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("SplitHostPort() error = %v", err)
	}

	line := knownhosts.Line([]string{fmt.Sprintf("[%s]:%s", host, port)}, hostKey)
	path := filepath.Join(t.TempDir(), "known_hosts")
	if err := os.WriteFile(path, []byte(line+"\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	return path
}

func writePrivateKeyForAppTest(t *testing.T) string {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}

	der := x509.MarshalPKCS1PrivateKey(privateKey)
	block := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: der}
	path := filepath.Join(t.TempDir(), "id_rsa")
	if err := os.WriteFile(path, pem.EncodeToMemory(block), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	return path
}

func readPublicKeyForAppTest(t *testing.T, identityFile string) ssh.PublicKey {
	t.Helper()

	data, err := os.ReadFile(identityFile)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	signer, err := ssh.ParsePrivateKey(data)
	if err != nil {
		t.Fatalf("ParsePrivateKey() error = %v", err)
	}
	return signer.PublicKey()
}

func newSignerForAppTest(t *testing.T) ssh.Signer {
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

func splitPortForAppTest(addr string) (string, int, error) {
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

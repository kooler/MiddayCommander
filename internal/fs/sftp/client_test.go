package sftp_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"

	sftpfs "github.com/kooler/MiddayCommander/internal/fs/sftp"
	"github.com/kooler/MiddayCommander/internal/profiles"
	pkgsftp "github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

func TestConnectOpensSSHAndSFTPClients(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("local loopback ssh test relies on unix-flavored paths and sockets")
	}

	identityFile := writePrivateKey(t, false)
	clientPublicKey := readPublicKey(t, identityFile)

	addr, knownHostsPath, cleanup := startTestSFTPServer(t, clientPublicKey)
	defer cleanup()

	host, port, err := splitPort(addr)
	if err != nil {
		t.Fatalf("splitPort() error = %v", err)
	}

	client, err := sftpfs.Connect(context.Background(), sftpfs.Options{
		Host:           host,
		Port:           port,
		User:           "tester",
		Path:           "/",
		Auth:           profiles.AuthKey,
		IdentityFile:   identityFile,
		KnownHostsFile: knownHostsPath,
	})
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	defer client.Close()

	if client.SSH() == nil {
		t.Fatal("Connect() SSH() = nil, want non-nil")
	}
	if client.SFTP() == nil {
		t.Fatal("Connect() SFTP() = nil, want non-nil")
	}

	if _, err := client.SFTP().ReadDir("/"); err != nil {
		t.Fatalf("ReadDir(/) error = %v", err)
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

func writeKnownHostsForAddr(t *testing.T, addr string, hostKey ssh.PublicKey) string {
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

func readPublicKey(t *testing.T, identityFile string) ssh.PublicKey {
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

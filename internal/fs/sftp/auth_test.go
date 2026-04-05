package sftp_test

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	sftpfs "github.com/kooler/MiddayCommander/internal/fs/sftp"
	"github.com/kooler/MiddayCommander/internal/profiles"
	"golang.org/x/crypto/ssh/agent"
)

func TestBuildAuthMethodUsesAgentSocket(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix socket ssh-agent test")
	}

	socket, cleanup := startTestAgent(t)
	defer cleanup()
	t.Setenv("SSH_AUTH_SOCK", socket)

	method, closer, err := sftpfs.BuildAuthMethod(sftpfs.Options{Auth: profiles.AuthAgent})
	if err != nil {
		t.Fatalf("BuildAuthMethod() error = %v", err)
	}
	if method == nil {
		t.Fatal("BuildAuthMethod() method = nil, want non-nil")
	}
	if closer == nil {
		t.Fatal("BuildAuthMethod() closer = nil, want non-nil for agent auth")
	}
	_ = closer.Close()
}

func TestBuildAuthMethodRejectsMissingAgentSocket(t *testing.T) {
	t.Setenv("SSH_AUTH_SOCK", "")

	_, _, err := sftpfs.BuildAuthMethod(sftpfs.Options{Auth: profiles.AuthAgent})
	if err == nil {
		t.Fatal("BuildAuthMethod() error = nil, want missing SSH_AUTH_SOCK failure")
	}
	if !strings.Contains(err.Error(), "SSH_AUTH_SOCK") {
		t.Fatalf("BuildAuthMethod() error = %v, want SSH_AUTH_SOCK hint", err)
	}
}

func TestBuildAuthMethodUsesKeyFile(t *testing.T) {
	identityFile := writePrivateKey(t, false)

	method, closer, err := sftpfs.BuildAuthMethod(sftpfs.Options{
		Auth:         profiles.AuthKey,
		IdentityFile: identityFile,
	})
	if err != nil {
		t.Fatalf("BuildAuthMethod() error = %v", err)
	}
	if method == nil {
		t.Fatal("BuildAuthMethod() method = nil, want non-nil")
	}
	if closer != nil {
		t.Fatal("BuildAuthMethod() closer != nil, want nil for key auth")
	}
}

func TestBuildAuthMethodRejectsMissingIdentityFile(t *testing.T) {
	_, _, err := sftpfs.BuildAuthMethod(sftpfs.Options{
		Auth:         profiles.AuthKey,
		IdentityFile: filepath.Join(t.TempDir(), "missing_key"),
	})
	if err == nil {
		t.Fatal("BuildAuthMethod() error = nil, want missing key failure")
	}
	if !strings.Contains(err.Error(), "read identity file") {
		t.Fatalf("BuildAuthMethod() error = %v, want read identity file context", err)
	}
}

func TestBuildAuthMethodRejectsEncryptedIdentityFile(t *testing.T) {
	identityFile := writePrivateKey(t, true)

	_, _, err := sftpfs.BuildAuthMethod(sftpfs.Options{
		Auth:         profiles.AuthKey,
		IdentityFile: identityFile,
	})
	if err == nil {
		t.Fatal("BuildAuthMethod() error = nil, want encrypted key failure")
	}
	if !strings.Contains(err.Error(), "passphrase") {
		t.Fatalf("BuildAuthMethod() error = %v, want passphrase guidance", err)
	}
}

func startTestAgent(t *testing.T) (string, func()) {
	t.Helper()

	keyring := agent.NewKeyring()
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}
	if err := keyring.Add(agent.AddedKey{PrivateKey: privateKey}); err != nil {
		t.Fatalf("keyring.Add() error = %v", err)
	}

	socket := filepath.Join(t.TempDir(), "agent.sock")
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatalf("Listen(unix) error = %v", err)
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

			go func() {
				defer conn.Close()
				_ = agent.ServeAgent(keyring, conn)
			}()
		}
	}()

	cleanup := func() {
		close(done)
		_ = listener.Close()
	}

	return socket, cleanup
}

func writePrivateKey(t *testing.T, encrypted bool) string {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}

	der := x509.MarshalPKCS1PrivateKey(privateKey)
	var block *pem.Block
	if encrypted {
		block, err = x509.EncryptPEMBlock(rand.Reader, "RSA PRIVATE KEY", der, []byte("secret"), x509.PEMCipherAES256)
		if err != nil {
			t.Fatalf("EncryptPEMBlock() error = %v", err)
		}
	} else {
		block = &pem.Block{Type: "RSA PRIVATE KEY", Bytes: der}
	}

	path := filepath.Join(t.TempDir(), "id_rsa")
	if err := os.WriteFile(path, pem.EncodeToMemory(block), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	return path
}

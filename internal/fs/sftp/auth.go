package sftp

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"

	"github.com/kooler/MiddayCommander/internal/profiles"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

// BuildAuthMethod returns an SSH auth method plus an optional closer for any
// resources that must remain open until authentication finishes.
func BuildAuthMethod(opts Options) (ssh.AuthMethod, io.Closer, error) {
	auth := strings.TrimSpace(opts.Auth)
	if auth == "" {
		auth = profiles.AuthAgent
	}

	switch auth {
	case profiles.AuthAgent:
		return buildAgentAuthMethod()
	case profiles.AuthKey:
		return buildKeyAuthMethod(opts.IdentityFile)
	default:
		return nil, nil, fmt.Errorf("unsupported auth %q", auth)
	}
}

func buildAgentAuthMethod() (ssh.AuthMethod, io.Closer, error) {
	socket := strings.TrimSpace(os.Getenv("SSH_AUTH_SOCK"))
	if socket == "" {
		return nil, nil, fmt.Errorf("SSH_AUTH_SOCK is not set; use auth=%q or start ssh-agent", profiles.AuthKey)
	}

	conn, err := net.Dial("unix", socket)
	if err != nil {
		return nil, nil, fmt.Errorf("connect to ssh-agent %q: %w", socket, err)
	}

	client := agent.NewClient(conn)
	return ssh.PublicKeysCallback(client.Signers), conn, nil
}

func buildKeyAuthMethod(identityFile string) (ssh.AuthMethod, io.Closer, error) {
	path, err := expandPath(identityFile)
	if err != nil {
		return nil, nil, fmt.Errorf("resolve identity file: %w", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("read identity file %q: %w", path, err)
	}

	signer, err := ssh.ParsePrivateKey(data)
	if err != nil {
		var passphraseErr *ssh.PassphraseMissingError
		if errors.As(err, &passphraseErr) {
			return nil, nil, fmt.Errorf("identity file %q requires a passphrase; load it into ssh-agent for Phase 2", path)
		}
		return nil, nil, fmt.Errorf("parse identity file %q: %w", path, err)
	}

	return ssh.PublicKeys(signer), nil, nil
}

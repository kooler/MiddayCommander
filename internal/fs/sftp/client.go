package sftp

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/kooler/MiddayCommander/internal/profiles"
	pkgsftp "github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

const defaultConnectTimeout = 15 * time.Second

// Client owns a single SSH transport plus the negotiated SFTP subsystem.
type Client struct {
	opts       Options
	sshClient  *ssh.Client
	sftpClient *pkgsftp.Client
}

// Connect opens an SSH transport, verifies the host key strictly, and starts
// an SFTP subsystem on top of the authenticated session.
func Connect(ctx context.Context, opts Options) (*Client, error) {
	normalized, err := normalizeOptions(opts)
	if err != nil {
		return nil, err
	}

	hostKeyCallback, err := StrictHostKeyCallback(normalized)
	if err != nil {
		return nil, err
	}

	authMethod, authCloser, err := BuildAuthMethod(normalized)
	if err != nil {
		return nil, err
	}
	if authCloser != nil {
		defer authCloser.Close()
	}

	sshClient, err := dialSSH(ctx, normalized, authMethod, hostKeyCallback)
	if err != nil {
		return nil, err
	}

	sftpClient, err := pkgsftp.NewClient(sshClient)
	if err != nil {
		_ = sshClient.Close()
		return nil, fmt.Errorf("open sftp subsystem on %s: %w", normalized.address(), err)
	}

	return &Client{
		opts:       normalized,
		sshClient:  sshClient,
		sftpClient: sftpClient,
	}, nil
}

// Options returns the normalized connection options used for this client.
func (c *Client) Options() Options {
	if c == nil {
		return Options{}
	}
	return c.opts
}

// SSH exposes the underlying SSH client.
func (c *Client) SSH() *ssh.Client {
	if c == nil {
		return nil
	}
	return c.sshClient
}

// SFTP exposes the negotiated SFTP client.
func (c *Client) SFTP() *pkgsftp.Client {
	if c == nil {
		return nil
	}
	return c.sftpClient
}

// Close closes the SFTP subsystem and the underlying SSH transport.
func (c *Client) Close() error {
	if c == nil {
		return nil
	}

	var err error
	if c.sftpClient != nil {
		err = errors.Join(err, c.sftpClient.Close())
	}
	if c.sshClient != nil {
		err = errors.Join(err, c.sshClient.Close())
	}
	return err
}

func dialSSH(
	ctx context.Context,
	opts Options,
	authMethod ssh.AuthMethod,
	hostKeyCallback ssh.HostKeyCallback,
) (*ssh.Client, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	config := &ssh.ClientConfig{
		User:            opts.User,
		Auth:            []ssh.AuthMethod{authMethod},
		HostKeyCallback: hostKeyCallback,
		Timeout:         defaultConnectTimeout,
	}

	addr := opts.address()
	conn, err := (&net.Dialer{Timeout: defaultConnectTimeout}).DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("dial ssh %s: %w", addr, err)
	}

	if deadline, ok := ctx.Deadline(); ok {
		if err := conn.SetDeadline(deadline); err != nil {
			_ = conn.Close()
			return nil, fmt.Errorf("set deadline for ssh %s: %w", addr, err)
		}
		defer conn.SetDeadline(time.Time{})
	}

	clientConn, chans, reqs, err := ssh.NewClientConn(conn, addr, config)
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("handshake ssh %s: %w", addr, err)
	}

	return ssh.NewClient(clientConn, chans, reqs), nil
}

func normalizeOptions(opts Options) (Options, error) {
	normalized, err := profiles.Normalize(profiles.Profile{
		Name:           "sftp",
		Host:           opts.Host,
		Port:           opts.Port,
		User:           opts.User,
		Path:           opts.Path,
		Auth:           opts.Auth,
		IdentityFile:   opts.IdentityFile,
		KnownHostsFile: opts.KnownHostsFile,
	})
	if err != nil {
		return Options{}, err
	}

	return Options{
		Host:           normalized.Host,
		Port:           normalized.Port,
		User:           normalized.User,
		Path:           cleanPath(normalized.Path),
		Auth:           normalized.Auth,
		IdentityFile:   normalized.IdentityFile,
		KnownHostsFile: normalized.KnownHostsFile,
	}, nil
}

func (o Options) address() string {
	port := o.Port
	if port == 0 {
		port = profiles.DefaultPort
	}
	return net.JoinHostPort(o.Host, strconv.Itoa(port))
}

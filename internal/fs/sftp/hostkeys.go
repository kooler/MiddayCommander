package sftp

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// ResolveKnownHostsPath returns the known_hosts file used for strict host key
// verification. Overrides are resolved first; otherwise ~/.ssh/known_hosts is
// used.
func ResolveKnownHostsPath(opts Options) (string, error) {
	if override := strings.TrimSpace(opts.KnownHostsFile); override != "" {
		return expandPath(override)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir for known_hosts: %w", err)
	}
	return filepath.Join(home, ".ssh", "known_hosts"), nil
}

// StrictHostKeyCallback builds an SSH host key callback that verifies hosts
// strictly against the resolved known_hosts file.
func StrictHostKeyCallback(opts Options) (ssh.HostKeyCallback, error) {
	path, err := ResolveKnownHostsPath(opts)
	if err != nil {
		return nil, err
	}

	callback, err := knownhosts.New(path)
	if err != nil {
		return nil, fmt.Errorf("load known_hosts %q: %w", path, err)
	}
	return callback, nil
}

func expandPath(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", fmt.Errorf("path is empty")
	}

	if value == "~" || strings.HasPrefix(value, "~/") || strings.HasPrefix(value, "~\\") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home dir for %q: %w", value, err)
		}
		suffix := value[1:]
		if suffix == "" {
			value = home
		} else {
			value = filepath.Join(home, suffix)
		}
	}

	if !filepath.IsAbs(value) {
		absValue, err := filepath.Abs(value)
		if err != nil {
			return "", fmt.Errorf("resolve absolute path for %q: %w", value, err)
		}
		value = absValue
	}

	return filepath.Clean(value), nil
}

package profiles

import (
	"fmt"
	"path"
	"strings"
)

const (
	DefaultPort = 22

	AuthAgent = "agent"
	AuthKey   = "key"
)

// Profile describes a named remote endpoint for future SFTP connections.
type Profile struct {
	Name           string `toml:"name"`
	Host           string `toml:"host"`
	Port           int    `toml:"port"`
	User           string `toml:"user"`
	Path           string `toml:"path"`
	Auth           string `toml:"auth"`
	IdentityFile   string `toml:"identity_file"`
	KnownHostsFile string `toml:"known_hosts_file"`
}

// Normalize trims and validates a profile, filling in default values.
func Normalize(profile Profile) (Profile, error) {
	profile.Name = strings.TrimSpace(profile.Name)
	profile.Host = strings.TrimSpace(profile.Host)
	profile.User = strings.TrimSpace(profile.User)
	profile.Path = strings.TrimSpace(profile.Path)
	profile.Auth = strings.ToLower(strings.TrimSpace(profile.Auth))
	profile.IdentityFile = strings.TrimSpace(profile.IdentityFile)
	profile.KnownHostsFile = strings.TrimSpace(profile.KnownHostsFile)

	if profile.Name == "" {
		return Profile{}, fmt.Errorf("name is required")
	}
	if profile.Host == "" {
		return Profile{}, fmt.Errorf("host is required")
	}
	if profile.User == "" {
		return Profile{}, fmt.Errorf("user is required")
	}

	if profile.Port == 0 {
		profile.Port = DefaultPort
	}
	if profile.Port < 1 || profile.Port > 65535 {
		return Profile{}, fmt.Errorf("port %d is out of range", profile.Port)
	}

	if profile.Path == "" {
		profile.Path = "/"
	} else {
		profile.Path = path.Clean(profile.Path)
		if profile.Path == "." {
			profile.Path = "/"
		}
	}

	if profile.Auth == "" {
		profile.Auth = AuthAgent
	}
	switch profile.Auth {
	case AuthAgent:
	case AuthKey:
		if profile.IdentityFile == "" {
			return Profile{}, fmt.Errorf("identity_file is required when auth = %q", AuthKey)
		}
	default:
		return Profile{}, fmt.Errorf("unsupported auth %q", profile.Auth)
	}

	return profile, nil
}

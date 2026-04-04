package profiles

import (
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/kooler/MiddayCommander/internal/config"
)

type Store struct {
	path     string
	profiles []Profile
}

type diskStore struct {
	Profiles []Profile `toml:"profiles"`
}

// LoadStore loads profiles from the default profiles.toml path.
func LoadStore() (*Store, error) {
	return LoadPath(config.ProfilesPath())
}

// LoadPath loads profiles from a specific TOML file path.
func LoadPath(path string) (*Store, error) {
	store := &Store{path: path}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return store, nil
		}
		return nil, err
	}

	var persisted diskStore
	if err := toml.Unmarshal(data, &persisted); err != nil {
		return nil, err
	}

	seen := make(map[string]struct{}, len(persisted.Profiles))
	store.profiles = make([]Profile, 0, len(persisted.Profiles))
	for index, profile := range persisted.Profiles {
		normalized, err := normalizeProfile(profile)
		if err != nil {
			return nil, fmt.Errorf("profiles[%d]: %w", index, err)
		}
		if _, ok := seen[normalized.Name]; ok {
			return nil, fmt.Errorf("profiles[%d]: duplicate profile name %q", index, normalized.Name)
		}
		seen[normalized.Name] = struct{}{}
		store.profiles = append(store.profiles, normalized)
	}

	return store, nil
}

func (s *Store) Path() string {
	return s.path
}

func (s *Store) All() []Profile {
	profiles := make([]Profile, len(s.profiles))
	copy(profiles, s.profiles)
	return profiles
}

func (s *Store) Find(name string) (Profile, bool) {
	target := strings.TrimSpace(name)
	for _, profile := range s.profiles {
		if profile.Name == target {
			return profile, true
		}
	}
	return Profile{}, false
}

func normalizeProfile(profile Profile) (Profile, error) {
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

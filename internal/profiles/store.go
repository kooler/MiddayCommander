package profiles

import (
	"fmt"
	"os"
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
		normalized, err := Normalize(profile)
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

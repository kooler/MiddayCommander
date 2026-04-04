package config

import (
	"os"
	"path/filepath"
)

// ConfigDir returns the mdc config directory path.
func ConfigDir() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "mdc")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "mdc")
}

// ConfigPath returns the main config.toml location.
func ConfigPath() string {
	return filepath.Join(ConfigDir(), "config.toml")
}

// ThemesDir returns the themes directory path.
func ThemesDir() string {
	return filepath.Join(ConfigDir(), "themes")
}

// BookmarksPath returns the bookmarks store path.
func BookmarksPath() string {
	return filepath.Join(ConfigDir(), "bookmarks.json")
}

// ProfilesPath returns the remote profiles path.
func ProfilesPath() string {
	return filepath.Join(ConfigDir(), "profiles.toml")
}

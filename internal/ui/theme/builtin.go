package theme

import (
	_ "embed"
	"sync"

	"github.com/BurntSushi/toml"
)

// DefaultKey is the config key of the theme applied when none is configured.
const DefaultKey = "catppuccin-mocha"

// catppuccin-mocha.toml is a copy of themes/catppuccin-mocha.toml, embedded so
// the default theme is available without a config file or network access.
//
//go:embed catppuccin-mocha.toml
var mochaTOML []byte

// mochaTheme builds the embedded Catppuccin Mocha theme (the default), once.
var mochaTheme = sync.OnceValue(func() Theme {
	var tf ThemeFile
	if err := toml.Unmarshal(mochaTOML, &tf); err != nil {
		return Default()
	}
	return buildTheme(tf)
})

// builtinThemes returns themes compiled into the binary, in display order.
// The first entry is the default.
func builtinThemes() []AvailableTheme {
	return []AvailableTheme{
		{Key: DefaultKey, Name: "Catppuccin Mocha", Source: SourceDefault, Theme: mochaTheme()},
		{Key: "mc-classic", Name: "MC Classic", Source: SourceDefault, Theme: Default()},
	}
}

// Resolve returns the theme for the given config key, honoring built-ins before
// disk lookup. An empty key selects the default (Catppuccin Mocha).
func Resolve(key string) Theme {
	if key == "" {
		key = DefaultKey
	}
	for _, b := range builtinThemes() {
		if b.Key == key {
			return b.Theme
		}
	}
	if loaded, err := LoadByName(key); err == nil {
		return loaded
	}
	return mochaTheme()
}

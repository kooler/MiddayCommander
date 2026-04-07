package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
)

// Config is the top-level application configuration.
type Config struct {
	Theme    string         `toml:"theme"`
	Keys     KeyBindings    `toml:"keys"`
	Behavior BehaviorConfig `toml:"behavior"`
	Icons    IconsConfig    `toml:"icons"`
}

// IconsConfig controls the display of Nerd Font icons in the file panel.
type IconsConfig struct {
	// Enabled controls whether icons are displayed (default: false).
	Enabled bool `toml:"enabled"`
	// Icon shown for directories.
	Folder string `toml:"folder"`
	// Icon shown for files without a matching extension.
	File string `toml:"file"`
	// Mapping of file extension (without dot) to icon.
	Extensions map[string]string `toml:"extensions"`
}

// DefaultIconsConfig returns the default icon configuration.
func DefaultIconsConfig() IconsConfig {
	return IconsConfig{
		Enabled: false,
		Folder:  "", // nf-fa-folder
		File:    "", // nf-fa-file
		Extensions: map[string]string{
			// Documents
			"txt":  "", // nf-fa-file_text
			"md":   "",
			"pdf":  "", // nf-fa-file_pdf
			"doc":  "", // nf-fa-file_word
			"docx": "",
			"xls":  "", // nf-fa-file_excel
			"xlsx": "",

			// Images
			"png":  "", // nf-fa-file_image
			"jpg":  "",
			"jpeg": "",
			"gif":  "",
			"svg":  "",
			"ico":  "",
			"bmp":  "",
			"webp": "",

			// Archives
			"zip": "", // nf-fa-file_archive
			"tar": "",
			"gz":  "",
			"bz2": "",
			"7z":  "",
			"rar": "",
			"xz":  "",

			// Code
			"go":   "", // nf-seti-go
			"py":   "", // nf-dev-python
			"js":   "", // nf-seti-javascript
			"jsx":  "",
			"ts":   "", // nf-seti-typescript
			"tsx":  "",
			"rs":   "", // nf-custom-rust
			"rb":   "", // nf-dev-ruby
			"java": "", // nf-custom-java
			"c":    "", // nf-seti-c
			"cpp":  "", // nf-seti-cpp
			"h":    "",
			"hpp":  "",
			"cs":   "󰌛", // nf-custom-csharp
			"php":  "", // nf-custom-php
			"sh":   "", // nf-oct-terminal
			"bash": "",

			// Config / Markup
			"json": "", // nf-oct-file_json
			"yaml": "", // nf-dev-config
			"yml":  "",
			"toml": "", // nf-md-file_toml
			"xml":  "󰗀", // nf-md-xml
			"html": "", // nf-dev-html5
			"css":  "", // nf-dev-css3

			// Media
			"mp3": "", // nf-fa-volume_up (audio)
			"wav": "",
			"mp4": "", // nf-fa-film (video)
			"avi": "",
			"mkv": "",

			// Go-specific
			"mod": "", // nf-md-go (module)
			"sum": "",
		},
	}
}

// BehaviorConfig controls configurable behaviors.
type BehaviorConfig struct {
	// What Enter does on a file: "edit" (default) or "preview"
	EnterAction string `toml:"enter_action"`
	// What Space does on a file: "preview" (default) or "edit"
	SpaceAction string `toml:"space_action"`
}

// KeyBindings defines all configurable key bindings.
// Each field is a string or list of strings representing key combos.
type KeyBindings struct {
	Quit        StringOrList `toml:"quit"`
	TogglePanel StringOrList `toml:"toggle_panel"`
	SwapPanels  StringOrList `toml:"swap_panels"`
	Copy        StringOrList `toml:"copy"`
	Move        StringOrList `toml:"move"`
	Mkdir       StringOrList `toml:"mkdir"`
	Delete      StringOrList `toml:"delete"`
	Rename      StringOrList `toml:"rename"`
	View        StringOrList `toml:"view"`
	Edit        StringOrList `toml:"edit"`

	// Navigation
	Up       StringOrList `toml:"up"`
	Down     StringOrList `toml:"down"`
	PageUp   StringOrList `toml:"page_up"`
	PageDown StringOrList `toml:"page_down"`
	Home     StringOrList `toml:"home"`
	End      StringOrList `toml:"end"`
	GoBack   StringOrList `toml:"go_back"`

	// Selection
	ToggleSelect StringOrList `toml:"toggle_select"`
	SelectUp     StringOrList `toml:"select_up"`
	SelectDown   StringOrList `toml:"select_down"`

	// Search
	QuickSearch StringOrList `toml:"quick_search"`

	// Go to path
	GoTo        StringOrList `toml:"goto"`
	FuzzyFind   StringOrList `toml:"fuzzy_find"`
	Bookmarks   StringOrList `toml:"bookmarks"`
	Help        StringOrList `toml:"help"`
	ThemePicker StringOrList `toml:"theme_picker"`
	CmdExec     StringOrList `toml:"cmd_exec"`
}

// StringOrList can unmarshal from either a single string or a list of strings.
type StringOrList []string

func (s *StringOrList) UnmarshalTOML(data any) error {
	switch v := data.(type) {
	case string:
		*s = []string{v}
	case []any:
		for _, item := range v {
			if str, ok := item.(string); ok {
				*s = append(*s, str)
			}
		}
	}
	return nil
}

// Default returns a config with all defaults.
func Default() Config {
	keys := DefaultKeyBindings()
	normalizeAllKeys(&keys)
	return Config{
		Theme:    "",
		Behavior: BehaviorConfig{EnterAction: "edit", SpaceAction: "preview"},
		Icons:    DefaultIconsConfig(),
		Keys:     keys,
	}
}

// DefaultKeyBindings returns the default key bindings.
func DefaultKeyBindings() KeyBindings {
	return KeyBindings{
		Quit:        StringOrList{"f10", "ctrl+c"},
		TogglePanel: StringOrList{"tab"},
		SwapPanels:  StringOrList{"ctrl+u"},
		Copy:        StringOrList{"f5"},
		Move:        StringOrList{"f6"},
		Mkdir:       StringOrList{"f7"},
		Delete:      StringOrList{"f8"},
		Rename:      StringOrList{"shift+f6"},
		View:        StringOrList{"f3"},
		Edit:        StringOrList{"f4"},

		Up:       StringOrList{"up", "k"},
		Down:     StringOrList{"down", "j"},
		PageUp:   StringOrList{"pgup"},
		PageDown: StringOrList{"pgdown"},
		Home:     StringOrList{"home"},
		End:      StringOrList{"end"},
		GoBack:   StringOrList{"backspace"},

		ToggleSelect: StringOrList{"insert"},
		SelectUp:     StringOrList{"shift+up"},
		SelectDown:   StringOrList{"shift+down"},

		QuickSearch: StringOrList{"ctrl+s"},

		GoTo:        StringOrList{"ctrl+g"},
		FuzzyFind:   StringOrList{"f9", "ctrl+p"},
		Bookmarks:   StringOrList{"f2", "ctrl+b"},
		Help:        StringOrList{"f1"},
		ThemePicker: StringOrList{"ctrl+t"},
		CmdExec:     StringOrList{"ctrl+r"},
	}
}

// Load reads config from ~/.config/mdc/config.toml, merging with defaults.
func Load() Config {
	cfg := Default()

	configPath := filepath.Join(configDirPath(), "config.toml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return cfg
	}

	var fileCfg Config
	if err := toml.Unmarshal(data, &fileCfg); err != nil {
		return cfg
	}

	// Merge: only override non-zero values
	if fileCfg.Theme != "" {
		cfg.Theme = fileCfg.Theme
	}
	if fileCfg.Behavior.EnterAction != "" {
		cfg.Behavior.EnterAction = fileCfg.Behavior.EnterAction
	}
	if fileCfg.Behavior.SpaceAction != "" {
		cfg.Behavior.SpaceAction = fileCfg.Behavior.SpaceAction
	}

	mergeKeys(&cfg.Keys, &fileCfg.Keys)
	normalizeAllKeys(&cfg.Keys)

	// Merge icons config: start with defaults, overlay file config.
	mergeIcons(&cfg.Icons, &fileCfg.Icons)

	return cfg
}

func mergeKeys(dst, src *KeyBindings) {
	mergeKey(&dst.Quit, src.Quit)
	mergeKey(&dst.TogglePanel, src.TogglePanel)
	mergeKey(&dst.SwapPanels, src.SwapPanels)
	mergeKey(&dst.Copy, src.Copy)
	mergeKey(&dst.Move, src.Move)
	mergeKey(&dst.Mkdir, src.Mkdir)
	mergeKey(&dst.Delete, src.Delete)
	mergeKey(&dst.Rename, src.Rename)
	mergeKey(&dst.View, src.View)
	mergeKey(&dst.Edit, src.Edit)
	mergeKey(&dst.Up, src.Up)
	mergeKey(&dst.Down, src.Down)
	mergeKey(&dst.PageUp, src.PageUp)
	mergeKey(&dst.PageDown, src.PageDown)
	mergeKey(&dst.Home, src.Home)
	mergeKey(&dst.End, src.End)
	mergeKey(&dst.GoBack, src.GoBack)
	mergeKey(&dst.ToggleSelect, src.ToggleSelect)
	mergeKey(&dst.SelectUp, src.SelectUp)
	mergeKey(&dst.SelectDown, src.SelectDown)
	mergeKey(&dst.QuickSearch, src.QuickSearch)
	mergeKey(&dst.GoTo, src.GoTo)
	mergeKey(&dst.FuzzyFind, src.FuzzyFind)
	mergeKey(&dst.Bookmarks, src.Bookmarks)
	mergeKey(&dst.Help, src.Help)
	mergeKey(&dst.ThemePicker, src.ThemePicker)
	mergeKey(&dst.CmdExec, src.CmdExec)
}

func mergeKey(dst *StringOrList, src StringOrList) {
	if len(src) > 0 {
		*dst = src
	}
}

// mergeIcons merges src IconsConfig into dst. Non-empty fields from src override dst.
// Extensions are merged (src overrides dst on conflict).
func mergeIcons(dst, src *IconsConfig) {
	if src.Folder != "" {
		dst.Folder = src.Folder
	}
	if src.File != "" {
		dst.File = src.File
	}
	// Enabled is only overridden if explicitly set in file config.
	// Since the default is false and we want false to mean "use default",
	// we always use the file value (false or true).
	dst.Enabled = src.Enabled
	if src.Extensions != nil {
		if dst.Extensions == nil {
			dst.Extensions = make(map[string]string)
		}
		for ext, icon := range src.Extensions {
			dst.Extensions[ext] = icon
		}
	}
}

// normalizeKey converts user-friendly "shift+fN" (N=1..8) to the BubbleTea
// key string "f(N+12)". BubbleTea v1.x reports Shift+F1..F8 as F13..F20.
func normalizeKey(k string) string {
	if !strings.HasPrefix(k, "shift+f") {
		return k
	}
	nStr := k[len("shift+f"):]
	n, err := strconv.Atoi(nStr)
	if err != nil || n < 1 || n > 8 {
		return k
	}
	return fmt.Sprintf("f%d", n+12)
}

func normalizeSlice(s *StringOrList) {
	for i, k := range *s {
		(*s)[i] = normalizeKey(k)
	}
}

func normalizeAllKeys(kb *KeyBindings) {
	normalizeSlice(&kb.Quit)
	normalizeSlice(&kb.TogglePanel)
	normalizeSlice(&kb.SwapPanels)
	normalizeSlice(&kb.Copy)
	normalizeSlice(&kb.Move)
	normalizeSlice(&kb.Mkdir)
	normalizeSlice(&kb.Delete)
	normalizeSlice(&kb.Rename)
	normalizeSlice(&kb.View)
	normalizeSlice(&kb.Edit)
	normalizeSlice(&kb.Up)
	normalizeSlice(&kb.Down)
	normalizeSlice(&kb.PageUp)
	normalizeSlice(&kb.PageDown)
	normalizeSlice(&kb.Home)
	normalizeSlice(&kb.End)
	normalizeSlice(&kb.GoBack)
	normalizeSlice(&kb.ToggleSelect)
	normalizeSlice(&kb.SelectUp)
	normalizeSlice(&kb.SelectDown)
	normalizeSlice(&kb.QuickSearch)
	normalizeSlice(&kb.GoTo)
	normalizeSlice(&kb.FuzzyFind)
	normalizeSlice(&kb.Bookmarks)
	normalizeSlice(&kb.Help)
	normalizeSlice(&kb.ThemePicker)
	normalizeSlice(&kb.CmdExec)
}

// SaveTheme writes the theme name to the config file, preserving other settings.
// An empty name means "use built-in default" and clears the theme setting.
func SaveTheme(name string) error {
	dir := configDirPath()
	configPath := filepath.Join(dir, "config.toml")

	data, err := os.ReadFile(configPath)
	if err != nil {
		if name == "" {
			return nil // no config file + default theme = nothing to write
		}
		// Config file doesn't exist — create it with just the theme line.
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		return os.WriteFile(configPath, []byte(fmt.Sprintf("theme = %q\n", name)), 0o644)
	}

	content := string(data)
	themeRe := regexp.MustCompile(`(?m)^\s*theme\s*=\s*"[^"]*"\n?`)

	if name == "" {
		// Remove the theme line entirely so the app falls back to Default().
		content = themeRe.ReplaceAllString(content, "")
	} else if themeRe.MatchString(content) {
		themeReCapture := regexp.MustCompile(`(?m)^(\s*)theme\s*=\s*"[^"]*"`)
		content = themeReCapture.ReplaceAllString(content, fmt.Sprintf("${1}theme = %q", name))
	} else {
		// Prepend theme line at the top.
		content = fmt.Sprintf("theme = %q\n", name) + content
	}

	return os.WriteFile(configPath, []byte(content), 0o644)
}

// ConfigDir returns the mdc config directory path.
func ConfigDir() string {
	return configDirPath()
}

// configDirPath returns ~/.config/mdc, respecting XDG_CONFIG_HOME.
func configDirPath() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "mdc")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "mdc")
}

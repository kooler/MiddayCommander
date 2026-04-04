package theme

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/charmbracelet/lipgloss"
	"github.com/kooler/MiddayCommander/internal/config"
)

// ThemeFile is the TOML structure for a theme file.
type ThemeFile struct {
	Name    string            `toml:"name"`
	Palette map[string]string `toml:"palette"`
	Panel   panelTOML         `toml:"panel"`
	Status  sectionTOML       `toml:"statusbar"`
	Menu    menuTOML          `toml:"menubar"`
}

type panelTOML struct {
	BorderFG       string   `toml:"border_fg"`
	BorderBG       string   `toml:"border_bg"`
	BorderActiveFG string   `toml:"border_active_fg"`
	BorderActiveBG string   `toml:"border_active_bg"`
	HeaderFG       string   `toml:"header_fg"`
	HeaderBG       string   `toml:"header_bg"`
	HeaderActiveFG string   `toml:"header_active_fg"`
	HeaderActiveBG string   `toml:"header_active_bg"`
	File           fileTOML `toml:"file"`
}

type fileTOML struct {
	NormalFG      string `toml:"normal_fg"`
	NormalBG      string `toml:"normal_bg"`
	DirFG         string `toml:"dir_fg"`
	DirBG         string `toml:"dir_bg"`
	DirBold       bool   `toml:"dir_bold"`
	ExecFG        string `toml:"exec_fg"`
	ExecBG        string `toml:"exec_bg"`
	SymlinkFG     string `toml:"symlink_fg"`
	SymlinkBG     string `toml:"symlink_bg"`
	CursorFG      string `toml:"cursor_fg"`
	CursorBG      string `toml:"cursor_bg"`
	CursorDirFG   string `toml:"cursor_dir_fg"`
	CursorDirBG   string `toml:"cursor_dir_bg"`
	CursorDirBold bool   `toml:"cursor_dir_bold"`
	SelectedFG    string `toml:"selected_fg"`
	SelectedBG    string `toml:"selected_bg"`
	SelectedBold  bool   `toml:"selected_bold"`
}

type sectionTOML struct {
	FG string `toml:"fg"`
	BG string `toml:"bg"`
}

type menuTOML struct {
	FG          string `toml:"fg"`
	BG          string `toml:"bg"`
	FKeyHintFG  string `toml:"fkey_hint_fg"`
	FKeyHintBG  string `toml:"fkey_hint_bg"`
	FKeyLabelFG string `toml:"fkey_label_fg"`
	FKeyLabelBG string `toml:"fkey_label_bg"`
}

// Theme source constants.
const (
	SourceDefault = "default"
	SourceLocal   = "local"
	SourceRemote  = "remote"
)

// AvailableTheme represents a discovered theme that can be selected.
type AvailableTheme struct {
	Key     string // config key (filename stem, e.g. "catppuccin-mocha")
	Name    string // display name from TOML "name" field, or filename stem
	Source  string // SourceDefault, SourceLocal, or SourceRemote
	Theme   Theme  // pre-loaded theme
	RawTOML []byte // original TOML content (set for remote themes, used to save locally on select)
}

// LoadFromFile loads a theme from a TOML file, falling back to Default() for missing values.
func LoadFromFile(path string) (Theme, error) {
	tf, err := loadThemeFile(path)
	if err != nil {
		return Default(), err
	}
	return buildTheme(tf), nil
}

// loadThemeFile reads and parses a theme TOML file.
func loadThemeFile(path string) (ThemeFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ThemeFile{}, err
	}
	var tf ThemeFile
	if err := toml.Unmarshal(data, &tf); err != nil {
		return ThemeFile{}, err
	}
	return tf, nil
}

// ListAvailable returns all available themes: the hardcoded default plus any
// TOML files found in ~/.config/mdc/themes/.
func ListAvailable() []AvailableTheme {
	result := []AvailableTheme{{
		Key:    "",
		Name:   "Default (MC Classic)",
		Source: SourceDefault,
		Theme:  Default(),
	}}

	themesDir := config.ThemesDir()
	entries, err := os.ReadDir(themesDir)
	if err != nil {
		return result
	}

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".toml") {
			continue
		}
		key := strings.TrimSuffix(e.Name(), ".toml")
		path := filepath.Join(themesDir, e.Name())
		tf, err := loadThemeFile(path)
		if err != nil {
			continue
		}
		name := tf.Name
		if name == "" {
			name = key
		}
		result = append(result, AvailableTheme{
			Key:    key,
			Name:   name,
			Source: SourceLocal,
			Theme:  buildTheme(tf),
		})
	}

	return result
}

// ParseTOML parses raw TOML bytes into an AvailableTheme.
func ParseTOML(key string, data []byte) (AvailableTheme, error) {
	var tf ThemeFile
	if err := toml.Unmarshal(data, &tf); err != nil {
		return AvailableTheme{}, err
	}
	name := tf.Name
	if name == "" {
		name = key
	}
	return AvailableTheme{
		Key:     key,
		Name:    name,
		Source:  SourceRemote,
		Theme:   buildTheme(tf),
		RawTOML: data,
	}, nil
}

// ThemesDir returns the path to ~/.config/mdc/themes/.
func ThemesDir() string {
	return config.ThemesDir()
}

// LoadByName loads a theme by name from ~/.config/mdc/themes/<name>.toml.
func LoadByName(name string) (Theme, error) {
	path := filepath.Join(config.ThemesDir(), name+".toml")
	return LoadFromFile(path)
}

func buildTheme(tf ThemeFile) Theme {
	p := tf.Palette
	resolve := func(val string) lipgloss.Color {
		if val == "" {
			return lipgloss.Color("")
		}
		// Check if it's a palette reference
		if resolved, ok := p[val]; ok {
			return lipgloss.Color(resolved)
		}
		return lipgloss.Color(val)
	}

	style := func(fg, bg string) lipgloss.Style {
		s := lipgloss.NewStyle()
		if fg != "" {
			s = s.Foreground(resolve(fg))
		}
		if bg != "" {
			s = s.Background(resolve(bg))
		}
		return s
	}

	boldStyle := func(fg, bg string, bold bool) lipgloss.Style {
		return style(fg, bg).Bold(bold)
	}

	// Fall back to defaults for empty values
	def := Default()
	th := Theme{
		PanelBorder:       orDefault(style(tf.Panel.BorderFG, tf.Panel.BorderBG), def.PanelBorder),
		PanelBorderActive: orDefault(style(tf.Panel.BorderActiveFG, tf.Panel.BorderActiveBG), def.PanelBorderActive),
		PanelHeader:       orDefault(boldStyle(tf.Panel.HeaderFG, tf.Panel.HeaderBG, true), def.PanelHeader),
		PanelHeaderActive: orDefault(boldStyle(tf.Panel.HeaderActiveFG, tf.Panel.HeaderActiveBG, true), def.PanelHeaderActive),

		FileNormal:    orDefault(style(tf.Panel.File.NormalFG, tf.Panel.File.NormalBG), def.FileNormal),
		FileDir:       orDefault(boldStyle(tf.Panel.File.DirFG, tf.Panel.File.DirBG, tf.Panel.File.DirBold), def.FileDir),
		FileExec:      orDefault(style(tf.Panel.File.ExecFG, tf.Panel.File.ExecBG), def.FileExec),
		FileSymlink:   orDefault(style(tf.Panel.File.SymlinkFG, tf.Panel.File.SymlinkBG), def.FileSymlink),
		FileCursor:    orDefault(style(tf.Panel.File.CursorFG, tf.Panel.File.CursorBG), def.FileCursor),
		FileCursorDir: orDefault(boldStyle(tf.Panel.File.CursorDirFG, tf.Panel.File.CursorDirBG, tf.Panel.File.CursorDirBold), def.FileCursorDir),
		FileSelected:  orDefault(boldStyle(tf.Panel.File.SelectedFG, tf.Panel.File.SelectedBG, tf.Panel.File.SelectedBold), def.FileSelected),

		StatusBar: orDefault(style(tf.Status.FG, tf.Status.BG), def.StatusBar),
		MenuBar:   orDefault(style(tf.Menu.FG, tf.Menu.BG), def.MenuBar),
		FKeyHint:  orDefault(style(tf.Menu.FKeyHintFG, tf.Menu.FKeyHintBG), def.FKeyHint),
		FKeyLabel: orDefault(style(tf.Menu.FKeyLabelFG, tf.Menu.FKeyLabelBG), def.FKeyLabel),

		CmdLine: def.CmdLine,
	}

	return th
}

// orDefault returns s if it has any foreground or background set, otherwise returns def.
func orDefault(s lipgloss.Style, def lipgloss.Style) lipgloss.Style {
	// If the style has been configured (has fg or bg), use it; otherwise use default.
	// We check by rendering — if both fg and bg are empty, use default.
	fg := s.GetForeground()
	bg := s.GetBackground()
	if fg == (lipgloss.NoColor{}) && bg == (lipgloss.NoColor{}) {
		return def
	}
	return s
}

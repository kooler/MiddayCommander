package panel

import (
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/kooler/MiddayCommander/internal/config"
)

// IconResolver resolves file/directory names to Nerd Font icons based on
// configuration. It is immutable after creation and safe for concurrent use.
type IconResolver struct {
	enabled  bool
	folder   string
	file     string
	extIcons map[string]string
}

// NewIconResolver creates a resolver from the given icon configuration.
// If cfg.Enabled is false, the resolver will return empty strings.
func NewIconResolver(cfg config.IconsConfig) IconResolver {
	if !cfg.Enabled {
		return IconResolver{}
	}
	r := IconResolver{
		enabled:  true,
		folder:   cfg.Folder,
		file:     cfg.File,
		extIcons: cfg.Extensions,
	}
	if r.folder == "" {
		r.folder = "\uF07B" // nf-fa-folder
	}
	if r.file == "" {
		r.file = "\uF016" // nf-fa-file
	}
	if r.extIcons == nil {
		r.extIcons = make(map[string]string)
	}
	return r
}

// ResolveIcon returns the Nerd Font icon for the given entry.
// Returns an empty string if icons are disabled or no match is found.
// isDir determines whether to use the folder icon or extension-based lookup.
func (r IconResolver) ResolveIcon(name string, isDir bool) string {
	if !r.enabled {
		return ""
	}
	if isDir {
		return r.folder
	}
	ext := strings.TrimPrefix(filepath.Ext(name), ".")
	if ext == "" {
		return r.file
	}
	if icon, ok := r.extIcons[strings.ToLower(ext)]; ok {
		return icon
	}
	return r.file
}

// IconWidth returns the display width (in characters/rune count) of the icon
// that would be rendered for the given entry. Returns 0 if icons are disabled.
func (r IconResolver) IconWidth(name string, isDir bool) int {
	icon := r.ResolveIcon(name, isDir)
	if icon == "" {
		return 0
	}
	return utf8.RuneCountInString(icon)
}

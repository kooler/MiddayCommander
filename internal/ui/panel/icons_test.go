package panel

import (
	"testing"

	"github.com/kooler/MiddayCommander/internal/config"
)

func TestIconResolver_Disabled(t *testing.T) {
	r := NewIconResolver(config.IconsConfig{Enabled: false})
	if got := r.ResolveIcon("test.go", false); got != "" {
		t.Errorf("expected empty string when disabled, got %q", got)
	}
	if got := r.ResolveIcon("src", true); got != "" {
		t.Errorf("expected empty string for dir when disabled, got %q", got)
	}
	if got := r.IconWidth("test.go", false); got != 0 {
		t.Errorf("expected width 0 when disabled, got %d", got)
	}
}

func TestIconResolver_Directory(t *testing.T) {
	r := NewIconResolver(config.DefaultIconsConfig())
	// Manually enable since default is false.
	r = NewIconResolver(config.IconsConfig{
		Enabled:    true,
		Folder:     "\uF07B",
		File:       "\uF016",
		Extensions: config.DefaultIconsConfig().Extensions,
	})
	if got := r.ResolveIcon("src", true); got != "\uF07B" {
		t.Errorf("expected folder icon, got %q", got)
	}
}

func TestIconResolver_ExtensionMatch(t *testing.T) {
	r := NewIconResolver(config.IconsConfig{
		Enabled: true,
		File:    "\uF016",
		Folder:  "\uF07B",
		Extensions: map[string]string{
			"go":  "\uF17E",
			"txt": "\uF0F6",
		},
	})
	tests := []struct {
		name  string
		isDir bool
		want  string
	}{
		{"main.go", false, "\uF17E"},
		{"readme.TXT", false, "\uF0F6"}, // case-insensitive
		{"notes.txt", false, "\uF0F6"},
		{"image.png", false, "\uF016"}, // no match → fallback to file icon
		{"src", true, "\uF07B"},        // directory → folder icon
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := r.ResolveIcon(tt.name, tt.isDir)
			if got != tt.want {
				t.Errorf("ResolveIcon(%q, %v) = %q, want %q", tt.name, tt.isDir, got, tt.want)
			}
		})
	}
}

func TestIconResolver_DefaultFallback(t *testing.T) {
	r := NewIconResolver(config.IconsConfig{
		Enabled:    true,
		Folder:     "\uF07B",
		File:       "\uF016",
		Extensions: map[string]string{"go": "\uF17E"},
	})
	// Unknown extension → file icon
	if got := r.ResolveIcon("unknown.xyz", false); got != "\uF016" {
		t.Errorf("expected file icon for unknown extension, got %q", got)
	}
	// No extension → file icon
	if got := r.ResolveIcon("Makefile", false); got != "\uF016" {
		t.Errorf("expected file icon for no extension, got %q", got)
	}
	// Directory → folder icon
	if got := r.ResolveIcon("src", true); got != "\uF07B" {
		t.Errorf("expected folder icon, got %q", got)
	}
}

func TestIconResolver_IconWidth(t *testing.T) {
	r := NewIconResolver(config.IconsConfig{
		Enabled:    true,
		File:       "\uF016",
		Extensions: map[string]string{"go": "\uF17E"},
	})
	if got := r.IconWidth("main.go", false); got != 1 {
		t.Errorf("expected width 1, got %d", got)
	}
	if got := r.IconWidth("unknown.xyz", false); got != 1 {
		t.Errorf("expected width 1 for unknown ext, got %d", got)
	}
	r2 := NewIconResolver(config.IconsConfig{Enabled: false})
	if got := r2.IconWidth("main.go", false); got != 0 {
		t.Errorf("expected width 0 when disabled, got %d", got)
	}
}

package theme

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestLoadByName(t *testing.T) {
	// Set up a temp config dir so LoadByName finds the theme file
	// (the repo's themes/ dir isn't in ~/.config/mdc/themes on CI).
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	themesDir := filepath.Join(tmp, "mdc", "themes")
	if err := os.MkdirAll(themesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "themes", "catppuccin-mocha.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(themesDir, "catppuccin-mocha.toml"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	th, err := LoadByName("catppuccin-mocha")
	if err != nil {
		t.Fatalf("LoadByName error: %v", err)
	}

	// Verify non-default colors were applied
	def := Default()

	// FileNormal should differ from default (catppuccin uses #cdd6f4 on #1e1e2e, not ANSI 15 on 4)
	if th.FileNormal.GetForeground() == def.FileNormal.GetForeground() &&
		th.FileNormal.GetBackground() == def.FileNormal.GetBackground() {
		t.Errorf("FileNormal was not overridden by theme")
	}

	t.Logf("FileNormal fg=%v bg=%v", th.FileNormal.GetForeground(), th.FileNormal.GetBackground())
	t.Logf("FileDir fg=%v bg=%v bold=%v", th.FileDir.GetForeground(), th.FileDir.GetBackground(), th.FileDir.GetBold())
	t.Logf("PanelBorder fg=%v bg=%v", th.PanelBorder.GetForeground(), th.PanelBorder.GetBackground())
	t.Logf("StatusBar fg=%v bg=%v", th.StatusBar.GetForeground(), th.StatusBar.GetBackground())
}

func TestOrDefault(t *testing.T) {
	def := lipgloss.NewStyle().Foreground(lipgloss.Color("15"))
	empty := lipgloss.NewStyle()
	colored := lipgloss.NewStyle().Foreground(lipgloss.Color("#ff0000"))

	result := orDefault(empty, def)
	if result.GetForeground() != def.GetForeground() {
		t.Errorf("orDefault should return default for empty style")
	}

	result = orDefault(colored, def)
	if result.GetForeground() == def.GetForeground() {
		t.Errorf("orDefault should return colored style, not default")
	}
}

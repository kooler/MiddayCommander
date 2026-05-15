//go:build !windows

package main

import (
	"fmt"
	"os"

	"github.com/charmbracelet/lipgloss"
)

// openControllingTTY opens /dev/tty for direct terminal I/O.
//
// With `cd "$(mdc -r)"`, fd 1 is a pipe to the shell. The TUI must not write
// there or it gets captured. Caller passes the returned file to
// tea.WithInput/WithOutput so bubbletea reads keystrokes and renders to the
// terminal, leaving fd 1 free for the final path.
//
// Side effect: rebinds lipgloss's default renderer to use the tty. Without
// this, termenv detects color support via isatty() on os.Stdout — which is
// the pipe, not a terminal — and falls back to the Ascii profile, killing
// all colors and styled selection in the TUI.
func openControllingTTY() (*os.File, error) {
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return nil, fmt.Errorf("open /dev/tty: %w", err)
	}
	lipgloss.SetDefaultRenderer(lipgloss.NewRenderer(tty))
	return tty, nil
}

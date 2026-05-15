package platform

import (
	"encoding/base64"
	"os"
)

// CopyToClipboard writes text to the terminal clipboard using the OSC 52
// escape sequence. Works in any modern terminal that supports OSC 52
// (iTerm2, kitty, WezTerm, Alacritty, recent xterm, tmux with set -g
// set-clipboard on, etc.).
func CopyToClipboard(text string) {
	enc := base64.StdEncoding.EncodeToString([]byte(text))
	os.Stdout.WriteString("\x1b]52;c;" + enc + "\x07")
}

package main

import (
	"context"
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/kooler/MiddayCommander/internal/app"
	"github.com/kooler/MiddayCommander/internal/platform"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)


func main() {
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-v") {
		fmt.Printf("mdc %s (%s) built %s\n", version, commit, date)
		os.Exit(0)
	}

	returnPath := hasFlag(os.Args[1:], "-r")

	// With -r, fd 1 is a pipe to the shell (`cd "$(mdc -r)"`). Route the TUI
	// through a separately-opened /dev/tty so fd 1 stays clean for the final
	// path on exit.
	var ttyFile *os.File
	if returnPath {
		var err error
		ttyFile, err = openControllingTTY()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		defer ttyFile.Close()
	}

	// Enable Kitty keyboard protocol (flag 1: disambiguate) so the terminal
	// reports modifier-only key presses (e.g. bare Shift). Terminals that
	// don't support the protocol silently ignore this sequence.
	uiOut := os.Stdout
	if ttyFile != nil {
		uiOut = ttyFile
	}
	_, _ = uiOut.WriteString("\x1b[>1u")
	defer func() { _, _ = uiOut.WriteString("\x1b[<u") }() // disable on exit

	opts := []tea.ProgramOption{
		tea.WithAltScreen(),
		tea.WithMouseAllMotion(),
		tea.WithFilter(app.KittyFilter),
	}
	if ttyFile != nil {
		opts = append(opts, tea.WithInput(ttyFile), tea.WithOutput(ttyFile))
	}

	p := tea.NewProgram(app.New(version), opts...)

	// Poll OS-level shift key state and send messages to the Bubble Tea program.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go pollShift(ctx, p)

	final, err := p.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if returnPath {
		if m, ok := final.(app.Model); ok {
			fmt.Println(m.ActivePanelPath())
		}
	}
}

func hasFlag(args []string, name string) bool {
	for _, a := range args {
		if a == name {
			return true
		}
	}
	return false
}

// pollShift checks the OS modifier state periodically and sends
// ShiftPressMsg / ShiftReleaseMsg when the state changes.
func pollShift(ctx context.Context, p *tea.Program) {
	var wasShift bool
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pressed := platform.IsShiftPressed()
			if pressed != wasShift {
				wasShift = pressed
				if pressed {
					p.Send(app.ShiftPressMsg{})
				} else {
					p.Send(app.ShiftReleaseMsg{})
				}
			}
		}
	}
}

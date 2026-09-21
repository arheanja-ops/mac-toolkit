package tui

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattn/go-isatty"
)

// Run starts the full-screen TUI. It requires an interactive terminal; without
// a TTY it returns a clear error suggesting the fallback command.
func Run(version string) error {
	if !isatty.IsTerminal(os.Stdout.Fd()) && !isatty.IsTerminal(os.Stdin.Fd()) {
		return fmt.Errorf("no interactive terminal (TTY) detected; run 'mac-toolkit menu' or a direct command like 'mac-toolkit analyze'")
	}

	p := tea.NewProgram(newModel(version), tea.WithAltScreen())
	_, err := p.Run()
	return err
}

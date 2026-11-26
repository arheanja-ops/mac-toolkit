package tui

import (
	"context"
	"fmt"
	"os"

	"github.com/arheanja-ops/mac-toolkit/internal/watch"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattn/go-isatty"
)

// Run starts the full-screen TUI. It requires an interactive terminal; without
// a TTY it returns a clear error suggesting the fallback command.
//
// The listening engine (internal/watch) starts with the TUI and stops when it
// closes; notify sets whether it shows macOS notifications.
func Run(version string, notify bool) error {
	if !isatty.IsTerminal(os.Stdout.Fd()) && !isatty.IsTerminal(os.Stdin.Fd()) {
		return fmt.Errorf("no interactive terminal (TTY) detected; run 'mac-toolkit menu' or a direct command like 'mac-toolkit analyze'")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	watchEngine = watch.NewEngine(notify)
	go watchEngine.Run(ctx)

	p := tea.NewProgram(newModel(version), tea.WithAltScreen())
	_, err := p.Run()
	return err
}

package cmd

import (
	"github.com/arheanja-ops/mac-toolkit/internal/core"
	"github.com/arheanja-ops/mac-toolkit/internal/tui"

	"github.com/spf13/cobra"
)

// tuiCmd is the explicit alias for the default (no-arg) behavior: launch the
// full-screen k9s-style TUI.
var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Launch the interactive two-pane TUI (k9s-style)",
	Run: func(cmd *cobra.Command, args []string) {
		runTUI()
	},
}

// menuCmd preserves the previous promptui menu as an explicit fallback for
// terminals without full-screen support or by user preference.
var menuCmd = &cobra.Command{
	Use:   "menu",
	Short: "Launch the classic arrow-key menu (promptui fallback)",
	Run: func(cmd *cobra.Command, args []string) {
		runMenu()
	},
}

// runTUI starts the TUI and degrades to a clear error (suggesting `menu`)
// when it cannot initialize, e.g. no TTY.
func runTUI() {
	if err := tui.Run(version); err != nil {
		core.Error("%v", err)
	}
}

func init() {
	rootCmd.AddCommand(tuiCmd)
	rootCmd.AddCommand(menuCmd)
}

package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var verbose bool

// Build metadata, injected at build time via -ldflags by GoReleaser.
// Defaults are used for local/dev builds.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

var rootCmd = &cobra.Command{
	Use:     "mac-toolkit",
	Short:   "Mac DevOps Toolkit Pro — disk cleanup & system monitoring",
	Long:    `CLI for macOS that covers disk cleanup (11 domains) and system monitors (battery, CPU, memory, network).`,
	Version: fmt.Sprintf("%s (commit %s, built %s)", version, commit, date),
	Run: func(cmd *cobra.Command, args []string) {
		runMenu()
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
}

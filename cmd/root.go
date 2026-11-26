package cmd

import (
	"fmt"
	"os"
	"runtime/debug"

	"github.com/arheanja-ops/mac-toolkit/internal/reporter"

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

// resolveBuildInfo fills in version metadata when ldflags did not. `go install
// module@vX.Y.Z` never runs GoReleaser, but the Go toolchain embeds the module
// version and VCS stamps in the binary, so we read them from there. Values
// injected by GoReleaser always win: we only touch fields still at defaults.
func resolveBuildInfo(ver, com, dt string, info *debug.BuildInfo) (string, string, string) {
	if ver != "dev" || info == nil {
		return ver, com, dt
	}
	if v := info.Main.Version; v != "" && v != "(devel)" {
		ver = v
	}
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			if com == "none" && s.Value != "" {
				com = s.Value
				if len(com) > 7 {
					com = com[:7]
				}
			}
		case "vcs.time":
			if dt == "unknown" && s.Value != "" {
				dt = s.Value
			}
		}
	}
	return ver, com, dt
}

func buildVersion() string {
	info, _ := debug.ReadBuildInfo()
	v, c, d := resolveBuildInfo(version, commit, date, info)
	// Every saved report records the toolkit version that wrote it.
	reporter.ToolVersion = v
	return fmt.Sprintf("%s (commit %s, built %s)", v, c, d)
}

var rootCmd = &cobra.Command{
	Use:     "mac-toolkit",
	Short:   "Mac DevOps Toolkit Pro — disk cleanup & system monitoring",
	Long:    `CLI for macOS that covers disk cleanup (11 domains) and system monitors (battery, CPU, memory, network).`,
	Version: buildVersion(),
	Run: func(cmd *cobra.Command, args []string) {
		runTUI()
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

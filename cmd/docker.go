package cmd

import (
	"fmt"
	"github.com/arheanja-ops/mac-toolkit/internal/core"
	"github.com/arheanja-ops/mac-toolkit/internal/docker"
	"strings"

	"github.com/spf13/cobra"
)

var dockerCmd = &cobra.Command{
	Use:   "docker",
	Short: "Docker operations: backup, cleanup, compact",
}

// --- docker backup ---

var dockerBackupCmd = &cobra.Command{
	Use:   "backup <container>",
	Short: "Backup a container's database (postgres/mysql auto-detected)",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		container := args[0]
		core.Info("Backing up %s...", container)

		result := docker.Backup(container)
		if result.Error != "" {
			core.Error("Backup failed: %s", result.Error)
			return
		}

		core.Success("Backup complete!")
		fmt.Println()
		fmt.Printf("  Engine:     %s\n", result.Engine)
		fmt.Printf("  User:       %s\n", result.User)
		fmt.Printf("  File:       %s\n", result.Path)
		fmt.Printf("  Size:       %s\n", core.FormatBytes(result.SizeBytes))
		fmt.Printf("  Databases:  %d\n", result.DatabasesFound)
		fmt.Printf("  Tables:     %d\n", result.TablesFound)
		fmt.Printf("  Verified:   %v\n", result.Verified)
	},
}

// --- docker cleanup ---

var (
	cleanupKeep    string
	cleanupExecute bool
)

var dockerCleanupCmd = &cobra.Command{
	Use:   "cleanup",
	Short: "Remove all Docker resources except --keep containers",
	Long: "Shows the exact cleanup plan. Without --execute nothing is deleted. " +
		"With --execute the plan is shown and a typed affirmative (\"si\") is required; " +
		"every action is written to an audit log.",
	Run: func(cmd *cobra.Command, args []string) {
		keepList := nonEmptyList(cleanupKeep)
		if len(keepList) == 0 {
			core.Error("--keep is required (comma-separated container names to preserve)")
			return
		}
		runDockerCleanup(keepList, cleanupExecute, docker.ExecRunner{})
	},
}

// --- docker compact ---

var dockerCompactCmd = &cobra.Command{
	Use:   "compact",
	Short: "Analyze Docker.raw size vs actual usage and recommend disk limit",
	Run: func(cmd *cobra.Command, args []string) {
		result := docker.Compact()
		if result.Error != "" && result.RawSize == 0 {
			core.Error(result.Error)
			return
		}

		fmt.Println()
		fmt.Printf("  Docker.raw on disk:  %s\n", result.RawSizeHuman)
		fmt.Printf("  Actual Docker usage: %s\n", result.ActualUsedHuman)
		fmt.Printf("  Recommended limit:   %s\n", result.RecommendedHuman)
		fmt.Println()
		fmt.Println(result.Instructions)
	},
}

func init() {
	dockerCleanupCmd.Flags().StringVar(&cleanupKeep, "keep", "", "comma-separated container names to preserve (required)")
	dockerCleanupCmd.Flags().BoolVar(&cleanupExecute, "execute", false, "actually delete (default: dry-run)")

	dockerCmd.AddCommand(dockerBackupCmd)
	dockerCmd.AddCommand(dockerCleanupCmd)
	dockerCmd.AddCommand(dockerCompactCmd)
	rootCmd.AddCommand(dockerCmd)
}

// --- helpers ---

func nonEmptyList(csv string) []string {
	var out []string
	for _, s := range strings.Split(csv, ",") {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}

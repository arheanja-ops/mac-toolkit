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
	Run: func(cmd *cobra.Command, args []string) {
		keepList := nonEmptyList(cleanupKeep)
		if len(keepList) == 0 {
			core.Error("--keep is required (comma-separated container names to preserve)")
			return
		}

		dryRun := !cleanupExecute
		if dryRun {
			core.Info("Dry-run mode — nothing will be deleted. Pass --execute to delete.")
		} else {
			core.Warn("Execute mode — deletions are permanent!")
		}

		result := docker.Cleanup(keepList, dryRun)
		if result.Error != "" {
			core.Error("Cleanup failed: %s", result.Error)
			return
		}

		fmt.Println()
		fmt.Printf("  Kept:               %s\n", strings.Join(result.Kept, ", "))
		fmt.Printf("  Containers removed: %d %s\n", len(result.DeletedContainers), nameList(result.DeletedContainers))
		fmt.Printf("  Images removed:     %d %s\n", len(result.DeletedImages), nameList(result.DeletedImages))
		fmt.Printf("  Volumes removed:    %d %s\n", len(result.DeletedVolumes), nameList(result.DeletedVolumes))
		fmt.Printf("  Build cache:        %s\n", result.BuildCacheFreed)

		if dryRun {
			fmt.Println()
			core.Info("This was a dry-run. Run with --execute to apply.")
		} else {
			fmt.Println()
			core.Success("Cleanup complete!")
		}
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

func nameList(names []string) string {
	if len(names) == 0 {
		return ""
	}
	if len(names) <= 5 {
		return "(" + strings.Join(names, ", ") + ")"
	}
	return fmt.Sprintf("(%s, ... +%d more)", strings.Join(names[:3], ", "), len(names)-3)
}

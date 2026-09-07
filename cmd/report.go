package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/arheanja-ops/mac-toolkit/internal/core"

	"github.com/spf13/cobra"
)

var reportLast bool

var reportCmd = &cobra.Command{
	Use:   "report",
	Short: "View saved reports",
	Run: func(cmd *cobra.Command, args []string) {
		reportsDir := "reports"
		entries, err := os.ReadDir(reportsDir)
		if err != nil {
			core.Error("No reports found: %v", err)
			return
		}

		if len(entries) == 0 {
			core.Info("No reports saved yet")
			return
		}

		// Sort by name (timestamp-based names sort chronologically)
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].Name() > entries[j].Name()
		})

		if reportLast {
			// Show the latest report
			latest := entries[0]
			mdPath := filepath.Join(reportsDir, latest.Name(), "report.md")
			data, err := os.ReadFile(mdPath)
			if err != nil {
				core.Error("Cannot read report: %v", err)
				return
			}
			fmt.Println(string(data))
		} else {
			// List all reports
			core.Bold("Saved Reports")
			for _, e := range entries {
				fmt.Printf("  %s\n", e.Name())
			}
		}
	},
}

func init() {
	reportCmd.Flags().BoolVar(&reportLast, "last", false, "show the last saved report")
	rootCmd.AddCommand(reportCmd)
}

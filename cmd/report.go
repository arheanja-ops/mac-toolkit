package cmd

import (
	"fmt"
	"os"

	"github.com/arheanja-ops/mac-toolkit/internal/core"

	"github.com/spf13/cobra"
)

var reportLast bool

var reportCmd = &cobra.Command{
	Use:   "report",
	Short: "View saved reports",
	Long: "View saved analysis reports. They live in a fixed folder (printed below), " +
		"so the command works from any directory. Create one with `analyze --save`.",
	Run: func(cmd *cobra.Command, args []string) {
		list, err := core.ListReports()
		if err != nil {
			core.Error("Reports: %v", err)
			return
		}
		core.Info("Carpeta de reportes: %s", core.ReportsDir())
		if len(list) == 0 {
			core.Info("Aún no hay reportes. Crea uno con: mac-toolkit analyze --save")
			return
		}
		if reportLast {
			data, err := os.ReadFile(list[0].Path)
			if err != nil {
				core.Error("Cannot read report: %v", err)
				return
			}
			fmt.Println(string(data))
			return
		}
		core.Bold("Saved Reports")
		for _, e := range list {
			fmt.Printf("  %s  %s\n", e.Name, core.FormatBytes(e.Size))
		}
	},
}

func init() {
	reportCmd.Flags().BoolVar(&reportLast, "last", false, "show the last saved report")
	rootCmd.AddCommand(reportCmd)
}

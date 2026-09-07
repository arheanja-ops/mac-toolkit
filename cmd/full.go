package cmd

import (
	"github.com/spf13/cobra"
)

var (
	fullMode    string
	fullExecute bool
)

var fullCmd = &cobra.Command{
	Use:   "full",
	Short: "Full flow: analyze + save reports + clean",
	Run: func(cmd *cobra.Command, args []string) {
		// First analyze + save
		results := runAnalysis("")
		if results != nil {
			saveReports(results)
		}
		// Then clean
		runClean("", fullMode, fullExecute)
	},
}

func init() {
	fullCmd.Flags().StringVar(&fullMode, "mode", "deal", "approval mode: deal|category|item|checklist")
	fullCmd.Flags().BoolVar(&fullExecute, "execute", false, "actually delete files (default: dry-run)")
	rootCmd.AddCommand(fullCmd)
}

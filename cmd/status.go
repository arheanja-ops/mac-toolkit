package cmd

import (
	"fmt"

	"github.com/arheanja-ops/mac-toolkit/internal/analyzer"
	"github.com/arheanja-ops/mac-toolkit/internal/core"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show registered domains and their risk levels",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println()
		core.Bold("📋 Registered Domains")
		fmt.Println("─────────────────────────────")
		fmt.Printf("  %-15s %s\n", "Domain", "Risk")
		fmt.Println("─────────────────────────────")
		for _, a := range analyzer.All() {
			color := core.RiskColor(a.Risk())
			fmt.Printf("  %-15s %s%s\033[0m\n", a.Domain(), color, string(a.Risk()))
		}
		fmt.Println("─────────────────────────────")
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

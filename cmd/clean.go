package cmd

import (
	"github.com/arheanja-ops/mac-toolkit/internal/cleaner"
	"github.com/arheanja-ops/mac-toolkit/internal/core"
	"github.com/arheanja-ops/mac-toolkit/internal/reporter"

	"github.com/spf13/cobra"
)

var (
	cleanMode    string
	cleanExecute bool
	cleanDomain  string
)

var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Analyze and clean disk with interactive approval",
	Run: func(cmd *cobra.Command, args []string) {
		runClean(cleanDomain, cleanMode, cleanExecute)
	},
}

func init() {
	cleanCmd.Flags().StringVar(&cleanMode, "mode", "deal", "approval mode: deal|category|item|checklist")
	cleanCmd.Flags().BoolVar(&cleanExecute, "execute", false, "actually delete files (default: dry-run)")
	cleanCmd.Flags().StringVar(&cleanDomain, "domain", "", "clean only this domain")
	rootCmd.AddCommand(cleanCmd)
}

func runClean(domain, mode string, execute bool) {
	results := runAnalysis(domain)
	if results == nil {
		return
	}

	// Show summary first
	tr := &reporter.TerminalReporter{}
	tr.Report(results)

	// Approval
	dryRun := !execute
	approvalMode := core.ApprovalMode(mode)
	engine := core.NewApprovalEngine(approvalMode, dryRun, execute)
	approved := engine.FilterItems(results)

	if len(approved) == 0 {
		core.Info("No items approved for deletion")
		return
	}

	// Preview
	reporter.PrintPreviewTable(approved)

	// Clean
	gc := cleaner.NewGenericCleaner(dryRun, execute)
	delResults := gc.Clean(approved)

	// Audit
	audit := &reporter.AuditReporter{
		DryRun:       dryRun,
		ApprovalMode: approvalMode,
	}
	if err := audit.WriteAudit(delResults); err != nil {
		core.Error("Audit log: %v", err)
	}

	// Summary
	var deleted int
	var freedBytes int64
	for _, r := range delResults {
		if r.Result == "success" {
			deleted++
			freedBytes += r.SizeBytes
		}
	}
	core.Success("Cleaned %d items, freed %s", deleted, core.FormatBytes(freedBytes))
}

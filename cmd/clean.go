package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"

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

	// Always show the full cleanup plan first (dry-run preview with risks).
	// Nothing is deleted at this stage.
	safeCount, _ := reporter.PrintCleanPlan(results)

	if !execute {
		core.Info("Dry-run — no files deleted. Re-run with --execute to clean with approval.")
		return
	}

	if safeCount == 0 {
		core.Info("No items marked safe to auto-delete; nothing to clean.")
		return
	}

	// Execution path: approval, then delete.
	approvalMode := core.ApprovalMode(mode)
	engine := core.NewApprovalEngine(approvalMode, false, true)
	approved := engine.FilterItems(results)

	if len(approved) == 0 {
		core.Info("No items approved for deletion")
		return
	}

	// Final confirmation table of exactly what will be deleted.
	reporter.PrintPreviewTable(approved)

	// Last-line-of-defense gate: in addition to the per-mode approval above,
	// require one explicit affirmative typed on stdin that names the exact
	// count and total size. This guarantees no deletion ever happens from a
	// stray flag or an accidental Enter — the user must consciously type "si".
	if !confirmFinalDeletion(approved) {
		core.Info("Aborted — nothing was deleted.")
		return
	}

	// Clean
	gc := cleaner.NewGenericCleaner(false, true)
	delResults := gc.Clean(approved)

	// Audit
	audit := &reporter.AuditReporter{
		DryRun:       false,
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

// finalConfirmReader is the source of the final-confirmation input. It is a
// package var defaulting to os.Stdin so tests can feed a scripted answer
// without a real terminal — mirroring the injectable-var pattern used
// elsewhere in the codebase.
var finalConfirmReader io.Reader = os.Stdin

// confirmFinalDeletion is the explicit, additional human gate shown right
// before any file is removed. It states the exact number of items and total
// size, then only returns true when the user types an unambiguous affirmative
// ("si"/"sí"/"yes"/"y"). Any other input — including an empty line or EOF —
// is treated as "no", so the safe default is always to NOT delete.
func confirmFinalDeletion(items []core.CleanableItem) bool {
	var total int64
	for _, it := range items {
		total += it.SizeBytes
	}

	core.Warn("Esto eliminará %d elementos (%s). Esta acción es irreversible.",
		len(items), core.FormatBytes(total))
	fmt.Fprint(os.Stderr, `Escribe "si" para confirmar: `)

	line, err := bufio.NewReader(finalConfirmReader).ReadString('\n')
	if err != nil && line == "" {
		return false // EOF / read error with no input → never delete
	}
	// Delegate the affirmative decision to the shared core gate so the CLI and
	// the TUI modal can never disagree about what counts as "yes".
	return core.IsAffirmative(line)
}

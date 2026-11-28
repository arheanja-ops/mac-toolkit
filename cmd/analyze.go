package cmd

import (
	"fmt"

	"github.com/arheanja-ops/mac-toolkit/internal/analyzer"
	"github.com/arheanja-ops/mac-toolkit/internal/core"
	"github.com/arheanja-ops/mac-toolkit/internal/reporter"

	"github.com/spf13/cobra"
)

var (
	analyzeSave    bool
	analyzeShare   bool
	analyzeOpen    bool
	analyzeOut     string
	analyzeMinSize int
	analyzeDomain  string
)

var analyzeCmd = &cobra.Command{
	Use:   "analyze",
	Short: "Run disk analysis across all domains",
	Run: func(cmd *cobra.Command, args []string) {
		results := runAnalysis(analyzeDomain)
		tr := &reporter.TerminalReporter{}
		tr.Report(results)

		// --share, --open and --out need the files on disk, so they imply --save.
		if analyzeSave || analyzeShare || analyzeOpen || analyzeOut != "" {
			saveReports(results, analyzeShare, analyzeOpen, analyzeOut)
		}
		// Explicit end-of-run signal: the human report goes to stderr, which can
		// read as "nothing happened" on success. A final ✓ confirms completion.
		core.Success("Listo (analyze)")
	},
}

func init() {
	analyzeCmd.Flags().BoolVar(&analyzeSave, "save", false, "save the report (md, json, html and the share variant)")
	analyzeCmd.Flags().BoolVar(&analyzeShare, "share", false, "print the paths of the share variant (no paths or names; implies --save)")
	analyzeCmd.Flags().BoolVar(&analyzeOpen, "open", false, "open the html report in the browser (implies --save)")
	analyzeCmd.Flags().StringVar(&analyzeOut, "out", "", "also copy the report to this folder, e.g. ~/Downloads (implies --save)")
	analyzeCmd.Flags().IntVar(&analyzeMinSize, "min-size", 50, "minimum file size in MB")
	analyzeCmd.Flags().StringVar(&analyzeDomain, "domain", "", "analyze only this domain")
	rootCmd.AddCommand(analyzeCmd)
}

// runAnalysis runs analyzers and returns results
func runAnalysis(domain string) []core.AnalysisResult {
	var tasks []core.RunnerTask

	if domain != "" {
		a := analyzer.ByDomain(domain)
		if a == nil {
			core.Error("Unknown domain: %s", domain)
			return nil
		}
		tasks = []core.RunnerTask{{Domain: a.Domain(), AnalyzeFn: a.Analyze}}
	} else {
		for _, a := range analyzer.All() {
			tasks = append(tasks, core.RunnerTask{Domain: a.Domain(), AnalyzeFn: a.Analyze})
		}
	}

	core.Info("Running analysis on %d domain(s)...", len(tasks))
	return core.RunAnalyzers(tasks, core.AnalyzerTimeoutSeconds)
}

// saveReports builds the single report model, saves it (full + share variant)
// into the canonical folder (core.ReportsDir) and prints the md, json and html
// paths of the requested variant. open launches the html with the system opener;
// out also copies that variant to a folder the user chose.
func saveReports(results []core.AnalysisResult, share, open bool, out string) {
	saved, err := reporter.SaveReport(reporter.Collect(results))
	if err != nil {
		core.Error("Report: %v", err)
		return
	}
	paths, label := saved.Full, "Reporte completo"
	if share {
		paths, label = saved.Share, "Reporte para compartir (sin rutas ni nombres)"
	}
	core.Info("%s:", label)
	for _, p := range paths {
		fmt.Println(p)
	}
	if out != "" {
		if err := exportReport(saved.Dir, out, share); err != nil {
			core.Error("Export: %v", err)
		}
	}
	if open {
		p, err := reporter.OpenReport(saved.Dir, share)
		if err != nil {
			core.Error("Open: %v", err)
			return
		}
		core.Info("Abierto: %s", p)
	}
}

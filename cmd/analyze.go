package cmd

import (
	"github.com/arheanja-ops/mac-toolkit/internal/analyzer"
	"github.com/arheanja-ops/mac-toolkit/internal/core"
	"github.com/arheanja-ops/mac-toolkit/internal/reporter"

	"github.com/spf13/cobra"
)

var (
	analyzeSave    bool
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

		if analyzeSave {
			saveReports(results)
		}
	},
}

func init() {
	analyzeCmd.Flags().BoolVar(&analyzeSave, "save", false, "save report to file")
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

// saveReports saves both markdown and JSON reports
func saveReports(results []core.AnalysisResult) {
	md := &reporter.MarkdownReporter{}
	if err := md.Report(results); err != nil {
		core.Error("Markdown report: %v", err)
	}
	jr := &reporter.JSONReporter{OutputDir: md.OutputDir}
	if err := jr.Report(results); err != nil {
		core.Error("JSON report: %v", err)
	}
}

package reporter

import (
	"fmt"

	"github.com/arheanja-ops/mac-toolkit/internal/core"
)

// SaveAnalysis writes the markdown and JSON reports of one analysis run into a
// new timestamped folder under core.ReportsDir() and returns that folder. It is
// the single save path shared by `analyze --save`, `full` and the TUI "Save
// report" action. quiet suppresses the per-file stderr confirmations.
func SaveAnalysis(results []core.AnalysisResult, quiet bool) (string, error) {
	md := &MarkdownReporter{Quiet: quiet}
	if err := md.Report(results); err != nil {
		return "", fmt.Errorf("markdown report: %w", err)
	}
	jr := &JSONReporter{OutputDir: md.OutputDir, Quiet: quiet}
	if err := jr.Report(results); err != nil {
		return md.OutputDir, fmt.Errorf("json report: %w", err)
	}
	return md.OutputDir, nil
}

package reporter

import (
	"fmt"
	"github.com/arheanja-ops/mac-toolkit/internal/core"
	"os"
	"strings"
)

type TerminalReporter struct{}

func (t *TerminalReporter) Name() string { return "terminal" }

func (t *TerminalReporter) Report(results []core.AnalysisResult) error {
	PrintSummary(results)
	PrintItems(results)
	return nil
}

// PrintSummary prints the domain summary table
func PrintSummary(results []core.AnalysisResult) {
	fmt.Fprintln(os.Stderr)
	core.Bold("📊 Analysis Summary")
	fmt.Fprintln(os.Stderr, strings.Repeat("─", 65))
	fmt.Fprintf(os.Stderr, "  %-15s %-10s %-12s %-8s %s\n",
		"Domain", "Severity", "Size", "Items", "Summary")
	fmt.Fprintln(os.Stderr, strings.Repeat("─", 65))

	var totalSize int64
	var totalItems int

	for _, r := range results {
		if r.Error != "" {
			fmt.Fprintf(os.Stderr, "  %-15s \033[31m%-10s\033[0m %-12s %-8s %s\n",
				r.Domain, "error", "-", "-", r.Error)
			continue
		}

		color := core.SeverityColor(r.Severity)
		fmt.Fprintf(os.Stderr, "  %-15s %s%-10s\033[0m %-12s %-8d %s\n",
			r.Domain, color, string(r.Severity),
			core.FormatBytes(r.TotalSize), len(r.Items), r.Summary)

		totalSize += r.TotalSize
		totalItems += len(r.Items)
	}

	fmt.Fprintln(os.Stderr, strings.Repeat("─", 65))
	core.Bold("Total: %s in %d items across %d domains",
		core.FormatBytes(totalSize), totalItems, len(results))
}

// PrintItems prints per-domain item details
func PrintItems(results []core.AnalysisResult) {
	for _, r := range results {
		if len(r.Items) == 0 {
			continue
		}

		fmt.Fprintln(os.Stderr)
		core.Bold("  %s (%d items)", r.Domain, len(r.Items))

		for _, item := range r.Items {
			safe := "✓"
			if !item.SafeToDelete {
				safe = "✗"
			}
			riskColor := core.RiskColor(item.Risk)
			fmt.Fprintf(os.Stderr, "    %s%-5s\033[0m %s %-12s %s\n",
				riskColor, string(item.Risk),
				safe, core.FormatBytes(item.SizeBytes), item.Label)
		}
	}
}

// PrintPreviewTable shows items about to be deleted
func PrintPreviewTable(items []core.CleanableItem) {
	fmt.Fprintln(os.Stderr)
	core.Bold("🗑  Preview: items to be deleted")
	fmt.Fprintln(os.Stderr, strings.Repeat("─", 70))
	fmt.Fprintf(os.Stderr, "  %-12s %-6s %-8s %-6s %s\n",
		"Size", "Risk", "Age", "Safe", "Path")
	fmt.Fprintln(os.Stderr, strings.Repeat("─", 70))

	for _, item := range items {
		safe := "✓"
		if !item.SafeToDelete {
			safe = "✗"
		}
		ageStr := "-"
		if item.AgeDays > 0 {
			ageStr = fmt.Sprintf("%dd", item.AgeDays)
		}
		riskColor := core.RiskColor(item.Risk)
		fmt.Fprintf(os.Stderr, "  %-12s %s%-6s\033[0m %-8s %-6s %s\n",
			core.FormatBytes(item.SizeBytes), riskColor, string(item.Risk),
			ageStr, safe, item.Path)
	}

	fmt.Fprintln(os.Stderr, strings.Repeat("─", 70))
}

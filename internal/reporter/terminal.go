package reporter

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/arheanja-ops/mac-toolkit/internal/core"
)

// maxItemsPerDomain caps the per-domain detail list; the remainder is summarized.
const maxItemsPerDomain = 8

type TerminalReporter struct{}

func (t *TerminalReporter) Name() string { return "terminal" }

func (t *TerminalReporter) Report(results []core.AnalysisResult) error {
	PrintSummary(results)
	PrintItems(results)
	return nil
}

// domainProportionBar renders a small bar showing this domain's share of the
// total reclaimable size, for quick visual scanning of where space goes.
func domainProportionBar(size, total int64, width int) string {
	pct := 0
	if total > 0 {
		pct = int(size * 100 / total)
	}
	filled := pct * width / 100
	return core.Cyan + strings.Repeat("█", filled) +
		core.Gray + strings.Repeat("░", width-filled) + core.Reset
}

// PrintSummary prints the domain summary table, sorted by size descending,
// with a proportion bar per domain.
func PrintSummary(results []core.AnalysisResult) {
	var totalSize int64
	var totalItems int
	for _, r := range results {
		if r.Error == "" {
			totalSize += r.TotalSize
			totalItems += len(r.Items)
		}
	}

	// Sort a copy by size desc so the biggest offenders come first.
	sorted := make([]core.AnalysisResult, len(results))
	copy(sorted, results)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].TotalSize > sorted[j].TotalSize
	})

	fmt.Fprintln(os.Stderr)
	core.Bold("📊 Analysis Summary")
	fmt.Fprintln(os.Stderr, core.Colorize(core.Gray, strings.Repeat("─", 72)))
	fmt.Fprintf(os.Stderr, "  %-13s %-10s %-11s %-7s %s\n",
		"Domain", "Severity", "Size", "Items", "Share")
	fmt.Fprintln(os.Stderr, core.Colorize(core.Gray, strings.Repeat("─", 72)))

	for _, r := range sorted {
		if r.Error != "" {
			fmt.Fprintf(os.Stderr, "  %-13s %s%-10s%s %-11s %-7s %s\n",
				r.Domain, core.RiskColor(core.RiskDanger), "error", core.Reset,
				"-", "-", core.Colorize(core.Gray, r.Error))
			continue
		}

		color := core.SeverityColor(r.Severity)
		bar := domainProportionBar(r.TotalSize, totalSize, 18)
		fmt.Fprintf(os.Stderr, "  %-13s %s%-10s%s %-11s %-7d %s\n",
			r.Domain, color, string(r.Severity), core.Reset,
			core.FormatBytes(r.TotalSize), len(r.Items), bar)
	}

	fmt.Fprintln(os.Stderr, core.Colorize(core.Gray, strings.Repeat("─", 72)))
	core.Bold("Total: %s in %d items across %d domains",
		core.FormatBytes(totalSize), totalItems, len(results))
}

// PrintItems prints per-domain item details, largest items first, capped at
// maxItemsPerDomain with a summarized remainder.
func PrintItems(results []core.AnalysisResult) {
	for _, r := range results {
		if len(r.Items) == 0 {
			continue
		}

		items := make([]core.CleanableItem, len(r.Items))
		copy(items, r.Items)
		sort.SliceStable(items, func(i, j int) bool {
			return items[i].SizeBytes > items[j].SizeBytes
		})

		fmt.Fprintln(os.Stderr)
		core.Bold("  %s — %s (%d items)", r.Domain, core.FormatBytes(r.TotalSize), len(r.Items))

		shown := items
		if len(shown) > maxItemsPerDomain {
			shown = shown[:maxItemsPerDomain]
		}

		for _, item := range shown {
			safe := core.Colorize(core.RiskColor(core.RiskSafe), "✓")
			if !item.SafeToDelete {
				safe = core.Colorize(core.RiskColor(core.RiskDanger), "✗")
			}
			riskColor := core.RiskColor(item.Risk)
			fmt.Fprintf(os.Stderr, "    %s %s%-6s%s %11s   %s\n",
				safe, riskColor, string(item.Risk), core.Reset,
				core.FormatBytes(item.SizeBytes), item.Label)
		}

		if remain := len(items) - len(shown); remain > 0 {
			var remainBytes int64
			for _, it := range items[len(shown):] {
				remainBytes += it.SizeBytes
			}
			fmt.Fprintln(os.Stderr, core.Colorize(core.Gray,
				fmt.Sprintf("    … and %d more (%s)", remain, core.FormatBytes(remainBytes))))
		}
	}
}

// PrintPreviewTable shows items about to be deleted, largest first.
func PrintPreviewTable(items []core.CleanableItem) {
	sorted := make([]core.CleanableItem, len(items))
	copy(sorted, items)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].SizeBytes > sorted[j].SizeBytes
	})

	fmt.Fprintln(os.Stderr)
	core.Bold("🗑  Preview: items to be deleted")
	fmt.Fprintln(os.Stderr, core.Colorize(core.Gray, strings.Repeat("─", 76)))
	fmt.Fprintf(os.Stderr, "  %-11s %-7s %-6s %s\n", "Size", "Risk", "Age", "Path")
	fmt.Fprintln(os.Stderr, core.Colorize(core.Gray, strings.Repeat("─", 76)))

	var total int64
	for _, item := range sorted {
		ageStr := "-"
		if item.AgeDays > 0 {
			ageStr = fmt.Sprintf("%dd", item.AgeDays)
		}
		riskColor := core.RiskColor(item.Risk)
		fmt.Fprintf(os.Stderr, "  %11s %s%-7s%s %-6s %s\n",
			core.FormatBytes(item.SizeBytes), riskColor, string(item.Risk), core.Reset,
			ageStr, item.Path)
		total += item.SizeBytes
	}

	fmt.Fprintln(os.Stderr, core.Colorize(core.Gray, strings.Repeat("─", 76)))
	core.Bold("Total to delete: %s in %d items", core.FormatBytes(total), len(sorted))
}

// PrintCleanPlan renders the full cleanup plan grouped by domain (largest
// domains first), with size, risk, age and path per item plus per-domain and
// grand totals. It is a read-only preview used in dry-run before any approval.
func PrintCleanPlan(results []core.AnalysisResult) (safeCount int, safeBytes int64) {
	sorted := make([]core.AnalysisResult, len(results))
	copy(sorted, results)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].TotalSize > sorted[j].TotalSize
	})

	fmt.Fprintln(os.Stderr)
	core.Bold("🔎 Cleanup preview (dry-run) — nothing will be deleted")
	fmt.Fprintln(os.Stderr, core.Colorize(core.Gray, strings.Repeat("─", 76)))
	fmt.Fprintf(os.Stderr, "  Legend: %s✓ safe%s auto-regenerated · %s⚠ warn%s review · %s✗ danger%s manual only\n",
		core.RiskColor(core.RiskSafe), core.Reset,
		core.RiskColor(core.RiskWarn), core.Reset,
		core.RiskColor(core.RiskDanger), core.Reset)
	fmt.Fprintln(os.Stderr, core.Colorize(core.Gray, strings.Repeat("─", 76)))

	var grandBytes int64
	var grandItems int

	for _, r := range sorted {
		if len(r.Items) == 0 {
			continue
		}

		items := make([]core.CleanableItem, len(r.Items))
		copy(items, r.Items)
		sort.SliceStable(items, func(i, j int) bool {
			return items[i].SizeBytes > items[j].SizeBytes
		})

		var domainBytes int64
		for _, item := range items {
			domainBytes += item.SizeBytes
		}

		fmt.Fprintln(os.Stderr)
		core.Bold("  %s — %s (%d items)", r.Domain, core.FormatBytes(domainBytes), len(items))

		shown := items
		if len(shown) > maxItemsPerDomain {
			shown = shown[:maxItemsPerDomain]
		}

		for _, item := range shown {
			safe := core.Colorize(core.RiskColor(core.RiskSafe), "✓")
			if !item.SafeToDelete {
				safe = core.Colorize(core.RiskColor(core.RiskDanger), "✗")
			}
			ageStr := "-"
			if item.AgeDays > 0 {
				ageStr = fmt.Sprintf("%dd", item.AgeDays)
			}
			riskColor := core.RiskColor(item.Risk)
			fmt.Fprintf(os.Stderr, "    %s %s%-6s%s %11s  %-6s %s\n",
				safe, riskColor, string(item.Risk), core.Reset,
				core.FormatBytes(item.SizeBytes), ageStr, item.Path)

			if item.SafeToDelete {
				safeCount++
				safeBytes += item.SizeBytes
			}
		}

		// Count the remainder toward safe totals even if not printed.
		if len(items) > len(shown) {
			var remainBytes int64
			for _, it := range items[len(shown):] {
				remainBytes += it.SizeBytes
				if it.SafeToDelete {
					safeCount++
					safeBytes += it.SizeBytes
				}
			}
			fmt.Fprintln(os.Stderr, core.Colorize(core.Gray,
				fmt.Sprintf("    … and %d more (%s)", len(items)-len(shown), core.FormatBytes(remainBytes))))
		}

		grandItems += len(items)
		grandBytes += domainBytes
	}

	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, core.Colorize(core.Gray, strings.Repeat("─", 76)))
	core.Bold("Total: %s in %d items · safe to auto-delete: %s%s in %d items%s",
		core.FormatBytes(grandBytes), grandItems,
		core.RiskColor(core.RiskSafe), core.FormatBytes(safeBytes), safeCount, core.Reset)
	fmt.Fprintln(os.Stderr, core.Colorize(core.Gray, strings.Repeat("─", 76)))
	return safeCount, safeBytes
}

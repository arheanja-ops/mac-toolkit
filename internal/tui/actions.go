package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/arheanja-ops/mac-toolkit/internal/analyzer"
	"github.com/arheanja-ops/mac-toolkit/internal/core"
	"github.com/arheanja-ops/mac-toolkit/internal/monitor"

	"github.com/charmbracelet/lipgloss"
)

// actionItem is a single read-only entry in the left panel. run adapts an
// analyzer/monitor/report to renderable text so the model never reimplements
// domain logic — it only presents the captured output.
type actionItem struct {
	label string
	group string
	run   func() (string, error)
	// quit marks a control action that must terminate the program via tea.Quit
	// rather than run(). run() returns (string,error) and cannot close the TUI,
	// so the model checks this flag in execSelected() instead.
	quit bool
	// openClean marks the "Clean (select & delete)" action: instead of running,
	// the model enters the interactive select mode (F2). Like quit, this is a
	// control action the model handles specially rather than via run().
	openClean bool
}

// buildActions returns the ordered, grouped action set. Phase TUI-1 actions
// (Analyze, Status, monitors, Last report, Quit) stay read-only. Phase TUI-2
// adds a Clean group: Preview (dry-run, never deletes) and Clean (opens the
// interactive select-and-delete flow, which still requires the F3 "si" gate).
func buildActions() []actionItem {
	return []actionItem{
		{label: "Analyze", group: "Disk", run: runAnalyze},
		{label: "Status", group: "Disk", run: runStatus},
		{label: "Preview", group: "Clean", run: runPreview},
		{label: "Clean (select & delete)", group: "Clean", openClean: true},
		{label: "Battery", group: "Monitors", run: monitorRunner(&monitor.BatteryMonitor{})},
		{label: "System", group: "Monitors", run: monitorRunner(&monitor.SystemMonitor{})},
		{label: "Processes", group: "Monitors", run: monitorRunner(&monitor.ProcessMonitor{})},
		{label: "Network", group: "Monitors", run: monitorRunner(&monitor.NetworkMonitor{})},
		{label: "Last report", group: "Reports", run: runLastReport},
		// Session controls. Quit has no run(); it is handled specially by the
		// model, which returns tea.Quit when this item is selected.
		{label: "Quit / Salir", group: "Session", quit: true},
	}
}

// runAnalyze reuses core.RunAnalyzers (via analysisFn) over the registered
// analyzers and formats the results as a plain-text table for the viewport.
func runAnalyze() (string, error) {
	return formatAnalysis(analysisFn()), nil
}

// formatAnalysis renders analysis results largest-first as an aligned, colored
// table with fixed-width columns (Domain, Severity, Size right-aligned, Items,
// and a proportion bar) so the detail pane reads like the doctor/reporter
// output rather than ragged plain text. Colors come from the shared palette
// keyed on severity, keeping parity with the terminal reporter.
func formatAnalysis(results []core.AnalysisResult) string {
	sorted := make([]core.AnalysisResult, len(results))
	copy(sorted, results)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].TotalSize > sorted[j].TotalSize
	})

	// The proportion bar is scaled against the largest single domain so the
	// biggest consumer fills the bar and everything else reads relative to it.
	var maxSize int64
	for _, r := range sorted {
		if r.Error == "" && r.TotalSize > maxSize {
			maxSize = r.TotalSize
		}
	}

	var b strings.Builder
	var total int64     // sum of genuinely reclaimable junk (excludes disk + errors)
	var diskUsage int64 // the "disk" domain reports volume usage, not junk
	var haveDisk bool

	fmt.Fprintln(&b, tableHeaderStyle.Render(
		fmt.Sprintf("%-14s %-9s %11s %6s  %s", "DOMAIN", "SEVERITY", "SIZE", "ITEMS", "SHARE")))
	fmt.Fprintln(&b, ruleStyle.Render(strings.Repeat("─", 62)))

	for _, r := range sorted {
		if r.Error != "" {
			// Errored domains have no trustworthy size; never fold into the total.
			b.WriteString(analysisErrorRow(r))
			continue
		}
		// The "disk" domain is the volume's used space (~hundreds of GB), not
		// deletable garbage. Track it separately as context, not as reclaimable.
		if r.Domain == "disk" {
			diskUsage = r.TotalSize
			haveDisk = true
		} else {
			total += r.TotalSize
		}
		b.WriteString(analysisRow(r, maxSize))
	}

	fmt.Fprintln(&b, ruleStyle.Render(strings.Repeat("─", 62)))
	if haveDisk {
		fmt.Fprintf(&b, "Disk usage (context): %s\n", core.FormatBytes(diskUsage))
	}
	// Summary lines are left unstyled so they remain machine-greppable (the CLI
	// and tests match these exact strings); the table above carries the color.
	fmt.Fprintf(&b, "Total reclaimable (excl. disk): %s\n", core.FormatBytes(total))
	return b.String()
}

// analysisRow renders one data row with a severity-colored domain name and a
// proportion bar scaled to maxSize.
func analysisRow(r core.AnalysisResult, maxSize int64) string {
	color := severityColor(r.Severity)
	domain := lipgloss.NewStyle().Foreground(color).Render(fmt.Sprintf("%-14s", r.Domain))
	sev := lipgloss.NewStyle().Foreground(color).Render(fmt.Sprintf("%-9s", string(r.Severity)))
	bar := proportionBar(r.TotalSize, maxSize, 12)
	return fmt.Sprintf("%s %s %11s %6d  %s\n",
		domain, sev, core.FormatBytes(r.TotalSize), len(r.Items), bar)
}

// analysisErrorRow renders a domain whose analysis failed. It carries no size,
// so it shows a dash and the error message in the danger color.
func analysisErrorRow(r core.AnalysisResult) string {
	domain := lipgloss.NewStyle().Foreground(riskDanger).Render(fmt.Sprintf("%-14s", r.Domain))
	sev := lipgloss.NewStyle().Foreground(riskDanger).Render(fmt.Sprintf("%-9s", "error"))
	return fmt.Sprintf("%s %s %11s %6s  %s\n", domain, sev, "-", "-", r.Error)
}

// proportionBar renders a fixed-width unicode bar showing size relative to the
// largest domain. It is intentionally uncolored (neutral faint) so the severity
// color on the row stays the only semantic color per line.
func proportionBar(size, max int64, width int) string {
	if max <= 0 || size <= 0 {
		return ruleStyle.Render(strings.Repeat("·", width))
	}
	filled := int(size * int64(width) / max)
	if filled > width {
		filled = width
	}
	if filled < 1 {
		filled = 1 // any non-zero domain shows at least one cell
	}
	return lipgloss.NewStyle().Foreground(accent).Render(strings.Repeat("▰", filled)) +
		ruleStyle.Render(strings.Repeat("▱", width-filled))
}

// runPreview renders the cleanup plan as a dry-run, tabulated for the viewport.
// It is the F1 "Preview" action: it NEVER deletes. It reuses formatAnalysis for
// the per-domain summary table, then appends a per-item safe/risk breakdown so
// the user sees exactly what the interactive Clean flow would offer — the same
// information as the CLI's PrintCleanPlan, adapted to the detail pane.
func runPreview() (string, error) {
	results := analysisFn()
	var b strings.Builder
	b.WriteString(formatAnalysis(results))
	b.WriteString("\n")
	b.WriteString(tableHeaderStyle.Render("Cleanup plan (dry-run) — nada se borra") + "\n")
	b.WriteString(ruleStyle.Render(strings.Repeat("─", 62)) + "\n")

	rows := buildCleanRows(results)
	if len(rows) == 0 {
		b.WriteString("  (no reclaimable items)\n")
		return b.String(), nil
	}
	safeBytes, safeCount := markedTotals(rows)
	for _, r := range rows {
		mark := cleanDimRow.Render("·")
		if r.markable {
			mark = lipgloss.NewStyle().Foreground(riskSafe).Render("✓")
		}
		risk := lipgloss.NewStyle().Foreground(riskColor(r.item.Risk)).
			Render(fmt.Sprintf("%-6s", string(r.item.Risk)))
		fmt.Fprintf(&b, "  %s %s %10s  %s · %s\n",
			mark, risk, core.FormatBytes(r.item.SizeBytes), r.item.Domain, r.item.Label)
	}
	b.WriteString(ruleStyle.Render(strings.Repeat("─", 62)) + "\n")
	fmt.Fprintf(&b, "Safe to auto-delete: %s in %d items\n",
		core.FormatBytes(safeBytes), safeCount)
	return b.String(), nil
}

// runStatus reuses the analyzer registry (same source as statusCmd) to list
// domains and their risk levels, each risk colored via the shared palette.
func runStatus() (string, error) {
	var b strings.Builder
	fmt.Fprintln(&b, tableHeaderStyle.Render(fmt.Sprintf("%-15s %s", "Domain", "Risk")))
	fmt.Fprintln(&b, ruleStyle.Render(strings.Repeat("─", 29)))
	for _, a := range analyzer.All() {
		risk := a.Risk()
		colored := lipgloss.NewStyle().Foreground(riskColor(risk)).Render(string(risk))
		fmt.Fprintf(&b, "%-15s %s\n", a.Domain(), colored)
	}
	return b.String(), nil
}

// monitorRunner adapts a monitor.Monitor to a text producer. We prefer
// Snapshot() over Display(): Display() writes ANSI to stdout/stderr, while
// Snapshot() returns structured data we can format cleanly for the viewport.
func monitorRunner(m monitor.Monitor) func() (string, error) {
	return func() (string, error) {
		data, err := m.Snapshot()
		if err != nil {
			return "", err
		}
		return formatSnapshot(m.Name(), data), nil
	}
}

// formatSnapshot renders a monitor snapshot as sorted "key: value" lines so
// the output is deterministic regardless of Go's map iteration order. The
// title and rule use the shared palette to match the analysis/status tables.
func formatSnapshot(name string, data map[string]any) string {
	var b strings.Builder
	fmt.Fprintln(&b, tableHeaderStyle.Render(name+" monitor"))
	fmt.Fprintln(&b, ruleStyle.Render(strings.Repeat("─", 40)))
	if len(data) == 0 {
		fmt.Fprintln(&b, "(no data available)")
		return b.String()
	}
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(&b, "  %s %v\n", hdrLabelStyle.Render(fmt.Sprintf("%-20s", k+":")), data[k])
	}
	return b.String()
}

// reportsDir is the directory reportCmd writes to; kept as a var so tests can
// point it at a temp dir.
var reportsDir = "reports"

// runLastReport reuses the same logic as `report --last`: read the newest
// report directory's markdown file.
func runLastReport() (string, error) {
	entries, err := os.ReadDir(reportsDir)
	if err != nil {
		return "", fmt.Errorf("no reports found: %w", err)
	}
	if len(entries) == 0 {
		return "", fmt.Errorf("no reports saved yet")
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() > entries[j].Name()
	})
	mdPath := filepath.Join(reportsDir, entries[0].Name(), "report.md")
	data, err := os.ReadFile(mdPath)
	if err != nil {
		return "", fmt.Errorf("cannot read report: %w", err)
	}
	return string(data), nil
}

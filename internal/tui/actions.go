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
}

// buildActions returns the ordered, grouped, read-only action set for Phase
// TUI-1 (F5). Destructive/dry-run actions (Preview, Clean, Full) are omitted.
func buildActions() []actionItem {
	return []actionItem{
		{label: "Analyze", group: "Disk", run: runAnalyze},
		{label: "Status", group: "Disk", run: runStatus},
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

// runAnalyze reuses core.RunAnalyzers over the registered analyzers and
// formats the results as a plain-text table for the viewport.
func runAnalyze() (string, error) {
	var tasks []core.RunnerTask
	for _, a := range analyzer.All() {
		tasks = append(tasks, core.RunnerTask{Domain: a.Domain(), AnalyzeFn: a.Analyze})
	}
	results := core.RunAnalyzers(tasks, core.AnalyzerTimeoutSeconds)
	return formatAnalysis(results), nil
}

// formatAnalysis renders analysis results largest-first without ANSI codes,
// keeping it independent from the stderr terminal reporter.
func formatAnalysis(results []core.AnalysisResult) string {
	sorted := make([]core.AnalysisResult, len(results))
	copy(sorted, results)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].TotalSize > sorted[j].TotalSize
	})

	var b strings.Builder
	var total int64          // sum of genuinely reclaimable junk (excludes disk + errors)
	var diskUsage int64      // the "disk" domain reports volume usage, not junk
	var haveDisk bool
	fmt.Fprintf(&b, "%-14s %-10s %-11s %s\n", "Domain", "Severity", "Size", "Items")
	fmt.Fprintln(&b, strings.Repeat("─", 48))
	for _, r := range sorted {
		if r.Error != "" {
			// Errored domains have no trustworthy size; never fold into the total.
			fmt.Fprintf(&b, "%-14s %-10s %-11s %s\n", r.Domain, "error", "-", r.Error)
			continue
		}
		// The "disk" domain is the volume's used space (~hundreds of GB), not
		// deletable garbage. Track it separately as context, not as reclaimable.
		if r.Domain == "disk" {
			diskUsage = r.TotalSize
			haveDisk = true
			fmt.Fprintf(&b, "%-14s %-10s %-11s %d\n",
				r.Domain, string(r.Severity), core.FormatBytes(r.TotalSize), len(r.Items))
			continue
		}
		total += r.TotalSize
		fmt.Fprintf(&b, "%-14s %-10s %-11s %d\n",
			r.Domain, string(r.Severity), core.FormatBytes(r.TotalSize), len(r.Items))
	}
	fmt.Fprintln(&b, strings.Repeat("─", 48))
	if haveDisk {
		fmt.Fprintf(&b, "Disk usage (context): %s\n", core.FormatBytes(diskUsage))
	}
	fmt.Fprintf(&b, "Total reclaimable (excl. disk): %s\n", core.FormatBytes(total))
	return b.String()
}

// runStatus reuses the analyzer registry (same source as statusCmd) to list
// domains and their risk levels.
func runStatus() (string, error) {
	var b strings.Builder
	fmt.Fprintf(&b, "%-15s %s\n", "Domain", "Risk")
	fmt.Fprintln(&b, strings.Repeat("─", 29))
	for _, a := range analyzer.All() {
		fmt.Fprintf(&b, "%-15s %s\n", a.Domain(), string(a.Risk()))
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
// the output is deterministic regardless of Go's map iteration order.
func formatSnapshot(name string, data map[string]any) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s monitor\n", name)
	fmt.Fprintln(&b, strings.Repeat("─", 40))
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
		fmt.Fprintf(&b, "  %-20s %v\n", k+":", data[k])
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

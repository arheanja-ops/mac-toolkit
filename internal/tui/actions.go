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
	run   func(width int) (string, error)
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
		{label: "Select & clean", group: "Clean", openClean: true},
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
// width is the detail pane's inner width so the table stays responsive.
func runAnalyze(width int) (string, error) {
	return formatAnalysis(analysisFn(), width), nil
}

// analysisLayout captures which columns fit at the current width. The table has
// a graceful-degradation order: the SHARE proportion bar is dropped first (it
// is decorative), then the DOMAIN column is narrowed and finally truncated with
// an ellipsis. SIZE always survives because it is the datum the user came for.
type analysisLayout struct {
	domainW int  // visible width of the DOMAIN column
	sevW    int  // visible width of the SEVERITY column
	showBar bool // whether the SHARE proportion bar is drawn
	barW    int  // proportion bar cells when shown
	width   int  // total usable width (never exceeded by any row)
}

// planAnalysisLayout decides the column widths for the usable width w. Fixed
// costs: SIZE is 11 right-aligned, ITEMS is 6, plus single-space gaps. We fit
// DOMAIN/SEVERITY into whatever remains and only keep the SHARE bar when there
// is comfortable room (>=58 cells), matching the "drop SHARE first" rule.
func planAnalysisLayout(w int) analysisLayout {
	const sizeW, itemsW = 11, 6
	l := analysisLayout{sevW: 9, barW: 12, width: w}
	// Columns + gaps without the bar: domain + 1 + sev + 1 + size + 1 + items.
	fixed := l.sevW + sizeW + itemsW + 3
	l.showBar = w >= 58
	if l.showBar {
		fixed += 2 + l.barW // "  " gap before the bar + the bar itself
	}
	l.domainW = w - fixed
	if l.domainW < 6 {
		// Too tight for the full severity word; shrink it before the domain so
		// the domain name (what the user reads) keeps as many cells as possible.
		l.sevW = 4
		fixed = l.sevW + sizeW + itemsW + 3
		l.showBar = false
		l.domainW = w - fixed
	}
	if l.domainW < 4 {
		l.domainW = 4
	}
	return l
}

// formatAnalysis renders analysis results largest-first as an aligned, colored
// table whose columns adapt to width (the detail pane's inner width). No
// rendered row ever exceeds width: the SHARE bar is dropped and the DOMAIN name
// truncated with an ellipsis before anything wraps. Colors come from the shared
// palette keyed on severity, keeping parity with the terminal reporter.
func formatAnalysis(results []core.AnalysisResult, width int) string {
	w := usableWidth(width)
	lay := planAnalysisLayout(w)

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

	fmt.Fprintln(&b, tableHeaderStyle.Render(analysisHeader(lay)))
	fmt.Fprintln(&b, rule(w))

	for _, r := range sorted {
		if r.Error != "" {
			// Errored domains have no trustworthy size; never fold into the total.
			b.WriteString(analysisErrorRow(r, lay))
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
		b.WriteString(analysisRow(r, maxSize, lay))
	}

	fmt.Fprintln(&b, rule(w))
	if haveDisk {
		fmt.Fprintln(&b, truncateToWidth(
			fmt.Sprintf("Disk usage (context): %s", core.FormatBytes(diskUsage)), w))
	}
	// Summary lines are left unstyled so they remain machine-greppable (the CLI
	// and tests match these exact strings); the table above carries the color.
	fmt.Fprintln(&b, truncateToWidth(
		fmt.Sprintf("Total reclaimable (excl. disk): %s", core.FormatBytes(total)), w))
	return b.String()
}

// analysisHeader builds the column header matching the active layout so the
// titles line up with the data rows regardless of which columns were dropped.
func analysisHeader(l analysisLayout) string {
	h := fmt.Sprintf("%-*s %-*s %11s %6s", l.domainW, "DOMAIN", l.sevW, "SEVERITY", "SIZE", "ITEMS")
	if l.showBar {
		h += "  SHARE"
	}
	return h
}

// analysisRow renders one data row with a severity-colored domain name and,
// when the layout allows, a proportion bar scaled to maxSize. The domain is
// truncated to the column width first, then padded, so color never leaks past
// the cell and the row width is deterministic.
func analysisRow(r core.AnalysisResult, maxSize int64, l analysisLayout) string {
	color := severityColor(r.Severity)
	name := truncateToWidth(r.Domain, l.domainW)
	domain := lipgloss.NewStyle().Foreground(color).Render(padCell(name, l.domainW))
	sev := lipgloss.NewStyle().Foreground(color).
		Render(padCell(truncateToWidth(string(r.Severity), l.sevW), l.sevW))
	row := fmt.Sprintf("%s %s %11s %6d", domain, sev, core.FormatBytes(r.TotalSize), len(r.Items))
	if l.showBar {
		row += "  " + proportionBar(r.TotalSize, maxSize, l.barW)
	}
	return row + "\n"
}

// analysisErrorRow renders a domain whose analysis failed. It carries no size,
// so it shows a dash and the error message (truncated to the remaining width).
func analysisErrorRow(r core.AnalysisResult, l analysisLayout) string {
	name := truncateToWidth(r.Domain, l.domainW)
	domain := lipgloss.NewStyle().Foreground(riskDanger).Render(padCell(name, l.domainW))
	sev := lipgloss.NewStyle().Foreground(riskDanger).
		Render(padCell(truncateToWidth("error", l.sevW), l.sevW))
	// Fit the message into whatever is left after domain+sev+size+items+gaps.
	used := l.domainW + l.sevW + 11 + 6 + 3
	msg := truncateToWidth(r.Error, l.width-used-2)
	return fmt.Sprintf("%s %s %11s %6s  %s\n", domain, sev, "-", "-", msg)
}

// padCell right-pads a visible string to exactly w cells using lipgloss.Width
// so wide runes do not overshoot the column (fmt's %-*s counts bytes, not cells).
func padCell(s string, w int) string {
	gap := w - lipgloss.Width(s)
	if gap <= 0 {
		return s
	}
	return s + strings.Repeat(" ", gap)
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
// information as the CLI's PrintCleanPlan, adapted to the detail pane. width is
// the detail pane's inner width so every line stays inside it.
func runPreview(width int) (string, error) {
	w := usableWidth(width)
	results := analysisFn()
	var b strings.Builder
	b.WriteString(formatAnalysis(results, width))
	b.WriteString("\n")
	b.WriteString(tableHeaderStyle.Render(
		truncateToWidth("Cleanup plan (dry-run) — nada se borra", w)) + "\n")
	b.WriteString(rule(w) + "\n")

	rows := buildCleanRows(results)
	if len(rows) == 0 {
		b.WriteString("  (no reclaimable items)\n")
		return b.String(), nil
	}
	safeBytes, safeCount := markedTotals(rows)
	for _, r := range rows {
		b.WriteString(previewRow(r, w) + "\n")
	}
	b.WriteString(rule(w) + "\n")
	fmt.Fprintf(&b, "Safe to auto-delete: %s in %d items\n",
		core.FormatBytes(safeBytes), safeCount)
	return b.String(), nil
}

// previewRow renders one dry-run plan line, truncating the domain·label tail to
// whatever remains after the fixed mark/risk/size columns so the row never
// exceeds the pane width.
func previewRow(r cleanRow, w int) string {
	mark := cleanDimRow.Render("·")
	if r.markable {
		mark = lipgloss.NewStyle().Foreground(riskSafe).Render("✓")
	}
	risk := lipgloss.NewStyle().Foreground(riskColor(r.item.Risk)).
		Render(padCell(truncateToWidth(string(r.item.Risk), 6), 6))
	// Fixed prefix: "  " + mark(1) + " " + risk(6) + " " + size(10) + "  ".
	descW := w - (2 + 1 + 1 + 6 + 1 + 10 + 2)
	if descW < 4 {
		descW = 4
	}
	desc := truncateToWidth(fmt.Sprintf("%s · %s", r.item.Domain, r.item.Label), descW)
	return fmt.Sprintf("  %s %s %10s  %s", mark, risk, core.FormatBytes(r.item.SizeBytes), desc)
}

// runStatus reuses the analyzer registry (same source as statusCmd) to list
// domains and their risk levels, each risk colored via the shared palette.
// width keeps the rule and rows inside the detail pane.
func runStatus(width int) (string, error) {
	w := usableWidth(width)
	var b strings.Builder
	fmt.Fprintln(&b, tableHeaderStyle.Render(fmt.Sprintf("%-15s %s", "Domain", "Risk")))
	fmt.Fprintln(&b, rule(min(29, w)))
	for _, a := range analyzer.All() {
		risk := a.Risk()
		colored := lipgloss.NewStyle().Foreground(riskColor(risk)).Render(string(risk))
		b.WriteString(truncateToWidth(fmt.Sprintf("%-15s %s", a.Domain(), colored), w) + "\n")
	}
	return b.String(), nil
}

// monitorRunner adapts a monitor.Monitor to a text producer. We prefer
// Snapshot() over Display(): Display() writes ANSI to stdout/stderr, while
// Snapshot() returns structured data we can format cleanly for the viewport.
func monitorRunner(m monitor.Monitor) func(width int) (string, error) {
	return func(width int) (string, error) {
		data, err := m.Snapshot()
		if err != nil {
			return "", err
		}
		return formatSnapshot(m.Name(), data, width), nil
	}
}

// formatSnapshot renders a monitor snapshot as sorted "key: value" lines so
// the output is deterministic regardless of Go's map iteration order. The
// title and rule use the shared palette to match the analysis/status tables.
// width bounds every line to the detail pane's inner width.
func formatSnapshot(name string, data map[string]any, width int) string {
	w := usableWidth(width)
	var b strings.Builder
	fmt.Fprintln(&b, tableHeaderStyle.Render(truncateToWidth(name+" monitor", w)))
	fmt.Fprintln(&b, rule(min(40, w)))
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
		label := hdrLabelStyle.Render(fmt.Sprintf("%-20s", k+":"))
		line := fmt.Sprintf("  %s %v", label, data[k])
		b.WriteString(truncateToWidth(line, w) + "\n")
	}
	return b.String()
}

// reportsDir is the directory reportCmd writes to; kept as a var so tests can
// point it at a temp dir.
var reportsDir = "reports"

// runLastReport reuses the same logic as `report --last`: read the newest
// report directory's markdown file. width is accepted for signature parity with
// the other actions; the markdown is shown verbatim and scrolls in the viewport.
func runLastReport(_ int) (string, error) {
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

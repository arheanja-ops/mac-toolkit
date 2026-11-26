package tui

import (
	"fmt"
	"strings"

	"github.com/arheanja-ops/mac-toolkit/internal/monitor"

	"github.com/charmbracelet/lipgloss"
)

// viewID identifies the dedicated panel currently shown in the detail pane. 'r'
// refreshes whichever view is open and nothing else, so the key never re-runs
// an unrelated action.
type viewID int

const (
	viewNone viewID = iota
	viewDoctor
	viewMemory // Overview
	viewMemorySwap
	viewMemoryApps
	viewCPU // Overview
	viewCPULoad
	viewCPUApps
	viewCPUThermal
	viewProcCPU
	viewProcMem
	viewProcApps
	viewProcSummary
	viewBattery
	viewSystem
	viewNetwork
	viewLastReport
	viewSavedReports
	viewSaveReport
	viewOpenReport
)

// viewRunners maps each dedicated view to its read-only producer. A package
// var (not a switch) keeps one line per view and lets tests assert that every
// view has a runner.
var viewRunners = map[viewID]func(width int) (string, error){
	viewDoctor:       runDoctor,
	viewMemory:       runMemory,
	viewMemorySwap:   runMemorySwap,
	viewMemoryApps:   runMemoryApps,
	viewCPU:          runCPU,
	viewCPULoad:      runCPULoad,
	viewCPUApps:      runCPUApps,
	viewCPUThermal:   runCPUThermal,
	viewProcCPU:      runProcTopCPU,
	viewProcMem:      runProcTopMem,
	viewProcApps:     runProcApps,
	viewProcSummary:  runProcSummary,
	viewBattery:      monitorRunner(&monitor.BatteryMonitor{}),
	viewSystem:       monitorRunner(&monitor.SystemMonitor{}),
	viewNetwork:      monitorRunner(&monitor.NetworkMonitor{}),
	viewLastReport:   runLastReport,
	viewSavedReports: runSavedReports,
	viewSaveReport:   runSaveReport,
	viewOpenReport:   runOpenReport,
}

// viewTitles label the detail pane per view, so the border reads as a tab.
var viewTitles = map[viewID]string{
	viewDoctor: "HEALTH · DOCTOR", viewMemory: "MEMORY · OVERVIEW", viewMemorySwap: "MEMORY · SWAP & PAGING",
	viewMemoryApps: "MEMORY · TOP APPS", viewCPU: "CPU · OVERVIEW", viewCPULoad: "CPU · LOAD",
	viewCPUApps: "CPU · TOP APPS", viewCPUThermal: "CPU · THERMAL", viewProcCPU: "PROCESSES · TOP BY CPU",
	viewProcMem: "PROCESSES · TOP BY MEMORY", viewProcApps: "PROCESSES · BY APP",
	viewProcSummary: "PROCESSES · SUMMARY", viewBattery: "SYSTEM · BATTERY", viewSystem: "SYSTEM · INFO",
	viewNetwork: "SYSTEM · NETWORK", viewLastReport: "REPORTS · LAST", viewSavedReports: "REPORTS · SAVED",
	viewSaveReport: "REPORTS · SAVE", viewOpenReport: "REPORTS · OPEN",
}

// runner returns the read-only producer behind a view (nil for viewNone).
func (v viewID) runner() func(width int) (string, error) { return viewRunners[v] }

// Bar sizing and the usage thresholds that pick a bar color.
const (
	warnPct = 70.0
	critPct = 85.0
)

// levelColor colors a usage percentage like the CLI bars: green below 70,
// yellow below 85, red from there.
func levelColor(pct float64) lipgloss.Color {
	switch {
	case pct >= critPct:
		return riskDanger
	case pct >= warnPct:
		return riskWarn
	default:
		return riskSafe
	}
}

// usageBar draws a width-cell bar with lipgloss only (never core.ProgressBar,
// whose raw ANSI cannot be trusted with lipgloss.Width). Every cell is one
// column, so the bar is exactly width cells wide.
func usageBar(pct float64, width int, color lipgloss.Color) string {
	if width < 1 {
		return ""
	}
	filled := int(pct*float64(width)/100 + 0.5)
	filled = max(0, min(filled, width))
	return lipgloss.NewStyle().Foreground(color).Render(strings.Repeat("▰", filled)) +
		ruleStyle.Render(strings.Repeat("▱", width-filled))
}

// kvLine renders "label  value" with a muted fixed-width label. The value may
// carry its own styling; the line is cut to w as a last resort.
func kvLine(label, value string, w int) string {
	return truncateToWidth(hdrLabelStyle.Render(padCell(label, viewLabelW))+" "+value, w)
}

// viewLabelW is the label column of the dedicated views.
const viewLabelW = 11

// numF reads a float64 from a monitor snapshot, 0 when absent or mistyped.
func numF(d map[string]any, k string) float64 {
	v, _ := d[k].(float64)
	return v
}

// numI reads an int from a monitor snapshot.
func numI(d map[string]any, k string) (int, bool) {
	v, ok := d[k].(int)
	return v, ok
}

// topAppLines renders the top-N application list: rank, name, magnitude and
// process count. Names are truncated before padding so no row can overflow w.
func topAppLines(title string, apps []monitor.AppUsage, byMem bool, w int) []string {
	out := []string{tableHeaderStyle.Render(truncateToWidth(title, w))}
	if len(apps) == 0 {
		return append(out, hdrLabelStyle.Render("  (sin datos)"))
	}
	const fixed = 3 + 1 + 9 + 1 + 5 // "1. " + gap + value + gap + "×NNN"
	nameW := max(w-fixed, 6)
	for i, a := range apps {
		val := fmt.Sprintf("%6.0f MB", a.MemMB)
		if !byMem {
			val = fmt.Sprintf("%8.1f%%", a.CPUPct)
		}
		name := padCell(truncateToWidth(a.Name, nameW), nameW)
		out = append(out, truncateToWidth(
			fmt.Sprintf("%d. %s %s %s", i+1, name, val,
				hdrLabelStyle.Render(fmt.Sprintf("×%d", a.Procs))), w))
	}
	return out
}

// noteLines wraps a cautious explanation in the warning color.
func noteLines(text string, w int) []string {
	style := lipgloss.NewStyle().Foreground(riskWarn)
	return wrapWords(words(text, &style), w, "", "")
}

package tui

import (
	"fmt"
	"strings"

	"github.com/arheanja-ops/mac-toolkit/internal/monitor"

	"github.com/charmbracelet/lipgloss"
)

// Labels of the dedicated, refreshable views in the left panel.
const (
	memoryLabel = "Memory"
	cpuLabel    = "CPU"
)

// viewID identifies the dedicated panel currently shown in the detail pane. It
// generalizes the old Doctor-only flag: 'r' refreshes whichever view is open
// and nothing else, so the key never re-runs an unrelated action.
type viewID int

const (
	viewNone viewID = iota
	viewDoctor
	viewMemory
	viewCPU
)

// viewForLabel maps a left-panel label to its dedicated view (viewNone for
// ordinary actions).
func viewForLabel(label string) viewID {
	switch label {
	case doctorLabel:
		return viewDoctor
	case memoryLabel:
		return viewMemory
	case cpuLabel:
		return viewCPU
	default:
		return viewNone
	}
}

// runner returns the read-only producer behind a view.
func (v viewID) runner() func(width int) (string, error) {
	switch v {
	case viewDoctor:
		return runDoctor
	case viewMemory:
		return runMemory
	case viewCPU:
		return runCPU
	default:
		return nil
	}
}

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

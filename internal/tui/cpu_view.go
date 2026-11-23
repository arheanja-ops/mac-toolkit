package tui

import (
	"fmt"
	"strings"

	"github.com/arheanja-ops/mac-toolkit/internal/monitor"

	"github.com/charmbracelet/lipgloss"
)

// cpuFn is the source of the CPU view; a package var so tests inject a fixed
// snapshot. The real monitor takes ~2 s (second `top` sample), which is why the
// view always runs inside a tea.Cmd with the spinner.
var cpuFn = func() (map[string]any, error) {
	return (&monitor.CPUMonitor{}).Snapshot()
}

// runCPU is the read-only CPU action.
func runCPU(width int) (string, error) {
	data, err := cpuFn()
	if err != nil {
		return "", err
	}
	return formatCPU(data, width), nil
}

// verdictColor maps a load verdict onto the palette.
func verdictColor(v string) lipgloss.Color {
	switch v {
	case monitor.LoadSaturated:
		return riskDanger
	case monitor.LoadHigh:
		return riskWarn
	default:
		return riskSafe
	}
}

// formatCPU lays out the CPU view for the detail pane. Pure and width-bounded.
func formatCPU(d map[string]any, width int) string {
	w := usableWidth(width)
	if len(d) == 0 {
		return "(no data available)\n"
	}
	var lines []string
	lines = append(lines, cpuSplitLines(d, w)...)
	lines = append(lines, "")
	lines = append(lines, cpuLoadLines(d, w)...)
	lines = append(lines, "")
	apps, _ := d["top_cpu"].([]monitor.AppUsage)
	lines = append(lines, topAppLines("Top 5 apps por CPU", apps, false, w)...)
	return strings.Join(lines, "\n") + "\n"
}

// cpuSplitLines draws the user/sys/idle bars. Idle uses the accent color: a
// full idle bar is good news, so a level color would mislead.
func cpuSplitLines(d map[string]any, w int) []string {
	if _, ok := d["user_pct"].(float64); !ok {
		return []string{kvLine("CPU", hdrLabelStyle.Render("sin lectura de uso"), w)}
	}
	rows := []struct {
		label string
		key   string
		color func(float64) lipgloss.Color
	}{
		{"user", "user_pct", levelColor},
		{"sys", "sys_pct", levelColor},
		{"idle", "idle_pct", func(float64) lipgloss.Color { return accent }},
	}
	var out []string
	for _, r := range rows {
		pct := numF(d, r.key)
		tail := fmt.Sprintf("%5.1f%%", pct)
		barW := max(min(w-viewLabelW-1-lipgloss.Width(tail)-1, 30), 6)
		out = append(out, kvLine(r.label, usageBar(pct, barW, r.color(pct))+" "+tail, w))
	}
	return out
}

// cpuLoadLines shows load averages, load per core with its verdict, core
// layout, thermal state and, when present, the cautious kernel-time note.
func cpuLoadLines(d map[string]any, w int) []string {
	var out []string
	if _, ok := d["load1"].(float64); ok {
		out = append(out, kvLine("Carga", fmt.Sprintf("%.2f · %.2f · %.2f %s",
			numF(d, "load1"), numF(d, "load5"), numF(d, "load15"),
			hdrLabelStyle.Render("(1/5/15 min)")), w))
	}
	if v, ok := d["load_verdict"].(string); ok {
		styled := lipgloss.NewStyle().Foreground(verdictColor(v)).Bold(true).Render(v)
		out = append(out, kvLine("Por núcleo",
			fmt.Sprintf("%.2f (5 min) → %s", numF(d, "load5_per_core"), styled), w))
	}
	if total, ok := numI(d, "cores_total"); ok {
		val := fmt.Sprintf("%d", total)
		p, hasP := numI(d, "perf_cores")
		e, hasE := numI(d, "eff_cores")
		if hasP && hasE {
			val += fmt.Sprintf(" (%d P + %d E)", p, e)
		}
		out = append(out, kvLine("Núcleos", val, w))
	}
	if t, ok := d["thermal"].(string); ok {
		style := lipgloss.NewStyle().Foreground(textMuted)
		if monitor.IsThermalAlert(t) {
			style = thermalAlertStyle
		}
		out = append(out, kvLine("Térmico", style.Render(t), w))
	}
	if hi, _ := d["kernel_high"].(bool); hi {
		out = append(out, noteLines(monitor.NoteKernelTime, w)...)
	}
	return out
}

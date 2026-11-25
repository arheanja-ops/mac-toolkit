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
	case monitor.LoadHigh, monitor.LoadWaiting:
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
	lines = append(lines, topAppLines("Top 5 apps por CPU", firstN(apps, 5), false, w)...)
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
		styled := lipgloss.NewStyle().Foreground(verdictColor(v)).Bold(true).Render(verdictShort(v))
		out = append(out, kvLine("Por núcleo",
			fmt.Sprintf("%.2f (5 min) → %s", numF(d, "load5_per_core"), styled), w))
		if v == monitor.LoadWaiting {
			out = append(out, noteLines(v+".", w)...)
		}
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

// verdictShort is the one-line form of a verdict; the long "waiting" verdict is
// printed in full as a wrapped note right below.
func verdictShort(v string) string {
	if v == monitor.LoadWaiting {
		return "Carga alta, CPU libre"
	}
	return v
}

// runCPULoad, runCPUApps and runCPUThermal are the CPU category's other views.
func runCPULoad(width int) (string, error) {
	data, err := cpuFn()
	if err != nil {
		return "", err
	}
	return formatCPULoad(data, width), nil
}

func runCPUApps(width int) (string, error) {
	data, err := cpuFn()
	if err != nil {
		return "", err
	}
	return formatCPUApps(data, width), nil
}

func runCPUThermal(width int) (string, error) {
	data, err := cpuFn()
	if err != nil {
		return "", err
	}
	return formatCPUThermal(data, width), nil
}

// formatCPUApps lists the top 10 apps by CPU, aggregated per application.
func formatCPUApps(d map[string]any, width int) string {
	apps, _ := d["top_cpu"].([]monitor.AppUsage)
	return strings.Join(topAppLines("Top 10 apps por CPU", firstN(apps, 10), false, usableWidth(width)), "\n") + "\n"
}

// formatCPULoad shows the three load averages, load per core, the P/E layout,
// the current usage split and the verdict (which weighs the idle share, see
// monitor.ClassifyLoad).
func formatCPULoad(d map[string]any, width int) string {
	w := usableWidth(width)
	if _, ok := d["load1"].(float64); !ok {
		return strings.Join([]string{kvLine("Carga", hdrLabelStyle.Render("sin lectura de carga"), w)}, "\n") + "\n"
	}
	lines := []string{
		kvLine("Carga", fmt.Sprintf("%.2f · %.2f · %.2f %s", numF(d, "load1"), numF(d, "load5"),
			numF(d, "load15"), hdrLabelStyle.Render("(1/5/15 min)")), w),
	}
	if _, ok := d["load_per_core"].(float64); ok {
		lines = append(lines, kvLine("Por núcleo", fmt.Sprintf("%.2f · %.2f · %.2f",
			numF(d, "load_per_core"), numF(d, "load5_per_core"), numF(d, "load15_per_core")), w))
	}
	if total, ok := numI(d, "cores_total"); ok {
		val := fmt.Sprintf("%d", total)
		if p, hasP := numI(d, "perf_cores"); hasP {
			if e, hasE := numI(d, "eff_cores"); hasE {
				val += fmt.Sprintf(" (%d P + %d E)", p, e)
			}
		}
		lines = append(lines, kvLine("Núcleos", val, w))
	}
	if _, ok := d["idle_pct"].(float64); ok {
		lines = append(lines, kvLine("Uso ahora", fmt.Sprintf("user %.0f%% · sys %.0f%% · idle %.0f%%",
			numF(d, "user_pct"), numF(d, "sys_pct"), numF(d, "idle_pct")), w))
	}
	if v, ok := d["load_verdict"].(string); ok {
		lines = append(lines, "", kvLine("Veredicto", lipgloss.NewStyle().
			Foreground(verdictColor(v)).Bold(true).Render(verdictShort(v)), w))
		if v == monitor.LoadWaiting {
			lines = append(lines, noteLines(v+". La carga también cuenta tareas bloqueadas; "+
				"con la CPU libre no implica saturación.", w)...)
		}
	}
	return strings.Join(lines, "\n") + "\n"
}

// formatCPUThermal shows the thermal state and, when macOS reports it, the CPU
// speed limit. Unknown stays muted: not knowing must not look like an alarm.
func formatCPUThermal(d map[string]any, width int) string {
	w := usableWidth(width)
	state, _ := d["thermal"].(string)
	if state == "" {
		state = monitor.ThermalUnknown
	}
	style := lipgloss.NewStyle().Foreground(textMuted)
	if monitor.IsThermalAlert(state) {
		style = thermalAlertStyle
	} else if state == monitor.ThermalNormal {
		style = lipgloss.NewStyle().Foreground(riskSafe)
	}
	lines := []string{kvLine("Estado", style.Render(state), w)}
	if lim, ok := numI(d, "cpu_speed_limit"); ok {
		pct := float64(lim)
		lines = append(lines, kvLine("Límite CPU", usageBar(pct, 12, riskSafe)+fmt.Sprintf(" %d%%", lim), w))
	} else {
		lines = append(lines, kvLine("Límite CPU", hdrLabelStyle.Render("no informado"), w))
	}
	lines = append(lines, "")
	lines = append(lines, noteLines(thermalNote(state), w)...)
	return strings.Join(lines, "\n") + "\n"
}

// thermalNote explains a state in cautious words.
func thermalNote(state string) string {
	switch state {
	case monitor.ThermalThrottled:
		return "macOS está reduciendo la velocidad de la CPU (límite por debajo del 100%). " +
			"Puede deberse a calor o a la carga sostenida."
	case monitor.ThermalWarning:
		return "macOS registró un nivel de advertencia térmica. Conviene revisar qué apps consumen más CPU."
	case monitor.ThermalNormal:
		return "Sin limitación térmica registrada."
	default:
		return "macOS no informó el estado térmico; sin datos no se asume ningún problema."
	}
}

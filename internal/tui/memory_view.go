package tui

import (
	"fmt"
	"strings"

	"github.com/arheanja-ops/mac-toolkit/internal/monitor"

	"github.com/charmbracelet/lipgloss"
)

// memoryFn is the source of the Memory view; a package var so tests inject a
// fixed snapshot and never run vm_stat/ps.
var memoryFn = func() (map[string]any, error) {
	return (&monitor.MemoryMonitor{}).Snapshot()
}

// runMemory is the read-only Memory action. It runs in a tea.Cmd goroutine.
func runMemory(width int) (string, error) {
	data, err := memoryFn()
	if err != nil {
		return "", err
	}
	return formatMemory(data, width), nil
}

// pressureColor maps a pressure label onto the palette; Unknown stays muted so
// a failed read never looks like an alarm.
func pressureColor(level string) lipgloss.Color {
	switch level {
	case monitor.PressureNormal:
		return riskSafe
	case monitor.PressureWarning:
		return riskWarn
	case monitor.PressureCritical:
		return riskDanger
	default:
		return textMuted
	}
}

// formatMemory lays out the Memory view for the detail pane. It is pure and
// bounded: every line is at most usableWidth(width) cells, and apps appear by
// name only (the snapshot carries no PIDs or paths).
func formatMemory(d map[string]any, width int) string {
	w := usableWidth(width)
	if len(d) == 0 {
		return "(no data available)\n"
	}
	var lines []string
	lines = append(lines, memoryBarLine("RAM", d, "used_pct", "used_gb", "ram_total_gb", w))
	lines = append(lines, pressureLine(d, w))
	lines = append(lines, "")
	lines = append(lines, memoryColumns(d, w)...)
	lines = append(lines, "")
	lines = append(lines, swapLines(d, w)...)
	lines = append(lines, "")
	apps, _ := d["top_memory"].([]monitor.AppUsage)
	lines = append(lines, topAppLines("Top 5 apps por RAM", apps, true, w)...)
	return strings.Join(lines, "\n") + "\n"
}

// memoryBarLine renders "LABEL ▰▰▱▱ 72%  used / total GB".
func memoryBarLine(label string, d map[string]any, pctKey, usedKey, totalKey string, w int) string {
	pct := numF(d, pctKey)
	tail := fmt.Sprintf("%3.0f%%  %.1f / %.1f GB", pct, numF(d, usedKey), numF(d, totalKey))
	barW := max(min(w-viewLabelW-1-lipgloss.Width(tail)-1, 30), 6)
	pctStyled := lipgloss.NewStyle().Foreground(levelColor(pct)).Render(tail)
	return kvLine(label, usageBar(pct, barW, levelColor(pct))+" "+pctStyled, w)
}

func pressureLine(d map[string]any, w int) string {
	level, _ := d["pressure_level"].(string)
	if level == "" {
		level = monitor.PressureUnknown
	}
	val := lipgloss.NewStyle().Foreground(pressureColor(level)).Bold(true).Render("● " + level)
	if f, ok := numI(d, "free_pct"); ok {
		val += hdrLabelStyle.Render(fmt.Sprintf("  (%d%% libre)", f))
	}
	return kvLine("Presión", val, w)
}

// memoryColumns prints the breakdown as aligned label/value cells, two per row
// when the pane is wide enough and one otherwise.
func memoryColumns(d map[string]any, w int) []string {
	type cell struct{ label, key string }
	cells := []cell{{"Apps", "app_gb"}, {"Wired", "wired_gb"}, {"Comprimida", "compressed_gb"},
		{"Caché arch.", "file_cache_gb"}, {"Activa", "active_gb"}, {"Inactiva", "inactive_gb"},
		{"Libre", "free_gb"}}
	const cellW = 11 + 1 + 7 + 3 // label + gap + "%7.1f" + " GB"
	perRow := 1
	if w >= 2*cellW+2 {
		perRow = 2
	}
	var rows []string
	var cur []string
	for _, c := range cells {
		if _, ok := d[c.key]; !ok {
			continue // e.g. file cache is not reported on every macOS version
		}
		cur = append(cur, fmt.Sprintf("%s %7.1f GB",
			hdrLabelStyle.Render(padCell(c.label, 11)), numF(d, c.key)))
		if len(cur) == perRow {
			rows = append(rows, truncateToWidth(strings.Join(cur, "  "), w))
			cur = nil
		}
	}
	if len(cur) > 0 {
		rows = append(rows, truncateToWidth(strings.Join(cur, "  "), w))
	}
	return rows
}

// swapLines shows swap occupancy, cumulative traffic since boot, uptime and,
// when present, the cautious paging note.
func swapLines(d map[string]any, w int) []string {
	var out []string
	if total := numF(d, "swap_total_mb"); total > 0 {
		used := numF(d, "swap_used_mb")
		pct := used * 100 / total
		tail := fmt.Sprintf("%3.0f%%  %.1f / %.1f GB", pct, used/1024, total/1024)
		barW := max(min(w-viewLabelW-1-lipgloss.Width(tail)-1, 30), 6)
		out = append(out, kvLine("Swap", usageBar(pct, barW, levelColor(pct))+" "+
			lipgloss.NewStyle().Foreground(levelColor(pct)).Render(tail), w))
	} else {
		out = append(out, kvLine("Swap", hdrLabelStyle.Render("sin swap en uso / no disponible"), w))
	}
	traffic := fmt.Sprintf("in %.1f GB · out %.1f GB",
		numF(d, "swapins_gb"), numF(d, "swapouts_gb"))
	if h, ok := d["uptime_hours"].(float64); ok {
		traffic += " · up " + formatUptime(h)
	}
	// Swap traffic is cumulative since boot, so the row says so in its label.
	out = append(out, kvLine("Desde boot", hdrLabelStyle.Render(traffic), w))
	if th, _ := d["swap_thrash"].(bool); th {
		out = append(out, noteLines(monitor.NoteHeavyPaging, w)...)
	}
	return out
}

// formatUptime renders hours as "5 h" or "2 d 4 h".
func formatUptime(h float64) string {
	if h < 48 {
		return fmt.Sprintf("%.0f h", h)
	}
	days := int(h) / 24
	return fmt.Sprintf("%d d %d h", days, int(h)-days*24)
}

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
	lines = append(lines, topAppLines("Top 5 apps por RAM", firstN(apps, 5), true, w)...)
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

// runMemorySwap is the "Swap & paging" view.
func runMemorySwap(width int) (string, error) {
	data, err := memoryFn()
	if err != nil {
		return "", err
	}
	return formatMemorySwap(data, width), nil
}

// runMemoryApps is the "Top apps" view of the Memory category.
func runMemoryApps(width int) (string, error) {
	data, err := memoryFn()
	if err != nil {
		return "", err
	}
	return formatMemoryApps(data, width), nil
}

// formatMemoryApps lists the top 10 apps by RAM, aggregated per application.
func formatMemoryApps(d map[string]any, width int) string {
	w := usableWidth(width)
	apps, _ := d["top_memory"].([]monitor.AppUsage)
	return strings.Join(topAppLines("Top 10 apps por RAM", firstN(apps, 10), true, w), "\n") + "\n"
}

// formatMemorySwap shows swap occupancy, cumulative and average paging
// traffic, the compressor and the (deliberately cautious) paging verdict.
// Counters are totals since boot; the per-hour figures are averages over the
// uptime, because the toolkit keeps no history to compute a live rate.
func formatMemorySwap(d map[string]any, width int) string {
	w := usableWidth(width)
	if len(d) == 0 {
		return "(no data available)\n"
	}
	lines := swapBarLine(d, w)
	lines = append(lines, trafficLine("Swap", d, "swapins_gb", "swapouts_gb", w)...)
	lines = append(lines, trafficLine("Paginación", d, "pageins_gb", "pageouts_gb", w)...)
	lines = append(lines, "")
	lines = append(lines, compressorLines(d, w)...)
	lines = append(lines, "")
	verdict, _ := d["paging_verdict"].(string)
	if verdict == "" {
		verdict = monitor.PagingOK
	}
	color := riskSafe
	if th, _ := d["swap_thrash"].(bool); th {
		color = riskWarn
	}
	lines = append(lines, kvLine("Veredicto", "", w))
	lines = append(lines, wrapWords(words(verdict, ptrStyle(lipgloss.NewStyle().Foreground(color))), w, "", "")...)
	if h, ok := d["uptime_hours"].(float64); ok {
		lines = append(lines, "", kvLine("Uptime", formatUptime(h), w))
	}
	return strings.Join(lines, "\n") + "\n"
}

func ptrStyle(s lipgloss.Style) *lipgloss.Style { return &s }

// swapBarLine is the swap occupancy bar (or a muted "no swap" line).
func swapBarLine(d map[string]any, w int) []string {
	total := numF(d, "swap_total_mb")
	if total <= 0 {
		return []string{kvLine("Swap", hdrLabelStyle.Render("sin swap en uso / no disponible"), w)}
	}
	used := numF(d, "swap_used_mb")
	pct := used * 100 / total
	tail := fmt.Sprintf("%3.0f%%  %.1f / %.1f GB", pct, used/1024, total/1024)
	barW := max(min(w-viewLabelW-1-lipgloss.Width(tail)-1, 30), 6)
	return []string{kvLine("Uso", usageBar(pct, barW, levelColor(pct))+" "+
		lipgloss.NewStyle().Foreground(levelColor(pct)).Render(tail), w)}
}

// trafficLine renders "in X GB · out Y GB" plus, when the uptime was known, a
// second row with the average hourly rate.
func trafficLine(label string, d map[string]any, inKey, outKey string, w int) []string {
	if _, ok := d[inKey]; !ok {
		return nil
	}
	out := []string{kvLine(label, fmt.Sprintf("in %.1f GB · out %.1f GB",
		numF(d, inKey), numF(d, outKey)), w)}
	if _, ok := d[inKey+"_per_h"]; ok {
		out = append(out, kvLine("", hdrLabelStyle.Render(fmt.Sprintf("media %.2f / %.2f GB por hora",
			numF(d, inKey+"_per_h"), numF(d, outKey+"_per_h"))), w))
	}
	return out
}

// compressorLines shows the compressor footprint and how well it compresses.
func compressorLines(d map[string]any, w int) []string {
	if _, ok := d["compressed_gb"]; !ok {
		return nil
	}
	val := fmt.Sprintf("%.1f GB ocupados", numF(d, "compressed_gb"))
	if r, ok := d["compression_ratio"].(float64); ok {
		val += fmt.Sprintf(" · ratio %.1fx", r)
	}
	return []string{kvLine("Compresor", val, w)}
}

// firstN returns at most n apps.
func firstN(apps []monitor.AppUsage, n int) []monitor.AppUsage {
	if len(apps) > n {
		return apps[:n]
	}
	return apps
}

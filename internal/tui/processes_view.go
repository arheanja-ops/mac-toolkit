package tui

import (
	"fmt"
	"strings"

	"github.com/arheanja-ops/mac-toolkit/internal/monitor"

	"github.com/charmbracelet/lipgloss"
)

// procFn is the source of the Processes views; a package var so tests inject a
// fixed snapshot and never run ps.
var procFn = func() (map[string]any, error) {
	return (&monitor.ProcessMonitor{}).Snapshot()
}

// procRun adapts a pure formatter over the process snapshot to a view runner.
func procRun(format func(map[string]any, int) string) func(int) (string, error) {
	return func(width int) (string, error) {
		data, err := procFn()
		if err != nil {
			return "", err
		}
		return format(data, width), nil
	}
}

var (
	runProcTopCPU   = procRun(func(d map[string]any, w int) string { return formatProcTop(d, false, w) })
	runProcTopMem   = procRun(func(d map[string]any, w int) string { return formatProcTop(d, true, w) })
	runProcApps     = procRun(formatProcApps)
	runProcSummary  = procRun(formatProcSummary)
	procColumnNames = []string{"PID", "%CPU", "%MEM", "RSS"}
)

// procLayout says which numeric columns fit. Narrow panes drop %MEM first and
// then RSS (decorative next to the sort key) so the name keeps room.
type procLayout struct {
	showMem, showRSS bool
	nameW            int
}

func planProcLayout(w int) procLayout {
	const pidW, cpuW, memW, rssW = 7, 6, 5, 7
	l := procLayout{showMem: true, showRSS: true}
	fixed := pidW + 1 + cpuW + 1 + memW + 1 + rssW + 1
	switch {
	case w < 32:
		l.showMem, l.showRSS = false, false
		fixed = pidW + 1 + cpuW + 1
	case w < 40:
		l.showMem = false
		fixed = pidW + 1 + cpuW + 1 + rssW + 1
	}
	l.nameW = max(w-fixed, 4)
	return l
}

// formatProcTop renders the top-15 table by CPU or by memory: PID, %CPU, %MEM,
// readable RSS and the process name (basename, never a path). Every row is cut
// to w cells; the PID is shown here because this is the user's own screen.
func formatProcTop(d map[string]any, byMem bool, width int) string {
	w := usableWidth(width)
	key, title := "top_cpu", "Top 15 por CPU"
	if byMem {
		key, title = "top_memory", "Top 15 por memoria"
	}
	procs, _ := d[key].([]monitor.ProcInfo)
	l := planProcLayout(w)
	lines := []string{tableHeaderStyle.Render(truncateToWidth(title, w)),
		tableHeaderStyle.Render(truncateToWidth(procHeader(l), w)), rule(w)}
	if len(procs) == 0 {
		return strings.Join(append(lines[:1], hdrLabelStyle.Render("  (sin datos)")), "\n") + "\n"
	}
	for _, p := range procs {
		lines = append(lines, procRow(p, l, byMem, w))
	}
	return strings.Join(lines, "\n") + "\n"
}

func procHeader(l procLayout) string {
	h := fmt.Sprintf("%7s %6s", procColumnNames[0], procColumnNames[1])
	if l.showMem {
		h += fmt.Sprintf(" %5s", procColumnNames[2])
	}
	if l.showRSS {
		h += fmt.Sprintf(" %7s", procColumnNames[3])
	}
	return h + " NAME"
}

// procRow renders one process. The sort key is colored by level; the name is
// truncated before padding so no row can exceed w.
func procRow(p monitor.ProcInfo, l procLayout, byMem bool, w int) string {
	cpu := fmt.Sprintf("%6.1f", p.CPU)
	mem := fmt.Sprintf("%5.1f", p.Mem)
	if byMem {
		mem = lipgloss.NewStyle().Foreground(levelColor(p.Mem * 4)).Render(mem)
	} else {
		cpu = lipgloss.NewStyle().Foreground(levelColor(p.CPU)).Render(cpu)
	}
	row := fmt.Sprintf("%7d %s", p.PID, cpu)
	if l.showMem {
		row += " " + mem
	}
	if l.showRSS {
		row += fmt.Sprintf(" %7s", monitor.FormatKB(p.RSSKB))
	}
	return truncateToWidth(row+" "+truncateToWidth(p.Name, l.nameW), w)
}

// formatMB renders a size in MB as "812 MB" or "1.4 GB".
func formatMB(mb float64) string {
	if mb >= 1024 {
		return fmt.Sprintf("%.1f GB", mb/1024)
	}
	return fmt.Sprintf("%.0f MB", mb)
}

// formatProcApps renders the per-application aggregate (largest RAM first):
// process count, total CPU, total RAM and the app name. It reuses the monitor's
// aggregation, so helper processes fold into their parent app.
func formatProcApps(d map[string]any, width int) string {
	w := usableWidth(width)
	apps, _ := d["apps"].([]monitor.AppUsage)
	const procsW, cpuW, ramW = 5, 7, 9
	nameW := max(w-(procsW+1+cpuW+1+ramW+1), 4)
	lines := []string{
		tableHeaderStyle.Render(truncateToWidth("Por aplicación (orden: RAM)", w)),
		tableHeaderStyle.Render(truncateToWidth(
			fmt.Sprintf("%*s %*s %*s NAME", procsW, "PROCS", cpuW, "%CPU", ramW, "RAM"), w)),
		rule(w),
	}
	if len(apps) == 0 {
		return strings.Join(append(lines[:1], hdrLabelStyle.Render("  (sin datos)")), "\n") + "\n"
	}
	for _, a := range apps {
		lines = append(lines, truncateToWidth(fmt.Sprintf("%*d %*.1f %*s %s",
			procsW, a.Procs, cpuW, a.CPUPct, ramW, formatMB(a.MemMB), truncateToWidth(a.Name, nameW)), w))
	}
	return strings.Join(lines, "\n") + "\n"
}

// formatProcSummary renders the process counts by state plus the memory
// pressure. Zombie and I/O-wait counts are colored only when non-zero.
func formatProcSummary(d map[string]any, width int) string {
	w := usableWidth(width)
	s, ok := d["summary"].(monitor.ProcSummary)
	if !ok {
		return "(no data available)\n"
	}
	warn := func(n int) string {
		txt := fmt.Sprintf("%d", n)
		if n > 0 {
			return lipgloss.NewStyle().Foreground(riskWarn).Render(txt)
		}
		return txt
	}
	lines := []string{
		kvLine("Procesos", fmt.Sprintf("%d", s.Total), w),
		kvLine("Running", fmt.Sprintf("%d", s.Running), w),
		kvLine("Sleeping", fmt.Sprintf("%d", s.Sleeping), w),
		kvLine("Stopped", fmt.Sprintf("%d", s.Stopped), w),
		kvLine("Zombie", warn(s.Zombie), w),
		kvLine("Espera I/O", warn(s.IOWait), w),
	}
	level, _ := d["pressure_level"].(string)
	if level == "" {
		level = monitor.PressureUnknown
	}
	lines = append(lines, "", kvLine("Presión mem.",
		lipgloss.NewStyle().Foreground(pressureColor(level)).Bold(true).Render("● "+level), w))
	if mp, ok := d["memory_pressure"].(string); ok && mp != "unknown" {
		lines = append(lines, kvLine("", hdrLabelStyle.Render(mp), w))
	}
	lines = append(lines, "", hdrLabelStyle.Render(truncateToWidth("Hilos: no se cuentan (costaría un ps extra por hilo).", w)))
	return strings.Join(lines, "\n") + "\n"
}

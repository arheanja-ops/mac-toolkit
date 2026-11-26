package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/arheanja-ops/mac-toolkit/internal/core"
	"github.com/arheanja-ops/mac-toolkit/internal/monitor"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// headerInterval is the pause between live-header refreshes.
// The system read inside a tick takes ~1 s (the CPU sample) and the next tick
// is only programmed once it returns, so the effective period is
// headerInterval + ~1 s. A slower interval keeps the header from adding load
// to a machine that is already busy.
const headerInterval = 3 * time.Second

// headerStats holds the lightweight, per-tick metrics from the system monitor.
// ok=false means the last system read failed; callers keep the last disk value
// and omit mem/CPU/thermal (degrade field by field).
type headerStats struct {
	memUsedPct int
	cpuUsedPct int
	thermal    string
	ok         bool
	watch      watchBadge // listening-mode indicator, independent of ok
}

// headerTickMsg carries a fresh disk+system reading produced off the event loop.
type headerTickMsg struct {
	disk  core.DiskSnapshot
	stats headerStats
}

// systemSnapshotFn is the source of system metrics. It is a package var so
// tests can inject a fake without touching real hardware.
var systemSnapshotFn = func() (map[string]any, error) {
	return (&monitor.SystemMonitor{Light: true}).Snapshot()
}

// readHeaderStats converts a system snapshot into headerStats, degrading field
// by field. Any missing field is simply left at its zero value.
func readHeaderStats() headerStats {
	s := readSystemStats()
	s.watch = readWatchBadge()
	return s
}

// readSystemStats prefers the listening engine's last sample, so the header
// does not read the machine a second time while the engine is sampling.
func readSystemStats() headerStats {
	if s, ok := engineHeaderStats(); ok {
		return s
	}
	data, err := systemSnapshotFn()
	if err != nil || data == nil {
		return headerStats{ok: false}
	}
	s := headerStats{ok: true}
	if v, ok := data["memory_percent"].(float64); ok {
		s.memUsedPct = int(v)
	}
	if v, ok := data["cpu_percent"].(float64); ok {
		s.cpuUsedPct = int(v)
	}
	if v, ok := data["thermal"].(string); ok {
		s.thermal = v
	}
	return s
}

// headerTick returns a tea.Cmd that, after headerInterval, reads the light
// metrics off the event loop and emits a headerTickMsg. The model reprograms
// the next tick on receipt, so the tick stops naturally once the program exits.
func headerTick() tea.Cmd {
	return tea.Tick(headerInterval, func(time.Time) tea.Msg {
		return headerTickMsg{
			disk:  core.ReadDiskSnapshot(),
			stats: readHeaderStats(),
		}
	})
}

// renderHeader builds the top status line. It always shows the version; each
// remaining field is appended only when available, so a failed disk or system
// read never breaks the layout. Fields are separated by a faint " · " so the
// metrics read as discrete, aligned columns rather than one run-on string.
func renderHeader(version string, disk core.DiskSnapshot, s headerStats, width int) string {
	sep := sepStyle.Render(" · ")
	parts := []string{bannerStyle.Render("mac-toolkit") + " " + version}

	if disk.OK {
		barWidth := 7
		if width < 80 {
			barWidth = 5
		}
		bar := core.ProgressBar(disk.PctUsed, barWidth)
		parts = append(parts, fmt.Sprintf("%s %s %d%% %s free",
			hdrLabelStyle.Render("Disk"), bar, disk.PctUsed,
			core.FormatBytes(disk.AvailBytes)))
	}

	if s.ok {
		parts = append(parts, fmt.Sprintf("%s %d%%", hdrLabelStyle.Render("Mem"), s.memUsedPct))
		// On narrow terminals collapse to disk + memory only (F2 constraint).
		if width >= 80 {
			parts = append(parts, fmt.Sprintf("%s %d%%", hdrLabelStyle.Render("CPU"), s.cpuUsedPct))
			// Thermal is the lowest-priority field; only append it when the
			// terminal is wider than 80 so the full header never overflows the
			// exact 80-column boundary (where CPU already fills the row).
			if width > 80 && s.thermal != "" {
				parts = append(parts, renderThermal(s.thermal))
			}
		}
	}

	out := strings.Join(parts, sep)
	if full, compact := s.watch.render(); full != "" {
		out = withBadge(parts, sep, full, compact, width)
	}
	// Final safety net: never let the header exceed the terminal width, even if
	// an unusually long disk/version string would otherwise push it over.
	if width > 0 {
		out = truncateToWidth(out, width)
	}
	return out
}

// withBadge appends the listening badge, trading detail for room: the full
// badge, then the compact one, then the compact one replacing the last metric.
// The badge is never truncated away.
func withBadge(parts []string, sep, full, compact string, width int) string {
	fits := func(p []string) bool { return width <= 0 || lipgloss.Width(strings.Join(p, sep)) <= width }
	for _, b := range []string{full, compact} {
		if p := append(append([]string{}, parts...), b); fits(p) {
			return strings.Join(p, sep)
		}
	}
	for n := len(parts) - 1; n >= 1; n-- {
		if p := append(append([]string{}, parts[:n]...), compact); fits(p) {
			return strings.Join(p, sep)
		}
	}
	return compact
}

// renderThermal colors the thermal field: only real alerts (Throttled,
// Warning) use the alert style. Normal and Unknown stay muted so a missing
// reading never looks like a problem.
func renderThermal(state string) string {
	glyph := "🌡 " + state
	if monitor.IsThermalAlert(state) {
		return thermalAlertStyle.Render(glyph)
	}
	return hdrLabelStyle.Render(glyph)
}

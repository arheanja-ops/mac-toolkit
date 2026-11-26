package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/arheanja-ops/mac-toolkit/internal/core"
	"github.com/arheanja-ops/mac-toolkit/internal/watch"

	"github.com/charmbracelet/lipgloss"
)

// watchEngine is the listening engine started by Run. It stays nil in tests
// (and when the TUI is built without Run), and every reader copes with that.
var watchEngine *watch.Engine

// historyStoreFn returns the history store; a package var for tests.
var historyStoreFn = func() watch.Store { return watch.Store{Dir: core.HistoryDir()} }

// watchBadge is what the header shows about the listening mode.
type watchBadge struct {
	on, paused, notify bool
	alerts             int
}

func readWatchBadge() watchBadge {
	if watchEngine == nil {
		return watchBadge{}
	}
	st := watchEngine.Status()
	return watchBadge{on: true, paused: st.Paused, notify: st.Notify, alerts: len(st.Active)}
}

// render returns the full and the compact badge ("" when not listening).
func (b watchBadge) render() (full, compact string) {
	if !b.on {
		return "", ""
	}
	dot := lipgloss.NewStyle().Foreground(riskSafe).Render("●")
	full = dot + " " + hdrLabelStyle.Render("escuchando")
	if b.paused {
		dot = hdrLabelStyle.Render("‖")
		full = dot + " " + hdrLabelStyle.Render("en pausa")
	}
	compact = dot
	if b.alerts > 0 {
		a := lipgloss.NewStyle().Foreground(riskWarn).Bold(true).Render(fmt.Sprintf("⚠ %d", b.alerts))
		full += " " + a
		compact += " " + a
	}
	if !b.notify {
		full += " " + hdrLabelStyle.Render("sin notif")
	}
	return full, compact
}

// liveSampleMaxAge is how old the engine's last sample may be to feed the
// header instead of a separate read (one reading source, less load).
const liveSampleMaxAge = 2 * watch.DefaultInterval

// engineHeaderStats returns header stats from the engine's last sample.
func engineHeaderStats() (headerStats, bool) {
	if watchEngine == nil {
		return headerStats{}, false
	}
	st := watchEngine.Status()
	s := st.Last
	if !st.HasLast || !s.MemOK || !s.CPUOK || time.Since(s.Time) > liveSampleMaxAge {
		return headerStats{}, false
	}
	return headerStats{ok: true, memUsedPct: int(s.MemUsedPct), cpuUsedPct: int(100 - s.IdlePct + 0.5), thermal: s.Thermal}, true
}

func sevStyle(sev string) lipgloss.Style {
	switch sev {
	case watch.SevCritical:
		return lipgloss.NewStyle().Foreground(riskDanger).Bold(true)
	case watch.SevWarn:
		return lipgloss.NewStyle().Foreground(riskWarn).Bold(true)
	default:
		return hdrLabelStyle
	}
}

// runActiveAlerts lists the firing alerts: rule, severity, since and value.
func runActiveAlerts(width int) (string, error) {
	w := usableWidth(width)
	if watchEngine == nil {
		return truncateToWidth("La escucha no está activa.", w) + "\n", nil
	}
	st := watchEngine.Status()
	lines := []string{kvLine("Escucha", listeningLabel(st), w)}
	if st.HasLast {
		s := st.Last
		lines = append(lines, kvLine("Muestra", fmt.Sprintf("%s · mem %.0f%% · swap %.0f%% · carga %.1f",
			s.Time.Format("15:04:05"), s.MemUsedPct, s.SwapUsedPct, s.LoadPerCore()), w))
	}
	lines = append(lines, rule(w))
	if len(st.Active) == 0 {
		lines = append(lines, lipgloss.NewStyle().Foreground(riskSafe).Render("✓ Sin alertas activas"))
	}
	for _, a := range st.Active {
		head := sevStyle(a.Severity).Render(watch.SevLabel(a.Severity)) + " " + reportValueStyle.Render(a.Title)
		lines = append(lines, truncateToWidth(head, w),
			truncateToWidth("  desde "+a.Since.Format("15:04")+" · "+a.Value, w))
		lines = append(lines, wrapWords(words("→ "+a.Action, &hdrLabelStyle), w, "  ", "    ")...)
	}
	lines = append(lines, rule(w),
		hdrLabelStyle.Render(truncateToWidth("n notificaciones · w pausar/reanudar · solo lectura", w)))
	return strings.Join(lines, "\n") + "\n", nil
}

func listeningLabel(st watch.Status) string {
	s := "activa cada " + watch.DefaultInterval.String()
	switch {
	case !st.Running:
		s = "detenida"
	case st.Paused:
		s = "en pausa"
	}
	if st.Notify {
		return s + " · notificaciones on"
	}
	return s + " · notificaciones off"
}

// runHistory summarizes the last 24 h by hour, newest first.
func runHistory(width int) (string, error) {
	w := usableWidth(width)
	now := time.Now()
	recs, err := historyStoreFn().Read(now.Add(-24*time.Hour), now.Add(time.Minute))
	if err != nil {
		return "", err
	}
	rows := watch.Hourly(recs)
	if len(rows) == 0 {
		lines := []string{
			tableHeaderStyle.Render("Sin historial en las últimas 24 h"),
			"",
			truncateToWidth("La escucha guarda una muestra cada 15 s mientras la TUI", w),
			truncateToWidth("está abierta o corre `mac-toolkit watch`.", w),
		}
		return strings.Join(lines, "\n") + "\n", nil
	}
	wide := w >= 50
	head := fmt.Sprintf("%-5s %5s %6s %6s", "HORA", "MUEST", "MEM↑", "CARGA↑")
	if wide {
		head += fmt.Sprintf(" %8s %7s", "SWAPOUT↑", "ALERTAS")
	}
	lines := []string{tableHeaderStyle.Render(truncateToWidth(head, w)), rule(w)}
	for _, r := range rows {
		line := fmt.Sprintf("%-5s %5d %5.0f%% %6.1f", r.Hour.Format("15:04"), r.Samples, r.MemMaxPct, r.LoadMax)
		if wide {
			al := fmt.Sprintf("%7d", r.AlertsFired)
			if r.AlertsFired > 0 {
				al = lipgloss.NewStyle().Foreground(riskWarn).Render(al)
			}
			line += fmt.Sprintf(" %8.1f %s", r.SwapOutMax, al)
		}
		lines = append(lines, truncateToWidth(line, w))
	}
	lines = append(lines, rule(w), hdrLabelStyle.Render(truncateToWidth("Rango libre: mac-toolkit history --since HH:MM", w)))
	return strings.Join(lines, "\n") + "\n", nil
}

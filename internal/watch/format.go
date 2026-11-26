package watch

import (
	"fmt"
	"strings"
	"time"
)

// SevLabel names a severity for people.
func SevLabel(sev string) string {
	switch sev {
	case SevCritical:
		return "crítica"
	case SevWarn:
		return "aviso"
	default:
		return "info"
	}
}

// EventLine is the one-line form `watch` prints for an event.
func EventLine(e Event) string {
	a := e.Alert
	switch e.Kind {
	case EventResolve:
		return fmt.Sprintf("%s ✓ resuelto  %s (%s)", e.Time.Format("15:04"), a.Title, a.Value)
	case EventEscalate:
		return fmt.Sprintf("%s ▲ %-7s %s: %s · %s", e.Time.Format("15:04"), SevLabel(a.Severity), a.Title, a.Value, a.Action)
	default:
		return fmt.Sprintf("%s ⚠ %-7s %s: %s desde %s · %s", e.Time.Format("15:04"), SevLabel(a.Severity),
			a.Title, a.Value, a.Since.Format("15:04"), a.Action)
	}
}

// StatusLine is the periodic summary of `watch`.
func StatusLine(st Status) string {
	if !st.HasLast {
		return time.Now().Format("15:04") + " sin muestras todavía"
	}
	s := st.Last
	line := fmt.Sprintf("%s · mem %.0f %% · swap %.0f %% · carga %.1f por núcleo · libre %.0f %% · alertas activas %d",
		s.Time.Format("15:04"), s.MemUsedPct, s.SwapUsedPct, s.LoadPerCore(), s.IdlePct, len(st.Active))
	return line
}

func statRow(label, unit string, s Stat) string {
	if s.N == 0 {
		return fmt.Sprintf("  %-22s sin datos\n", label)
	}
	return fmt.Sprintf("  %-22s %7.1f %7.1f %7.1f %7.1f  %s\n", label, s.Min, s.Avg, s.P95, s.Max, unit)
}

// FormatSummary renders the history summary for the terminal.
func FormatSummary(sum Summary) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Historial %s → %s · %d muestras\n\n", sum.From.Format("2006-01-02 15:04"),
		sum.To.Format("2006-01-02 15:04"), sum.Samples)
	if sum.Samples == 0 && len(sum.Alerts) == 0 {
		b.WriteString("Sin datos en ese rango. La escucha corre con la TUI abierta o con `mac-toolkit watch`.\n")
		return b.String()
	}
	fmt.Fprintf(&b, "  %-22s %7s %7s %7s %7s\n", "MÉTRICA", "MÍN", "PROM", "P95", "MÁX")
	b.WriteString(statRow("Memoria usada", "%", sum.MemUsedPct))
	b.WriteString(statRow("Swap usado", "%", sum.SwapUsedPct))
	b.WriteString(statRow("Swap-out", "MB/s", sum.SwapOutMBps))
	b.WriteString(statRow("Carga por núcleo (5m)", "", sum.LoadPerCore))
	b.WriteString(statRow("CPU libre", "%", sum.IdlePct))
	b.WriteString(statRow("Disco libre", "GB", sum.DiskFreeGB))
	b.WriteString("\nAlertas\n")
	if len(sum.Alerts) == 0 {
		b.WriteString("  ninguna\n")
	}
	for _, a := range sum.Alerts {
		end := "sigue activa"
		if !a.End.IsZero() {
			end = a.End.Format("15:04")
		}
		fmt.Fprintf(&b, "  %s → %-12s %-8s %s\n", a.Start.Format("01-02 15:04"), end, SevLabel(a.Severity), a.Title)
	}
	for _, g := range []struct {
		title string
		list  []NameCount
	}{{"Apps más veces arriba por CPU", sum.TopCPU}, {"Apps más veces arriba por memoria", sum.TopMem}} {
		if len(g.list) == 0 {
			continue
		}
		b.WriteString("\n" + g.title + "\n")
		for _, n := range g.list {
			fmt.Fprintf(&b, "  %-30s %d muestras\n", n.Name, n.Count)
		}
	}
	return b.String()
}

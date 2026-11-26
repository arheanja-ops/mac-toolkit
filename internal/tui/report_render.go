package tui

import (
	"fmt"
	"strings"

	"github.com/arheanja-ops/mac-toolkit/internal/core"
	"github.com/arheanja-ops/mac-toolkit/internal/reporter"

	"github.com/charmbracelet/lipgloss"
)

// Report palette. It reuses the semantic colors of styles.go so a "danger" in
// the report reads exactly like a "danger" anywhere else in the TUI.
var (
	reportTitleStyle   = lipgloss.NewStyle().Foreground(accent).Bold(true)
	reportSectionStyle = lipgloss.NewStyle().Foreground(accent).Bold(true)
	reportValueStyle   = lipgloss.NewStyle().Bold(true)
)

// reportDetailItems caps the items listed per domain in the TUI; the model keeps
// more, the full list is in the md/html files.
const reportDetailItems = 5

// RenderReport draws a saved report as structured, width-bounded lines: the
// same Report the md/json/html files were rendered from, so every figure
// matches them. w is the usable width; no line exceeds it (40-80+ columns).
// It is exported so `mac-toolkit report` prints the same view as the TUI.
func RenderReport(r reporter.Report, w int) string {
	w = max(w, 20)
	var out []string
	out = append(out, reportHeader(r, w)...)
	out = append(out, reportSummary(r, w)...)
	out = append(out, reportHost(r, w)...)
	out = append(out, reportSystem(r, w)...)
	out = append(out, reportDisk(r, w)...)
	out = append(out, reportDetails(r, w)...)
	out = append(out, reportProcesses(r, w)...)
	out = append(out, reportRecommendations(r, w)...)
	for i, l := range out {
		out[i] = truncateToWidth(l, w)
	}
	return strings.Join(out, "\n") + "\n"
}

func section(title string, w int) []string {
	return []string{"", reportSectionStyle.Render(truncateToWidth(strings.ToUpper(title), w)), rule(w)}
}

func statusColor(status string) lipgloss.Color {
	switch status {
	case reporter.StatusCritical:
		return riskDanger
	case reporter.StatusAttention:
		return riskWarn
	default:
		return riskSafe
	}
}

func findingColor(sev string) lipgloss.Color { return insightColor(sev) }

func reportHeader(r reporter.Report, w int) []string {
	title := "Reporte · " + r.GeneratedAt.Format("2006-01-02 15:04")
	status := lipgloss.NewStyle().Foreground(statusColor(r.Summary.Status)).Bold(true).
		Render("● " + reporter.StatusLabel(r.Summary.Status))
	lines := []string{reportTitleStyle.Render(truncateToWidth(title, w)), kvLine("Estado", status, w)}
	if r.Variant == reporter.VariantShare {
		lines = append(lines, noteLines("Variante compartible: sin rutas, nombres de archivo, etiquetas ni nombres de apps.", w)...)
	}
	return lines
}

func reportSummary(r reporter.Report, w int) []string {
	out := section("Resumen", w)
	if len(r.Summary.Top) == 0 {
		return append(out, hdrLabelStyle.Render("Sin hallazgos."))
	}
	for i, f := range r.Summary.Top {
		out = append(out, findingLines(fmt.Sprintf("%d.", i+1), f, w)...)
	}
	return out
}

// findingLines renders "N. ● title" plus the action wrapped under it.
func findingLines(prefix string, f reporter.Finding, w int) []string {
	dot := lipgloss.NewStyle().Foreground(findingColor(f.Severity)).Render("●")
	head := prefix + " " + dot + " "
	indent := strings.Repeat(" ", lipgloss.Width(prefix)+3)
	lines := wrapWords(words(f.Title, &reportValueStyle), w, head, indent)
	if f.Action != "" {
		lines = append(lines, wrapWords(words("→ "+f.Action, &hdrLabelStyle), w, indent, indent+"  ")...)
	}
	return lines
}

func reportHost(r reporter.Report, w int) []string {
	out := section("Equipo", w)
	for _, kv := range r.HostRows() {
		if kv[0] == "Fecha" {
			continue // already in the title
		}
		out = append(out, kvLine(kv[0], kv[1], w))
	}
	return out
}

func reportSystem(r reporter.Report, w int) []string {
	s := r.System
	if s.Memory == nil && s.CPU == nil && s.Thermal == "" {
		return nil
	}
	out := section("Sistema", w)
	barW := max(min(w-viewLabelW-1-24, 20), 5)
	if m := s.Memory; m != nil {
		bar := usageBar(m.UsedPct, barW, levelColor(m.UsedPct))
		out = append(out,
			kvLine("Memoria", fmt.Sprintf("%s %.0f%% %.1f de %.1f GB", bar, m.UsedPct, m.UsedGB, m.TotalGB), w),
			kvLine("Presión", m.Pressure, w))
		if sw := m.SwapLine(); sw != "" {
			out = append(out, kvLine("Swap", sw, w))
		}
		out = append(out, kvLine("Compresor", m.CompressorLine(), w))
		if m.PagingVerdict != "" {
			out = append(out, wrapWords(words(m.PagingVerdict, &hdrLabelStyle), w, strings.Repeat(" ", viewLabelW+1), strings.Repeat(" ", viewLabelW+1))...)
		}
	}
	if c := s.CPU; c != nil {
		busy := c.UserPct + c.SysPct
		bar := usageBar(busy, barW, levelColor(busy))
		out = append(out,
			kvLine("CPU", fmt.Sprintf("%s %.0f%% en uso", bar, busy), w),
			kvLine("Carga", fmt.Sprintf("%.2f · %.2f · %.2f (%.2f por núcleo)", c.Load1, c.Load5, c.Load15, c.LoadPerCore), w))
		if c.Verdict != "" {
			out = append(out, wrapWords(words(c.Verdict, &hdrLabelStyle), w, strings.Repeat(" ", viewLabelW+1), strings.Repeat(" ", viewLabelW+1))...)
		}
	}
	if s.Thermal != "" {
		th := s.Thermal
		if s.SpeedLimitPct != nil {
			th += fmt.Sprintf(" · límite CPU %d%%", *s.SpeedLimitPct)
		}
		out = append(out, kvLine("Térmico", th, w))
	}
	return out
}

func reportDisk(r reporter.Report, w int) []string {
	d := r.Disk
	out := section("Disco", w)
	if v := d.Volume; v.Known {
		out = append(out, kvLine("Volumen", fmt.Sprintf("%s usados de %s (%d%%) · libre %s",
			core.FormatBytes(v.UsedBytes), core.FormatBytes(v.TotalBytes), v.UsedPct, core.FormatBytes(v.FreeBytes)), w))
	}
	out = append(out, hdrLabelStyle.Render(truncateToWidth("El volumen es contexto: no se suma a lo recuperable.", w)))
	rc := d.Reclaimable
	safe := lipgloss.NewStyle().Foreground(riskSafe)
	warn := lipgloss.NewStyle().Foreground(riskWarn)
	out = append(out,
		kvLine("Seguro", safe.Render(core.FormatBytes(rc.SafeBytes)), w),
		kvLine("A revisar", warn.Render(core.FormatBytes(rc.ReviewBytes)), w),
		kvLine("Recuperable", reportValueStyle.Render(core.FormatBytes(rc.TotalBytes)), w))
	if len(d.Domains) > 0 {
		out = append(out, "")
		out = append(out, domainTable(d.Domains, w)...)
	}
	for _, f := range d.Failed {
		msg := f.Domain + ": " + f.Error
		out = append(out, lipgloss.NewStyle().Foreground(riskDanger).Render(truncateToWidth("✗ "+msg, w)))
	}
	return out
}

// domainTable: DOMINIO SEV TAMAÑO ELEM [bar %]. The bar is dropped first when
// the width is tight, then the domain column shrinks.
func domainTable(rows []reporter.DomainRow, w int) []string {
	const sevW, sizeW, itemsW, pctW = 8, 10, 5, 4
	barW := 0
	if w >= 60 {
		barW = 10
	}
	fixed := sevW + sizeW + itemsW + 3
	if barW > 0 {
		fixed += barW + pctW + 2
	}
	domW := max(w-fixed, 4)
	head := padCell("DOMINIO", domW) + " " + padCell("SEV", sevW) + " " +
		fmt.Sprintf("%*s %*s", sizeW, "TAMAÑO", itemsW, "ELEM")
	if barW > 0 {
		head += " " + padCell("", barW) + " " + fmt.Sprintf("%*s", pctW, "%")
	}
	out := []string{tableHeaderStyle.Render(truncateToWidth(head, w))}
	for _, d := range rows {
		sev := lipgloss.NewStyle().Foreground(severityColor(core.Severity(d.Severity))).
			Render(padCell(truncateToWidth(d.Severity, sevW), sevW))
		line := padCell(truncateToWidth(d.Domain, domW), domW) + " " + sev + " " +
			fmt.Sprintf("%*s %*d", sizeW, core.FormatBytes(d.SizeBytes), itemsW, d.Items)
		if barW > 0 {
			line += " " + ruleStyle.Render(reporter.TextBar(d.SharePct, barW)) + " " + fmt.Sprintf("%*.0f%%", pctW-1, d.SharePct)
		}
		out = append(out, line)
	}
	return out
}

func reportDetails(r reporter.Report, w int) []string {
	if len(r.Details) == 0 {
		return nil
	}
	out := section("Detalle por dominio", w)
	const sizeW, riskW = 10, 7
	labelW := max(w-sizeW-riskW-4, 6)
	for _, d := range r.Details {
		out = append(out, tableHeaderStyle.Render(truncateToWidth(d.Domain+" · "+core.FormatBytes(d.SizeBytes), w)))
		for i, it := range d.Items {
			if i >= reportDetailItems {
				break
			}
			mark := lipgloss.NewStyle().Foreground(riskSafe).Render("✓")
			if !it.Safe {
				mark = lipgloss.NewStyle().Foreground(riskDanger).Render("✗")
			}
			risk := lipgloss.NewStyle().Foreground(riskColor(core.RiskLevel(it.Risk))).Render(padCell(it.Risk, riskW))
			out = append(out, fmt.Sprintf("%s %*s %s %s", mark, sizeW, core.FormatBytes(it.SizeBytes), risk,
				truncateToWidth(it.Label, labelW)))
		}
		if hidden := len(d.Items) - reportDetailItems + d.More; hidden > 0 {
			out = append(out, hdrLabelStyle.Render(truncateToWidth(fmt.Sprintf("  … y %d más (ver el html)", hidden), w)))
		}
	}
	return out
}

func reportProcesses(r reporter.Report, w int) []string {
	p := r.Processes
	if len(p.TopMemory) == 0 && len(p.TopCPU) == 0 {
		return nil
	}
	out := section("Procesos", w)
	out = append(out, reportApps("Top por memoria", p.TopMemory, true, w)...)
	out = append(out, reportApps("Top por CPU", p.TopCPU, false, w)...)
	return out
}

func reportApps(title string, apps []reporter.App, byMem bool, w int) []string {
	out := []string{tableHeaderStyle.Render(truncateToWidth(title, w))}
	if len(apps) == 0 {
		return append(out, hdrLabelStyle.Render("  (sin datos)"))
	}
	const fixed = 3 + 1 + 9 + 1 + 5
	nameW := max(w-fixed, 6)
	for i, a := range apps {
		val := fmt.Sprintf("%6.0f MB", a.MemMB)
		if !byMem {
			val = fmt.Sprintf("%8.1f%%", a.CPUPct)
		}
		out = append(out, fmt.Sprintf("%d. %s %s %s", i+1, padCell(truncateToWidth(a.Name, nameW), nameW), val,
			hdrLabelStyle.Render(fmt.Sprintf("×%d", a.Procs))))
	}
	return out
}

func reportRecommendations(r reporter.Report, w int) []string {
	rec := r.Recommendations
	if len(rec.Performance)+len(rec.QuickWins)+len(rec.Review) == 0 {
		return nil
	}
	out := section("Recomendaciones", w)
	if rec.Engine != "" {
		out = append(out, hdrLabelStyle.Render(truncateToWidth("Motor: "+rec.Engine+" · sugerencias, nada se ejecuta", w)))
	}
	for _, g := range []struct {
		title string
		list  []reporter.Finding
	}{{"Rendimiento", rec.Performance}, {"Quick wins", rec.QuickWins}, {"A revisar", rec.Review}} {
		if len(g.list) == 0 {
			continue
		}
		out = append(out, tableHeaderStyle.Render(truncateToWidth(g.title, w)))
		for _, f := range g.list {
			out = append(out, findingLines("•", f, w)...)
		}
	}
	return out
}

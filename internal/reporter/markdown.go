package reporter

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/arheanja-ops/mac-toolkit/internal/core"
)

// statusEmoji is the traffic light shown next to the status in markdown.
func statusEmoji(status string) string {
	switch status {
	case StatusCritical:
		return "🔴"
	case StatusAttention:
		return "🟡"
	default:
		return "🟢"
	}
}

// Markdown renders the report as GitHub-flavoured markdown: aligned tables that
// read the same as plain text and as rendered output (GitHub, Obsidian).
func (r Report) Markdown() string {
	var b strings.Builder
	b.WriteString("# Reporte de Mac Toolkit\n\n")
	if r.Variant == VariantShare {
		b.WriteString("> Variante compartible: no incluye rutas, nombres de archivo, " +
			"etiquetas de elementos ni nombres de apps.\n\n")
	}
	mdHost(&b, r)
	mdSummary(&b, r)
	mdSystem(&b, r)
	mdDisk(&b, r)
	mdDetails(&b, r)
	mdProcesses(&b, r)
	mdRecommendations(&b, r)
	return b.String()
}

// mdTable writes an aligned GFM table. Columns flagged in right are
// right-aligned (numbers); cells are escaped so a "|" cannot split a row.
func mdTable(b *strings.Builder, head []string, rows [][]string, right ...int) {
	isRight := map[int]bool{}
	for _, c := range right {
		isRight[c] = true
	}
	w := make([]int, len(head))
	cell := func(s string) string { return strings.NewReplacer("|", "\\|", "\n", " ").Replace(s) }
	for i, h := range head {
		w[i] = utf8.RuneCountInString(h)
	}
	for _, row := range rows {
		for i := range row {
			row[i] = cell(row[i])
			w[i] = max(w[i], utf8.RuneCountInString(row[i]))
		}
	}
	line := func(cells []string) {
		b.WriteString("|")
		for i, c := range cells {
			pad := strings.Repeat(" ", w[i]-utf8.RuneCountInString(c))
			if isRight[i] {
				b.WriteString(" " + pad + c + " |")
			} else {
				b.WriteString(" " + c + pad + " |")
			}
		}
		b.WriteString("\n")
	}
	line(head)
	b.WriteString("|")
	for i := range head {
		if isRight[i] {
			b.WriteString(" " + strings.Repeat("-", max(w[i]-1, 1)) + ": |")
		} else {
			b.WriteString(" " + strings.Repeat("-", max(w[i], 3)) + " |")
		}
	}
	b.WriteString("\n")
	for _, row := range rows {
		line(row)
	}
	b.WriteString("\n")
}

func mdHost(b *strings.Builder, r Report) {
	b.WriteString("## 1. Equipo\n\n")
	var rows [][]string
	for _, kv := range r.HostRows() {
		rows = append(rows, []string{kv[0], kv[1]})
	}
	mdTable(b, []string{"Campo", "Valor"}, rows)
}

func mdSummary(b *strings.Builder, r Report) {
	b.WriteString("## 2. Resumen ejecutivo\n\n")
	fmt.Fprintf(b, "**Estado general: %s %s**\n\n", statusEmoji(r.Summary.Status), StatusLabel(r.Summary.Status))
	if len(r.Summary.Top) == 0 {
		b.WriteString("Sin hallazgos.\n\n")
		return
	}
	var rows [][]string
	for i, f := range r.Summary.Top {
		rows = append(rows, []string{fmt.Sprint(i + 1), f.Title, SeverityLabel(f.Severity), f.Action})
	}
	mdTable(b, []string{"#", "Hallazgo", "Severidad", "Acción"}, rows)
}

func mdSystem(b *strings.Builder, r Report) {
	b.WriteString("## 3. Sistema\n\n")
	if m := r.System.Memory; m != nil {
		b.WriteString("### Memoria\n\n")
		rows := [][]string{
			{"Usada", fmt.Sprintf("%.0f%% (%.1f GB de %.1f GB)", m.UsedPct, m.UsedGB, m.TotalGB)},
			{"Presión", m.Pressure},
		}
		if s := m.SwapLine(); s != "" {
			rows = append(rows, []string{"Swap", s})
		}
		rows = append(rows, []string{"Tráfico de swap", m.SwapTrafficLine()},
			[]string{"Compresor", m.CompressorLine()})
		if m.PagingVerdict != "" {
			rows = append(rows, []string{"Paginación", m.PagingVerdict})
		}
		mdTable(b, []string{"Métrica", "Valor"}, rows)
	}
	if c := r.System.CPU; c != nil {
		b.WriteString("### CPU\n\n")
		rows := [][]string{
			{"Uso", fmt.Sprintf("usuario %.1f%% · sistema %.1f%% · libre %.1f%%", c.UserPct, c.SysPct, c.IdlePct)},
			{"Carga (1, 5 y 15 min)", fmt.Sprintf("%.2f · %.2f · %.2f", c.Load1, c.Load5, c.Load15)},
			{"Carga por núcleo (5 min)", fmt.Sprintf("%.2f", c.LoadPerCore)},
		}
		if c.Verdict != "" {
			rows = append(rows, []string{"Veredicto", c.Verdict})
		}
		mdTable(b, []string{"Métrica", "Valor"}, rows)
	}
	if r.System.Thermal != "" {
		fmt.Fprintf(b, "**Térmico:** %s\n\n", thermalLine(r.System))
	}
}

func thermalLine(s System) string {
	if s.SpeedLimitPct != nil {
		return fmt.Sprintf("%s (límite de velocidad de CPU: %d%%)", s.Thermal, *s.SpeedLimitPct)
	}
	return s.Thermal
}

func mdDisk(b *strings.Builder, r Report) {
	d := r.Disk
	b.WriteString("## 4. Disco\n\n### Uso del volumen (contexto, no se suma)\n\n")
	if d.Volume.Known {
		mdTable(b, []string{"Total", "Usado", "Libre", "Uso"}, [][]string{{
			core.FormatBytes(d.Volume.TotalBytes), core.FormatBytes(d.Volume.UsedBytes),
			core.FormatBytes(d.Volume.FreeBytes), fmt.Sprintf("%d%%", d.Volume.UsedPct),
		}}, 0, 1, 2, 3)
	} else {
		b.WriteString("No disponible.\n\n")
	}
	b.WriteString("### Recuperable\n\n")
	rc := d.Reclaimable
	mdTable(b, []string{"Concepto", "Tamaño"}, [][]string{
		{"Seguro (safe)", core.FormatBytes(rc.SafeBytes)},
		{"A revisar (warn y danger)", core.FormatBytes(rc.ReviewBytes)},
		{"**Total recuperable**", "**" + core.FormatBytes(rc.TotalBytes) + "**"},
	}, 1)
	if len(d.Domains) > 0 {
		var rows [][]string
		for _, dr := range d.Domains {
			rows = append(rows, []string{dr.Domain, dr.Severity, core.FormatBytes(dr.SizeBytes),
				fmt.Sprint(dr.Items), TextBar(dr.SharePct, 12), fmt.Sprintf("%.0f%%", dr.SharePct)})
		}
		mdTable(b, []string{"Dominio", "Severidad", "Tamaño", "Elementos", "Proporción", "%"}, rows, 2, 3, 5)
	}
	if len(d.Failed) > 0 {
		b.WriteString("### Dominios con error\n\n")
		var rows [][]string
		for _, f := range d.Failed {
			rows = append(rows, []string{f.Domain, f.Error})
		}
		mdTable(b, []string{"Dominio", "Error"}, rows)
	}
}

func mdDetails(b *strings.Builder, r Report) {
	if len(r.Details) == 0 {
		return
	}
	b.WriteString("## 5. Detalle por dominio\n\n")
	for _, d := range r.Details {
		fmt.Fprintf(b, "### %s · %s\n\n", d.Domain, core.FormatBytes(d.SizeBytes))
		var rows [][]string
		for _, it := range d.Items {
			rows = append(rows, []string{core.FormatBytes(it.SizeBytes), it.Risk, safeMark(it.Safe),
				it.Label, "`" + ShortPath(it.Path) + "`"})
		}
		mdTable(b, []string{"Tamaño", "Riesgo", "Seguro", "Elemento", "Ruta"}, rows, 0)
		if d.More > 0 {
			fmt.Fprintf(b, "… y %d más (%s)\n\n", d.More, core.FormatBytes(d.MoreBytes))
		}
	}
}

func safeMark(safe bool) string {
	if safe {
		return "sí"
	}
	return "no"
}

func mdProcesses(b *strings.Builder, r Report) {
	b.WriteString("## 6. Procesos\n\nApps agregadas por aplicación, sin PIDs.\n\n")
	for _, t := range []struct {
		title string
		apps  []App
	}{{"Top por memoria", r.Processes.TopMemory}, {"Top por CPU", r.Processes.TopCPU}} {
		b.WriteString("### " + t.title + "\n\n")
		if len(t.apps) == 0 {
			b.WriteString("Sin datos.\n\n")
			continue
		}
		var rows [][]string
		for i, a := range t.apps {
			rows = append(rows, []string{fmt.Sprint(i + 1), a.Name, fmt.Sprintf("%.0f MB", a.MemMB),
				fmt.Sprintf("%.1f%%", a.CPUPct), fmt.Sprint(a.Procs)})
		}
		mdTable(b, []string{"#", "App", "Memoria", "CPU", "Procesos"}, rows, 2, 3, 4)
	}
}

func mdRecommendations(b *strings.Builder, r Report) {
	b.WriteString("## 7. Recomendaciones\n\n")
	fmt.Fprintf(b, "Motor: %s. Son sugerencias: el toolkit no ejecuta ninguna.\n\n", r.Recommendations.Engine)
	for _, g := range []struct {
		title string
		list  []Finding
	}{
		{"Rendimiento", r.Recommendations.Performance},
		{"Quick wins (seguros)", r.Recommendations.QuickWins},
		{"A revisar", r.Recommendations.Review},
	} {
		if len(g.list) == 0 {
			continue
		}
		b.WriteString("### " + g.title + "\n\n")
		var rows [][]string
		for _, f := range g.list {
			rows = append(rows, []string{f.Title, SeverityLabel(f.Severity), f.Detail, f.Action})
		}
		mdTable(b, []string{"Hallazgo", "Severidad", "Detalle", "Acción"}, rows)
	}
}

package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/arheanja-ops/mac-toolkit/internal/core"
	"github.com/arheanja-ops/mac-toolkit/internal/reporter"

	"github.com/charmbracelet/lipgloss"
)

// saveReportFn runs the analysis, builds the single report model and writes
// md+json+html plus the share variant into the canonical reports folder. A
// package var so tests never scan the machine. It is the same code path as
// `analyze --save` (reporter.Collect + reporter.SaveReport), which never writes
// to stderr, so it is safe while the TUI owns the screen.
var saveReportFn = func() (reporter.Saved, int, error) {
	results := analysisFn()
	saved, err := reporter.SaveReport(reporter.Collect(results))
	return saved, len(results), err
}

// openReportFn opens the html of a run folder. Tests replace it (and
// reporter.OpenFn) so nothing is ever opened on the machine running them.
var openReportFn = reporter.OpenReport

// tildePath shows a path relative to $HOME as "~/...", so the screen (and any
// pasted screenshot) does not carry the user name.
func tildePath(p string) string {
	home, _ := os.UserHomeDir()
	return reporter.TildePath(p, home)
}

// noReportsMessage is the friendly empty state: how to create the first report
// and where it will be stored. It is a normal result, not an error.
func noReportsMessage(w int) string {
	lines := []string{
		tableHeaderStyle.Render("Aún no hay reportes guardados"),
		"",
		"Para crear uno:",
		"  • elige Save report en REPORTS, o",
		"  • ejecuta: mac-toolkit analyze --save",
		"",
		hdrLabelStyle.Render("Carpeta:"),
		hdrLabelStyle.Render("  " + tildePath(core.ReportsDir())),
	}
	for i, l := range lines {
		lines[i] = truncateToWidth(l, w)
	}
	return strings.Join(lines, "\n") + "\n"
}

// runLastReport shows the newest saved report. A v1.10+ folder has a
// report.json, rendered as a structured view; older folders only have the
// markdown, shown with the plain renderer.
func runLastReport(width int) (string, error) {
	w := usableWidth(width)
	entry, ok, err := core.LatestReport()
	if err != nil {
		return "", err
	}
	if !ok {
		return noReportsMessage(w), nil
	}
	if r, err := reporter.LoadJSON(filepath.Join(entry.Dir, reporter.FileJSON)); err == nil {
		foot := []string{rule(w), hdrLabelStyle.Render(truncateToWidth(tildePath(entry.Dir), w))}
		if hasFormat(entry, "html") {
			foot = append(foot, hdrLabelStyle.Render(truncateToWidth("o: abrir el html en el navegador", w)))
		}
		return RenderReport(r, w) + strings.Join(foot, "\n") + "\n", nil
	}
	data, err := os.ReadFile(entry.Path)
	if err != nil {
		return "", fmt.Errorf("cannot read report: %w", err)
	}
	head := []string{
		tableHeaderStyle.Render(truncateToWidth("Último reporte · "+entry.ModTime.Format("2006-01-02 15:04"), w)),
		hdrLabelStyle.Render(truncateToWidth(tildePath(entry.Dir), w)),
		rule(w),
	}
	return strings.Join(append(head, renderMarkdown(string(data), w)...), "\n") + "\n", nil
}

func hasFormat(e core.ReportEntry, f string) bool {
	for _, x := range e.Formats {
		if x == f {
			return true
		}
	}
	return false
}

// renderMarkdown turns a legacy report's markdown into plain, width-bounded
// lines: headings become headers, table rows become spaced columns (the
// separator row is dropped) and backticks/bold markers disappear.
func renderMarkdown(src string, w int) []string {
	var out []string
	for _, raw := range strings.Split(strings.TrimRight(src, "\n"), "\n") {
		line := strings.TrimSpace(raw)
		switch {
		case strings.HasPrefix(line, "#"):
			title := strings.TrimSpace(strings.TrimLeft(line, "#"))
			out = append(out, tableHeaderStyle.Render(truncateToWidth(title, w)))
		case strings.HasPrefix(line, "|"):
			if isSeparatorRow(line) {
				continue
			}
			out = append(out, truncateToWidth(tableCells(line), w))
		default:
			out = append(out, truncateToWidth(strings.NewReplacer("**", "", "`", "", "*", "").Replace(line), w))
		}
	}
	return out
}

// isSeparatorRow detects the "|---|---|" line of a markdown table.
func isSeparatorRow(line string) bool {
	return strings.Trim(line, "|-: ") == ""
}

// tableCells joins the cells of a markdown table row with two spaces.
func tableCells(line string) string {
	cells := strings.Split(strings.Trim(line, "| "), "|")
	for i, c := range cells {
		cells[i] = strings.NewReplacer("`", "", "**", "").Replace(strings.TrimSpace(c))
	}
	return strings.Join(cells, "  ")
}

// formatsLabel is "md·json·html +share" for a saved run.
func formatsLabel(e core.ReportEntry) string {
	s := strings.Join(e.Formats, "·")
	if e.HasShare {
		s += " +share"
	}
	return s
}

// runSavedReports lists the saved reports: date, size, formats and variants.
func runSavedReports(width int) (string, error) {
	w := usableWidth(width)
	list, err := core.ListReports()
	if err != nil {
		return "", err
	}
	if len(list) == 0 {
		return noReportsMessage(w), nil
	}
	const dateW, sizeW = 16, 9
	fmtW := max(w-dateW-sizeW-2, 4)
	lines := []string{
		tableHeaderStyle.Render(truncateToWidth(fmt.Sprintf("%-*s %*s %s", dateW, "FECHA", sizeW, "TAMAÑO", "FORMATOS"), w)),
		rule(w),
	}
	for _, e := range list {
		lines = append(lines, truncateToWidth(fmt.Sprintf("%-*s %*s %s", dateW,
			e.ModTime.Format("2006-01-02 15:04"), sizeW, core.FormatBytes(e.Size),
			truncateToWidth(formatsLabel(e), fmtW)), w))
	}
	lines = append(lines, rule(w),
		hdrLabelStyle.Render(truncateToWidth(tildePath(core.ReportsDir()), w)),
		hdrLabelStyle.Render(truncateToWidth("share: copia sin rutas ni nombres, para compartir", w)))
	return strings.Join(lines, "\n") + "\n", nil
}

// runSaveReport analyzes the machine and saves the report (md, json, html and
// the share variant). It is the only TUI action that writes, and only inside
// the toolkit's own reports folder; nothing is deleted or changed elsewhere.
func runSaveReport(width int) (string, error) {
	w := usableWidth(width)
	saved, n, err := saveReportFn()
	if err != nil {
		return "", err
	}
	ok := lipgloss.NewStyle().Foreground(riskSafe).Bold(true).Render("✓ Reporte guardado")
	lines := []string{
		ok,
		"",
		kvLine("Dominios", fmt.Sprintf("%d analizados", n), w),
		kvLine("Carpeta", "", w),
		"  " + hdrLabelStyle.Render(truncateToWidth(tildePath(saved.Dir), w-2)),
		kvLine("Completo", "report.md · json · html", w),
		kvLine("Compartir", "share/ (sin rutas ni nombres)", w),
		"",
		hdrLabelStyle.Render(truncateToWidth("Léelo en REPORTS → Last report · o abre el html.", w)),
		hdrLabelStyle.Render(truncateToWidth("e: exportarlo a otra carpeta (p. ej. ~/Downloads).", w)),
	}
	return strings.Join(lines, "\n") + "\n", nil
}

// runOpenReport opens the html of the newest saved report with the system
// opener. It never writes or deletes; it only launches `open` on a file the
// toolkit wrote inside its reports folder (reporter.OpenReport enforces it).
func runOpenReport(width int) (string, error) {
	w := usableWidth(width)
	entry, ok, err := core.LatestReport()
	if err != nil {
		return "", err
	}
	if !ok {
		return noReportsMessage(w), nil
	}
	if !hasFormat(entry, "html") {
		return truncateToWidth("El último reporte no tiene html (es anterior a v1.10). Guarda uno nuevo con Save report.", w) + "\n", nil
	}
	p, err := openReportFn(entry.Dir, false)
	if err != nil {
		return "", err
	}
	lines := []string{
		lipgloss.NewStyle().Foreground(riskSafe).Bold(true).Render("✓ Abriendo el reporte en el navegador"),
		"",
		hdrLabelStyle.Render(truncateToWidth(tildePath(p), w)),
	}
	return strings.Join(lines, "\n") + "\n", nil
}

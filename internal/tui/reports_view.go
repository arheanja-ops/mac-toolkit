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

// saveReportFn runs the analysis and writes md+json into the canonical reports
// folder. A package var so tests never scan the machine. It reuses
// reporter.SaveAnalysis (the same code behind `analyze --save`) in quiet mode:
// the TUI owns the screen, so reporters must not write to stderr.
var saveReportFn = func() (string, int, error) {
	results := analysisFn()
	dir, err := reporter.SaveAnalysis(results, true)
	return dir, len(results), err
}

// tildePath shows a path relative to $HOME as "~/...", so the screen (and any
// pasted screenshot) does not carry the user name.
func tildePath(p string) string {
	if home, err := os.UserHomeDir(); err == nil && home != "" && strings.HasPrefix(p, home) {
		return "~" + strings.TrimPrefix(p, home)
	}
	return p
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

// runLastReport shows the newest saved analysis report, rendered for the pane.
func runLastReport(width int) (string, error) {
	w := usableWidth(width)
	entry, ok, err := core.LatestReport()
	if err != nil {
		return "", err
	}
	if !ok {
		return noReportsMessage(w), nil
	}
	data, err := os.ReadFile(entry.Path)
	if err != nil {
		return "", fmt.Errorf("cannot read report: %w", err)
	}
	head := []string{
		tableHeaderStyle.Render(truncateToWidth("Último reporte · "+entry.ModTime.Format("2006-01-02 15:04"), w)),
		hdrLabelStyle.Render(truncateToWidth(tildePath(filepath.Dir(entry.Path)), w)),
		rule(w),
	}
	return strings.Join(append(head, renderMarkdown(string(data), w)...), "\n") + "\n", nil
}

// renderMarkdown turns the report's markdown into plain, width-bounded lines:
// headings become headers, table rows become spaced columns (the separator row
// is dropped) and backticks/bold markers disappear.
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

// runSavedReports lists the saved reports: date, size and folder name.
func runSavedReports(width int) (string, error) {
	w := usableWidth(width)
	list, err := core.ListReports()
	if err != nil {
		return "", err
	}
	if len(list) == 0 {
		return noReportsMessage(w), nil
	}
	nameW := max(w-(16+1+9+1), 4)
	lines := []string{
		tableHeaderStyle.Render(truncateToWidth(fmt.Sprintf("%-16s %9s %s", "FECHA", "TAMAÑO", "REPORTE"), w)),
		rule(w),
	}
	for _, e := range list {
		lines = append(lines, truncateToWidth(fmt.Sprintf("%-16s %9s %s",
			e.ModTime.Format("2006-01-02 15:04"), core.FormatBytes(e.Size),
			truncateToWidth(e.Name, nameW)), w))
	}
	lines = append(lines, rule(w), hdrLabelStyle.Render(truncateToWidth(tildePath(core.ReportsDir()), w)))
	return strings.Join(lines, "\n") + "\n", nil
}

// runSaveReport analyzes the machine and saves report.md + report.json. It is
// the only TUI action that writes, and only inside the toolkit's own reports
// folder; nothing is deleted or changed elsewhere.
func runSaveReport(width int) (string, error) {
	w := usableWidth(width)
	dir, n, err := saveReportFn()
	if err != nil {
		return "", err
	}
	ok := lipgloss.NewStyle().Foreground(riskSafe).Bold(true).Render("✓ Reporte guardado")
	lines := []string{
		ok,
		"",
		kvLine("Dominios", fmt.Sprintf("%d analizados", n), w),
		kvLine("Carpeta", "", w),
		"  " + hdrLabelStyle.Render(truncateToWidth(tildePath(dir), w-2)),
		kvLine("Archivos", "report.md · report.json", w),
		"",
		hdrLabelStyle.Render(truncateToWidth("Léelo en REPORTS → Last report.", w)),
	}
	return strings.Join(lines, "\n") + "\n", nil
}

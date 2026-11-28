package tui

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/arheanja-ops/mac-toolkit/internal/core"
	"github.com/arheanja-ops/mac-toolkit/internal/reporter"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// exportReportFn copies a saved run to a folder the user chose. A package var
// so tests never write outside a temp dir.
var exportReportFn = reporter.ExportReport

// enterExport opens the export form (key e, or REPORTS → Export report): the
// folder starts at ~/Downloads and the full report is the default variant.
func (m model) enterExport() model {
	m.mode = exportMode
	m.exportInput = reporter.DefaultExportDir
	m.exportShare = false
	return m
}

// onExportKey owns the keyboard while the form is open, so typing a folder
// never fires a shortcut. Tab switches the variant, ⏎ exports, Esc cancels.
func (m model) onExportKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.mode = normalMode
		return m, nil
	case tea.KeyTab:
		m.exportShare = !m.exportShare
	case tea.KeyEnter:
		if m.running {
			return m, nil
		}
		dest, share := m.exportInput, m.exportShare
		m.mode = normalMode
		m.openView = viewNone
		return m.launch(func(width int) (string, error) { return runExportReport(width, dest, share) })
	case tea.KeyBackspace:
		if r := []rune(m.exportInput); len(r) > 0 {
			m.exportInput = string(r[:len(r)-1])
		}
	case tea.KeyRunes, tea.KeySpace:
		m.exportInput += string(msg.Runes)
	}
	return m, nil
}

// renderExportForm is the detail pane of the export form.
func renderExportForm(input string, share bool, w int) string {
	variant := "completo (con rutas y nombres)"
	if share {
		variant = "share (sin rutas ni nombres)"
	}
	lines := []string{
		tableHeaderStyle.Render(truncateToWidth("Exportar el último reporte", w)),
		"",
		truncateToWidth("Carpeta de destino:", w),
		truncateToWidth("> "+input+"_", w),
		"",
		kvLine("Variante", variant, w),
		"",
		hdrLabelStyle.Render(truncateToWidth("Se crea una carpeta nueva; nada se sobrescribe.", w)),
		hdrLabelStyle.Render(truncateToWidth("⏎ exportar · Tab completo/share · Esc cancelar", w)),
	}
	return strings.Join(lines, "\n") + "\n"
}

// runExportReport copies the newest saved report (md, json, html of the chosen
// variant) into a new folder under dest. It reads only the reports folder and
// writes only that new folder.
func runExportReport(width int, dest string, share bool) (string, error) {
	w := usableWidth(width)
	entry, ok, err := core.LatestReport()
	if err != nil {
		return "", err
	}
	if !ok {
		return noReportsMessage(w), nil
	}
	home, _ := os.UserHomeDir()
	dir, files, err := exportReportFn(entry.Dir, reporter.ExpandHome(dest, home), share)
	if err != nil {
		return "", err
	}
	title := "✓ Reporte exportado"
	if share {
		title = "✓ Reporte para compartir exportado"
	}
	lines := []string{
		lipgloss.NewStyle().Foreground(riskSafe).Bold(true).Render(title),
		"",
		kvLine("Carpeta", "", w),
		"  " + hdrLabelStyle.Render(truncateToWidth(tildePath(dir), w-2)),
		kvLine("Archivos", files2names(files), w),
	}
	return strings.Join(lines, "\n") + "\n", nil
}

// files2names lists the base names of the copied files ("report.md · …").
func files2names(files []string) string {
	names := make([]string, len(files))
	for i, f := range files {
		names[i] = filepath.Base(f)
	}
	return strings.Join(names, " · ")
}

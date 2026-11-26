package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/arheanja-ops/mac-toolkit/internal/core"
	"github.com/arheanja-ops/mac-toolkit/internal/reporter"
	"github.com/arheanja-ops/mac-toolkit/internal/tui"

	"github.com/spf13/cobra"
)

var (
	reportList  bool
	reportShare bool
	reportOpen  bool
)

var reportCmd = &cobra.Command{
	Use:   "report",
	Short: "Show the last saved report (or list, open them)",
	Long: "Show the last saved report, formatted for the terminal. Reports live in a fixed " +
		"folder, so the command works from any directory. Create one with `analyze --save`.\n\n" +
		"--share shows the share variant (no paths, file names or app names), --open opens " +
		"the html in the browser and --list lists every saved report.",
	Run: func(cmd *cobra.Command, args []string) {
		if err := runReport(reportList, reportShare, reportOpen); err != nil {
			core.Error("Reports: %v", err)
		}
	},
}

func init() {
	reportCmd.Flags().BoolVar(&reportList, "list", false, "list the saved reports")
	reportCmd.Flags().BoolVar(&reportShare, "share", false, "use the share variant (no paths or names)")
	reportCmd.Flags().BoolVar(&reportOpen, "open", false, "open the html report in the browser")
	rootCmd.AddCommand(reportCmd)
}

func runReport(list, share, open bool) error {
	entries, err := core.ListReports()
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		core.Info("Carpeta de reportes: %s", core.ReportsDir())
		core.Info("Aún no hay reportes. Crea uno con: mac-toolkit analyze --save")
		return nil
	}
	if list {
		printReportList(entries)
		return nil
	}
	last := entries[0]
	if open {
		p, err := reporter.OpenReport(last.Dir, share)
		if err != nil {
			return err
		}
		core.Info("Abierto: %s", p)
		return nil
	}
	return printReport(last, share)
}

// printReport writes the last report to stdout with the TUI renderer, so the
// terminal and the TUI show the same view. Reports saved before v1.10 only
// have markdown, which is printed as is.
func printReport(e core.ReportEntry, share bool) error {
	dir := e.Dir
	if share {
		if !e.HasShare {
			return fmt.Errorf("el último reporte no tiene variante share (es anterior a v1.10)")
		}
		dir = filepath.Join(dir, reporter.ShareDir)
	}
	if r, err := reporter.LoadJSON(filepath.Join(dir, reporter.FileJSON)); err == nil {
		fmt.Print(tui.RenderReport(r, terminalWidth()))
		core.Info("Carpeta: %s", dir)
		return nil
	}
	data, err := os.ReadFile(filepath.Join(dir, reporter.FileMarkdown))
	if err != nil {
		return fmt.Errorf("cannot read report: %w", err)
	}
	fmt.Println(string(data))
	return nil
}

func printReportList(entries []core.ReportEntry) {
	core.Info("Carpeta de reportes: %s", core.ReportsDir())
	core.Bold("Saved Reports")
	for _, e := range entries {
		formats := strings.Join(e.Formats, "·")
		if e.HasShare {
			formats += " +share"
		}
		fmt.Printf("  %s  %9s  %s\n", e.Name, core.FormatBytes(e.Size), formats)
	}
}

// terminalWidth reads $COLUMNS (set by most shells) and falls back to 80. It is
// capped so very wide terminals still get readable lines.
func terminalWidth() int {
	if n, err := strconv.Atoi(os.Getenv("COLUMNS")); err == nil && n >= 40 {
		return min(n, 100)
	}
	return 80
}

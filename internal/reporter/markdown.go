package reporter

import (
	"fmt"
	"github.com/arheanja-ops/mac-toolkit/internal/core"
	"os"
	"path/filepath"
	"time"
)

type MarkdownReporter struct {
	OutputDir string
}

func (m *MarkdownReporter) Name() string { return "markdown" }

func (m *MarkdownReporter) Report(results []core.AnalysisResult) error {
	if m.OutputDir == "" {
		m.OutputDir = fmt.Sprintf("reports/analysis_%s", time.Now().Format("20060102_150405"))
	}
	if err := os.MkdirAll(m.OutputDir, 0o755); err != nil {
		return fmt.Errorf("creating report dir: %w", err)
	}

	path := filepath.Join(m.OutputDir, "report.md")
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("creating report file: %w", err)
	}
	defer f.Close()

	fmt.Fprintf(f, "# Mac Toolkit Analysis Report\n\n")
	fmt.Fprintf(f, "Generated: %s\n\n", time.Now().Format(time.RFC3339))

	// Summary table
	fmt.Fprintf(f, "## Summary\n\n")
	fmt.Fprintf(f, "| Domain | Severity | Size | Items | Summary |\n")
	fmt.Fprintf(f, "|--------|----------|------|-------|---------|\n")

	var totalSize int64
	for _, r := range results {
		if r.Error != "" {
			fmt.Fprintf(f, "| %s | error | - | - | %s |\n", r.Domain, r.Error)
			continue
		}
		fmt.Fprintf(f, "| %s | %s | %s | %d | %s |\n",
			r.Domain, r.Severity, core.FormatBytes(r.TotalSize), len(r.Items), r.Summary)
		totalSize += r.TotalSize
	}

	fmt.Fprintf(f, "\n**Total: %s**\n", core.FormatBytes(totalSize))

	// Per-domain items (top 20 each)
	for _, r := range results {
		if len(r.Items) == 0 {
			continue
		}
		fmt.Fprintf(f, "\n## %s\n\n", r.Domain)
		fmt.Fprintf(f, "| Size | Risk | Safe | Label | Path |\n")
		fmt.Fprintf(f, "|------|------|------|-------|------|\n")
		max := 20
		if len(r.Items) < max {
			max = len(r.Items)
		}
		for _, item := range r.Items[:max] {
			safe := "✓"
			if !item.SafeToDelete {
				safe = "✗"
			}
			fmt.Fprintf(f, "| %s | %s | %s | %s | `%s` |\n",
				core.FormatBytes(item.SizeBytes), item.Risk, safe, item.Label, item.Path)
		}
		if len(r.Items) > 20 {
			fmt.Fprintf(f, "\n*...and %d more items*\n", len(r.Items)-20)
		}
	}

	core.Success("Markdown report saved: %s", path)
	return nil
}

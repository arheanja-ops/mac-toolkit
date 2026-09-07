package reporter

import (
	"encoding/json"
	"fmt"
	"github.com/arheanja-ops/mac-toolkit/internal/core"
	"os"
	"path/filepath"
	"time"
)

type JSONReporter struct {
	OutputDir string
}

func (j *JSONReporter) Name() string { return "json" }

type jsonReport struct {
	Timestamp string                `json:"timestamp"`
	Domains   []core.AnalysisResult `json:"domains"`
}

func (j *JSONReporter) Report(results []core.AnalysisResult) error {
	if j.OutputDir == "" {
		j.OutputDir = fmt.Sprintf("reports/analysis_%s", time.Now().Format("20060102_150405"))
	}
	if err := os.MkdirAll(j.OutputDir, 0o755); err != nil {
		return fmt.Errorf("creating report dir: %w", err)
	}

	report := jsonReport{
		Timestamp: time.Now().Format(time.RFC3339),
		Domains:   results,
	}

	path := filepath.Join(j.OutputDir, "report.json")
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("creating json file: %w", err)
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(report); err != nil {
		return fmt.Errorf("encoding json: %w", err)
	}

	core.Success("JSON report saved: %s", path)
	return nil
}

package reporter

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/arheanja-ops/mac-toolkit/internal/core"
)

// Report file names inside a run folder; the share variant lives in a "share"
// subfolder with the same names.
const (
	FileMarkdown = "report.md"
	FileJSON     = "report.json"
	FileHTML     = "report.html"
	ShareDir     = "share"
)

// Saved lists what SaveReport wrote.
type Saved struct {
	Dir   string   // run folder
	Full  []string // md, json, html of the full report
	Share []string // md, json, html of the share variant
}

// SaveReport writes the report in its three formats, plus the share variant, in
// a new timestamped folder under core.ReportsDir(). It is the single save path
// of `analyze --save`, `full` and the TUI. These are the only files the toolkit
// writes: nothing outside its own reports folder is touched.
func SaveReport(r Report) (Saved, error) {
	dir := core.NewReportDir(core.AnalysisPrefix)
	full, err := writeVariant(dir, r)
	if err != nil {
		return Saved{Dir: dir}, err
	}
	share, err := writeVariant(filepath.Join(dir, ShareDir), r.Share())
	if err != nil {
		return Saved{Dir: dir, Full: full}, err
	}
	return Saved{Dir: dir, Full: full, Share: share}, nil
}

// writeVariant renders one report into dir. Files are private (0600): the full
// report describes the machine; the user decides what to share.
func writeVariant(dir string, r Report) ([]string, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("creating report dir: %w", err)
	}
	js, err := r.JSON()
	if err != nil {
		return nil, err
	}
	html, err := r.HTML()
	if err != nil {
		return nil, err
	}
	files := []struct {
		name string
		data []byte
	}{{FileMarkdown, []byte(r.Markdown())}, {FileJSON, js}, {FileHTML, html}}
	var paths []string
	for _, f := range files {
		p := filepath.Join(dir, f.name)
		if err := os.WriteFile(p, f.data, 0o600); err != nil {
			return paths, fmt.Errorf("writing %s: %w", f.name, err)
		}
		paths = append(paths, p)
	}
	return paths, nil
}

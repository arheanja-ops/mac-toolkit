package core

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ReportsDirOverride redirects the reports folder. It exists only so tests never
// touch the real one; production code leaves it empty.
var ReportsDirOverride string

// Prefixes of the per-run report folders. Analysis runs hold report.md and
// report.json; cleanup runs hold audit.json.
const (
	AnalysisPrefix = "analysis_"
	CleanupPrefix  = "cleanup_"
)

// reportTimeLayout makes folder names sort chronologically.
const reportTimeLayout = "20060102_150405"

// ReportsDir is the ONE place where the toolkit stores reports and audit logs:
// <user config dir>/mac-toolkit/reports (on macOS, ~/Library/Application
// Support/mac-toolkit/reports). The path is absolute on purpose: a relative
// "reports" folder depended on the directory the terminal was launched from,
// so a report saved from one place could not be found from another.
func ReportsDir() string {
	if ReportsDirOverride != "" {
		return ReportsDirOverride
	}
	if base, err := os.UserConfigDir(); err == nil {
		return filepath.Join(base, "mac-toolkit", "reports")
	}
	// No config dir (HOME unset): fall back to a hidden folder under home, and
	// to the temp dir as a last resort, so writers always get an absolute path.
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".mac-toolkit", "reports")
	}
	return filepath.Join(os.TempDir(), "mac-toolkit", "reports")
}

// NewReportDir returns the folder path for a new run, e.g.
// <ReportsDir>/analysis_20260102_150405. It does not create it.
func NewReportDir(prefix string) string {
	return filepath.Join(ReportsDir(), prefix+time.Now().Format(reportTimeLayout))
}

// ReportEntry describes one saved analysis report.
type ReportEntry struct {
	Name    string // folder name, e.g. analysis_20260102_150405
	Path    string // absolute path of report.md
	Size    int64  // bytes of report.md
	ModTime time.Time
}

// ListReports returns the saved analysis reports, newest first. A missing
// folder is not an error (nothing saved yet); folders without a report.md
// (cleanup audits, partial runs) are skipped.
func ListReports() ([]ReportEntry, error) {
	dir := ReportsDir()
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", dir, err)
	}
	var out []ReportEntry
	for _, e := range entries {
		if !e.IsDir() || !strings.HasPrefix(e.Name(), AnalysisPrefix) {
			continue
		}
		md := filepath.Join(dir, e.Name(), "report.md")
		info, err := os.Stat(md)
		if err != nil {
			continue
		}
		out = append(out, ReportEntry{Name: e.Name(), Path: md, Size: info.Size(), ModTime: info.ModTime()})
	}
	// Names embed the timestamp, so a reverse name sort is newest-first.
	sort.Slice(out, func(i, j int) bool { return out[i].Name > out[j].Name })
	return out, nil
}

// LatestReport returns the newest saved analysis report; ok is false when none
// exists.
func LatestReport() (ReportEntry, bool, error) {
	list, err := ListReports()
	if err != nil || len(list) == 0 {
		return ReportEntry{}, false, err
	}
	return list[0], true, nil
}

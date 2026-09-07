package analyzer

import (
	"io/fs"
	"github.com/arheanja-ops/mac-toolkit/internal/core"
	"os"
	"path/filepath"
	"time"
)

// Analyzer is the interface all domain analyzers implement
type Analyzer interface {
	Domain() string
	Risk() core.RiskLevel
	Analyze() (core.AnalysisResult, error)
}

// DirSize calculates total size of all files under dir recursively.
// Skips entries that return permission errors.
func DirSize(dir string) int64 {
	var total int64
	filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip
		}
		if !d.IsDir() {
			info, err := d.Info()
			if err == nil {
				total += info.Size()
			}
		}
		return nil
	})
	return total
}

// FileAge returns the age in days based on modification time.
// Returns 0 if the file doesn't exist.
func FileAge(path string) int {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return int(time.Since(info.ModTime()).Hours() / 24)
}

// MakeResult is a convenience factory for building AnalysisResult
func MakeResult(domain string, items []core.CleanableItem, totalSize int64, summary string) core.AnalysisResult {
	return core.AnalysisResult{
		Domain:    domain,
		Severity:  core.SeverityFromSize(totalSize),
		TotalSize: totalSize,
		Items:     items,
		Summary:   summary,
		Timestamp: time.Now(),
	}
}

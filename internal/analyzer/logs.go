package analyzer

import (
	"fmt"
	"io/fs"
	"github.com/arheanja-ops/mac-toolkit/internal/core"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type LogsAnalyzer struct{}

func init() { Register(&LogsAnalyzer{}) }

func (l *LogsAnalyzer) Domain() string      { return "logs" }
func (l *LogsAnalyzer) Risk() core.RiskLevel { return core.RiskSafe }

func (l *LogsAnalyzer) Analyze() (core.AnalysisResult, error) {
	var items []core.CleanableItem
	var totalSize int64
	cutoff := time.Now().AddDate(0, 0, -7)

	for _, dir := range core.LogDirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			continue
		}
		filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			name := strings.ToLower(d.Name())
			if !strings.Contains(name, ".log") {
				return nil
			}
			info, err := d.Info()
			if err != nil {
				return nil
			}
			if info.ModTime().After(cutoff) {
				return nil // too recent
			}
			size := info.Size()
			totalSize += size
			ageDays := int(time.Since(info.ModTime()).Hours() / 24)
			items = append(items, core.CleanableItem{
				Path:         path,
				SizeBytes:    size,
				Label:        filepath.Base(path),
				Domain:       "logs",
				SafeToDelete: true,
				Reason:       fmt.Sprintf("Log file >7 days old (%d days)", ageDays),
				AgeDays:      ageDays,
				Risk:         core.RiskSafe,
			})
			return nil
		})
	}

	summary := fmt.Sprintf("Logs: %s in %d files older than 7 days", core.FormatBytes(totalSize), len(items))
	return MakeResult("logs", items, totalSize, summary), nil
}

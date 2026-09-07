package analyzer

import (
	"fmt"
	"github.com/arheanja-ops/mac-toolkit/internal/core"
	"os"
)

type DevCachesAnalyzer struct{}

func init() { Register(&DevCachesAnalyzer{}) }

func (d *DevCachesAnalyzer) Domain() string      { return "dev_caches" }
func (d *DevCachesAnalyzer) Risk() core.RiskLevel { return core.RiskSafe }

func (d *DevCachesAnalyzer) Analyze() (core.AnalysisResult, error) {
	var items []core.CleanableItem
	var totalSize int64

	for tool, cachePath := range core.DevCachePaths {
		if _, err := os.Stat(cachePath); os.IsNotExist(err) {
			continue
		}
		size := DirSize(cachePath)
		if size == 0 {
			continue
		}
		totalSize += size
		items = append(items, core.CleanableItem{
			Path:         cachePath,
			SizeBytes:    size,
			Label:        fmt.Sprintf("%s cache", tool),
			Domain:       "dev_caches",
			SafeToDelete: true,
			Reason:       "Dev cache — auto-regenerated on next install",
			Risk:         core.RiskSafe,
		})
	}

	summary := fmt.Sprintf("Dev caches: %s across %d tools", core.FormatBytes(totalSize), len(items))
	return MakeResult("dev_caches", items, totalSize, summary), nil
}

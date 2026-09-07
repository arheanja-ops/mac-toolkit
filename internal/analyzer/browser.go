package analyzer

import (
	"fmt"
	"github.com/arheanja-ops/mac-toolkit/internal/core"
	"os"
)

type BrowserAnalyzer struct{}

func init() { Register(&BrowserAnalyzer{}) }

func (b *BrowserAnalyzer) Domain() string      { return "browser" }
func (b *BrowserAnalyzer) Risk() core.RiskLevel { return core.RiskSafe }

func (b *BrowserAnalyzer) Analyze() (core.AnalysisResult, error) {
	var items []core.CleanableItem
	var totalSize int64

	for browser, cachePath := range core.BrowserCachePaths {
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
			Label:        fmt.Sprintf("%s cache", browser),
			Domain:       "browser",
			SafeToDelete: true,
			Reason:       "Browser cache — auto-regenerated",
			Risk:         core.RiskSafe,
		})
	}

	summary := fmt.Sprintf("Browser caches: %s across %d browsers", core.FormatBytes(totalSize), len(items))
	return MakeResult("browser", items, totalSize, summary), nil
}

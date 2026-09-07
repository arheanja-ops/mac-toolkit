package analyzer

import (
	"fmt"
	"github.com/arheanja-ops/mac-toolkit/internal/core"
	"os"
)

type AppSupportAnalyzer struct{}

func init() { Register(&AppSupportAnalyzer{}) }

func (a *AppSupportAnalyzer) Domain() string      { return "appsupport" }
func (a *AppSupportAnalyzer) Risk() core.RiskLevel { return core.RiskWarn }

func (a *AppSupportAnalyzer) Analyze() (core.AnalysisResult, error) {
	dir := core.AppSupportDir
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return MakeResult("appsupport", nil, 0, "Application Support not found"), nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return MakeResult("appsupport", nil, 0, fmt.Sprintf("Error reading: %v", err)), nil
	}

	var items []core.CleanableItem
	var totalSize int64
	minBytes := int64(core.DefaultMinSizeMB) * 1024 * 1024

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		fullPath := dir + "/" + entry.Name()
		size := DirSize(fullPath)
		if size < minBytes {
			continue
		}
		totalSize += size
		items = append(items, core.CleanableItem{
			Path:         fullPath,
			SizeBytes:    size,
			Label:        entry.Name(),
			Domain:       "appsupport",
			SafeToDelete: false,
			Reason:       "App Support — verify app is uninstalled",
			Risk:         core.RiskWarn,
		})
	}

	summary := fmt.Sprintf("App Support: %s in %d dirs >%dMB", core.FormatBytes(totalSize), len(items), core.DefaultMinSizeMB)
	return MakeResult("appsupport", items, totalSize, summary), nil
}

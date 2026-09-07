package analyzer

import (
	"fmt"
	"github.com/arheanja-ops/mac-toolkit/internal/core"
	"os"
)

type XcodeAnalyzer struct{}

func init() { Register(&XcodeAnalyzer{}) }

func (x *XcodeAnalyzer) Domain() string      { return "xcode" }
func (x *XcodeAnalyzer) Risk() core.RiskLevel { return core.RiskSafe }

func (x *XcodeAnalyzer) Analyze() (core.AnalysisResult, error) {
	var items []core.CleanableItem
	var totalSize int64

	xcodePaths := []struct {
		path  string
		label string
		safe  bool
		risk  core.RiskLevel
	}{
		{core.XcodeDerivedData, "DerivedData", true, core.RiskSafe},
		{core.XcodeSimulators, "Simulators", true, core.RiskSafe},
		{core.XcodeArchives, "Archives", false, core.RiskWarn},
	}

	for _, xp := range xcodePaths {
		if _, err := os.Stat(xp.path); os.IsNotExist(err) {
			continue
		}
		size := DirSize(xp.path)
		if size == 0 {
			continue
		}
		totalSize += size

		reason := "Xcode build cache — auto-regenerated"
		if !xp.safe {
			reason = "Xcode archives — verify before deleting"
		}

		items = append(items, core.CleanableItem{
			Path:         xp.path,
			SizeBytes:    size,
			Label:        fmt.Sprintf("Xcode %s", xp.label),
			Domain:       "xcode",
			SafeToDelete: xp.safe,
			Reason:       reason,
			Risk:         xp.risk,
		})
	}

	summary := fmt.Sprintf("Xcode: %s in %d locations", core.FormatBytes(totalSize), len(items))
	return MakeResult("xcode", items, totalSize, summary), nil
}

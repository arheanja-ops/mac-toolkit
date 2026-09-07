package analyzer

import (
	"fmt"
	"github.com/arheanja-ops/mac-toolkit/internal/core"
	"os"
)

type TrashAnalyzer struct{}

func init() { Register(&TrashAnalyzer{}) }

func (t *TrashAnalyzer) Domain() string      { return "trash" }
func (t *TrashAnalyzer) Risk() core.RiskLevel { return core.RiskWarn }

func (t *TrashAnalyzer) Analyze() (core.AnalysisResult, error) {
	if _, err := os.Stat(core.TrashDir); os.IsNotExist(err) {
		return MakeResult("trash", nil, 0, "Trash is empty"), nil
	}

	size := DirSize(core.TrashDir)
	if size == 0 {
		return MakeResult("trash", nil, 0, "Trash is empty"), nil
	}

	items := []core.CleanableItem{{
		Path:         core.TrashDir,
		SizeBytes:    size,
		Label:        "User Trash",
		Domain:       "trash",
		SafeToDelete: false,
		Reason:       "Trash — review contents before emptying",
		Risk:         core.RiskWarn,
	}}

	summary := fmt.Sprintf("Trash: %s", core.FormatBytes(size))
	return MakeResult("trash", items, size, summary), nil
}

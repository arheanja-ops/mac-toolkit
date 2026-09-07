package reporter

import "github.com/arheanja-ops/mac-toolkit/internal/core"

// Reporter generates output from analysis/cleanup results
type Reporter interface {
	Name() string
	Report(results []core.AnalysisResult) error
}

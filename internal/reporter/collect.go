package reporter

import (
	"os"
	"time"

	"github.com/arheanja-ops/mac-toolkit/internal/ai"
	"github.com/arheanja-ops/mac-toolkit/internal/core"
)

// Collect gathers the machine state once and builds the report from it. The
// monitors are read a single time and the same reading feeds both the report
// sections and the diagnosis, so they cannot disagree. The diagnosis is the
// heuristic one unless GROQ_API_KEY is already set (ai.Diagnose decides).
func Collect(results []core.AnalysisResult) Report {
	m := ai.CollectMetrics()
	disk := core.ReadDiskSnapshot()
	diag := ai.Diagnose(ai.SnapshotFromMetrics(results, m, disk), ai.Options{})
	home, _ := os.UserHomeDir()
	return Build(Inputs{
		Now: time.Now(), Home: home, Results: results, Metrics: m, Disk: disk,
		Diagnosis: diag, Engine: ai.EngineLabel(diag.Engine),
	})
}

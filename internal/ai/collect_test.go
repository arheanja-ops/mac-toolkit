package ai

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/arheanja-ops/mac-toolkit/internal/core"
)

// TestBuildDoctorSnapshot_UsesInjectedAnalysis proves the analysis source is
// the injected one (no filesystem scan) and that the snapshot stays PII-free.
func TestBuildDoctorSnapshot_UsesInjectedAnalysis(t *testing.T) {
	called := 0
	analyze := func() []core.AnalysisResult {
		called++
		return []core.AnalysisResult{{
			Domain: "repos", Severity: core.SeverityHigh, TotalSize: 3 * gb,
			Items: []core.CleanableItem{{
				Path: "/Users/someone/secret-repo/node_modules", Label: "secret-repo",
				SizeBytes: 3 * gb, SafeToDelete: true, Risk: core.RiskSafe,
			}},
		}}
	}
	snap := BuildDoctorSnapshot(analyze)
	if called != 1 {
		t.Fatalf("analyze called %d times, want 1", called)
	}
	raw := strings.ToLower(mustJSON(t, snap))
	for _, leak := range []string{"/users/", "secret-repo", "node_modules"} {
		if strings.Contains(raw, leak) {
			t.Fatalf("snapshot leaks %q: %s", leak, raw)
		}
	}
}

func TestEngineLabel(t *testing.T) {
	t.Setenv("GROQ_API_KEY", "")
	if got := EngineLabel(EngineHeuristic); got != "heurístico" {
		t.Fatalf("heuristic label = %q", got)
	}
	// Without a key ModelName is empty, so groq degrades to the bare label.
	if got := EngineLabel(EngineGroq); got != "groq" {
		t.Fatalf("groq label without model = %q", got)
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(b)
}

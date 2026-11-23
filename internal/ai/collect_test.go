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

// TestCopyAllowedDropsAppListsAndUnknownKeys is the privacy gate for the new
// monitor keys: the dedicated monitors also return top-app lists, which must
// never be merged into the map that feeds the snapshot.
func TestCopyAllowedDropsAppListsAndUnknownKeys(t *testing.T) {
	sys := map[string]any{}
	copyAllowed(sys, "memory", map[string]any{
		"swap_thrash": true, "pressure_code": 2,
		"top_memory": "App Alpha 1000 MB", "uptime_hours": 3.0,
	})
	copyAllowed(sys, "cpu", map[string]any{
		"user_pct": 10.0, "sys_pct": 5.0, "load5_per_core": 0.5,
		"top_cpu": "App Alpha", "thermal": "Normal",
	})
	want := map[string]bool{"swap_thrash": true, "pressure_code": true,
		"user_pct": true, "sys_pct": true, "load5_per_core": true}
	if len(sys) != len(want) {
		t.Fatalf("unexpected keys merged: %v", sys)
	}
	for k := range sys {
		if !want[k] {
			t.Fatalf("key %q must not be merged", k)
		}
	}
}

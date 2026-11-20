package ai

import (
	"encoding/json"

	"github.com/arheanja-ops/mac-toolkit/internal/core"
	"github.com/arheanja-ops/mac-toolkit/internal/monitor"
)

// BuildDoctorSnapshot is the single place that assembles the PII-free snapshot
// for CLI, MCP and TUI. The analysis source is injected because each caller has
// its own (CLI prints progress, MCP stays quiet, the TUI is stubbed in tests);
// everything else — monitors, disk, aggregation — is shared so the privacy gate
// (BuildSnapshot) cannot drift between front-ends.
func BuildDoctorSnapshot(analyze func() []core.AnalysisResult) AISnapshot {
	return BuildSnapshot(analyze(), systemMetrics(), core.ReadDiskSnapshot())
}

// systemMetrics reuses SystemMonitor for CPU/mem/swap/thermal and injects the
// hottest process as plain name+pct (no PID) sourced from ProcessMonitor. A
// degraded monitor yields a nil/partial map, which BuildSnapshot tolerates.
func systemMetrics() map[string]any {
	sys, err := (&monitor.SystemMonitor{}).Snapshot()
	if err != nil || sys == nil {
		sys = map[string]any{}
	}
	if name, pct, ok := topCPUProcess(); ok {
		sys["top_cpu_name"] = name
		sys["top_cpu_pct"] = pct
	}
	return sys
}

// topCPUProcess extracts the #1 CPU process (name + cpu%) from ProcessMonitor.
// The monitor's native slice type is unexported, so we round-trip through JSON
// rather than reimplement `ps` parsing — and we deliberately drop the PID.
func topCPUProcess() (name string, cpuPct float64, ok bool) {
	data, err := (&monitor.ProcessMonitor{}).Snapshot()
	if err != nil {
		return "", 0, false
	}
	raw, err := json.Marshal(data["top_cpu"])
	if err != nil {
		return "", 0, false
	}
	var procs []struct {
		CPU  float64 `json:"CPU"`
		Name string  `json:"Name"`
	}
	if json.Unmarshal(raw, &procs) != nil || len(procs) == 0 {
		return "", 0, false
	}
	return procs[0].Name, procs[0].CPU, true
}

// EngineLabel renders the transparency label for a Diagnosis engine using the
// configured Groq model. Shared so CLI and TUI read identically.
func EngineLabel(engine string) string {
	return EngineLabelFor(engine, ModelName())
}

// EngineLabelFor is EngineLabel with an explicit model, so a caller that has
// already resolved the model (or a test) does not depend on the environment.
func EngineLabelFor(engine, model string) string {
	if engine == EngineGroq {
		if model != "" {
			return "groq · " + model
		}
		return "groq"
	}
	return "heurístico"
}

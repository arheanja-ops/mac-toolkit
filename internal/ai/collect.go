package ai

import (
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

// Metrics is ONE reading of the three system monitors. Report v2 and the
// diagnosis both consume the same value, so a report never shows numbers that
// differ from the ones its recommendations were derived from.
type Metrics struct {
	System map[string]any
	Memory map[string]any
	CPU    map[string]any
}

// metricsFn reads the monitors; a package var so tests never touch the machine.
var metricsFn = func() Metrics {
	var m Metrics
	m.System, _ = (&monitor.SystemMonitor{}).Snapshot()
	m.Memory, _ = (&monitor.MemoryMonitor{}).Snapshot()
	m.CPU, _ = (&monitor.CPUMonitor{}).Snapshot()
	return m
}

// CollectMetrics returns one reading of the system, memory and CPU monitors. A
// failing monitor yields a nil map, which every consumer tolerates.
func CollectMetrics() Metrics { return metricsFn() }

// Merged returns the system map the snapshot builder expects: the system
// monitor's keys plus the allowlisted numeric keys of memory and CPU (see
// loadKeys: nothing with app names or paths rides along). The caller adds the
// hottest process, since its source differs per front-end.
func (m Metrics) Merged() map[string]any {
	sys := map[string]any{}
	for k, v := range m.System {
		sys[k] = v
	}
	copyAllowed(sys, "memory", m.Memory)
	copyAllowed(sys, "cpu", m.CPU)
	return sys
}

// SnapshotFromMetrics builds the PII-free snapshot from an already collected
// reading. The hottest process is the busiest app of the CPU monitor (name and
// percentage only).
func SnapshotFromMetrics(results []core.AnalysisResult, m Metrics, disk core.DiskSnapshot) AISnapshot {
	sys := m.Merged()
	if apps, _ := m.CPU["top_cpu"].([]monitor.AppUsage); len(apps) > 0 {
		sys["top_cpu_name"] = apps[0].Name
		sys["top_cpu_pct"] = apps[0].CPUPct
	}
	return BuildSnapshot(results, sys, disk)
}

// systemMetrics reuses CollectMetrics and injects the hottest process as plain
// name+pct (no PID) sourced from ProcessMonitor. A degraded monitor yields a
// nil/partial map, which BuildSnapshot tolerates.
func systemMetrics() map[string]any {
	sys := CollectMetrics().Merged()
	if name, pct, ok := topCPUProcess(); ok {
		sys["top_cpu_name"] = name
		sys["top_cpu_pct"] = pct
	}
	return sys
}

// loadKeys is the allowlist of monitor keys copied into the system map. It is
// an allowlist on purpose: the monitors also return top-app lists, and those
// must never be merged into something that gets serialized.
var loadKeys = map[string][]string{
	"memory": {"swap_thrash", "pressure_code"},
	"cpu":    {"user_pct", "sys_pct", "idle_pct", "load5_per_core"},
}

// copyAllowed copies only the allowlisted keys of one monitor's snapshot.
func copyAllowed(sys map[string]any, name string, data map[string]any) {
	for _, k := range loadKeys[name] {
		if v, ok := data[k]; ok {
			sys[k] = v
		}
	}
}

// topCPUProcess extracts the #1 CPU process (name + cpu%) from ProcessMonitor.
// It reads only those two fields of monitor.ProcInfo: the PID the local views
// show is never copied, so it cannot reach the snapshot.
func topCPUProcess() (name string, cpuPct float64, ok bool) {
	data, err := (&monitor.ProcessMonitor{}).Snapshot()
	if err != nil {
		return "", 0, false
	}
	procs, _ := data["top_cpu"].([]monitor.ProcInfo)
	if len(procs) == 0 {
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

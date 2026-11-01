// Package ai turns raw toolkit metrics into a prioritized, actionable
// diagnosis. It builds a PII-free aggregated snapshot (F3), runs it through a
// deterministic heuristic engine (F4) or an optional Groq LLM (F2), and always
// degrades gracefully to the heuristic when AI is unavailable.
//
// The package is strictly read-only: it never deletes files nor executes
// cleanup actions. Privacy is enforced at snapshot-build time, not by trusting
// a prompt — absolute paths, file names, repo names and PIDs never leave the
// process.
package ai

import (
	"github.com/arheanja-ops/mac-toolkit/internal/core"
)

// DomainSummary is the per-domain aggregate sent to the AI. It deliberately
// carries no paths, file names or item labels — only shape and magnitude.
type DomainSummary struct {
	Domain    string `json:"domain"`
	Severity  string `json:"severity"`
	SizeBytes int64  `json:"size_bytes"`
	ItemCount int    `json:"item_count"`
	TopRisk   string `json:"top_risk"`
}

// TopProcess is the single hottest CPU process, name only (no PID), used as a
// hint for thermal/performance reasoning.
type TopProcess struct {
	Name   string  `json:"name"`
	CPUPct float64 `json:"cpu_pct"`
}

// AISnapshot is the aggregated, PII-free view of the machine. Everything here
// is safe to send to a third-party model: no paths, no file names, no PIDs.
type AISnapshot struct {
	// Disk is reported as context only. It is never folded into reclaimable.
	DiskUsedPct    int   `json:"disk_used_pct"`
	DiskFreeBytes  int64 `json:"disk_free_bytes"`
	DiskTotalBytes int64 `json:"disk_total_bytes"`

	Domains []DomainSummary `json:"domains"`

	// ReclaimableSafeBytes already excludes the `disk` domain and any domain
	// that errored. It is the "safe to delete" total, not the volume usage.
	ReclaimableSafeBytes  int64 `json:"reclaimable_safe_bytes"`
	ReclaimableTotalBytes int64 `json:"reclaimable_total_bytes"`

	MemoryUsedPct float64 `json:"memory_used_pct"`
	SwapUsedBytes int64   `json:"swap_used_bytes"`

	CPUUsagePct  float64 `json:"cpu_usage_pct"`
	ThermalState string  `json:"thermal_state"`

	TopCPUProcess TopProcess `json:"top_cpu_process"`
}

// diskDomain is the analyzer domain that reports whole-volume usage. It must be
// treated as context and excluded from any "reclaimable" total — folding it in
// is the exact bug this feature exists to fix.
const diskDomain = "disk"

// BuildSnapshot assembles the PII-free aggregate from analysis results, the
// system monitor map, and a disk snapshot.
//
// Privacy (F3) is enforced here: for each domain we copy only domain, severity,
// size, item count and the highest risk seen — never item paths or labels.
func BuildSnapshot(results []core.AnalysisResult, sys map[string]any, disk core.DiskSnapshot) AISnapshot {
	snap := AISnapshot{
		DiskUsedPct:    disk.PctUsed,
		DiskFreeBytes:  disk.AvailBytes,
		DiskTotalBytes: disk.TotalBytes,
	}

	for _, r := range results {
		if r.Domain == diskDomain {
			// disk is context (already captured above), never a domain summary
			// and never counted as reclaimable.
			continue
		}

		snap.Domains = append(snap.Domains, DomainSummary{
			Domain:    r.Domain,
			Severity:  string(r.Severity),
			SizeBytes: r.TotalSize,
			ItemCount: len(r.Items),
			TopRisk:   string(topRisk(r.Items)),
		})

		// A domain that errored has no trustworthy totals — skip its bytes so a
		// partial failure never inflates the reclaimable figure.
		if r.Error != "" {
			continue
		}
		for _, item := range r.Items {
			snap.ReclaimableTotalBytes += item.SizeBytes
			if item.SafeToDelete {
				snap.ReclaimableSafeBytes += item.SizeBytes
			}
		}
	}

	applySystemMetrics(&snap, sys)
	return snap
}

// applySystemMetrics folds the system monitor's loosely-typed map into the
// snapshot. Missing or wrong-typed keys are skipped so a degraded monitor never
// breaks the diagnosis.
func applySystemMetrics(snap *AISnapshot, sys map[string]any) {
	if sys == nil {
		return
	}
	if v, ok := sys["cpu_percent"].(float64); ok {
		snap.CPUUsagePct = v
	}
	if v, ok := sys["memory_percent"].(float64); ok {
		snap.MemoryUsedPct = v
	}
	if v, ok := sys["swap_used_mb"].(float64); ok {
		// SystemMonitor reports swap in MB; the snapshot speaks bytes.
		snap.SwapUsedBytes = int64(v * 1024 * 1024)
	}
	if v, ok := sys["thermal"].(string); ok {
		snap.ThermalState = v
	}
	// Top CPU process is injected by the caller as plain name+pct (no PID),
	// because the monitor's native type is unexported.
	if v, ok := sys["top_cpu_name"].(string); ok {
		snap.TopCPUProcess.Name = v
	}
	if v, ok := sys["top_cpu_pct"].(float64); ok {
		snap.TopCPUProcess.CPUPct = v
	}
}

// topRisk returns the highest risk level present across a domain's items,
// ordered danger > warn > safe. Empty item sets report safe.
func topRisk(items []core.CleanableItem) core.RiskLevel {
	highest := core.RiskSafe
	rank := map[core.RiskLevel]int{core.RiskSafe: 0, core.RiskWarn: 1, core.RiskDanger: 2}
	for _, it := range items {
		if rank[it.Risk] > rank[highest] {
			highest = it.Risk
		}
	}
	return highest
}

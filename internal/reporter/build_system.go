package reporter

import (
	"github.com/arheanja-ops/mac-toolkit/internal/ai"
)

// Typed readers for the monitors' loosely typed maps. A missing or mistyped key
// yields the zero value, which the renderers treat as "not available".
func fnum(d map[string]any, k string) float64 { v, _ := d[k].(float64); return v }
func inum(d map[string]any, k string) int     { v, _ := d[k].(int); return v }
func str(d map[string]any, k string) string   { v, _ := d[k].(string); return v }

// buildHost reads the hardware identity. The serial number is never requested.
func buildHost(m ai.Metrics) Host {
	return Host{
		Model: str(m.System, "model"), Chip: str(m.System, "chip"),
		RAM: str(m.System, "ram"), OS: str(m.System, "os_version"),
		Cores: inum(m.CPU, "cores_total"), PerfCores: inum(m.CPU, "perf_cores"),
		EffCores: inum(m.CPU, "eff_cores"), UptimeHours: fnum(m.Memory, "uptime_hours"),
	}
}

func buildSystem(m ai.Metrics) System {
	s := System{}
	if len(m.Memory) > 0 {
		s.Memory = buildMemory(m.Memory)
	}
	if len(m.CPU) > 0 {
		s.CPU = buildCPU(m.CPU)
		s.Thermal = str(m.CPU, "thermal")
		if lim, ok := m.CPU["cpu_speed_limit"].(int); ok {
			s.SpeedLimitPct = &lim
		}
	}
	if s.Thermal == "" {
		s.Thermal = str(m.System, "thermal")
	}
	return s
}

func buildMemory(d map[string]any) *Memory {
	return &Memory{
		UsedPct: fnum(d, "used_pct"), UsedGB: fnum(d, "used_gb"), TotalGB: fnum(d, "ram_total_gb"),
		Pressure:   str(d, "pressure_level"),
		SwapUsedMB: fnum(d, "swap_used_mb"), SwapTotalMB: fnum(d, "swap_total_mb"),
		SwapInGB: fnum(d, "swapins_gb"), SwapOutGB: fnum(d, "swapouts_gb"),
		SwapInGBPerHour: fnum(d, "swapins_gb_per_h"), SwapOutGBPerHour: fnum(d, "swapouts_gb_per_h"),
		CompressedGB: fnum(d, "compressed_gb"), CompressionRatio: fnum(d, "compression_ratio"),
		PagingVerdict: str(d, "paging_verdict"),
	}
}

func buildCPU(d map[string]any) *CPU {
	return &CPU{
		UserPct: fnum(d, "user_pct"), SysPct: fnum(d, "sys_pct"), IdlePct: fnum(d, "idle_pct"),
		Load1: fnum(d, "load1"), Load5: fnum(d, "load5"), Load15: fnum(d, "load15"),
		LoadPerCore: fnum(d, "load5_per_core"), Verdict: str(d, "load_verdict"),
	}
}

// severityRank orders findings, worst first.
var severityRank = map[string]int{ai.SevHigh: 3, ai.SevMed: 2, ai.SevLow: 1}

// buildSummary derives the traffic light from the performance findings only:
// they describe what is wrong now. Quick wins and review items are
// opportunities (a Docker disk is "danger" to delete, not a sick machine), so
// they must not turn a healthy Mac red. The top findings fill up with them.
func buildSummary(rec Recommendations) Summary {
	status := StatusOK
	for _, f := range rec.Performance {
		switch f.Severity {
		case ai.SevHigh:
			status = StatusCritical
		case ai.SevMed:
			if status == StatusOK {
				status = StatusAttention
			}
		}
	}
	return Summary{Status: status, Top: topFindings(rec)}
}

// topFindings takes the worst findings first: performance (by severity), then
// quick wins, then review items; a stable sort keeps the engine's own order.
func topFindings(rec Recommendations) []Finding {
	var all []Finding
	for _, group := range [][]Finding{rec.Performance, rec.QuickWins, rec.Review} {
		g := append([]Finding(nil), group...)
		sortBySeverity(g)
		all = append(all, g...)
	}
	if len(all) > topFindingsN {
		all = all[:topFindingsN]
	}
	return all
}

func sortBySeverity(g []Finding) {
	for i := 1; i < len(g); i++ { // insertion sort: stable, tiny inputs
		for j := i; j > 0 && severityRank[g[j].Severity] > severityRank[g[j-1].Severity]; j-- {
			g[j], g[j-1] = g[j-1], g[j]
		}
	}
}

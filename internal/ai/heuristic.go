package ai

import (
	"fmt"
	"strings"

	"github.com/arheanja-ops/mac-toolkit/internal/core"
)

// Heuristic thresholds (F4). Named so the rules read like the spec table.
const (
	swapPressureBytes = 10 * 1024 * 1024 * 1024 // >10GB swap => memory pressure
	memPressurePct    = 85.0                    // >85% used  => memory pressure
	hotCPUPct         = 40.0                    // top proc >40% => thermal concern
	diskCriticalPct   = 85                      // >=85% used => critical
	diskRecommendPct  = 70                      // 70-84% used => recommended
)

// Heuristic produces a deterministic Diagnosis from the snapshot, with no
// network calls. It is the schema's source of truth and is exhaustively
// table-tested. The LLM path must conform to whatever this produces.
func Heuristic(snap AISnapshot) Diagnosis {
	d := Diagnosis{
		Engine:               EngineHeuristic,
		ReclaimableSafeBytes: snap.ReclaimableSafeBytes,
		Context: DiagnosisContext{
			DiskUsedPct:    snap.DiskUsedPct,
			DiskFreeBytes:  snap.DiskFreeBytes,
			DiskTotalBytes: snap.DiskTotalBytes,
		},
	}

	d.Performance = performanceInsights(snap)
	if disk := diskInsight(snap); disk != nil {
		// Disk pressure is a performance/urgency signal, grouped with it.
		d.Performance = append(d.Performance, *disk)
	}
	d.QuickWins = quickWinInsights(snap)
	d.Review = reviewInsights(snap)
	return d
}

// performanceInsights covers the memory and thermal rules. Both are "something
// is wrong right now" findings, independent of disk cleanup.
func performanceInsights(snap AISnapshot) []Insight {
	var out []Insight

	// Memory pressure: either sustained high usage or heavy swapping.
	if snap.SwapUsedBytes > swapPressureBytes || snap.MemoryUsedPct > memPressurePct {
		out = append(out, Insight{
			Title: "Presión de memoria",
			Detail: fmt.Sprintf("Memoria al %.0f%%, swap en uso %s.",
				snap.MemoryUsedPct, core.FormatBytes(snap.SwapUsedBytes)),
			Action:   "Cierra apps que no uses o reinicia para liberar memoria y reducir el swapping.",
			Severity: SevHigh,
		})
	}

	// Thermal: a non-nominal thermal state driven by a hot process.
	if isThermalElevated(snap.ThermalState) && snap.TopCPUProcess.CPUPct > hotCPUPct {
		out = append(out, Insight{
			Title: "Proceso caliente",
			Detail: fmt.Sprintf("Estado térmico %q con un proceso consumiendo %.0f%% de CPU.",
				snap.ThermalState, snap.TopCPUProcess.CPUPct),
			Action:   fmt.Sprintf("Revisa el proceso %q; si no lo necesitas, ciérralo para bajar la temperatura.", procLabel(snap.TopCPUProcess.Name)),
			Severity: SevMed,
		})
	}
	return out
}

// diskInsight maps disk usage bands to an urgency. Returns nil when there is no
// usable reading so we never emit a bogus "0% disk" insight.
func diskInsight(snap AISnapshot) *Insight {
	if snap.DiskTotalBytes <= 0 {
		return nil
	}
	free := core.FormatBytes(snap.DiskFreeBytes)
	switch {
	case snap.DiskUsedPct >= diskCriticalPct:
		return &Insight{
			Title:    "Disco crítico",
			Detail:   fmt.Sprintf("Disco al %d%% (%s libres).", snap.DiskUsedPct, free),
			Action:   "Libera espacio ya: aplica los quick wins y revisa los elementos grandes.",
			Severity: SevHigh,
		}
	case snap.DiskUsedPct >= diskRecommendPct:
		return &Insight{
			Title:    "Disco alto",
			Detail:   fmt.Sprintf("Disco al %d%% (%s libres).", snap.DiskUsedPct, free),
			Action:   "Recomendado liberar espacio pronto con los quick wins seguros.",
			Severity: SevMed,
		}
	default:
		return &Insight{
			Title:    "Disco con holgura",
			Detail:   fmt.Sprintf("Disco al %d%% (%s libres).", snap.DiskUsedPct, free),
			Action:   "Opcional: limpiar caches si quieres recuperar espacio.",
			Severity: SevLow,
		}
	}
}

// quickWinInsights lists domains whose highest risk is safe — these can be
// reclaimed without judgment calls.
func quickWinInsights(snap AISnapshot) []Insight {
	var out []Insight
	for _, dm := range snap.Domains {
		if dm.TopRisk != string(core.RiskSafe) || dm.SizeBytes <= 0 {
			continue
		}
		out = append(out, Insight{
			Title:    dm.Domain,
			Detail:   fmt.Sprintf("%s en %d elementos, todos marcados seguros.", core.FormatBytes(dm.SizeBytes), dm.ItemCount),
			Action:   fmt.Sprintf("Recuperable sin riesgo: toolkit clean --domain %s.", dm.Domain),
			Severity: SevLow,
		})
	}
	return out
}

// reviewInsights lists domains that contain warn/danger items. These are never
// auto-actioned — the user must decide.
func reviewInsights(snap AISnapshot) []Insight {
	var out []Insight
	for _, dm := range snap.Domains {
		if dm.TopRisk != string(core.RiskWarn) && dm.TopRisk != string(core.RiskDanger) {
			continue
		}
		out = append(out, Insight{
			Title:    dm.Domain,
			Detail:   fmt.Sprintf("%s en %d elementos; riesgo máximo %q.", core.FormatBytes(dm.SizeBytes), dm.ItemCount, dm.TopRisk),
			Action:   "Requiere revisión manual: verifica antes de borrar, puede incluir datos útiles.",
			Severity: reviewSeverity(dm.TopRisk),
		})
	}
	return out
}

// isThermalElevated reports whether the thermal state is anything other than a
// nominal/OK reading. SystemMonitor emits "Normal"/"Warning"; we also tolerate
// "nominal"/"OK" so the rule matches the spec wording regardless of source.
func isThermalElevated(state string) bool {
	switch strings.ToLower(strings.TrimSpace(state)) {
	case "", "normal", "nominal", "ok":
		return false
	default:
		return true
	}
}

// reviewSeverity maps a domain's top risk to an insight severity.
func reviewSeverity(topRisk string) string {
	if topRisk == string(core.RiskDanger) {
		return SevHigh
	}
	return SevMed
}

// procLabel guards against an empty process name so the action text stays
// readable even when the monitor could not resolve it.
func procLabel(name string) string {
	if strings.TrimSpace(name) == "" {
		return "más activo"
	}
	return name
}

package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/arheanja-ops/mac-toolkit/internal/ai"
	"github.com/arheanja-ops/mac-toolkit/internal/core"
	"github.com/arheanja-ops/mac-toolkit/internal/monitor"

	"github.com/spf13/cobra"
)

var (
	doctorNoAI bool
	doctorJSON bool
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Diagnose system health and produce a prioritized, read-only action plan",
	Long: `doctor runs every analyzer plus the system monitors, builds a PII-free
aggregate, and turns it into a prioritized diagnosis (performance, quick wins,
items needing review). It uses Groq when GROQ_API_KEY is set, otherwise a
deterministic heuristic engine. It is strictly read-only — it never deletes.`,
	Run: func(cmd *cobra.Command, args []string) {
		snap := buildDoctorSnapshot()
		diag := ai.Diagnose(snap, ai.Options{NoAI: doctorNoAI})

		if doctorJSON {
			emitDoctorJSON(diag)
			return
		}
		renderDiagnosis(diag)
		// Confirm completion of the read-only diagnosis (human path only; the
		// JSON path stays pristine for piping).
		core.Success("Listo (doctor)")
	},
}

func init() {
	doctorCmd.Flags().BoolVar(&doctorNoAI, "no-ai", false, "force the deterministic heuristic engine (skip Groq)")
	doctorCmd.Flags().BoolVar(&doctorJSON, "json", false, "emit the Diagnosis as JSON (no colors) for scripting")
	rootCmd.AddCommand(doctorCmd)
}

// buildDoctorSnapshot gathers metrics by reusing the existing analysis,
// monitors and disk reader — it never reimplements collection logic.
func buildDoctorSnapshot() ai.AISnapshot {
	results := runAnalysis("")

	sys := systemMetrics()
	disk := core.ReadDiskSnapshot()

	return ai.BuildSnapshot(results, sys, disk)
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

// emitDoctorJSON writes the Diagnosis to stdout (so it can be piped), while all
// other command chatter goes to stderr like the rest of the CLI.
func emitDoctorJSON(diag ai.Diagnosis) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(diag); err != nil {
		core.Error("doctor: encode json: %v", err)
	}
}

// renderDiagnosis prints the grouped, colored report (F5) to stderr, reusing
// core's color/format helpers.
func renderDiagnosis(diag ai.Diagnosis) {
	fmt.Fprintln(os.Stderr)
	core.Bold("🩺 Diagnóstico  (motor: %s)", engineLabel(diag.Engine))

	renderGroup("⚡ PERFORMANCE", diag.Performance, "")
	renderGroup("🧹 QUICK WINS", diag.QuickWins,
		fmt.Sprintf("(%s recuperables sin riesgo)", core.FormatBytes(diag.ReclaimableSafeBytes)))
	renderGroup("⚠ REQUIERE REVISIÓN", diag.Review, "")

	renderDiskContext(diag.Context)
}

// renderGroup prints one priority section. Empty groups are skipped so the
// output stays focused on what actually matters.
func renderGroup(heading string, insights []ai.Insight, suffix string) {
	if len(insights) == 0 {
		return
	}
	fmt.Fprintln(os.Stderr)
	if suffix != "" {
		heading = heading + "  " + suffix
	}
	fmt.Fprintf(os.Stderr, "  %s\n", core.Colorize(core.Bold_, heading))
	for _, in := range insights {
		color := severityColor(in.Severity)
		fmt.Fprintf(os.Stderr, "     • %s — %s\n",
			core.Colorize(color, in.Title), in.Detail)
		if in.Action != "" {
			fmt.Fprintf(os.Stderr, "       %s\n", core.Colorize(core.Gray, in.Action))
		}
	}
}

// renderDiskContext prints the disk line (F5). It reports usage as context,
// never as reclaimable space.
func renderDiskContext(ctx ai.DiagnosisContext) {
	if ctx.DiskTotalBytes <= 0 {
		return
	}
	fmt.Fprintln(os.Stderr)
	bar := core.ProgressBar(ctx.DiskUsedPct, 20)
	fmt.Fprintf(os.Stderr, "  Contexto: disco al %d%% %s (%s libres de %s)\n",
		ctx.DiskUsedPct, bar,
		core.FormatBytes(ctx.DiskFreeBytes), core.FormatBytes(ctx.DiskTotalBytes))
}

// engineLabel renders the transparency label: the Groq model when AI produced
// the result, or a plain "heurístico".
func engineLabel(engine string) string {
	if engine == ai.EngineGroq {
		if model := ai.ModelName(); model != "" {
			return "groq · " + model
		}
		return "groq"
	}
	return "heurístico"
}

// severityColor maps an insight severity to an ANSI color from core.
func severityColor(sev string) string {
	switch sev {
	case ai.SevHigh:
		return core.SeverityColor(core.SeverityCritical)
	case ai.SevMed:
		return core.SeverityColor(core.SeverityMedium)
	default:
		return core.SeverityColor(core.SeverityLow)
	}
}

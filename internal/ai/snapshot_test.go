package ai

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/arheanja-ops/mac-toolkit/internal/core"
)

// TestBuildSnapshot_NoPII is the privacy guarantee (F3): given results whose
// items carry real paths/labels, the serialized AISnapshot must not leak any
// path, file name, repo name or PID.
func TestBuildSnapshot_NoPII(t *testing.T) {
	results := []core.AnalysisResult{
		{
			Domain:    "repos",
			Severity:  core.SeverityHigh,
			TotalSize: 5 * gb,
			Items: []core.CleanableItem{
				{
					Path:         "/Users/someone/secret-repo/node_modules",
					Label:        "node_modules (secret-repo)",
					SizeBytes:    5 * gb,
					SafeToDelete: true,
					Risk:         core.RiskSafe,
				},
			},
		},
		{
			Domain:    "downloads",
			Severity:  core.SeverityMedium,
			TotalSize: 2 * gb,
			Items: []core.CleanableItem{
				{Path: "/Users/someone/Downloads/private-tax-return.pdf", Label: "tax", SizeBytes: 2 * gb, Risk: core.RiskWarn},
			},
		},
	}
	sys := map[string]any{"top_cpu_name": "WindowServer", "top_cpu_pct": 12.0}
	disk := core.DiskSnapshot{TotalBytes: 500 * gb, AvailBytes: 100 * gb, PctUsed: 80, OK: true}

	snap := BuildSnapshot(results, sys, disk)

	raw, err := json.Marshal(snap)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	blob := string(raw)

	forbidden := []string{
		"/Users/someone",
		"secret-repo",
		"node_modules",
		"private-tax-return.pdf",
		"Downloads",
		".pdf",
	}
	for _, f := range forbidden {
		if strings.Contains(blob, f) {
			t.Fatalf("snapshot leaked PII %q in: %s", f, blob)
		}
	}
}

// TestBuildSnapshot_DiskIsContextNotReclaimable verifies the core bug fix: the
// `disk` domain must be context only, never summed into reclaimable, and never
// appear as a domain summary.
func TestBuildSnapshot_DiskIsContextNotReclaimable(t *testing.T) {
	results := []core.AnalysisResult{
		{
			Domain:    "disk",
			TotalSize: 346 * gb,
			Items: []core.CleanableItem{
				{Path: "/", Label: "volume", SizeBytes: 346 * gb, SafeToDelete: true, Risk: core.RiskSafe},
			},
		},
		{
			Domain:    "browser",
			TotalSize: 2 * gb,
			Items: []core.CleanableItem{
				{Path: "/x", Label: "cache", SizeBytes: 2 * gb, SafeToDelete: true, Risk: core.RiskSafe},
			},
		},
	}
	disk := core.DiskSnapshot{TotalBytes: 500 * gb, AvailBytes: 154 * gb, PctUsed: 69}

	snap := BuildSnapshot(results, nil, disk)

	if snap.ReclaimableSafeBytes != 2*gb {
		t.Fatalf("reclaimable = %d, want %d (disk must be excluded)", snap.ReclaimableSafeBytes, 2*gb)
	}
	for _, d := range snap.Domains {
		if d.Domain == "disk" {
			t.Fatalf("disk must not appear as a domain summary")
		}
	}
	if snap.DiskUsedPct != 69 || snap.DiskTotalBytes != 500*gb {
		t.Fatalf("disk context not carried through: %+v", snap)
	}
}

// TestBuildSnapshot_ErroredDomainExcludedFromReclaimable ensures a partial
// failure never inflates the reclaimable total.
func TestBuildSnapshot_ErroredDomainExcludedFromReclaimable(t *testing.T) {
	results := []core.AnalysisResult{
		{
			Domain:    "xcode",
			TotalSize: 8 * gb,
			Error:     "scan timed out",
			Items: []core.CleanableItem{
				{Path: "/x", SizeBytes: 8 * gb, SafeToDelete: true, Risk: core.RiskSafe},
			},
		},
		{
			Domain:    "logs",
			TotalSize: 1 * gb,
			Items: []core.CleanableItem{
				{Path: "/y", SizeBytes: 1 * gb, SafeToDelete: true, Risk: core.RiskSafe},
			},
		},
	}
	snap := BuildSnapshot(results, nil, core.DiskSnapshot{})

	if snap.ReclaimableSafeBytes != 1*gb {
		t.Fatalf("reclaimable = %d, want %d (errored domain excluded)", snap.ReclaimableSafeBytes, 1*gb)
	}
	// The errored domain is still summarized (for visibility), just not counted.
	if !hasDomain(snap.Domains, "xcode") {
		t.Fatalf("errored domain should still be summarized")
	}
}

func TestBuildSnapshot_TopRisk(t *testing.T) {
	results := []core.AnalysisResult{{
		Domain: "mixed",
		Items: []core.CleanableItem{
			{Risk: core.RiskSafe}, {Risk: core.RiskDanger}, {Risk: core.RiskWarn},
		},
	}}
	snap := BuildSnapshot(results, nil, core.DiskSnapshot{})
	if snap.Domains[0].TopRisk != string(core.RiskDanger) {
		t.Fatalf("top risk = %q, want danger", snap.Domains[0].TopRisk)
	}
}

func hasDomain(domains []DomainSummary, name string) bool {
	for _, d := range domains {
		if d.Domain == name {
			return true
		}
	}
	return false
}

// TestBuildSnapshot_LoadFieldsAreNumbersOnly extends the privacy gate to the
// load/paging fields: they must be filled from the monitor keys, serialize
// under their documented names, and be numeric or boolean (a string field could
// carry a name or a path).
func TestBuildSnapshot_LoadFieldsAreNumbersOnly(t *testing.T) {
	sys := map[string]any{
		"load5_per_core": 2.5, "user_pct": 20.0, "sys_pct": 55.0,
		"swap_thrash": true, "pressure_code": 4,
		// Keys that must be ignored even if a caller passes them by mistake.
		"top_memory": "/Applications/Alpha.app", "top_cpu": "Alpha",
	}
	snap := BuildSnapshot(nil, sys, core.DiskSnapshot{})
	if snap.LoadPerCore != 2.5 || snap.CPUUserPct != 20 || snap.CPUSysPct != 55 ||
		!snap.SwapThrash || snap.MemoryPressureLevel != 4 {
		t.Fatalf("load fields not applied: %+v", snap)
	}
	blob := mustJSON(t, snap)
	for _, key := range []string{"load_per_core", "cpu_user_pct", "cpu_sys_pct", "swap_thrash", "memory_pressure_level"} {
		if !strings.Contains(blob, `"`+key+`"`) {
			t.Fatalf("snapshot missing %q: %s", key, blob)
		}
	}
	for _, leak := range []string{"Alpha", "/Applications", ".app"} {
		if strings.Contains(blob, leak) {
			t.Fatalf("snapshot leaked %q: %s", leak, blob)
		}
	}
	rt := reflect.TypeOf(snap)
	for _, name := range []string{"LoadPerCore", "CPUUserPct", "CPUSysPct", "SwapThrash", "MemoryPressureLevel"} {
		f, _ := rt.FieldByName(name)
		switch f.Type.Kind() {
		case reflect.Float64, reflect.Int, reflect.Bool:
		default:
			t.Fatalf("%s must be numeric or bool, got %s", name, f.Type.Kind())
		}
	}
}

// Wrong-typed or missing monitor keys leave the zero value (healthy/unknown).
func TestBuildSnapshot_LoadFieldsToleratesBadTypes(t *testing.T) {
	snap := BuildSnapshot(nil, map[string]any{"load5_per_core": "x", "swap_thrash": 1}, core.DiskSnapshot{})
	if snap.LoadPerCore != 0 || snap.SwapThrash {
		t.Fatalf("bad types must be ignored: %+v", snap)
	}
}

// The idle share is a plain number; it feeds the "high load, free CPU" insight
// without adding any identifying field to the snapshot.
func TestBuildSnapshot_IdleShareIsNumberOnly(t *testing.T) {
	sys := map[string]any{"load5_per_core": 4.7, "user_pct": 20.0, "sys_pct": 13.5, "idle_pct": 66.5,
		"top_cpu": []string{"alpha"}, "pid": 1234}
	snap := BuildSnapshot(nil, sys, core.DiskSnapshot{})
	if snap.CPUIdlePct != 66.5 {
		t.Fatalf("idle not applied: %+v", snap)
	}
	blob := mustJSON(t, snap)
	if !strings.Contains(blob, `"cpu_idle_pct":66.5`) || strings.Contains(blob, "alpha") || strings.Contains(blob, "1234") {
		t.Fatalf("unexpected snapshot: %s", blob)
	}
	if got := idleOf(AISnapshot{}); got != -1 {
		t.Fatalf("no split read must mean unknown (-1), got %v", got)
	}
}

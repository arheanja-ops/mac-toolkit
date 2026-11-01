package ai

import (
	"testing"
)

const gb = 1024 * 1024 * 1024

// hasTitle reports whether any insight in the group carries the given title.
func hasTitle(insights []Insight, title string) bool {
	for _, in := range insights {
		if in.Title == title {
			return true
		}
	}
	return false
}

func TestHeuristic_MemoryPressure(t *testing.T) {
	cases := []struct {
		name string
		snap AISnapshot
		want bool
	}{
		{"high swap", AISnapshot{SwapUsedBytes: 12 * gb}, true},
		{"high mem pct", AISnapshot{MemoryUsedPct: 90}, true},
		{"calm", AISnapshot{SwapUsedBytes: 1 * gb, MemoryUsedPct: 40}, false},
		{"swap boundary (10GB, not >)", AISnapshot{SwapUsedBytes: 10 * gb}, false},
		{"mem boundary (85, not >)", AISnapshot{MemoryUsedPct: 85}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := Heuristic(tc.snap)
			got := hasTitle(d.Performance, "Presión de memoria")
			if got != tc.want {
				t.Fatalf("memory pressure = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestHeuristic_Thermal(t *testing.T) {
	cases := []struct {
		name    string
		thermal string
		cpu     float64
		want    bool
	}{
		{"warning + hot cpu", "Warning", 55, true},
		{"warning + cool cpu", "Warning", 10, false},
		{"normal + hot cpu", "Normal", 90, false},
		{"nominal + hot cpu", "nominal", 90, false},
		{"empty + hot cpu", "", 90, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			snap := AISnapshot{
				ThermalState:  tc.thermal,
				TopCPUProcess: TopProcess{Name: "proc", CPUPct: tc.cpu},
			}
			d := Heuristic(snap)
			got := hasTitle(d.Performance, "Proceso caliente")
			if got != tc.want {
				t.Fatalf("thermal insight = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestHeuristic_DiskBands(t *testing.T) {
	cases := []struct {
		name  string
		pct   int
		title string
	}{
		{"critical at 85", 85, "Disco crítico"},
		{"critical at 95", 95, "Disco crítico"},
		{"recommend at 70", 70, "Disco alto"},
		{"recommend at 84", 84, "Disco alto"},
		{"optional at 69", 69, "Disco con holgura"},
		{"optional at 10", 10, "Disco con holgura"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			snap := AISnapshot{
				DiskUsedPct:    tc.pct,
				DiskTotalBytes: 500 * gb,
				DiskFreeBytes:  100 * gb,
			}
			d := Heuristic(snap)
			if !hasTitle(d.Performance, tc.title) {
				t.Fatalf("expected disk insight %q for %d%%, got %+v", tc.title, tc.pct, d.Performance)
			}
		})
	}
}

func TestHeuristic_NoDiskReadingSkipsInsight(t *testing.T) {
	// A zero total means df failed; we must not emit a bogus "0% disk" insight.
	d := Heuristic(AISnapshot{DiskUsedPct: 0, DiskTotalBytes: 0})
	for _, in := range d.Performance {
		if in.Title == "Disco crítico" || in.Title == "Disco alto" || in.Title == "Disco con holgura" {
			t.Fatalf("expected no disk insight when total is 0, got %q", in.Title)
		}
	}
}

func TestHeuristic_QuickWinsAndReview(t *testing.T) {
	snap := AISnapshot{
		DiskTotalBytes:       500 * gb,
		ReclaimableSafeBytes: 3 * gb,
		Domains: []DomainSummary{
			{Domain: "browser", SizeBytes: 2 * gb, ItemCount: 5, TopRisk: "safe"},
			{Domain: "dev_caches", SizeBytes: 1 * gb, ItemCount: 3, TopRisk: "safe"},
			{Domain: "downloads", SizeBytes: 4 * gb, ItemCount: 2, TopRisk: "warn"},
			{Domain: "repos", SizeBytes: 9 * gb, ItemCount: 1, TopRisk: "danger"},
			{Domain: "empty_safe", SizeBytes: 0, ItemCount: 0, TopRisk: "safe"},
		},
	}
	d := Heuristic(snap)

	if !hasTitle(d.QuickWins, "browser") || !hasTitle(d.QuickWins, "dev_caches") {
		t.Fatalf("expected safe domains in quick wins, got %+v", d.QuickWins)
	}
	// A zero-byte safe domain is not worth surfacing as a quick win.
	if hasTitle(d.QuickWins, "empty_safe") {
		t.Fatalf("zero-byte domain should not be a quick win")
	}
	if !hasTitle(d.Review, "downloads") || !hasTitle(d.Review, "repos") {
		t.Fatalf("expected warn/danger domains in review, got %+v", d.Review)
	}
	// Quick wins must never contain warn/danger domains.
	if hasTitle(d.QuickWins, "downloads") || hasTitle(d.QuickWins, "repos") {
		t.Fatalf("warn/danger domain leaked into quick wins")
	}
	if d.ReclaimableSafeBytes != 3*gb {
		t.Fatalf("reclaimable = %d, want %d", d.ReclaimableSafeBytes, 3*gb)
	}
	if d.Engine != EngineHeuristic {
		t.Fatalf("engine = %q, want %q", d.Engine, EngineHeuristic)
	}
}

func TestHeuristic_DangerReviewSeverityIsHigh(t *testing.T) {
	snap := AISnapshot{
		DiskTotalBytes: 500 * gb,
		Domains:        []DomainSummary{{Domain: "repos", SizeBytes: 5 * gb, ItemCount: 1, TopRisk: "danger"}},
	}
	d := Heuristic(snap)
	if len(d.Review) != 1 || d.Review[0].Severity != SevHigh {
		t.Fatalf("danger domain should map to high severity review, got %+v", d.Review)
	}
}

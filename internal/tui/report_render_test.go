package tui

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/arheanja-ops/mac-toolkit/internal/ai"
	"github.com/arheanja-ops/mac-toolkit/internal/core"
	"github.com/arheanja-ops/mac-toolkit/internal/monitor"
	"github.com/arheanja-ops/mac-toolkit/internal/reporter"

	tea "github.com/charmbracelet/bubbletea"
)

const gib = int64(1) << 30

// sampleReport is a synthetic report (generic names, fake home).
func sampleReport() reporter.Report {
	return reporter.Build(reporter.Inputs{
		Now:  time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
		Home: "/Users/someone",
		Results: []core.AnalysisResult{
			{Domain: "disk", Severity: core.SeverityHigh, TotalSize: 400 * gib},
			{Domain: "dev_caches_with_a_long_name", Severity: core.SeverityMedium, TotalSize: 3 * gib,
				Items: []core.CleanableItem{{Path: "/Users/someone/Library/Caches/tool", Label: "a very long cache label that must be truncated in narrow panes", SizeBytes: 3 * gib, SafeToDelete: true, Risk: core.RiskSafe}}},
			{Domain: "docker", Severity: core.SeverityCritical, TotalSize: 20 * gib,
				Items: []core.CleanableItem{{Path: "/Users/someone/Docker.raw", Label: "Docker.raw", SizeBytes: 20 * gib, Risk: core.RiskDanger}}},
			{Domain: "ollama", Error: "timeout"},
		},
		Metrics: ai.Metrics{
			System: map[string]any{"model": "MacBook Pro", "chip": "Apple M2", "ram": "16 GB"},
			Memory: map[string]any{"used_pct": 88.0, "used_gb": 14.0, "ram_total_gb": 16.0, "pressure_level": "warn",
				"swap_used_mb": 512.0, "swap_total_mb": 1024.0,
				"paging_verdict": "paginación moderada durante las últimas horas de uso intenso",
				"top_memory":     []monitor.AppUsage{{Name: "Editor with a very long application name", MemMB: 2048, Procs: 12}}},
			CPU: map[string]any{"user_pct": 30.0, "sys_pct": 10.0, "load1": 3.0, "load5": 2.5, "load15": 2.0,
				"load5_per_core": 0.25, "thermal": "Normal", "top_cpu": []monitor.AppUsage{{Name: "Builder", CPUPct: 80, Procs: 2}}},
		},
		Disk: core.DiskSnapshot{TotalBytes: 500 * gib, UsedBytes: 400 * gib, AvailBytes: 100 * gib, PctUsed: 80, OK: true},
		Diagnosis: ai.Diagnosis{
			Performance: []ai.Insight{{Title: "Memoria alta con presión sostenida en el equipo", Action: "cierra apps que no uses y revisa las pestañas abiertas", Severity: ai.SevHigh}},
			QuickWins:   []ai.Insight{{Title: "Cachés de desarrollo", Action: "limpia con Select & clean", Severity: ai.SevLow}},
		},
		Engine: "heurístico",
	})
}

func TestRenderReportFitsWidthsAndShowsSections(t *testing.T) {
	for _, r := range []reporter.Report{sampleReport(), sampleReport().Share()} {
		for _, w := range []int{40, 50, 60, 80, 100} {
			out := RenderReport(r, w)
			assertLinesFit(t, "render", out, w)
			assertNoStructLeak(t, "render", out)
			for _, want := range []string{"RESUMEN", "DISCO", "Recuperable", "Crítico"} {
				if !strings.Contains(out, want) {
					t.Fatalf("%s@%d missing %q:\n%s", r.Variant, w, want, out)
				}
			}
		}
	}
	out := RenderReport(sampleReport(), 80)
	// Total recuperable = 23 GB; the 400 GB volume is context only.
	if !strings.Contains(out, core.FormatBytes(23*gib)) || !strings.Contains(out, "contexto") {
		t.Fatalf("reclaimable total wrong:\n%s", out)
	}
	if strings.Index(out, "docker") > strings.Index(out, "dev_caches") {
		t.Fatal("domains must be largest first")
	}
	share := RenderReport(sampleReport().Share(), 80)
	for _, bad := range []string{"Docker.raw", "Builder", "someone", "cache label"} {
		if strings.Contains(share, bad) {
			t.Fatalf("share view leaks %q", bad)
		}
	}
}

// saveSample writes a full v1.10 report into the (test) reports folder.
func saveSample(t *testing.T) reporter.Saved {
	t.Helper()
	saved, err := reporter.SaveReport(sampleReport())
	if err != nil {
		t.Fatal(err)
	}
	return saved
}

func TestLastReportIsStructuredFromJSON(t *testing.T) {
	withReportsDir(t, t.TempDir())
	saveSample(t)
	out, err := runLastReport(80)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"RESUMEN", "PROCESOS", "o: abrir el html"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "|") || strings.Contains(out, "##") {
		t.Fatalf("structured view must not show raw markdown:\n%s", out)
	}
}

func TestSavedReportsShowsFormatsAndShare(t *testing.T) {
	withReportsDir(t, t.TempDir())
	saveSample(t)
	writeReport(t, "analysis_20200101_000000", "# Legacy\n")
	out, err := runSavedReports(60)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "md·json·html +share") || !strings.Contains(out, "FORMATOS") {
		t.Fatalf("formats missing:\n%s", out)
	}
	assertLinesFit(t, "saved", out, 60)
}

func TestOpenReportUsesInjectedOpener(t *testing.T) {
	withReportsDir(t, t.TempDir())
	saved := saveSample(t)
	var got []string
	orig := openReportFn
	openReportFn = func(dir string, share bool) (string, error) {
		got = append(got, dir)
		return filepath.Join(dir, reporter.FileHTML), nil
	}
	t.Cleanup(func() { openReportFn = orig })
	out, err := runOpenReport(60)
	if err != nil || len(got) != 1 || got[0] != saved.Dir {
		t.Fatalf("open: err=%v calls=%v", err, got)
	}
	if !strings.Contains(out, "Abriendo") {
		t.Fatalf("unexpected output:\n%s", out)
	}
}

func TestOpenReportWithoutHTMLDoesNotOpen(t *testing.T) {
	withReportsDir(t, t.TempDir())
	writeReport(t, "analysis_20200101_000000", "# Legacy\n")
	orig := openReportFn
	openReportFn = func(string, bool) (string, error) { t.Fatal("must not open"); return "", nil }
	t.Cleanup(func() { openReportFn = orig })
	if _, err := runOpenReport(60); err != nil {
		t.Fatal(err)
	}
}

func TestKeyOOpensReportView(t *testing.T) {
	withReportsDir(t, t.TempDir())
	m := newModel("v-test")
	nm, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = nm.(model)
	nm, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("o")})
	if nm.(model).openView != viewOpenReport || cmd == nil {
		t.Fatalf("o must open the Open report view, got %v", nm.(model).openView)
	}
}

func TestLastReportFrameFitsEightyByTwentyFour(t *testing.T) {
	withReportsDir(t, t.TempDir())
	saveSample(t)
	m := newModel("v-test")
	nm, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = nm.(model)
	for _, v := range []viewID{viewLastReport, viewSavedReports} {
		maxW, lines := widestLine(ranViewFrame(t, m, v))
		if maxW > 80 || lines > 24 {
			t.Fatalf("view %d frame is %dx%d (max 80x24)", v, maxW, lines)
		}
	}
}

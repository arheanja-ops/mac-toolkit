package tui

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/arheanja-ops/mac-toolkit/internal/core"
	"github.com/arheanja-ops/mac-toolkit/internal/monitor"
	"github.com/arheanja-ops/mac-toolkit/internal/reporter"

	tea "github.com/charmbracelet/bubbletea"
)

// TestMain keeps the suite off the real machine: newModel() reads the system
// monitor for the header, which spawns system_profiler/top and made every test
// that builds a model take seconds.
func TestMain(m *testing.M) {
	systemSnapshotFn = func() (map[string]any, error) {
		return map[string]any{"memory_percent": 50.0, "cpu_percent": 10.0, "thermal": "Normal"}, nil
	}
	// Reports never touch the real folder: every test reads/writes a temp dir.
	dir, err := os.MkdirTemp("", "toolkit-reports-")
	if err != nil {
		panic(err)
	}
	core.ReportsDirOverride = dir
	// No test may open a file on the machine: the system opener is disabled
	// (tests that exercise Open report inject their own fake on top).
	reporter.OpenFn = func(string) error { return errors.New("opener disabled in tests") }
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

// sampleMemory is a synthetic, generic snapshot (no real app names).
func sampleMemory() map[string]any {
	return map[string]any{
		"ram_total_gb": 16.0, "used_gb": 11.5, "used_pct": 72.0,
		"app_gb": 5.0, "wired_gb": 2.1, "compressed_gb": 4.4, "file_cache_gb": 3.0,
		"active_gb": 4.0, "inactive_gb": 3.5, "free_gb": 0.8,
		"pressure_level": monitor.PressureWarning, "pressure_code": 2, "free_pct": 18,
		"swap_total_mb": 4096.0, "swap_used_mb": 3700.0,
		"swapins_gb": 12.5, "swapouts_gb": 40.0, "swap_thrash": true, "uptime_hours": 130.0,
		"top_memory": []monitor.AppUsage{
			{Name: "Alpha", MemMB: 3000, Procs: 40},
			{Name: "Beta with an extremely long application name that must be cut", MemMB: 1200, Procs: 3},
			{Name: "Gamma", MemMB: 800, Procs: 1}, {Name: "Delta", MemMB: 500, Procs: 2},
			{Name: "Epsilon", MemMB: 300, Procs: 1},
		},
	}
}

func sampleCPU() map[string]any {
	return map[string]any{
		"user_pct": 20.0, "sys_pct": 55.0, "idle_pct": 25.0, "kernel_high": true,
		"load1": 30.5, "load5": 28.0, "load15": 20.1, "load_per_core": 3.05,
		"load5_per_core": 2.8, "load_verdict": monitor.LoadSaturated,
		"cores_total": 10, "perf_cores": 6, "eff_cores": 4, "thermal": "Throttled",
		"top_cpu": []monitor.AppUsage{
			{Name: "Alpha", CPUPct: 90.5, Procs: 12}, {Name: "Beta", CPUPct: 40, Procs: 1},
			{Name: "Gamma", CPUPct: 10, Procs: 1}, {Name: "Delta", CPUPct: 5, Procs: 2},
			{Name: "Epsilon", CPUPct: 1, Procs: 1},
		},
	}
}

func withViews(fn func()) {
	om, oc := memoryFn, cpuFn
	memoryFn = func() (map[string]any, error) { return sampleMemory(), nil }
	cpuFn = func() (map[string]any, error) { return sampleCPU(), nil }
	defer func() { memoryFn, cpuFn = om, oc }()
	fn()
}

func assertFits(t *testing.T, s string, maxW int) {
	t.Helper()
	if w, _ := widestLine(s); w > maxW {
		t.Fatalf("line wider than %d (got %d):\n%s", maxW, w, s)
	}
}

func TestFormatMemoryContentAndWidths(t *testing.T) {
	for _, width := range []int{30, 40, 53, 80, 120} {
		out := formatMemory(sampleMemory(), width)
		assertFits(t, out, usableWidth(width))
	}
	out := formatMemory(sampleMemory(), 53)
	for _, want := range []string{"RAM", "72%", "Presión", "Warning", "18% libre", "Wired",
		"Swap", "in 12.5 GB", "up 5 d 10 h", "Paginación intensa", "Top 5 apps por RAM", "Alpha"} {
		if !strings.Contains(out, want) {
			t.Fatalf("memory view missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "/") && strings.Contains(out, ".app") {
		t.Fatalf("view must not show paths:\n%s", out)
	}
}

func TestFormatMemoryDegradesWithoutOptionalData(t *testing.T) {
	out := formatMemory(map[string]any{"used_pct": 10.0, "used_gb": 1.0, "ram_total_gb": 16.0}, 53)
	for _, want := range []string{"Unknown", "sin swap", "(sin datos)"} {
		if !strings.Contains(out, want) {
			t.Fatalf("degraded view missing %q:\n%s", want, out)
		}
	}
	if !strings.Contains(formatMemory(nil, 53), "no data available") {
		t.Fatal("empty snapshot should say so")
	}
}

func TestFormatCPUContentAndWidths(t *testing.T) {
	for _, width := range []int{30, 40, 53, 80, 120} {
		assertFits(t, formatCPU(sampleCPU(), width), usableWidth(width))
	}
	out := formatCPU(sampleCPU(), 53)
	for _, want := range []string{"user", "sys", "idle", "Carga", "30.50", "Saturada",
		"6 P + 4 E", "Throttled", "Tiempo de kernel elevado", "Puede deberse", "Top 5 apps por CPU"} {
		if !strings.Contains(out, want) {
			t.Fatalf("cpu view missing %q:\n%s", want, out)
		}
	}
}

func TestFormatCPUIntelAndMissingReadings(t *testing.T) {
	out := formatCPU(map[string]any{"cores_total": 8, "thermal": "Unknown"}, 53)
	if strings.Contains(out, "P +") || !strings.Contains(out, "sin lectura de uso") {
		t.Fatalf("unexpected degraded cpu view:\n%s", out)
	}
}

func TestMemoryAndCPUKeysRunAndFillViewport(t *testing.T) {
	withViews(func() {
		for key, tc := range map[rune]struct {
			view viewID
			want string
		}{'m': {viewMemory, "Top 5 apps por RAM"}, 'u': {viewCPU, "Top 5 apps por CPU"}} {
			m := sizedModel()
			nm, cmd := m.Update(keyRunes(key))
			m = nm.(model)
			if !m.running || cmd == nil || m.openView != tc.view {
				t.Fatalf("%q: running=%v cmd=%v view=%d", key, m.running, cmd != nil, m.openView)
			}
			_, rightW, _ := m.paneGeometry()
			out, err := tc.view.runner()(rightW)
			if err != nil {
				t.Fatal(err)
			}
			nm, _ = m.Update(actionResultMsg{out: out})
			if !strings.Contains(nm.(model).detail.View(), tc.want) || nm.(model).running {
				t.Fatalf("%q: viewport should show %q", key, tc.want)
			}
		}
	})
}

func TestRefreshKeyReRunsOpenView(t *testing.T) {
	for _, v := range []viewID{viewDoctor, viewMemory, viewCPU} {
		m := sizedModel()
		m.openView = v
		nm, cmd := m.Update(keyRunes('r'))
		if cmd == nil || !nm.(model).running || nm.(model).openView != v {
			t.Fatalf("'r' should refresh view %d", v)
		}
		// A second press while running must not stack another run.
		if _, cmd := nm.Update(keyRunes('r')); cmd != nil {
			t.Fatalf("'r' while running must be ignored (view %d)", v)
		}
	}
	m := sizedModel()
	if _, cmd := m.Update(keyRunes('r')); cmd != nil {
		t.Fatal("'r' with no open view must do nothing")
	}
}

func TestOtherActionClosesView(t *testing.T) {
	withSampleAnalysis(func() {
		m := sizedModel()
		m.openView = viewMemory
		nm, _ := m.Update(keyRunes('a'))
		if nm.(model).openView != viewNone {
			t.Fatal("running another action must clear openView")
		}
	})
}

func TestMemoryCPUKeysIgnoredInModesAndFilter(t *testing.T) {
	withSampleAnalysis(func() {
		for _, k := range []rune{'m', 'u'} {
			m := sizedModel().enterSelect()
			nm, cmd := m.Update(keyRunes(k))
			if cmd != nil || nm.(model).running || nm.(model).mode != selectMode || nm.(model).openView != viewNone {
				t.Fatalf("%q in select mode must not open a view", k)
			}
		}
	})
	m := sizedModel()
	nm, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	nm, cmd := nm.Update(keyRunes('m')) // typed into the filter, not a shortcut
	if cmd != nil && nm.(model).openView != viewNone {
		t.Fatal("'m' while filtering must not open Memory")
	}
	if nm.(model).openView != viewNone {
		t.Fatal("'m' while filtering must not open Memory")
	}
}

func TestPreviousShortcutsStillWork(t *testing.T) {
	withSampleAnalysis(func() {
		withDoctor(func() {
			for key, view := range map[rune]viewID{'a': viewNone, 'p': viewNone, 'd': viewDoctor} {
				nm, cmd := sizedModel().Update(keyRunes(key))
				if cmd == nil || !nm.(model).running || nm.(model).openView != view {
					t.Fatalf("%q regressed: running=%v view=%d", key, nm.(model).running, nm.(model).openView)
				}
			}
			nm, _ := sizedModel().Update(keyRunes('c'))
			if nm.(model).mode != selectMode {
				t.Fatal("'c' should still open the clean flow")
			}
		})
	})
}

func TestEnterOnMemoryOverviewOpensView(t *testing.T) {
	withViews(func() {
		m := sizedModel()
		for i := 0; i < 40 && !isItem(m, "Memory", "Overview"); i++ {
			nm, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
			m = nm.(model)
		}
		nm, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		if cmd == nil || nm.(model).openView != viewMemory {
			t.Fatal("Enter on Memory > Overview should open the memory view")
		}
	})
}

// isItem reports whether the menu cursor is on group/label.
func isItem(m model, group, label string) bool {
	it, ok := m.actions.SelectedItem().(listItem)
	return ok && !it.header && it.action.group == group && it.action.label == label
}

func TestFooterCompactKeepsShortcutsWithinWidth(t *testing.T) {
	for _, w := range []int{80, 89, 120} {
		f := renderFooter(normalMode, w)
		assertFits(t, f, w)
		for _, want := range []string{"salir", "cpu"} {
			if !strings.Contains(f, want) {
				t.Fatalf("footer at %d cols missing %q: %s", w, want, f)
			}
		}
	}
	if !strings.Contains(renderFooter(normalMode, 80), "mem") {
		t.Fatal("compact footer should show m")
	}
}

func TestViewsAreReadOnly(t *testing.T) {
	for _, a := range buildActions() {
		if a.view != viewNone && (a.quit || a.openClean) {
			t.Fatalf("%s must be a plain view, not a control action", a.label)
		}
	}
}

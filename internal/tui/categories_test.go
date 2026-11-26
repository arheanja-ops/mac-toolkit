package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/arheanja-ops/mac-toolkit/internal/core"
	"github.com/arheanja-ops/mac-toolkit/internal/monitor"
	"github.com/arheanja-ops/mac-toolkit/internal/reporter"

	"github.com/charmbracelet/lipgloss"

	tea "github.com/charmbracelet/bubbletea"
)

// withReportsDir points the canonical reports folder at dir for one test.
func withReportsDir(t *testing.T, dir string) {
	t.Helper()
	orig := core.ReportsDirOverride
	core.ReportsDirOverride = dir
	t.Cleanup(func() { core.ReportsDirOverride = orig })
}

// writeReport creates a synthetic analysis report folder.
func writeReport(t *testing.T, name, body string) string {
	t.Helper()
	d := filepath.Join(core.ReportsDir(), name)
	if err := os.MkdirAll(d, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(d, "report.md")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func sampleProc() map[string]any {
	procs := []monitor.ProcInfo{
		{PID: 101, CPU: 88.2, Mem: 12.5, RSSKB: 2_097_152, Name: "alpha"},
		{PID: 20202, CPU: 12.0, Mem: 30.1, RSSKB: 4_500_000, Name: "a-process-with-a-very-long-synthetic-name-that-must-be-cut"},
		{PID: 3, CPU: 0.1, Mem: 0.2, RSSKB: 900, Name: "gamma"},
	}
	return map[string]any{
		"top_cpu": procs, "top_memory": procs,
		"apps": []monitor.AppUsage{
			{Name: "Alpha", Procs: 12, CPUPct: 90.5, MemMB: 3000},
			{Name: "Beta with an extremely long application name", Procs: 1, CPUPct: 4, MemMB: 1500},
		},
		"summary":         monitor.ProcSummary{Total: 400, Running: 3, Sleeping: 390, Stopped: 1, Zombie: 2, IOWait: 4},
		"pressure_level":  monitor.PressureWarning,
		"memory_pressure": "System-wide memory free percentage: 18%",
	}
}

func sampleSwap() map[string]any {
	d := sampleMemory()
	d["pageins_gb"], d["pageouts_gb"] = 100.0, 20.0
	d["compressor_gb"] = 4.4
	return d
}

// assertNoStructLeak fails when a rendering carries Go struct/map syntax.
func assertNoStructLeak(t *testing.T, name, s string) {
	t.Helper()
	if strings.ContainsAny(s, "{}") || strings.Contains(s, "map[") {
		t.Fatalf("%s leaks Go syntax:\n%s", name, s)
	}
}

// assertLinesFit checks every line separately with lipgloss.Width.
func assertLinesFit(t *testing.T, name, s string, maxW int) {
	t.Helper()
	for _, ln := range strings.Split(s, "\n") {
		if w := lipgloss.Width(ln); w > maxW {
			t.Fatalf("%s: line of %d cells exceeds %d: %q", name, w, maxW, ln)
		}
	}
}

func TestNewViewsFitWidthsAndHideStructs(t *testing.T) {
	withReportsDir(t, t.TempDir())
	writeReport(t, "analysis_20260102_030405",
		"# Report\n\n| Domain | Size |\n|---|---|\n| `caches` | **1.0 GB** |\n\nplain text line that is quite long and must be cut to the width\n")
	origSave := saveReportFn
	saveReportFn = func() (reporter.Saved, int, error) {
		return reporter.Saved{Dir: filepath.Join(core.ReportsDir(), "analysis_x")}, 11, nil
	}
	t.Cleanup(func() { saveReportFn = origSave })
	origProc, origMem, origCPU := procFn, memoryFn, cpuFn
	procFn = func() (map[string]any, error) { return sampleProc(), nil }
	memoryFn = func() (map[string]any, error) { return sampleSwap(), nil }
	cpuFn = func() (map[string]any, error) { return sampleCPU(), nil }
	t.Cleanup(func() { procFn, memoryFn, cpuFn = origProc, origMem, origCPU })

	views := map[string]func(int) (string, error){
		"proc-cpu": runProcTopCPU, "proc-mem": runProcTopMem, "proc-apps": runProcApps,
		"proc-summary": runProcSummary, "mem-swap": runMemorySwap, "mem-apps": runMemoryApps,
		"cpu-load": runCPULoad, "cpu-apps": runCPUApps, "cpu-thermal": runCPUThermal,
		"last-report": runLastReport, "saved-reports": runSavedReports, "save-report": runSaveReport,
		"open-report": runOpenReport,
	}
	for name, run := range views {
		for _, width := range []int{40, 50, 60, 80} {
			out, err := run(width)
			if err != nil {
				t.Fatalf("%s@%d: %v", name, width, err)
			}
			if strings.TrimSpace(out) == "" {
				t.Fatalf("%s@%d: empty output", name, width)
			}
			assertLinesFit(t, name, out, width)
			assertNoStructLeak(t, name, out)
		}
	}
}

func TestEmptyStatesDoNotLeakStructs(t *testing.T) {
	for _, width := range []int{40, 80} {
		for name, out := range map[string]string{
			"top":     formatProcTop(map[string]any{}, false, width),
			"apps":    formatProcApps(map[string]any{}, width),
			"summary": formatProcSummary(map[string]any{}, width),
			"swap":    formatMemorySwap(map[string]any{}, width),
			"load":    formatCPULoad(map[string]any{}, width),
		} {
			assertLinesFit(t, name, out, width)
			assertNoStructLeak(t, name, out)
		}
	}
}

func TestProcViewPropagatesError(t *testing.T) {
	orig := procFn
	procFn = func() (map[string]any, error) { return nil, errBoom }
	t.Cleanup(func() { procFn = orig })
	if _, err := runProcTopCPU(80); err == nil {
		t.Fatal("expected the snapshot error")
	}
}

func TestLastReportNoReportsIsFriendly(t *testing.T) {
	withReportsDir(t, filepath.Join(t.TempDir(), "missing"))
	for _, run := range []func(int) (string, error){runLastReport, runSavedReports} {
		out, err := run(60)
		if err != nil {
			t.Fatalf("no reports must not be an error: %v", err)
		}
		for _, want := range []string{"Aún no hay reportes", "analyze --save", "Save report"} {
			if !strings.Contains(out, want) {
				t.Fatalf("missing %q in:\n%s", want, out)
			}
		}
		assertNoStructLeak(t, "empty-state", out)
		assertLinesFit(t, "empty-state", out, 60)
	}
}

func TestLastReportIgnoresCwd(t *testing.T) {
	withReportsDir(t, t.TempDir())
	writeReport(t, "analysis_20260102_030405", "# Synthetic\n\nFOUND-ME\n")
	t.Chdir(t.TempDir()) // a different directory, with no reports
	out, err := runLastReport(80)
	if err != nil || !strings.Contains(out, "FOUND-ME") {
		t.Fatalf("report must be found from any cwd: err=%v out=%q", err, out)
	}
	if list, _ := core.ListReports(); len(list) != 1 || !filepath.IsAbs(list[0].Path) {
		t.Fatalf("list should hold one absolute path, got %+v", list)
	}
}

func TestReportsSkipCleanupAuditsAndSortNewestFirst(t *testing.T) {
	withReportsDir(t, t.TempDir())
	writeReport(t, "analysis_20250101_000000", "# a")
	writeReport(t, "analysis_20260101_000000", "# b")
	if err := os.MkdirAll(filepath.Join(core.ReportsDir(), "cleanup_20270101_000000"), 0o755); err != nil {
		t.Fatal(err)
	}
	list, err := core.ListReports()
	if err != nil || len(list) != 2 || list[0].Name != "analysis_20260101_000000" {
		t.Fatalf("unexpected list %+v err=%v", list, err)
	}
}

func TestNavigationCoversMenuAt80x24(t *testing.T) {
	m := newModel("v-test")
	nm, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = nm.(model)

	first := m.actions.Index()
	if !isItem(m, "Health", doctorLabel) {
		t.Fatal("menu should open on the first item")
	}
	seen := map[int]bool{first: true}
	for i := 0; i < 60; i++ {
		nm, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m = nm.(model)
		seen[m.actions.Index()] = true
		v := m.View()
		if h := strings.Count(v, "\n") + 1; h > 24 {
			t.Fatalf("View has %d lines at cursor %d (max 24)", h, m.actions.Index())
		}
		assertLinesFit(t, "view", v, 80)
	}
	last, ok := m.actions.SelectedItem().(listItem)
	if !ok || !last.action.quit {
		t.Fatal("the cursor must reach the last item (Quit)")
	}
	if !strings.Contains(m.View(), "Quit") {
		t.Fatal("the last item must be visible when selected")
	}
	// Up/Home/End/PgUp return to the edges without getting stuck on headers.
	for _, k := range []tea.KeyMsg{{Type: tea.KeyHome}, {Type: tea.KeyEnd}, {Type: tea.KeyPgUp}, {Type: tea.KeyPgDown}, {Type: tea.KeyUp}} {
		nm, _ = m.Update(k)
		m = nm.(model)
		if isHeaderAt(m.actions.VisibleItems(), m.actions.Index()) {
			t.Fatalf("cursor landed on a header after %v", k)
		}
		assertLinesFit(t, "view", m.View(), 80)
	}
	nm, _ = m.Update(tea.KeyMsg{Type: tea.KeyHome})
	if !isItem(nm.(model), "Health", doctorLabel) {
		t.Fatal("Home must return to the first item")
	}
}

func TestAllMenuItemsAreSelectableAndGrouped(t *testing.T) {
	want := []string{"Health", "Disk", "Clean", "Memory", "CPU", "Processes", "System", "Reports", "Session"}
	var got []string
	for _, a := range buildActions() {
		if len(got) == 0 || got[len(got)-1] != a.group {
			got = append(got, a.group)
		}
		if a.view != viewNone && a.view.runner() == nil {
			t.Fatalf("%s has no runner", a.label)
		}
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("categories = %v, want %v", got, want)
	}
}

func TestShortcutsOpenViewsAndSyncCursor(t *testing.T) {
	withViews(func() {
		for key, tc := range map[rune]struct {
			view         viewID
			group, label string
		}{
			'd': {viewDoctor, "Health", doctorLabel},
			'm': {viewMemory, "Memory", "Overview"},
			'u': {viewCPU, "CPU", "Overview"},
		} {
			withDoctor(func() {
				nm, cmd := sizedModel().Update(keyRunes(key))
				mm := nm.(model)
				if cmd == nil || !mm.running || mm.openView != tc.view || !isItem(mm, tc.group, tc.label) {
					t.Fatalf("%q: running=%v view=%d", key, mm.running, mm.openView)
				}
			})
		}
	})
}

func TestShortcutsIgnoredInCleanModes(t *testing.T) {
	withSampleAnalysis(func() {
		for _, k := range []rune{'d', 'm', 'u', 'r', 'p'} {
			m := sizedModel().enterSelect()
			nm, cmd := m.Update(keyRunes(k))
			if cmd != nil || nm.(model).openView != viewNone || nm.(model).running {
				t.Fatalf("%q leaked into select mode", k)
			}
		}
		m := sizedModel().enterSelect()
		m.mode = confirmMode
		nm, _ := m.Update(keyRunes('m'))
		if nm.(model).mode != confirmMode || nm.(model).confirmInput != "m" {
			t.Fatal("confirm modal must capture typed keys")
		}
	})
}

func TestEveryViewOpensFromEnter(t *testing.T) {
	m := sizedModel()
	opened := map[viewID]bool{}
	for i := 0; i < 60; i++ {
		it, ok := m.actions.SelectedItem().(listItem)
		if ok && !it.header && it.action.view != viewNone {
			nm, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
			if cmd == nil || nm.(model).openView != it.action.view {
				t.Fatalf("Enter on %q did not open its view", it.action.label)
			}
			opened[it.action.view] = true
		}
		nm, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m = nm.(model)
	}
	if len(opened) != len(viewRunners) {
		t.Fatalf("opened %d of %d views", len(opened), len(viewRunners))
	}
}

func TestOnlySaveReportWrites(t *testing.T) {
	for _, a := range buildActions() {
		if a.openClean && a.label != "Select & clean" {
			t.Fatalf("%s must not open the delete flow", a.label)
		}
	}
	withReportsDir(t, t.TempDir())
	origSave := saveReportFn
	saveReportFn = func() (reporter.Saved, int, error) { return reporter.Saved{Dir: core.ReportsDir()}, 0, nil }
	t.Cleanup(func() { saveReportFn = origSave })
	if _, err := runSaveReport(60); err != nil {
		t.Fatal(err)
	}
}

func TestTildePathHidesHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		t.Skip("no home")
	}
	if got := tildePath(filepath.Join(home, "x", "y")); got != "~/x/y" {
		t.Fatalf("got %q", got)
	}
}

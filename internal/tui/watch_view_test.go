package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/arheanja-ops/mac-toolkit/internal/core"
	"github.com/arheanja-ops/mac-toolkit/internal/monitor"
	"github.com/arheanja-ops/mac-toolkit/internal/watch"

	"github.com/charmbracelet/lipgloss"

	tea "github.com/charmbracelet/bubbletea"
)

// fakeEngine is a listening engine fed by synthetic readings: it never reads
// the machine, never notifies and writes its history into a temp dir.
func fakeEngine(t *testing.T, swapUsedMB float64) *watch.Engine {
	t.Helper()
	now := time.Now().Add(-3 * time.Minute)
	var calls int
	eng := &watch.Engine{
		Interval: watch.DefaultInterval, Eval: watch.NewEvaluator(), Store: &watch.Store{Dir: t.TempDir()},
		Sampler: &watch.Sampler{Read: func(monitor.LightOptions) monitor.LightReading {
			return monitor.LightReading{MemOK: true, MemUsedPct: 61, CPUOK: true, IdlePct: 70, LoadOK: true, Load5: 2, Cores: 8,
				SwapOK: true, SwapUsedMB: swapUsedMB, SwapTotalMB: 1000, PressureCode: 1, TopCPUApp: "Builder"}
		}},
		Notifier: &watch.Notifier{Run: func(string, ...string) error { calls++; return nil }, Enabled: true, PerHour: 6},
		Now:      func() time.Time { return now },
	}
	for i := 0; i < 13; i++ {
		eng.Step()
		now = now.Add(watch.DefaultInterval)
	}
	return eng
}

func withEngine(t *testing.T, eng *watch.Engine) {
	t.Helper()
	orig, origStore := watchEngine, historyStoreFn
	watchEngine = eng
	if eng != nil {
		historyStoreFn = func() watch.Store { return *eng.Store }
	}
	t.Cleanup(func() { watchEngine, historyStoreFn = orig, origStore })
}

func TestAlertViewsFitWidths(t *testing.T) {
	withEngine(t, fakeEngine(t, 950))
	for name, run := range map[string]func(int) (string, error){"alerts": runActiveAlerts, "history": runHistory} {
		for _, width := range []int{40, 50, 60, 80} {
			out, err := run(width)
			if err != nil {
				t.Fatal(err)
			}
			assertLinesFit(t, name, out, width)
			assertNoStructLeak(t, name, out)
		}
	}
	out, _ := runActiveAlerts(80)
	if !strings.Contains(out, "Swap casi lleno") || !strings.Contains(out, "swap al 95 %") {
		t.Fatalf("active alert missing:\n%s", out)
	}
	hist, _ := runHistory(80)
	if !strings.Contains(hist, "ALERTAS") || !strings.Contains(hist, "13") {
		t.Fatalf("history missing samples:\n%s", hist)
	}
}

func TestAlertViewsWithoutEngine(t *testing.T) {
	withEngine(t, nil)
	out, err := runActiveAlerts(60)
	if err != nil || !strings.Contains(out, "no está activa") {
		t.Fatalf("got %q %v", out, err)
	}
}

func TestHeaderBadgeFitsAndUsesEngineSample(t *testing.T) {
	withEngine(t, fakeEngine(t, 950))
	st := readHeaderStats()
	if !st.ok || st.memUsedPct != 61 || st.cpuUsedPct != 30 || !st.watch.on || st.watch.alerts != 1 {
		t.Fatalf("header must reuse the engine sample: %+v", st)
	}
	for _, w := range []int{40, 60, 80, 120} {
		h := renderHeader("v1.11.0", core.DiskSnapshot{OK: true, TotalBytes: 500 << 30, AvailBytes: 100 << 30, PctUsed: 80}, st, w)
		if lipgloss.Width(h) > w || !strings.Contains(h, "⚠ 1") {
			t.Fatalf("header @%d: %q", w, h)
		}
	}
}

func TestKeysNAndWToggleWithoutBreakingOthers(t *testing.T) {
	eng := fakeEngine(t, 100)
	withEngine(t, eng)
	m := newModel("v-test")
	nm, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = nm.(model)
	nm, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	m = nm.(model)
	if eng.Status().Notify || m.hdr.watch.notify {
		t.Fatal("n must turn notifications off")
	}
	nm, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("w")})
	m = nm.(model)
	if !eng.Status().Paused || !m.hdr.watch.paused {
		t.Fatal("w must pause the listening")
	}
	nm, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("m")})
	if nm.(model).openView != viewMemory || cmd == nil {
		t.Fatal("existing shortcuts must keep working")
	}
	if maxW, lines := widestLine(nm.(model).View()); maxW > 80 || lines > 24 {
		t.Fatalf("frame %dx%d", maxW, lines)
	}
}

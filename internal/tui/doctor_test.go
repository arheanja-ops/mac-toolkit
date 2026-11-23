package tui

import (
	"strings"
	"testing"

	"github.com/arheanja-ops/mac-toolkit/internal/ai"
	"github.com/arheanja-ops/mac-toolkit/internal/core"
)

// sampleDiagnosis carries all three sections plus disk context, with long
// texts so wrapping and truncation are exercised at narrow widths.
func sampleDiagnosis() ai.Diagnosis {
	return ai.Diagnosis{
		Engine: ai.EngineHeuristic,
		Performance: []ai.Insight{{
			Title: "Swap alto", Detail: "21.0 GB en uso por procesos en segundo plano",
			Action: "Cierra apps pesadas o reinicia para liberar memoria", Severity: ai.SevHigh,
		}},
		QuickWins: []ai.Insight{{
			Title: "Cachés de dev", Detail: "5.6 GB", Action: "rm -rf algo", Severity: ai.SevLow,
		}},
		Review: []ai.Insight{{
			Title: "Application Support", Detail: "36 GB entre muchas aplicaciones distintas",
			Action: "Revisa manualmente (no borrable aquí)", Severity: ai.SevMed,
		}},
		ReclaimableSafeBytes: 24 << 30,
		Context:              ai.DiagnosisContext{DiskUsedPct: 62, DiskFreeBytes: 128 << 30, DiskTotalBytes: 460 << 30},
	}
}

func TestFormatDiagnosisRespectsWidth(t *testing.T) {
	for _, width := range []int{40, 50, 60, 80} {
		out := formatDiagnosis(sampleDiagnosis(), "", width)
		if maxW, _ := widestLine(out); maxW > width {
			t.Fatalf("width=%d: line exceeds it (got %d):\n%s", width, maxW, out)
		}
	}
}

func TestFormatDiagnosisContent(t *testing.T) {
	out := formatDiagnosis(sampleDiagnosis(), "", 80)
	for _, want := range []string{"motor: heurístico", "PERFORMANCE", "QUICK WINS",
		"REQUIERE REVISIÓN", "presiona c para limpiar", "Contexto: disco 62%"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
	// Quick-win Action is replaced by the bridge, never shown as a command.
	if strings.Contains(out, "rm -rf") {
		t.Fatalf("shell-like action leaked into panel:\n%s", out)
	}
}

func TestFormatDiagnosisOmitsEmptyAndContext(t *testing.T) {
	d := ai.Diagnosis{Engine: ai.EngineHeuristic, Performance: sampleDiagnosis().Performance}
	out := formatDiagnosis(d, "", 80)
	for _, bad := range []string{"QUICK WINS", "REQUIERE", "Contexto", "[c] limpiar"} {
		if strings.Contains(out, bad) {
			t.Fatalf("%q should be omitted:\n%s", bad, out)
		}
	}
}

func TestFormatDiagnosisEngineLine(t *testing.T) {
	d := sampleDiagnosis()
	d.Engine = ai.EngineGroq
	if out := formatDiagnosis(d, "llama-x", 80); !strings.Contains(out, "motor: groq · llama-x") {
		t.Fatalf("groq engine line missing:\n%s", out)
	}
	if out := formatDiagnosis(d, "", 80); !strings.Contains(out, "motor: groq") {
		t.Fatalf("bare groq engine line missing:\n%s", out)
	}
}

func TestFormatDiagnosisNarrowDropsEmojiAndBar(t *testing.T) {
	out := formatDiagnosis(sampleDiagnosis(), "", 40)
	if strings.ContainsAny(out, "⚡🧹⚠▰") {
		t.Fatalf("narrow panel should drop emoji and bar:\n%s", out)
	}
}

// withDoctor injects a fixed Diagnosis so tests never touch the filesystem,
// the monitors or Groq.
func withDoctor(fn func()) {
	orig := doctorFn
	doctorFn = sampleDiagnosis
	defer func() { doctorFn = orig }()
	fn()
}

func TestDoctorKeyDRunsAndFillsViewport(t *testing.T) {
	withDoctor(func() {
		m := sizedModel()
		nm, cmd := m.Update(keyRunes('d'))
		m = nm.(model)
		if !m.running || cmd == nil || m.openView != viewDoctor {
			t.Fatalf("'d' should start the doctor: running=%v cmd=%v open=%d",
				m.running, cmd != nil, m.openView)
		}
		_, rightW, _ := m.paneGeometry()
		out, err := runDoctor(rightW)
		if err != nil {
			t.Fatal(err)
		}
		nm, _ = m.Update(actionResultMsg{out: out})
		m = nm.(model)
		if m.running || !strings.Contains(m.detail.View(), "PERFORMANCE") {
			t.Fatalf("result should fill the viewport:\n%s", m.detail.View())
		}
	})
}

func TestDoctorKeyDIgnoredWhileRunning(t *testing.T) {
	m := sizedModel()
	m.running = true
	if _, cmd := m.Update(keyRunes('d')); cmd != nil {
		t.Fatal("'d' while running must not launch a second run")
	}
}

func TestDoctorKeyRRefreshesOnlyWhenOpen(t *testing.T) {
	m := sizedModel()
	if _, cmd := m.Update(keyRunes('r')); cmd != nil {
		t.Fatal("'r' outside the Doctor panel must not run anything")
	}
	m.openView = viewDoctor
	nm, cmd := m.Update(keyRunes('r'))
	if cmd == nil || !nm.(model).running {
		t.Fatal("'r' in the Doctor panel should re-run it")
	}
	// Launching another action leaves the Doctor panel.
	nm, _ = m.Update(keyRunes('a'))
	if nm.(model).openView != viewNone {
		t.Fatal("running another action should clear openView")
	}
}

func TestDoctorKeyDIgnoredInSelectMode(t *testing.T) {
	withSampleAnalysis(func() {
		m := sizedModel().enterSelect()
		nm, cmd := m.Update(keyRunes('d'))
		if cmd != nil || nm.(model).running || nm.(model).mode != selectMode {
			t.Fatal("'d' in select mode must not trigger the doctor")
		}
	})
}

func TestDoctorActionRegisteredInDiskGroup(t *testing.T) {
	for _, a := range buildActions() {
		if a.label == doctorLabel {
			if a.group != "Disk" || a.run == nil || a.quit || a.openClean {
				t.Fatalf("bad doctor action: %+v", a)
			}
			return
		}
	}
	t.Fatal("Doctor action not registered")
}

func diskForTest() core.DiskSnapshot {
	return core.DiskSnapshot{OK: true, TotalBytes: 500 << 30, AvailBytes: 100 << 30, PctUsed: 80}
}

// TestDoctorFallbackWithoutKey: no GROQ_API_KEY → heuristic, no error, and the
// panel says so.
func TestDoctorFallbackWithoutKey(t *testing.T) {
	t.Setenv("GROQ_API_KEY", "")
	d := ai.Diagnose(ai.BuildSnapshot(sampleResults(), nil, diskForTest()), ai.Options{})
	if d.Engine != ai.EngineHeuristic {
		t.Fatalf("engine = %q, want heuristic", d.Engine)
	}
	if out := formatDiagnosis(d, "", 80); !strings.Contains(out, "motor: heurístico") {
		t.Fatalf("panel should show the heuristic engine:\n%s", out)
	}
}

// TestDoctorPanelHasNoPaths: sampleResults carries real-looking paths; the
// Diagnosis (aggregates only) must not surface any of them.
func TestDoctorPanelHasNoPaths(t *testing.T) {
	t.Setenv("GROQ_API_KEY", "")
	results := sampleResults()
	d := ai.Diagnose(ai.BuildSnapshot(results, nil, diskForTest()), ai.Options{})
	out := formatDiagnosis(d, "", 80)
	banned := []string{"/Users/", "/Library/", "/private/"}
	for _, r := range results {
		for _, it := range r.Items {
			banned = append(banned, it.Path)
		}
	}
	for _, b := range banned {
		if b != "" && strings.Contains(out, b) {
			t.Fatalf("panel leaks %q:\n%s", b, out)
		}
	}
}

// TestDoctorIsReadOnly: running Doctor and pressing its keys never reaches the
// cleaner and never leaves normalMode.
func TestDoctorIsReadOnly(t *testing.T) {
	withDoctor(func() {
		rec := &recordingCleaner{}
		withInjectedCleaner(t, rec, func() {
			m := sizedModel()
			for _, k := range []rune{'d', 'r'} {
				nm, _ := m.Update(keyRunes(k))
				m = nm.(model)
				m.running = false // simulate the result having arrived
			}
			if m.mode != normalMode {
				t.Fatalf("mode = %v, want normalMode", m.mode)
			}
			if len(rec.got) != 0 {
				t.Fatalf("Clean() invoked with %d items", len(rec.got))
			}
		})
	})
}

package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestExportFlowAsksForFolderAndVariant(t *testing.T) {
	var gotDest string
	var gotShare bool
	orig := exportReportFn
	exportReportFn = func(_ string, dest string, share bool) (string, []string, error) {
		gotDest, gotShare = dest, share
		return dest + "/mac-toolkit-analysis_x", []string{dest + "/mac-toolkit-analysis_x/report.html"}, nil
	}
	t.Cleanup(func() { exportReportFn = orig })
	withReportsDir(t, t.TempDir())
	writeReport(t, "analysis_20260102_150405", "# r\n")

	m := sizedAt(80, 24)
	nm, _ := m.onKey(keyRunes('e'))
	m = nm.(model)
	if m.mode != exportMode || m.exportInput != "~/Downloads" {
		t.Fatalf("e must open the export form with ~/Downloads, got mode=%v input=%q", m.mode, m.exportInput)
	}
	view := m.View()
	if !strings.Contains(view, "~/Downloads") || !strings.Contains(view, "completo") {
		t.Fatalf("form must show the folder and the variant:\n%s", view)
	}
	assertLinesFit(t, "export form", view, 80)
	if h := strings.Count(view, "\n") + 1; h > 24 {
		t.Fatalf("frame taller than 24 rows: %d", h)
	}

	// Typing never triggers shortcuts: "q" and "a" go into the folder.
	for _, k := range []tea.KeyMsg{{Type: tea.KeyBackspace}, keyRunes('q'), keyRunes('a')} {
		nm, _ = m.onKey(k)
		m = nm.(model)
	}
	if m.exportInput != "~/Downloadqa" || m.mode != exportMode {
		t.Fatalf("typed text must edit the folder, got %q", m.exportInput)
	}
	nm, _ = m.onKey(tea.KeyMsg{Type: tea.KeyTab})
	m = nm.(model)
	if !m.exportShare {
		t.Fatal("Tab must switch to the share variant")
	}

	nm, cmd := m.onKey(tea.KeyMsg{Type: tea.KeyEnter})
	m = nm.(model)
	if m.mode != normalMode || cmd == nil {
		t.Fatal("Enter must leave the form and start the export")
	}
	var res actionResultMsg
	for _, msg := range drainCmd(cmd) {
		if r, ok := msg.(actionResultMsg); ok {
			res = r
		}
	}
	if !strings.HasSuffix(gotDest, "/Downloadqa") || !strings.HasPrefix(gotDest, "/") || !gotShare {
		t.Fatalf("export called with dest=%q share=%v", gotDest, gotShare)
	}
	if !strings.Contains(res.out, "exportado") {
		t.Fatalf("result must confirm the export, got %q (%v)", res.out, res.err)
	}
}

func TestExportEscCancelsWithoutExporting(t *testing.T) {
	called := false
	orig := exportReportFn
	exportReportFn = func(string, string, bool) (string, []string, error) { called = true; return "", nil, nil }
	t.Cleanup(func() { exportReportFn = orig })

	m := sizedAt(80, 24)
	nm, _ := m.onKey(keyRunes('e'))
	nm, cmd := nm.(model).onKey(tea.KeyMsg{Type: tea.KeyEsc})
	if nm.(model).mode != normalMode || cmd != nil || called {
		t.Fatal("Esc must cancel the export without running it")
	}
}

func TestExportWithoutReportsIsFriendly(t *testing.T) {
	withReportsDir(t, t.TempDir())
	out, err := runExportReport(60, "/tmp/x", false)
	if err != nil || !strings.Contains(out, "Aún no hay reportes") {
		t.Fatalf("want the empty state, got %q (%v)", out, err)
	}
}

func sizedAt(w, h int) model {
	nm, _ := newModel("t").Update(tea.WindowSizeMsg{Width: w, Height: h})
	return nm.(model)
}

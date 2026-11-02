package tui

import (
	"strings"
	"testing"

	"github.com/arheanja-ops/mac-toolkit/internal/core"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// widestLine returns the maximum visible width over all lines and the line
// count. It measures with lipgloss.Width so ANSI escapes and wide runes are
// counted as display cells, not bytes.
func widestLine(s string) (maxW, lines int) {
	for _, ln := range strings.Split(s, "\n") {
		if w := lipgloss.Width(ln); w > maxW {
			maxW = w
		}
		lines++
	}
	return maxW, lines
}

// TestViewFitsEightyByTwentyFour is the headless layout guard: with an 80x24
// window, the full frame must never produce a line wider than 80 cells nor more
// than 24 rows. This is the regression test for the "desordenado / se sale"
// reports — any renderer that overflows the pane width or height fails here.
func TestViewFitsEightyByTwentyFour(t *testing.T) {
	withSampleAnalysis(func() {
		m := newModel("v-test")
		nm, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
		m = nm.(model)

		// Exercise the welcome screen and then a ran action (the analysis table
		// is the widest content) so both initial and post-action frames are
		// checked at 80 cols.
		for _, frame := range []string{m.View(), ranAnalysisView(t, m)} {
			maxW, lines := widestLine(frame)
			if maxW > 80 {
				t.Fatalf("frame line exceeds 80 cols (got %d):\n%s", maxW, frame)
			}
			if lines > 24 {
				t.Fatalf("frame exceeds 24 lines (got %d):\n%s", lines, frame)
			}
		}
	})
}

// ranAnalysisView drives the model through an Analyze run and returns the
// resulting full-frame View(), so the layout guard also covers the busiest
// content (the responsive table) rather than only the welcome screen.
func ranAnalysisView(t *testing.T, m model) string {
	t.Helper()
	_, rightW, _ := m.paneGeometry()
	out, err := runAnalyze(rightW)
	if err != nil {
		t.Fatalf("runAnalyze: %v", err)
	}
	nm, _ := m.Update(actionResultMsg{out: out})
	return nm.(model).View()
}

// TestFormatAnalysisRespectsNarrowWidth verifies the responsive table never
// emits a line wider than the pane it was handed, even at a cramped 50 cells
// where the SHARE bar must be dropped and the domain truncated.
func TestFormatAnalysisRespectsNarrowWidth(t *testing.T) {
	results := []core.AnalysisResult{
		{Domain: "a-very-long-domain-name-that-will-not-fit", TotalSize: 9 << 30,
			Severity: core.SeverityCritical},
		{Domain: "docker", TotalSize: 8 << 30, Severity: core.SeverityHigh},
		{Domain: "ollama", Error: "a long read failure message that must be truncated too"},
	}
	for _, width := range []int{40, 50, 60, 80} {
		out := formatAnalysis(results, width)
		if maxW, _ := widestLine(out); maxW > width {
			t.Fatalf("width=%d: line exceeds it (got %d):\n%s", width, maxW, out)
		}
	}
}

// TestRenderSelectRespectsWidth verifies the clean select list, which carries
// long domain·label tails, never overflows the pane width across a range of
// narrow widths.
func TestRenderSelectRespectsWidth(t *testing.T) {
	rows := buildCleanRows(sampleResults())
	for _, width := range []int{40, 50, 60, 80} {
		out := renderSelect(rows, 0, width)
		if maxW, _ := widestLine(out); maxW > width {
			t.Fatalf("width=%d: select line exceeds it (got %d):\n%s", width, maxW, out)
		}
	}
}

// TestGeometrySumsToHeight verifies the vertical budget is exact: header(1) +
// air(1) + body-with-borders + footer(1) must equal the terminal height, with
// no phantom scroll. The body occupies bodyH inner rows plus 2 border rows.
func TestGeometrySumsToHeight(t *testing.T) {
	for _, h := range []int{24, 30, 40, 50} {
		_, _, bodyH := geometryFor(80, h)
		// header + air + (bodyH + top/bottom borders) + footer.
		total := 1 + 1 + (bodyH + paneBorderWidth) + 1
		if total != h {
			t.Fatalf("height=%d: frame rows sum to %d, want %d (bodyH=%d)",
				h, total, h, bodyH)
		}
	}
}

// TestHeaderFitsEighty verifies the live header never overflows an 80-column
// terminal — the field it would otherwise clip is the last one, so a too-wide
// header is a real usability bug at the exact boundary width.
func TestHeaderFitsEighty(t *testing.T) {
	disk := core.DiskSnapshot{OK: true, PctUsed: 62, AvailBytes: 120 << 30, TotalBytes: 500 << 30}
	stats := headerStats{ok: true, memUsedPct: 71, cpuUsedPct: 34, thermal: "Normal"}
	h := renderHeader("v1.2.3", disk, stats, 80)
	if w := lipgloss.Width(h); w > 80 {
		t.Fatalf("header exceeds 80 cols (got %d): %q", w, h)
	}
}

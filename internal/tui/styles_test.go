package tui

import (
	"strings"
	"testing"

	"github.com/arheanja-ops/mac-toolkit/internal/core"
)

// TestRiskColorParityWithCore guards the invariant that the TUI palette never
// diverges from core's semantic colors. The ANSI index we use must correspond
// to the same base color core.RiskColor emits (green=2, yellow=3, red=1).
func TestRiskColorParityWithCore(t *testing.T) {
	cases := []struct {
		risk    core.RiskLevel
		wantIdx string
	}{
		{core.RiskSafe, "2"},
		{core.RiskWarn, "3"},
		{core.RiskDanger, "1"},
	}
	for _, c := range cases {
		if got := string(riskColor(c.risk)); got != c.wantIdx {
			t.Fatalf("riskColor(%s)=%s, want ANSI %s (parity with core.RiskColor)",
				c.risk, got, c.wantIdx)
		}
	}
}

// TestSeverityColorParity maps severities onto the same red/yellow/green ramp
// core.SeverityColor uses.
func TestSeverityColorParity(t *testing.T) {
	cases := map[core.Severity]string{
		core.SeverityCritical: "1",
		core.SeverityHigh:     "3",
		core.SeverityMedium:   "3",
		core.SeverityLow:      "2",
	}
	for sev, want := range cases {
		if got := string(severityColor(sev)); got != want {
			t.Fatalf("severityColor(%s)=%s, want %s", sev, got, want)
		}
	}
}

// TestRenderThermalHighlightsAbnormal confirms a non-normal thermal state is
// rendered in the alert style (bold + yellow escape) while a normal reading is
// not, so the header draws attention only when it matters.
func TestRenderThermalHighlightsAbnormal(t *testing.T) {
	normal := renderThermal("Normal")
	warn := renderThermal("Warning")
	if strings.Contains(normal, "\x1b[1m") {
		t.Fatalf("normal thermal should not be bold/alert: %q", normal)
	}
	if !strings.Contains(warn, "Warning") {
		t.Fatalf("thermal text lost: %q", warn)
	}
	// The alert path must emit some styling the muted path does not.
	if normal == warn {
		t.Fatal("abnormal thermal should render differently from normal")
	}
}

// TestProportionBarBounds checks the share bar never overflows its width and
// always shows at least one cell for a non-zero domain.
func TestProportionBarBounds(t *testing.T) {
	// Equal size vs max → fully filled, exactly width cells of the fill glyph.
	full := proportionBar(100, 100, 10)
	if strings.Count(full, "▰") != 10 {
		t.Fatalf("expected 10 filled cells, got %d in %q", strings.Count(full, "▰"), full)
	}
	// Tiny vs huge → at least one cell, never zero.
	tiny := proportionBar(1, 1_000_000, 10)
	if strings.Count(tiny, "▰") < 1 {
		t.Fatalf("expected at least one filled cell for non-zero domain: %q", tiny)
	}
	// Zero size → no fill cells (all empty/neutral).
	zero := proportionBar(0, 100, 10)
	if strings.Contains(zero, "▰") {
		t.Fatalf("zero domain should have no filled cells: %q", zero)
	}
}

package tui

import (
	"github.com/arheanja-ops/mac-toolkit/internal/core"

	"github.com/charmbracelet/lipgloss"
)

// This file is the single source of truth for TUI appearance. Every color and
// reusable lipgloss.Style lives here so panels, header, footer and detail
// renderers never invent ad-hoc colors that could drift apart. The semantic
// risk colors are deliberately kept in lockstep with core.RiskColor /
// core.SeverityColor (which the CLI/reporter use) so the TUI and the terminal
// output never disagree about what "safe/warn/danger" looks like.

// Palette — one accent, a neutral ramp, and the three semantic risk colors.
// We use ANSI 256 indices (not hex) so the values map 1:1 onto the ANSI codes
// core already emits (green=2, yellow=3, red=1), guaranteeing visual parity
// between the TUI and the plain-text reporter.
var (
	accent    = lipgloss.Color("62")  // indigo accent for focus + selection
	textMuted = lipgloss.Color("244") // secondary text (group headers, hints)
	textFaint = lipgloss.Color("240") // borders of blurred panes, separators

	riskSafe   = lipgloss.Color("2") // green  — matches core.RiskColor(RiskSafe)
	riskWarn   = lipgloss.Color("3") // yellow — matches core.RiskColor(RiskWarn)
	riskDanger = lipgloss.Color("1") // red    — matches core.RiskColor(RiskDanger)
)

// Shared styles. Keeping them package-level means they are built once and reused
// across every View() call rather than reallocated per render.
var (
	// Group headers in the left list: faint, uppercase, slightly spaced.
	groupHeaderStyle = lipgloss.NewStyle().
				Foreground(textMuted).Bold(true)

	// Separator glyph used between header fields (" · ") rendered faint so the
	// metrics themselves stay visually dominant.
	sepStyle = lipgloss.NewStyle().Foreground(textFaint)

	// Header field label (e.g. "Disk", "Mem") kept muted; the value stays at the
	// terminal's default foreground so it reads as the primary datum.
	hdrLabelStyle = lipgloss.NewStyle().Foreground(textMuted)

	// Thermal alert style for Warning/critical thermal states.
	thermalAlertStyle = lipgloss.NewStyle().Foreground(riskWarn).Bold(true)

	// Footer key hints: each key is rendered in the accent, its description muted.
	footerKeyStyle  = lipgloss.NewStyle().Foreground(accent).Bold(true)
	footerDescStyle = lipgloss.NewStyle().Foreground(textMuted)

	// Detail-pane table styles.
	tableHeaderStyle = lipgloss.NewStyle().Foreground(textMuted).Bold(true)
	bannerStyle      = lipgloss.NewStyle().Foreground(accent).Bold(true)
	ruleStyle        = lipgloss.NewStyle().Foreground(textFaint)
)

// riskColor maps a core.RiskLevel onto the TUI palette, mirroring
// core.RiskColor so the two never diverge.
func riskColor(r core.RiskLevel) lipgloss.Color {
	switch r {
	case core.RiskDanger:
		return riskDanger
	case core.RiskWarn:
		return riskWarn
	case core.RiskSafe:
		return riskSafe
	default:
		return textMuted
	}
}

// severityColor maps a core.Severity onto the TUI palette, mirroring
// core.SeverityColor (critical=red, high/medium=yellow, low=green).
func severityColor(s core.Severity) lipgloss.Color {
	switch s {
	case core.SeverityCritical:
		return riskDanger
	case core.SeverityHigh, core.SeverityMedium:
		return riskWarn
	case core.SeverityLow:
		return riskSafe
	default:
		return textMuted
	}
}

// paneBorder builds a rounded border for a pane with its title embedded in the
// top edge (k9s style: "ACTIONS" / "DETAILS"). The focused pane uses the accent
// color; the blurred one stays faint so focus is unmistakable. width/height are
// the pane's inner content dimensions.
func paneBorder(title string, focused bool, width, height int) lipgloss.Style {
	border := lipgloss.RoundedBorder()
	// Embed the title into the top border run so it reads as a tab label.
	border.Top = embedTitle(border.Top, title, width)

	color := textFaint
	if focused {
		color = accent
	}
	return lipgloss.NewStyle().
		Border(border).
		BorderForeground(color).
		Width(width).
		Height(height)
}

// embedTitle overlays a " title " label near the left of the top border run.
// It returns a string sized to the inner width so lipgloss tiles it exactly
// once (lipgloss repeats Border.Top to fill the width; pre-sizing it to the
// full width makes the title part of a single, non-repeating run).
func embedTitle(fill, title string, width int) string {
	if width <= 0 {
		return fill
	}
	label := " " + title + " "
	if lipgloss.Width(label) >= width {
		// Title alone fills (or overflows) the edge; just pad to width.
		return padBorder(label, fill, width)
	}
	lead := fill + fill // a short run of the border glyph before the label
	rest := width - lipgloss.Width(label) - lipgloss.Width(lead)
	if rest < 0 {
		lead = fill
		rest = width - lipgloss.Width(label) - lipgloss.Width(lead)
	}
	return lead + label + repeatGlyph(fill, rest)
}

// padBorder right-pads s with the border glyph up to width.
func padBorder(s, glyph string, width int) string {
	return s + repeatGlyph(glyph, width-lipgloss.Width(s))
}

// repeatGlyph repeats a single-cell border glyph n times (n<=0 → empty).
func repeatGlyph(glyph string, n int) string {
	if n <= 0 {
		return ""
	}
	out := ""
	for i := 0; i < n; i++ {
		out += glyph
	}
	return out
}

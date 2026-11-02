package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// This file centralizes width-aware layout primitives. Every detail-pane
// renderer (analysis table, select list, snapshot, status) must fit inside the
// right pane's inner width (rightW). Because the content mixes ANSI color codes
// and wide runes (emoji, box glyphs), raw len() lies about display width — we
// measure and cut with lipgloss.Width, which counts visible cells.

// contentPad is the 1-column horizontal breathing room added inside the right
// pane so text never sits flush against the border. It is subtracted from the
// usable width the renderers get, and prepended to every content line.
const contentPad = 1

// usableWidth returns the width available for a detail line after reserving the
// internal horizontal padding on both sides. It never returns < 1 so callers
// can safely build repeated rules of this width.
func usableWidth(paneInner int) int {
	w := paneInner - contentPad*2
	if w < 1 {
		w = 1
	}
	return w
}

// padLines prefixes every line with contentPad spaces so the detail content has
// a one-column left margin inside the pane. Applied once by the caller after a
// block is fully built, so individual row builders stay padding-agnostic.
func padLines(s string) string {
	pad := strings.Repeat(" ", contentPad)
	lines := strings.Split(s, "\n")
	for i, ln := range lines {
		if ln == "" {
			continue // keep genuine blank lines blank (no trailing spaces)
		}
		lines[i] = pad + ln
	}
	return strings.Join(lines, "\n")
}

// truncateToWidth cuts a possibly styled string so its visible width is at most
// max cells, appending an ellipsis when it had to cut. It walks the string rune
// by rune measuring with lipgloss.Width so ANSI escapes and wide runes are
// accounted for correctly. Styled input loses its trailing escape on cut, which
// is acceptable here because the detail lines re-style per cell, not per line.
func truncateToWidth(s string, max int) string {
	if max <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= max {
		return s
	}
	if max == 1 {
		return "…"
	}
	// Reserve one cell for the ellipsis, then accumulate runes until the next
	// one would exceed the budget.
	budget := max - 1
	var b strings.Builder
	width := 0
	for _, r := range s {
		rw := lipgloss.Width(string(r))
		if width+rw > budget {
			break
		}
		b.WriteRune(r)
		width += rw
	}
	return b.String() + "…"
}

// rule returns a horizontal separator of exactly w cells using the faint rule
// style, clamped to a sane minimum so it never collapses to nothing.
func rule(w int) string {
	if w < 1 {
		w = 1
	}
	return ruleStyle.Render(strings.Repeat("─", w))
}

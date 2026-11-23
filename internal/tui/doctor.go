package tui

import (
	"fmt"
	"strings"

	"github.com/arheanja-ops/mac-toolkit/internal/ai"
	"github.com/arheanja-ops/mac-toolkit/internal/core"

	"github.com/charmbracelet/lipgloss"
)

// doctorLabel is the left-panel label of the Doctor action. The model compares
// against it (instead of a control flag on actionItem) via viewForLabel, which
// is what lets 'r' refresh the open panel.
const doctorLabel = "Doctor"

// Below this usable width the emoji headings and the disk bar are dropped:
// emoji have ambiguous cell widths across terminals and would desync borders.
const (
	doctorEmojiMinW = 50
	doctorBarMaxW   = 20
)

// doctorFn produces the Diagnosis. It is a package var so tests inject a fixed
// Diagnosis and never touch the filesystem, the monitors or Groq. The TUI never
// sets NoAI: Diagnose falls back to the heuristic silently without a key or on
// any Groq failure, so no AI error is ever shown.
var doctorFn = func() ai.Diagnosis {
	snap := ai.BuildDoctorSnapshot(analysisFn)
	return ai.Diagnose(snap, ai.Options{})
}

// runDoctor is the read-only Doctor action: diagnose, then format. It runs in a
// tea.Cmd goroutine (see runAction) so the Groq call never freezes the loop.
func runDoctor(width int) (string, error) {
	d := doctorFn()
	model := ""
	if d.Engine == ai.EngineGroq {
		model = ai.ModelName()
	}
	return formatDiagnosis(d, model, width), nil
}

// word is a unit of wrapped text. A nil style means the text is already
// rendered (or plain) and is emitted verbatim.
type word struct {
	text  string
	style *lipgloss.Style
}

func (wd word) render() string {
	if wd.style == nil {
		return wd.text
	}
	return wd.style.Render(wd.text)
}

// words splits plain text into styled words so wrapping can break on spaces
// while each word keeps its own color.
func words(text string, style *lipgloss.Style) []word {
	fields := strings.Fields(text)
	out := make([]word, len(fields))
	for i, f := range fields {
		out[i] = word{text: f, style: style}
	}
	return out
}

// wrapWords word-wraps into lines of at most w visible cells. first/cont are
// the plain prefixes of the first and following lines (bullet / indent). A
// single word wider than the room left is truncated, so no line ever exceeds w.
func wrapWords(ws []word, w int, first, cont string) []string {
	var lines []string
	cur, curW, empty := first, lipgloss.Width(first), true
	contW := lipgloss.Width(cont)
	for _, wd := range ws {
		ww := lipgloss.Width(wd.text)
		sep := 1
		if empty {
			sep = 0
		}
		if !empty && curW+sep+ww > w {
			lines = append(lines, cur)
			cur, curW, empty, sep = cont, contW, true, 0
		}
		if room := w - curW - sep; ww > room {
			wd.text = truncateToWidth(wd.text, room)
			ww = lipgloss.Width(wd.text)
		}
		if ww == 0 {
			continue
		}
		if !empty {
			cur += " "
		}
		cur += wd.render()
		curW += sep + ww
		empty = false
	}
	if !empty {
		lines = append(lines, cur)
	}
	return lines
}

// insightColor maps an insight severity to the TUI palette, mirroring the
// CLI's severityColor (high→danger, medium→warn, low→safe).
func insightColor(sev string) lipgloss.Color {
	switch sev {
	case ai.SevHigh:
		return riskDanger
	case ai.SevMed:
		return riskWarn
	default:
		return riskSafe
	}
}

// formatDiagnosis renders a Diagnosis for the detail pane. It is pure: it only
// lays out the aggregates it receives (no filesystem access, no paths) and
// never emits a line wider than the usable width. Insight text, including the
// Action coming from Groq, is displayed as text and never interpreted.
func formatDiagnosis(d ai.Diagnosis, model string, width int) string {
	w := usableWidth(width)
	emoji := w >= doctorEmojiMinW
	muted := lipgloss.NewStyle().Foreground(textMuted)

	var lines []string
	lines = append(lines,
		muted.Render(truncateToWidth("motor: "+ai.EngineLabelFor(d.Engine, model), w)),
		rule(w))
	cleanHint := d.ReclaimableSafeBytes > 0

	lines = append(lines, doctorSection(heading("⚡", "PERFORMANCE", "", emoji),
		d.Performance, false, w)...)
	suffix := fmt.Sprintf("(%s recuperables sin riesgo)", core.FormatBytes(d.ReclaimableSafeBytes))
	lines = append(lines, doctorSection(heading("🧹", "QUICK WINS", suffix, emoji),
		d.QuickWins, cleanHint, w)...)
	if cleanHint && len(d.QuickWins) > 0 {
		lines = append(lines, muted.Render(truncateToWidth("[c] limpiar seguros · [p] preview", w)))
	}
	lines = append(lines, doctorSection(heading("⚠", "REQUIERE REVISIÓN", "", emoji),
		d.Review, false, w)...)

	lines = append(lines, rule(w))
	lines = append(lines, diskContext(d.Context, w)...)
	return strings.Join(lines, "\n") + "\n"
}

// heading builds a section title, omitting the emoji on narrow panes.
func heading(icon, title, suffix string, emoji bool) string {
	h := title
	if emoji {
		h = icon + " " + title
	}
	if suffix != "" {
		h += "  " + suffix
	}
	return h
}

// doctorSection renders one priority group; empty groups are skipped, like the
// CLI's renderGroup. cleanBridge swaps each Action for the static pointer to
// the TUI-2 select mode, so the panel never suggests shell commands.
func doctorSection(head string, ins []ai.Insight, cleanBridge bool, w int) []string {
	if len(ins) == 0 {
		return nil
	}
	muted := lipgloss.NewStyle().Foreground(textMuted)
	out := []string{tableHeaderStyle.Render(truncateToWidth(head, w))}
	for _, in := range ins {
		title := lipgloss.NewStyle().Foreground(insightColor(in.Severity))
		ws := append(words(in.Title, &title), word{text: "—"})
		ws = append(ws, words(in.Detail, nil)...)
		out = append(out, wrapWords(ws, w, " • ", "   ")...)

		action := in.Action
		if cleanBridge {
			action = "→ presiona c para limpiar"
		}
		if action != "" {
			out = append(out, wrapWords(words(action, &muted), w, "   ", "   ")...)
		}
	}
	return out
}

// diskContext renders the disk line as context only — it is never summed into
// the reclaimable total. Omitted when the total is unknown.
func diskContext(c ai.DiagnosisContext, w int) []string {
	if c.DiskTotalBytes <= 0 {
		return nil
	}
	ws := words(fmt.Sprintf("Contexto: disco %d%%", c.DiskUsedPct), nil)
	if w >= doctorEmojiMinW {
		// Own lipgloss bar (not core.ProgressBar, which emits raw ANSI that
		// lipgloss.Width cannot be trusted with at narrow widths).
		ws = append(ws, word{text: proportionBar(int64(c.DiskUsedPct), 100, min(doctorBarMaxW, w-30))})
	}
	ws = append(ws, word{text: "·"})
	ws = append(ws, words(fmt.Sprintf("%s libres de %s",
		core.FormatBytes(c.DiskFreeBytes), core.FormatBytes(c.DiskTotalBytes)), nil)...)
	return wrapWords(ws, w, "", "")
}

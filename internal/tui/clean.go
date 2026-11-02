package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/arheanja-ops/mac-toolkit/internal/analyzer"
	"github.com/arheanja-ops/mac-toolkit/internal/cleaner"
	"github.com/arheanja-ops/mac-toolkit/internal/core"
	"github.com/arheanja-ops/mac-toolkit/internal/reporter"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// cleanMode enumerates the interactive states layered on top of the normal
// two-pane browser (F2/F3). normalMode is the read-only browser from TUI-1.
type cleanMode int

const (
	normalMode cleanMode = iota
	selectMode
	confirmMode
	cleaningMode
)

// cleanRow is one selectable line in the select view (spec Data model). markable
// is the hard safety gate: only safe && SafeToDelete items may ever be marked,
// enforced in the model (not merely hidden in the UI).
type cleanRow struct {
	item     core.CleanableItem
	marked   bool
	markable bool
}

// cleanSummary is the post-deletion result shown after cleaningMode completes.
type cleanSummary struct {
	freedBytes int64
	count      int
	auditPath  string
	err        error
}

// analysisFn is the source of analysis results for the clean flow. It is a
// package var so tests can inject deterministic rows without touching the real
// analyzers (which read the live filesystem).
var analysisFn = func() []core.AnalysisResult {
	var tasks []core.RunnerTask
	for _, a := range analyzer.All() {
		tasks = append(tasks, core.RunnerTask{Domain: a.Domain(), AnalyzeFn: a.Analyze})
	}
	return core.RunAnalyzers(tasks, core.AnalyzerTimeoutSeconds)
}

// cleanerFn builds the real deletion path by default: NewGenericCleaner(false,
// true) routes every item through cleaner.Delete, which consults the blacklist.
// It is a package var so tests can inject a cleaner writing to a temp dir and
// assert exactly which items were passed to Clean().
var cleanerFn = func() cleaner.Cleaner {
	return cleaner.NewGenericCleaner(false, true)
}

// auditDirOverride lets tests redirect the audit log to a temp dir. Empty means
// the AuditReporter uses its default timestamped reports/ path (CLI behavior).
var auditDirOverride string

// buildCleanRows turns analysis results into selectable rows, largest first.
// Safe items are pre-marked (spec F2). warn/danger rows are included but kept
// unmarkable so the user sees why they are excluded without being able to
// select them. The "disk" domain is skipped: it is volume usage, not junk.
func buildCleanRows(results []core.AnalysisResult) []cleanRow {
	var items []core.CleanableItem
	for _, r := range results {
		if r.Domain == "disk" || r.Error != "" {
			continue
		}
		items = append(items, r.Items...)
	}
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].SizeBytes > items[j].SizeBytes
	})

	rows := make([]cleanRow, 0, len(items))
	for _, it := range items {
		markable := it.Risk == core.RiskSafe && it.SafeToDelete
		rows = append(rows, cleanRow{item: it, marked: markable, markable: markable})
	}
	return rows
}

// markedItems returns the currently marked rows' items. It re-checks markable
// as defense in depth: even if a non-markable row were somehow marked, it is
// never returned for deletion.
func markedItems(rows []cleanRow) []core.CleanableItem {
	var out []core.CleanableItem
	for _, r := range rows {
		if r.marked && r.markable {
			out = append(out, r.item)
		}
	}
	return out
}

// markedTotals sums size and count of the marked, markable rows for the live
// footer and the confirmation modal.
func markedTotals(rows []cleanRow) (bytes int64, count int) {
	for _, r := range rows {
		if r.marked && r.markable {
			bytes += r.item.SizeBytes
			count++
		}
	}
	return bytes, count
}

// hasMarkable reports whether any row can be cleaned at all; used to show the
// "nothing safe to clean" state (spec F2).
func hasMarkable(rows []cleanRow) bool {
	for _, r := range rows {
		if r.markable {
			return true
		}
	}
	return false
}

// toggleMark flips the mark on row i, but ONLY if that row is markable. Marking
// a warn/danger row is a no-op — this is the model-level defense the spec
// requires, not just a UI affordance.
func toggleMark(rows []cleanRow, i int) {
	if i < 0 || i >= len(rows) || !rows[i].markable {
		return
	}
	rows[i].marked = !rows[i].marked
}

// markAllSafe marks every markable row; unmarkAll clears every mark. Neither
// can touch a non-markable row.
func markAllSafe(rows []cleanRow) {
	for i := range rows {
		if rows[i].markable {
			rows[i].marked = true
		}
	}
}

func unmarkAll(rows []cleanRow) {
	for i := range rows {
		rows[i].marked = false
	}
}

// cleanDoneMsg carries the result of a background deletion back to the event
// loop so cleaningMode can render the summary without blocking.
type cleanDoneMsg struct {
	summary cleanSummary
}

// runCleanCmd deletes the given items off the event loop (spec F3.4). It first
// re-filters to markable-safe items as a final guard: if any non-safe item is
// present, it aborts the ENTIRE deletion and reports an error, deleting
// nothing. Otherwise it runs the shared cleaner (blacklist-aware) and writes
// the audit log exactly like the CLI.
func runCleanCmd(rows []cleanRow) tea.Cmd {
	// Snapshot the safe items and verify nothing unsafe slipped through before
	// we hand anything to the deleter. Done on the caller's goroutine-free path
	// so the abort decision is made from the model's own state.
	safe := markedItems(rows)
	marked, _ := markedTotals(rows)
	// Count how many marks exist in total (including any that are not markable).
	var totalMarks int
	for _, r := range rows {
		if r.marked {
			totalMarks++
		}
	}
	return func() tea.Msg {
		if totalMarks != len(safe) {
			// A non-safe row was marked somehow → abort everything (spec F3).
			return cleanDoneMsg{summary: cleanSummary{
				err: fmt.Errorf("aborted: a non-safe item was marked; nothing deleted"),
			}}
		}
		if marked == 0 || len(safe) == 0 {
			return cleanDoneMsg{summary: cleanSummary{}}
		}
		results := cleanerFn().Clean(safe)
		summary := summarize(results)
		summary.auditPath = writeCleanAudit(results)
		return cleanDoneMsg{summary: summary}
	}
}

// summarize folds delete results into freed bytes + success count.
func summarize(results []core.DeleteResult) cleanSummary {
	var s cleanSummary
	for _, r := range results {
		if r.Result == "success" {
			s.count++
			s.freedBytes += r.SizeBytes
		}
	}
	return s
}

// writeCleanAudit persists the audit log for a real TUI deletion, mirroring the
// CLI. ApprovalMode is "tui-select" so audit logs distinguish the TUI path from
// the CLI approval modes. Returns the audit directory (or "" on failure).
func writeCleanAudit(results []core.DeleteResult) string {
	audit := &reporter.AuditReporter{
		OutputDir:    auditDirOverride,
		DryRun:       false,
		ApprovalMode: core.ApprovalMode("tui-select"),
	}
	if err := audit.WriteAudit(results); err != nil {
		core.Error("Audit log: %v", err)
		return ""
	}
	return audit.OutputDir
}

// ── Rendering ───────────────────────────────────────────────────────────────

// cleanSelectStyles are the row styles for the select view. Marked/unmarkable
// states reuse the shared palette so the TUI and reporter agree on risk colors.
var (
	cleanCheckStyle = lipgloss.NewStyle().Foreground(accent).Bold(true)
	cleanCursorRow  = lipgloss.NewStyle().Foreground(accent).Bold(true)
	cleanDimRow     = lipgloss.NewStyle().Foreground(textFaint)
)

// renderSelect draws the interactive checkbox list (spec F2 mockup). cursor is
// the highlighted row. width is the pane's inner width; every line is bounded to
// the usable width (minus the 1-column content pad the caller adds) so nothing
// overflows the pane border. height is unused here because the caller frames it
// inside a pane border.
func renderSelect(rows []cleanRow, cursor, width int) string {
	w := usableWidth(width)
	var b strings.Builder
	title := "CLEAN — espacio marca · a todos · ⏎ continuar"
	b.WriteString(tableHeaderStyle.Render(truncateToWidth(title, w)) + "\n")
	b.WriteString(rule(w) + "\n")

	if !hasMarkable(rows) {
		b.WriteString("\n  " + cleanDimRow.Render("Nada seguro para limpiar.") + "\n")
		b.WriteString("\n  " + footerKeyStyle.Render("[Esc]") + " " +
			footerDescStyle.Render("volver") + "\n")
		return b.String()
	}

	for i, r := range rows {
		b.WriteString(renderCleanRow(r, i == cursor, w) + "\n")
	}

	bytes, count := markedTotals(rows)
	b.WriteString(rule(w) + "\n")
	foot := truncateToWidth(
		fmt.Sprintf("Marcado: %s en %d grupos seguros", core.FormatBytes(bytes), count), w)
	b.WriteString(foot + "\n")
	hint := footerKeyStyle.Render("[⏎]") + " " + footerDescStyle.Render("continuar") + "  " +
		footerKeyStyle.Render("[Esc]") + " " + footerDescStyle.Render("cancelar")
	b.WriteString(hint + "\n")
	return b.String()
}

// renderCleanRow renders one checkbox line: "[x] safe  5.6 GB  domain · label".
// The domain·label tail is truncated to the width left after the fixed
// checkbox/risk/size columns so a long path never wraps. Unmarkable rows are
// dimmed and annotated so the user sees why they cannot be selected (spec F2).
func renderCleanRow(r cleanRow, cursor bool, w int) string {
	box := "[ ]"
	if r.marked {
		box = cleanCheckStyle.Render("[x]")
	}
	risk := lipgloss.NewStyle().Foreground(riskColor(r.item.Risk)).
		Render(fmt.Sprintf("%-6s", string(r.item.Risk)))
	size := fmt.Sprintf("%9s", core.FormatBytes(r.item.SizeBytes))
	// Fixed prefix width: caret(2) + box(3) + " " + risk(6) + " " + size(9) + "  ".
	descW := w - (2 + 3 + 1 + 6 + 1 + 9 + 2)
	if descW < 4 {
		descW = 4
	}

	desc := truncateToWidth(fmt.Sprintf("%s · %s", r.item.Domain, r.item.Label), descW)
	line := fmt.Sprintf("%s %s %s  %s", box, risk, size, desc)
	if !r.markable {
		reason := r.item.Reason
		if reason == "" {
			reason = "no auto-marcable"
		}
		tail := truncateToWidth(fmt.Sprintf("%s (%s)",
			fmt.Sprintf("%s · %s", r.item.Domain, r.item.Label), reason), descW)
		line = cleanDimRow.Render(fmt.Sprintf("%s %s %s  %s",
			"[ ]", fmt.Sprintf("%-6s", string(r.item.Risk)), size, tail))
	}

	caret := "  "
	if cursor {
		caret = cleanCursorRow.Render("▸ ")
		if r.markable {
			line = cleanCursorRow.Render(line)
		}
	}
	return caret + line
}

// renderConfirmModal builds the centered confirmation overlay (spec F3 mockup).
// It shows the exact count and size, a text field echoing confirmInput, and the
// cancel hint. Centering is done by the caller via lipgloss.Place.
func renderConfirmModal(count int, bytes int64, confirmInput string) string {
	var b strings.Builder
	b.WriteString(bannerStyle.Render("Confirmar limpieza") + "\n\n")
	fmt.Fprintf(&b, "Vas a borrar %d grupos · %s\n", count, core.FormatBytes(bytes))
	b.WriteString(thermalAlertStyle.Render("Esta acción es irreversible.") + "\n\n")
	b.WriteString(`Escribe "si" y pulsa ⏎ para confirmar:` + "\n")
	fmt.Fprintf(&b, "> %s\n\n", confirmInput+"_")
	b.WriteString(footerKeyStyle.Render("[Esc]") + " " + footerDescStyle.Render("cancelar"))

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(accent).
		Padding(1, 2)
	return box.Render(b.String())
}

// renderCleaning shows progress/summary for cleaningMode. Before the result
// arrives it shows a spinner line; afterwards the freed summary or an error.
func renderCleaning(spin string, result *cleanSummary) string {
	if result == nil {
		return "\n  " + spin + " limpiando…\n"
	}
	if result.err != nil {
		return "\n  " + thermalAlertStyle.Render("Error: "+result.err.Error()) + "\n"
	}
	var b strings.Builder
	b.WriteString("\n  " + bannerStyle.Render("Limpieza completada") + "\n\n")
	fmt.Fprintf(&b, "  Liberaste %s · %d items", core.FormatBytes(result.freedBytes), result.count)
	if result.auditPath != "" {
		fmt.Fprintf(&b, " · audit: %s", result.auditPath)
	}
	b.WriteString("\n\n  " + footerDescStyle.Render("Pulsa cualquier tecla para volver.") + "\n")
	return b.String()
}

package tui

import (
	"testing"

	"github.com/arheanja-ops/mac-toolkit/internal/cleaner"
	"github.com/arheanja-ops/mac-toolkit/internal/core"

	tea "github.com/charmbracelet/bubbletea"
)

// recordingCleaner is a test double for cleaner.Cleaner that records exactly
// which items were handed to Clean(), so tests can assert that only marked-safe
// items ever reach the deleter — and that a non-safe item aborts everything.
type recordingCleaner struct {
	got []core.CleanableItem
}

func (c *recordingCleaner) Clean(items []core.CleanableItem) []core.DeleteResult {
	c.got = append(c.got, items...)
	out := make([]core.DeleteResult, 0, len(items))
	for _, it := range items {
		out = append(out, core.DeleteResult{Path: it.Path, Result: "success", SizeBytes: it.SizeBytes})
	}
	return out
}

// sampleResults returns a deterministic analysis: two safe items, one warn, one
// danger, plus a disk domain and an errored domain that must be ignored.
func sampleResults() []core.AnalysisResult {
	return []core.AnalysisResult{
		{Domain: "disk", TotalSize: 365 << 30, Items: []core.CleanableItem{
			{Path: "/vol", SizeBytes: 365 << 30, Risk: core.RiskSafe, SafeToDelete: true, Domain: "disk"},
		}},
		{Domain: "dev_caches", Items: []core.CleanableItem{
			{Path: "/caches/brew", SizeBytes: 5 << 30, Label: "brew", Domain: "dev_caches",
				Risk: core.RiskSafe, SafeToDelete: true},
		}},
		{Domain: "browser", Items: []core.CleanableItem{
			{Path: "/caches/chrome", SizeBytes: 2 << 30, Label: "Chrome cache", Domain: "browser",
				Risk: core.RiskSafe, SafeToDelete: true},
		}},
		{Domain: "downloads", Items: []core.CleanableItem{
			{Path: "/dl/big.zip", SizeBytes: 1 << 30, Label: "big.zip", Domain: "downloads",
				Risk: core.RiskWarn, SafeToDelete: false, Reason: "requiere revisión"},
		}},
		{Domain: "docker", Items: []core.CleanableItem{
			{Path: "/docker/Docker.raw", SizeBytes: 32 << 30, Label: "Docker.raw", Domain: "docker",
				Risk: core.RiskDanger, SafeToDelete: false, Reason: "VM disk"},
		}},
		{Domain: "ollama", Error: "read failed", Items: []core.CleanableItem{
			{Path: "/should/not/appear", SizeBytes: 9 << 30, Risk: core.RiskSafe, SafeToDelete: true},
		}},
	}
}

// withSampleAnalysis swaps analysisFn for the deterministic sample, restoring it
// afterward. Keeps tests off the live filesystem.
func withSampleAnalysis(fn func()) {
	orig := analysisFn
	analysisFn = sampleResults
	defer func() { analysisFn = orig }()
	fn()
}

// TestBuildCleanRowsMarkability verifies only safe items are markable and
// pre-marked; warn/danger appear but are never markable (model-level defense).
// The disk domain and errored-domain items must be excluded entirely.
func TestBuildCleanRowsMarkability(t *testing.T) {
	rows := buildCleanRows(sampleResults())

	// 2 safe + 1 warn + 1 danger = 4 rows; disk + errored ollama excluded.
	if len(rows) != 4 {
		t.Fatalf("expected 4 rows (safe/warn/danger, no disk/errored), got %d", len(rows))
	}
	for _, r := range rows {
		wantMarkable := r.item.Risk == core.RiskSafe && r.item.SafeToDelete
		if r.markable != wantMarkable {
			t.Fatalf("%s: markable=%v want %v", r.item.Path, r.markable, wantMarkable)
		}
		// Safe items must be pre-marked; non-safe must not be.
		if r.marked != wantMarkable {
			t.Fatalf("%s: marked=%v want %v (safe pre-marked)", r.item.Path, r.marked, wantMarkable)
		}
		if r.item.Domain == "disk" || r.item.Path == "/should/not/appear" {
			t.Fatalf("excluded item leaked into rows: %s", r.item.Path)
		}
	}
}

// TestToggleMarkWarnDangerIsNoop verifies that toggling a warn/danger row never
// changes its marked state — the core safety invariant of F2.
func TestToggleMarkWarnDangerIsNoop(t *testing.T) {
	rows := buildCleanRows(sampleResults())
	for i, r := range rows {
		if r.markable {
			continue
		}
		before := rows[i].marked
		toggleMark(rows, i)
		if rows[i].marked != before {
			t.Fatalf("toggling non-markable row %d changed marked state", i)
		}
	}
}

// TestToggleMarkSafeFlips verifies a safe row toggles on/off as expected.
func TestToggleMarkSafeFlips(t *testing.T) {
	rows := buildCleanRows(sampleResults())
	// find first markable row
	idx := -1
	for i, r := range rows {
		if r.markable {
			idx = i
			break
		}
	}
	if idx < 0 {
		t.Fatal("expected at least one markable row")
	}
	start := rows[idx].marked // pre-marked true
	toggleMark(rows, idx)
	if rows[idx].marked == start {
		t.Fatal("toggling a markable row should flip it")
	}
	toggleMark(rows, idx)
	if rows[idx].marked != start {
		t.Fatal("toggling twice should restore original state")
	}
}

// TestMarkedItemsOnlySafe verifies markedItems returns only marked AND markable
// items, even if a non-markable row were somehow marked.
func TestMarkedItemsOnlySafe(t *testing.T) {
	rows := buildCleanRows(sampleResults())
	// Force-mark every row (including warn/danger) to simulate corruption.
	for i := range rows {
		rows[i].marked = true
	}
	got := markedItems(rows)
	if len(got) != 2 {
		t.Fatalf("markedItems must return only the 2 safe items, got %d", len(got))
	}
	for _, it := range got {
		if it.Risk != core.RiskSafe || !it.SafeToDelete {
			t.Fatalf("non-safe item returned for deletion: %+v", it)
		}
	}
}

// withInjectedCleaner swaps cleanerFn + auditDirOverride for a recording
// cleaner writing audit to a temp dir, restoring both afterward.
func withInjectedCleaner(t *testing.T, rec *recordingCleaner, fn func()) {
	t.Helper()
	origC, origA := cleanerFn, auditDirOverride
	cleanerFn = func() cleaner.Cleaner { return rec }
	auditDirOverride = t.TempDir()
	defer func() { cleanerFn = origC; auditDirOverride = origA }()
	fn()
}

// TestModeTransitionsNormalToCleaning drives the full keypath with synthetic
// keys: normal → (c) select → (space keeps marks) → (⏎) confirm → (type "si",
// ⏎) cleaning, asserting each transition and that the injected cleaner received
// exactly the two safe items. No real files are touched.
func TestModeTransitionsNormalToCleaning(t *testing.T) {
	withSampleAnalysis(func() {
		rec := &recordingCleaner{}
		withInjectedCleaner(t, rec, func() {
			m := sizedModel()

			// 'c' opens select mode (F4 shortcut / F2 entry). Must NOT delete.
			nm, _ := m.Update(keyRunes('c'))
			m = nm.(model)
			if m.mode != selectMode {
				t.Fatalf("'c' should enter selectMode, got %v", m.mode)
			}
			if len(rec.got) != 0 {
				t.Fatal("entering selection must not delete anything")
			}
			// Safe items are pre-marked → count is 2.
			if _, count := markedTotals(m.rows); count != 2 {
				t.Fatalf("expected 2 pre-marked safe items, got %d", count)
			}

			// ⏎ advances to the confirmation modal (marks present).
			nm, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
			m = nm.(model)
			if m.mode != confirmMode {
				t.Fatalf("⏎ with marks should enter confirmMode, got %v", m.mode)
			}
			if len(rec.got) != 0 {
				t.Fatal("entering confirm must not delete anything yet")
			}

			// Type "si" then ⏎ → cleaningMode + deletion runs.
			for _, r := range []rune{'s', 'i'} {
				nm, _ = m.Update(keyRunes(r))
				m = nm.(model)
			}
			nm, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
			m = nm.(model)
			if m.mode != cleaningMode {
				t.Fatalf("affirmative ⏎ should enter cleaningMode, got %v", m.mode)
			}
			if cmd == nil {
				t.Fatal("cleaningMode should dispatch the deletion command")
			}

			// Execute the batched command to run the deletion off-loop, then
			// feed the resulting cleanDoneMsg back into the model.
			runCmdUntilDone(t, &m, cmd)
			if m.cleanResult == nil {
				t.Fatal("expected a clean result after the command completes")
			}
			if m.cleanResult.err != nil {
				t.Fatalf("unexpected clean error: %v", m.cleanResult.err)
			}

			// Only the two safe items must have reached the cleaner.
			if len(rec.got) != 2 {
				t.Fatalf("cleaner should receive exactly 2 safe items, got %d", len(rec.got))
			}
			for _, it := range rec.got {
				if it.Risk != core.RiskSafe || !it.SafeToDelete {
					t.Fatalf("non-safe item reached cleaner: %+v", it)
				}
			}

			// Any key after the result returns to normalMode.
			nm, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
			m = nm.(model)
			if m.mode != normalMode {
				t.Fatalf("a key after cleaning result should return to normal, got %v", m.mode)
			}
		})
	})
}

// runCmdUntilDone executes a tea.Cmd (which may be a batch) and feeds any
// cleanDoneMsg it produces back into the model, so the test observes the
// post-deletion state without a running event loop.
func runCmdUntilDone(t *testing.T, m *model, cmd tea.Cmd) {
	t.Helper()
	msgs := drainCmd(cmd)
	for _, msg := range msgs {
		if done, ok := msg.(cleanDoneMsg); ok {
			nm, _ := m.Update(done)
			*m = nm.(model)
		}
	}
}

// drainCmd flattens a tea.Cmd into the messages it emits. tea.Batch wraps its
// children as a tea.BatchMsg ([]tea.Cmd); we run each child once. This is
// enough to reach the single cleanDoneMsg the deletion command emits.
func drainCmd(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		var out []tea.Msg
		for _, c := range batch {
			if c == nil {
				continue
			}
			out = append(out, c())
		}
		return out
	}
	return []tea.Msg{msg}
}

// keyRunes builds a KeyMsg for a rune key like 'c' or 'a'.
func keyRunes(r rune) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}} }

// TestConfirmGateRejectsNonAffirmative verifies the inviolable gate: in
// confirmMode, a non-affirmative input (or empty) on ⏎ cancels to normalMode
// and NEVER dispatches a deletion command.
func TestConfirmGateRejectsNonAffirmative(t *testing.T) {
	withSampleAnalysis(func() {
		rec := &recordingCleaner{}
		withInjectedCleaner(t, rec, func() {
			for _, bad := range []string{"", "no", "n", "maybe", "s i"} {
				m := sizedModel()
				m = m.enterSelect()  // selectMode with safe items pre-marked
				m.mode = confirmMode // jump straight to the modal
				m.confirmInput = bad
				nm, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
				m = nm.(model)
				if m.mode != normalMode {
					t.Fatalf("input %q should cancel to normalMode, got %v", bad, m.mode)
				}
				// No deletion command, and nothing reached the cleaner.
				if cmd != nil {
					for _, msg := range drainCmd(cmd) {
						if _, ok := msg.(cleanDoneMsg); ok {
							t.Fatalf("input %q must not trigger deletion", bad)
						}
					}
				}
				if len(rec.got) != 0 {
					t.Fatalf("input %q must not delete anything", bad)
				}
			}
		})
	})
}

// TestConfirmEscCancels verifies Esc in the modal cancels without deleting.
func TestConfirmEscCancels(t *testing.T) {
	withSampleAnalysis(func() {
		rec := &recordingCleaner{}
		withInjectedCleaner(t, rec, func() {
			m := sizedModel()
			m = m.enterSelect()
			m.mode = confirmMode
			m.confirmInput = "si" // even with "si" typed, Esc must cancel
			nm, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
			m = nm.(model)
			if m.mode != normalMode {
				t.Fatalf("Esc should cancel to normalMode, got %v", m.mode)
			}
			if len(rec.got) != 0 {
				t.Fatal("Esc must not delete anything")
			}
		})
	})
}

// TestRunCleanAbortsOnNonSafeMarked verifies the final defense: if a non-safe
// row is somehow marked, runCleanCmd aborts the ENTIRE deletion and reports an
// error, deleting nothing.
func TestRunCleanAbortsOnNonSafeMarked(t *testing.T) {
	withSampleAnalysis(func() {
		rec := &recordingCleaner{}
		withInjectedCleaner(t, rec, func() {
			rows := buildCleanRows(sampleResults())
			// Corrupt state: mark a non-markable (danger) row directly.
			for i := range rows {
				if !rows[i].markable {
					rows[i].marked = true
				}
			}
			msg, ok := runCleanCmd(rows)().(cleanDoneMsg)
			if !ok {
				t.Fatal("runCleanCmd should emit a cleanDoneMsg")
			}
			if msg.summary.err == nil {
				t.Fatal("marking a non-safe item must abort with an error")
			}
			if len(rec.got) != 0 {
				t.Fatal("abort must delete nothing")
			}
		})
	})
}

// TestEnterSelectNeverDeletes verifies entering selection (via 'c' or the
// action) is pure preparation — the cleaner is never invoked.
func TestEnterSelectNeverDeletes(t *testing.T) {
	withSampleAnalysis(func() {
		rec := &recordingCleaner{}
		withInjectedCleaner(t, rec, func() {
			m := sizedModel()
			m = m.enterSelect()
			if m.mode != selectMode {
				t.Fatalf("enterSelect should set selectMode, got %v", m.mode)
			}
			if len(rec.got) != 0 {
				t.Fatal("entering selection must not delete anything")
			}
		})
	})
}

// TestSelectEscReturnsNormal verifies Esc in selectMode returns to the browser
// without deleting.
func TestSelectEscReturnsNormal(t *testing.T) {
	withSampleAnalysis(func() {
		m := sizedModel()
		m = m.enterSelect()
		nm, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
		m = nm.(model)
		if m.mode != normalMode {
			t.Fatalf("Esc in selectMode should return to normalMode, got %v", m.mode)
		}
	})
}

// TestSelectSpaceOnDangerIsNoop drives the space key over a danger row via the
// model and asserts the mark never flips (end-to-end F2 guard through Update).
func TestSelectSpaceOnDangerIsNoop(t *testing.T) {
	withSampleAnalysis(func() {
		m := sizedModel()
		m = m.enterSelect()
		// Move the cursor onto the first non-markable row.
		danger := -1
		for i, r := range m.rows {
			if !r.markable {
				danger = i
				break
			}
		}
		if danger < 0 {
			t.Fatal("sample must contain a non-markable row")
		}
		m.cursor = danger
		before := m.rows[danger].marked
		nm, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
		m = nm.(model)
		if m.rows[danger].marked != before {
			t.Fatal("space on a danger row must be a no-op")
		}
	})
}

// TestPreviewNeverDeletes verifies the Preview action only produces text and
// never invokes the cleaner.
func TestPreviewNeverDeletes(t *testing.T) {
	withSampleAnalysis(func() {
		rec := &recordingCleaner{}
		withInjectedCleaner(t, rec, func() {
			out, err := runPreview()
			if err != nil {
				t.Fatalf("preview error: %v", err)
			}
			if out == "" {
				t.Fatal("preview should render a plan")
			}
			if len(rec.got) != 0 {
				t.Fatal("preview must never delete")
			}
		})
	})
}

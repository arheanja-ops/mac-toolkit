package tui

import (
	"strings"
	"testing"

	"github.com/arheanja-ops/mac-toolkit/internal/core"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

// sizedModel returns a model that has already processed a WindowSizeMsg, so the
// panes and viewport are initialized for transition tests.
func sizedModel() model {
	m := newModel("v-test")
	nm, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	return nm.(model)
}

func TestTabTogglesFocus(t *testing.T) {
	m := sizedModel()
	if m.focus != actionsPanel {
		t.Fatalf("expected initial focus actionsPanel, got %v", m.focus)
	}
	nm, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = nm.(model)
	if m.focus != detailPanel {
		t.Fatalf("Tab should move focus to detailPanel, got %v", m.focus)
	}
	nm, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = nm.(model)
	if m.focus != actionsPanel {
		t.Fatalf("Tab should toggle back to actionsPanel, got %v", m.focus)
	}
}

func TestSlashEntersFilterMode(t *testing.T) {
	m := sizedModel()
	nm, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	m = nm.(model)
	if m.actions.FilterState() != list.Filtering {
		t.Fatalf("'/' should put the list into filtering state, got %v", m.actions.FilterState())
	}
}

func TestQuitReturnsQuitCmd(t *testing.T) {
	m := sizedModel()
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatal("'q' should return a command")
	}
	if msg := cmd(); !isQuit(msg) {
		t.Fatalf("'q' should produce tea.Quit, got %T", msg)
	}
}

func TestEscQuitsWhenNotFiltering(t *testing.T) {
	m := sizedModel()
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil || !isQuit(cmd()) {
		t.Fatal("Esc without a filter should quit")
	}
}

func TestEnterOnHeaderIsNoop(t *testing.T) {
	m := sizedModel()
	// The first list item is the "Disk" group header (index 0, non-executable).
	m.actions.Select(0)
	nm, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = nm.(model)
	if m.running {
		t.Fatal("executing a header must not start a run")
	}
	if cmd != nil {
		if _, isResult := cmd().(actionResultMsg); isResult {
			t.Fatal("header selection should not dispatch an action")
		}
	}
}

func TestEnterOnActionStartsRun(t *testing.T) {
	m := sizedModel()
	// Index 1 is the first real action ("Analyze") after the "Disk" header.
	m.actions.Select(1)
	nm, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = nm.(model)
	if !m.running {
		t.Fatal("selecting an action should set running=true")
	}
	if cmd == nil {
		t.Fatal("expected a command to run the action")
	}
}

func TestActionResultRendersError(t *testing.T) {
	m := sizedModel()
	m.running = true
	nm, _ := m.Update(actionResultMsg{err: errBoom})
	m = nm.(model)
	if m.running {
		t.Fatal("running should be cleared after a result")
	}
	// Error must be shown in the viewport, never crash the program.
	if !strings.Contains(m.detail.View(), "Error") {
		t.Fatalf("error not rendered in detail: %q", m.detail.View())
	}
}

func TestHeaderTickUpdatesStatsAndReprograms(t *testing.T) {
	m := sizedModel()
	newStats := headerStats{ok: true, memUsedPct: 55, cpuUsedPct: 20, thermal: "Normal"}
	nm, cmd := m.Update(headerTickMsg{
		disk:  core.DiskSnapshot{OK: true, PctUsed: 40},
		stats: newStats,
	})
	m = nm.(model)
	if m.hdr != newStats {
		t.Fatalf("header stats not applied: %+v", m.hdr)
	}
	if !m.disk.OK || m.disk.PctUsed != 40 {
		t.Fatalf("disk not applied: %+v", m.disk)
	}
	if cmd == nil {
		t.Fatal("headerTickMsg must reprogram the next tick")
	}
}

func TestHeaderTickStatsNotOkPreservesDisk(t *testing.T) {
	m := sizedModel()
	m.disk = core.DiskSnapshot{OK: true, PctUsed: 62, AvailBytes: 100}
	nm, _ := m.Update(headerTickMsg{
		disk:  core.DiskSnapshot{OK: false}, // this tick's disk read failed
		stats: headerStats{ok: false},
	})
	m = nm.(model)
	// Disk must be preserved from the last good value; header stays renderable.
	if !m.disk.OK || m.disk.PctUsed != 62 {
		t.Fatalf("disk should be preserved on failed tick: %+v", m.disk)
	}
	if m.hdr.ok {
		t.Fatal("stats should reflect the failed system read (ok=false)")
	}
}

// isQuit reports whether a message is tea.Quit's sentinel.
func isQuit(msg tea.Msg) bool {
	_, ok := msg.(tea.QuitMsg)
	return ok
}

// quitActionIndex finds the list index of the special quit action so the test
// does not hardcode a position that shifts when actions are reordered.
func quitActionIndex(m model) int {
	for i, it := range m.actions.Items() {
		li, ok := it.(listItem)
		if ok && li.action.quit {
			return i
		}
	}
	return -1
}

func TestEnterOnQuitActionQuits(t *testing.T) {
	m := sizedModel()
	idx := quitActionIndex(m)
	if idx < 0 {
		t.Fatal("expected a quit action in the list")
	}
	m.actions.Select(idx)
	nm, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = nm.(model)
	if m.running {
		t.Fatal("selecting Quit must not start a run")
	}
	if cmd == nil || !isQuit(cmd()) {
		t.Fatal("selecting the Quit action should return tea.Quit")
	}
}

func TestWelcomeContentIsInitialDetail(t *testing.T) {
	m := sizedModel()
	// Before any action runs, the detail pane must show the welcome dashboard,
	// not a bare hint.
	view := m.detail.View()
	if !strings.Contains(view, "System summary") {
		t.Fatalf("expected welcome dashboard as initial detail, got: %q", view)
	}
}

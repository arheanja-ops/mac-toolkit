package tui

import (
	"strings"

	"github.com/arheanja-ops/mac-toolkit/internal/core"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// panel identifies which pane currently has keyboard focus.
type panel int

const (
	actionsPanel panel = iota
	detailPanel
)

// listItem adapts an actionItem to the Bubbles list.Item interface. Group
// headers are represented as items with a nil action and header=true so the
// list renders them but the model refuses to execute them.
type listItem struct {
	action actionItem
	header bool
}

func (i listItem) FilterValue() string {
	if i.header {
		return "" // headers never match the filter
	}
	return i.action.label
}
func (i listItem) Title() string {
	if i.header {
		return i.action.group
	}
	return i.action.label
}
func (i listItem) Description() string { return "" }

// actionResultMsg is emitted when an action's run() finishes off the event loop.
type actionResultMsg struct {
	out string
	err error
}

// model is the root Bubble Tea state for the TUI (see spec Data model).
type model struct {
	version string
	focus   panel
	actions list.Model
	detail  viewport.Model
	spinner spinner.Model
	disk    core.DiskSnapshot
	hdr     headerStats
	running bool
	width   int
	height  int
	ready   bool

	// TUI-2 clean flow state (spec Data model). mode is normalMode unless the
	// user is in the interactive select/confirm/cleaning flow.
	mode         cleanMode
	rows         []cleanRow    // selectable rows while in selectMode/confirmMode
	cursor       int           // highlighted row in selectMode
	confirmInput string        // text typed into the F3 confirmation modal
	cleanResult  *cleanSummary // result shown in cleaningMode once deletion ends
}

// newModel builds the initial model with the read-only action set. It does not
// size the panes yet; that happens on the first WindowSizeMsg.
func newModel(version string) model {
	items := buildListItems(buildActions())

	// Custom delegate renders the k9s-style hierarchy (group headers + caret).
	l := list.New(items, actionDelegate{}, 0, 0)
	l.Title = "ACTIONS"
	l.SetShowTitle(false) // the pane border carries the "ACTIONS" title instead
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	l.SetFilteringEnabled(true)

	sp := spinner.New()
	sp.Spinner = spinner.Dot

	return model{
		version: version,
		focus:   actionsPanel,
		actions: l,
		spinner: sp,
		disk:    core.ReadDiskSnapshot(),
		hdr:     readHeaderStats(),
	}
}

// buildListItems flattens grouped actions into list items, inserting a
// non-selectable header before each new group.
func buildListItems(actions []actionItem) []list.Item {
	var items []list.Item
	seen := map[string]bool{}
	for _, a := range actions {
		if !seen[a.group] {
			seen[a.group] = true
			items = append(items, listItem{action: actionItem{group: a.group}, header: true})
		}
		items = append(items, listItem{action: a})
	}
	return items
}

func (m model) Init() tea.Cmd {
	return tea.Batch(headerTick(), m.spinner.Tick)
}

// Update is the single reducer. It is split into focused helpers so each stays
// well under the complexity/length limits.
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return m.onResize(msg), nil
	case headerTickMsg:
		// Preserve prior disk if this tick's read momentarily failed, so the
		// header degrades instead of blanking. Keep new stats regardless
		// (ok=false signals system read failure → mem/CPU/thermal omitted).
		if msg.disk.OK {
			m.disk = msg.disk
		}
		m.hdr = msg.stats
		return m, headerTick() // reprogram next tick
	case actionResultMsg:
		return m.onActionResult(msg), nil
	case cleanDoneMsg:
		// Deletion finished off the event loop; capture the summary so
		// cleaningMode renders freed space / audit path (or an error).
		m.running = false
		res := msg.summary
		m.cleanResult = &res
		return m, nil
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case tea.KeyMsg:
		return m.onKey(msg)
	}
	return m, nil
}

// onResize recomputes pane geometry. Left pane is a fixed slice of the width,
// the viewport takes the rest; both leave room for header, a one-line air gap,
// and the footer. Each pane has a rounded border costing 2 columns; a 1-column
// gap separates them so the borders never visually collide (╮╭). Total
// horizontal chrome = 2+2 borders + 1 gap = 5 columns, subtracted from the
// detail width so tables never overflow.
func (m model) onResize(msg tea.WindowSizeMsg) model {
	m.width, m.height = msg.Width, msg.Height
	leftW, rightW, bodyH := geometryFor(msg.Width, msg.Height)
	m.actions.SetSize(leftW, bodyH)
	if !m.ready {
		m.detail = viewport.New(rightW, bodyH)
		// Open on a welcome dashboard, not a bare hint, for a k9s-like feel.
		m.detail.SetContent(padLines(welcomeContent(m.version, m.disk, m.hdr, rightW)))
		m.ready = true
	} else {
		m.detail.Width, m.detail.Height = rightW, bodyH
	}
	return m
}

// onActionResult renders an action's output or error into the detail viewport.
// An error is shown in the panel and never terminates the program. Content is
// left-padded one column so it does not sit flush against the pane border.
func (m model) onActionResult(msg actionResultMsg) model {
	m.running = false
	if msg.err != nil {
		m.detail.SetContent(padLines("Error: " + msg.err.Error()))
	} else {
		m.detail.SetContent(padLines(msg.out))
	}
	m.detail.GotoTop()
	return m
}

// onKey routes key presses. Clean-flow modes (select/confirm/cleaning) capture
// keys first so the normal browser shortcuts never leak into a modal. In
// normalMode, Quit and Tab are global and k9s-style shortcuts (a/c/p) trigger
// existing actions; everything else is delegated to the focused pane.
func (m model) onKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Clean flow owns the keyboard while active (F2/F3).
	switch m.mode {
	case selectMode:
		return m.onSelectKey(msg)
	case confirmMode:
		return m.onConfirmKey(msg)
	case cleaningMode:
		return m.onCleaningKey(msg)
	}

	// While filtering, let the list consume keys (Esc/Enter close the filter).
	if m.actions.FilterState() == list.Filtering {
		var cmd tea.Cmd
		m.actions, cmd = m.actions.Update(msg)
		return m, cmd
	}

	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "esc":
		return m, tea.Quit // no active filter: Esc exits the TUI
	case "tab":
		m.focus = togglePanel(m.focus)
		return m, nil
	case "enter":
		if m.focus == actionsPanel {
			return m.execSelected()
		}
	case "a":
		return m.runAction(runAnalyze)
	case "p":
		return m.runAction(runPreview)
	case "c":
		return m.enterSelect(), nil
	}

	return m.forwardToFocused(msg)
}

// forwardToFocused sends navigation keys to whichever pane holds focus.
func (m model) forwardToFocused(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	if m.focus == actionsPanel {
		m.actions, cmd = m.actions.Update(msg)
	} else {
		m.detail, cmd = m.detail.Update(msg)
	}
	return m, cmd
}

// execSelected runs the selected action off the event loop. Headers are not
// executable, so selecting one is a no-op. A control action flagged quit=true
// terminates the program; openClean=true enters the interactive select flow
// (F2) instead of invoking run().
func (m model) execSelected() (tea.Model, tea.Cmd) {
	it, ok := m.actions.SelectedItem().(listItem)
	if !ok || it.header {
		return m, nil
	}
	if it.action.quit {
		return m, tea.Quit
	}
	if it.action.openClean {
		return m.enterSelect(), nil
	}
	if it.action.run == nil {
		return m, nil
	}
	return m.runAction(it.action.run)
}

// runAction starts a read-only action off the event loop (shared by execSelected
// and the a/p shortcuts). It never deletes — run() only produces viewport text.
// The current right-pane width is captured so the action renders responsively.
func (m model) runAction(run func(width int) (string, error)) (tea.Model, tea.Cmd) {
	m.running = true
	_, rightW, _ := m.paneGeometry()
	return m, tea.Batch(m.spinner.Tick, func() tea.Msg {
		out, err := run(rightW)
		return actionResultMsg{out: out, err: err}
	})
}

// enterSelect builds the clean rows from a fresh analysis and switches to
// selectMode. Entering selection NEVER deletes; it is pure preparation (F2).
// Safe items arrive pre-marked; the cursor starts on the first row.
func (m model) enterSelect() model {
	m.rows = buildCleanRows(analysisFn())
	m.cursor = 0
	m.confirmInput = ""
	m.cleanResult = nil
	m.mode = selectMode
	return m
}

// onSelectKey handles the checkbox list keys (F2): ↑↓ move, space toggles a
// markable row (no-op otherwise), 'a' marks all safe, 'n' clears, Esc cancels
// to normal, ⏎ advances to the confirmation modal. If nothing is markable the
// only exit is Esc.
func (m model) onSelectKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = normalMode
		return m, nil
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.rows)-1 {
			m.cursor++
		}
	case " ":
		toggleMark(m.rows, m.cursor) // model-level guard: no-op on warn/danger
	case "a":
		markAllSafe(m.rows)
	case "n":
		unmarkAll(m.rows)
	case "enter":
		if _, count := markedTotals(m.rows); count > 0 {
			m.mode = confirmMode
			m.confirmInput = ""
		}
	}
	return m, nil
}

// onConfirmKey handles the modal (F3): typing fills confirmInput, Esc cancels
// to normal (no deletion), ⏎ evaluates the gate. Only core.IsAffirmative opens
// it; anything else cancels without deleting.
func (m model) onConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.mode = normalMode
		m.confirmInput = ""
		return m, nil
	case tea.KeyEnter:
		if !core.IsAffirmative(m.confirmInput) {
			// No explicit "si" → cancel, delete nothing (the inviolable gate).
			m.mode = normalMode
			m.confirmInput = ""
			return m, nil
		}
		m.mode = cleaningMode
		m.running = true
		m.cleanResult = nil
		return m, tea.Batch(m.spinner.Tick, runCleanCmd(m.rows))
	case tea.KeyBackspace:
		if n := len(m.confirmInput); n > 0 {
			m.confirmInput = m.confirmInput[:n-1]
		}
	case tea.KeyRunes, tea.KeySpace:
		m.confirmInput += string(msg.Runes)
	}
	return m, nil
}

// onCleaningKey ignores input while the deletion runs; once the result is in,
// any key returns to the normal browser.
func (m model) onCleaningKey(_ tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.cleanResult != nil {
		m.mode = normalMode
		m.cleanResult = nil
		m.rows = nil
	}
	return m, nil
}

func togglePanel(p panel) panel {
	if p == actionsPanel {
		return detailPanel
	}
	return actionsPanel
}

const (
	// paneBorderWidth is the horizontal cost of a lipgloss rounded border
	// (left + right = 2 columns).
	paneBorderWidth = 2
	// panelGap is the blank-column separation inserted between the two panes so
	// their borders never touch (avoids the ╮╭ collision).
	panelGap = 1
	// verticalChrome is the number of rows consumed outside the pane body:
	// header(1) + air(1) + footer(1) + pane top/bottom borders(2). Subtracting
	// it from the terminal height yields the inner body height so the whole
	// frame fits exactly in H rows (no scroll phantom at 24 lines).
	verticalChrome = 5
)

var (
	footerStyle = lipgloss.NewStyle().Foreground(textFaint)
)

// View renders header + two bordered panes (with titles embedded in the top
// border, k9s-style) + a key-hint footer. Render logic only; state never
// mutates here. Clean-flow modes swap the detail content and, for confirmMode,
// overlay a centered modal.
func (m model) View() string {
	if !m.ready {
		return "Loading…"
	}

	header := renderHeader(m.version, m.disk, m.hdr, m.width)

	detail := m.cleanDetail()
	if m.running && m.mode == normalMode {
		detail = m.spinner.View() + " running…\n\n" + detail
	}

	// Inner content dimensions mirror what onResize handed to each pane, so the
	// border wraps the content exactly — nothing is clipped or left ragged.
	leftW, rightW, bodyH := m.paneGeometry()
	left := paneBorder("ACTIONS", m.focus == actionsPanel, leftW, bodyH).Render(m.actions.View())
	right := paneBorder(m.detailTitle(), m.focus == detailPanel, rightW, bodyH).Render(detail)

	// gap keeps the two bordered panes visually separated; its width matches the
	// panelGap subtracted in onResize so geometry and rendering stay in sync.
	gap := strings.Repeat(" ", panelGap)
	body := lipgloss.JoinHorizontal(lipgloss.Top, left, gap, right)

	// Hard-clip the body to the exact frame size. The Bubbles list can report a
	// render height a few rows taller than the size we set it (blank group
	// spacers the delegate emits are not counted in its Height()), which would
	// otherwise push the whole frame past the terminal height and cause a
	// phantom scroll. MaxHeight/MaxWidth guarantee the body never exceeds its
	// budget: body rows = bodyH inner + 2 border rows.
	body = lipgloss.NewStyle().
		MaxWidth(m.width).
		MaxHeight(bodyH + paneBorderWidth).
		Render(body)

	// The confirmation modal (F3) is drawn centered over the whole body so it
	// reads as a focused overlay, not inline text.
	if m.mode == confirmMode {
		bytes, count := markedTotals(m.rows)
		modal := renderConfirmModal(count, bytes, m.confirmInput)
		bw, bh := lipgloss.Width(body), lipgloss.Height(body)
		body = lipgloss.Place(bw, bh, lipgloss.Center, lipgloss.Center, modal)
	}

	// One blank "air" line between the header and the bordered body so the top
	// of the frame is not cramped against the header. Its single row is part of
	// verticalChrome, keeping header+air+body+footer == height exactly.
	return strings.Join([]string{header, "", body, renderFooter(m.mode, m.width)}, "\n")
}

// cleanDetail returns the detail-pane content for the current mode. normalMode
// shows the viewport; the clean modes render their own views. Clean-mode output
// is left-padded one column to match the viewport's inner margin.
func (m model) cleanDetail() string {
	switch m.mode {
	case selectMode:
		_, rightW, _ := m.paneGeometry()
		return padLines(renderSelect(m.rows, m.cursor, rightW))
	case cleaningMode:
		return padLines(renderCleaning(m.spinner.View(), m.cleanResult))
	case confirmMode:
		// The modal is overlaid in View(); behind it we keep the select list so
		// the frame size stays stable.
		_, rightW, _ := m.paneGeometry()
		return padLines(renderSelect(m.rows, m.cursor, rightW))
	default:
		return m.detail.View()
	}
}

// detailTitle labels the right pane per mode so the border reads as a tab.
func (m model) detailTitle() string {
	switch m.mode {
	case selectMode, confirmMode:
		return "CLEAN"
	case cleaningMode:
		return "CLEANING"
	default:
		return "DETAILS"
	}
}

// paneGeometry recomputes the inner (content) dimensions of both panes from the
// current window size, using the exact arithmetic as onResize so View() and the
// pre-sized widgets agree.
func (m model) paneGeometry() (leftW, rightW, bodyH int) {
	return geometryFor(m.width, m.height)
}

// geometryFor is the single source of truth for pane dimensions, shared by
// onResize and paneGeometry so the pre-sized widgets and View() never disagree.
// Vertical budget in a height-H terminal: header(1) + air(1) + footer(1) +
// pane top/bottom borders(2) = 5, so the inner body height is H-5 and the frame
// (header + air + bordered body + footer) sums to exactly H with no overflow.
// The left pane has a 22-column minimum so labels like "Clean (select &
// delete)" read without being clipped on an 80-column terminal.
func geometryFor(width, height int) (leftW, rightW, bodyH int) {
	leftW = width / 4
	if leftW < 22 {
		leftW = 22
	}
	// leftW is the inner list width; the border adds 2. Subtract both panes'
	// borders (4) plus the 1-column gap to get the detail's inner width.
	rightW = width - leftW - paneBorderWidth*2 - panelGap
	if rightW < 10 {
		rightW = 10
	}
	bodyH = height - verticalChrome
	if bodyH < 3 {
		bodyH = 3
	}
	return leftW, rightW, bodyH
}

// renderFooter builds the key-hint bar for the current mode. normalMode shows
// the browser + k9s shortcuts; the clean modes show their own keys so the user
// always sees what the active mode accepts. width truncates the joined hints so
// the footer never wraps onto a second row (which would break the height math).
func renderFooter(mode cleanMode, width int) string {
	var hints []struct{ key, desc string }
	switch mode {
	case selectMode:
		hints = []struct{ key, desc string }{
			{"↑↓", "mover"}, {"espacio", "marcar"}, {"a", "todos"},
			{"n", "ninguno"}, {"⏎", "continuar"}, {"Esc", "cancelar"},
		}
	case confirmMode:
		hints = []struct{ key, desc string }{
			{"si+⏎", "confirmar"}, {"Esc", "cancelar"},
		}
	case cleaningMode:
		hints = []struct{ key, desc string }{{"⏎", "volver"}}
	default:
		// Full hint set for wide terminals.
		hints = []struct{ key, desc string }{
			{"↑↓", "nav"}, {"Tab", "panel"}, {"⏎", "ejecutar"},
			{"a", "analyze"}, {"c", "clean"}, {"p", "preview"},
			{"/", "filtrar"}, {"q", "salir"},
		}
		// On an 80-column (or narrower) terminal the full set overflows, so drop
		// the self-evident navigation hints and keep the action shortcuts plus
		// the all-important "q salir" — never truncate that away.
		if width > 0 && width < 90 {
			hints = []struct{ key, desc string }{
				{"⏎", "ejecutar"}, {"a", "analyze"}, {"c", "clean"},
				{"p", "preview"}, {"/", "filtrar"}, {"q", "salir"},
			}
		}
	}
	parts := make([]string, 0, len(hints))
	for _, h := range hints {
		parts = append(parts, footerKeyStyle.Render(h.key)+" "+footerDescStyle.Render(h.desc))
	}
	joined := footerStyle.Render(strings.Join(parts, sepStyle.Render(" · ")))
	if width > 0 {
		joined = truncateToWidth(joined, width)
	}
	return joined
}

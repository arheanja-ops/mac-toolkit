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
}

// newModel builds the initial model with the read-only action set. It does not
// size the panes yet; that happens on the first WindowSizeMsg.
func newModel(version string) model {
	items := buildListItems(buildActions())

	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = false

	l := list.New(items, delegate, 0, 0)
	l.Title = "ACTIONS"
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
// the viewport takes the rest; both leave room for header and footer.
func (m model) onResize(msg tea.WindowSizeMsg) model {
	m.width, m.height = msg.Width, msg.Height
	leftW := msg.Width / 4
	if leftW < 18 {
		leftW = 18
	}
	rightW := msg.Width - leftW - 4
	bodyH := msg.Height - 4
	if bodyH < 3 {
		bodyH = 3
	}
	m.actions.SetSize(leftW, bodyH)
	if !m.ready {
		m.detail = viewport.New(rightW, bodyH)
		m.detail.SetContent(detailHint)
		m.ready = true
	} else {
		m.detail.Width, m.detail.Height = rightW, bodyH
	}
	return m
}

// onActionResult renders an action's output or error into the detail viewport.
// An error is shown in the panel and never terminates the program.
func (m model) onActionResult(msg actionResultMsg) model {
	m.running = false
	if msg.err != nil {
		m.detail.SetContent("Error: " + msg.err.Error())
	} else {
		m.detail.SetContent(msg.out)
	}
	m.detail.GotoTop()
	return m
}

// onKey routes key presses based on focus and filtering state. Quit and Tab
// are global; everything else is delegated to the focused pane.
func (m model) onKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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
// executable, so selecting one is a no-op.
func (m model) execSelected() (tea.Model, tea.Cmd) {
	it, ok := m.actions.SelectedItem().(listItem)
	if !ok || it.header || it.action.run == nil {
		return m, nil
	}
	m.running = true
	run := it.action.run
	return m, tea.Batch(m.spinner.Tick, func() tea.Msg {
		out, err := run()
		return actionResultMsg{out: out, err: err}
	})
}

func togglePanel(p panel) panel {
	if p == actionsPanel {
		return detailPanel
	}
	return actionsPanel
}

const detailHint = "⏎ para ejecutar la acción seleccionada"

var (
	focusedBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("62"))
	blurredBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("240"))
	footerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
)

// View renders header + two bordered panes + footer. Render logic only; state
// never mutates here.
func (m model) View() string {
	if !m.ready {
		return "Loading…"
	}

	header := renderHeader(m.version, m.disk, m.hdr, m.width)

	detail := m.detail.View()
	if m.running {
		detail = m.spinner.View() + " running…\n\n" + detail
	}

	left := paneStyle(m.focus == actionsPanel).Render(m.actions.View())
	right := paneStyle(m.focus == detailPanel).Render(detail)
	body := lipgloss.JoinHorizontal(lipgloss.Top, left, right)

	footer := footerStyle.Render("↑↓ nav · Tab foco · ⏎ ejecutar · / filtrar · q salir")

	return strings.Join([]string{header, body, footer}, "\n")
}

func paneStyle(focused bool) lipgloss.Style {
	if focused {
		return focusedBorder
	}
	return blurredBorder
}

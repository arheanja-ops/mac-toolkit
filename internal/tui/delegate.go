package tui

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// actionDelegate is a compact, single-line list delegate that renders the
// professional left-panel hierarchy:
//   - group headers (DISK/MONITORS/REPORTS/SESSION) as faint uppercase labels
//     with a blank line of separation above them;
//   - action items with a leading selection caret (▸) in the accent color when
//     selected, and the label bold on the selected row only.
//
// We implement list.ItemDelegate directly rather than tweaking the default
// delegate because the default has no concept of non-selectable headers and
// insists on a two-space gutter that fought with our caret alignment.
type actionDelegate struct{}

func (d actionDelegate) Height() int                             { return 1 }
func (d actionDelegate) Spacing() int                            { return 0 }
func (d actionDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

// Render draws one row. index==m.Index() marks the cursor row.
func (d actionDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	it, ok := item.(listItem)
	if !ok {
		return
	}

	if it.header {
		// Blank spacer above every group except the first keeps the groups
		// visually distinct without drawing separator rules.
		prefix := "\n"
		if index == 0 {
			prefix = ""
		}
		fmt.Fprint(w, prefix+groupHeaderStyle.Render(strings.ToUpper(it.action.group)))
		return
	}

	selected := index == m.Index()
	caret := "  "
	label := it.action.label
	if selected {
		caret = caretStyle.Render("▸ ")
		label = selectedItemStyle.Render(label)
	} else {
		label = itemStyle.Render(label)
	}
	fmt.Fprint(w, caret+label)
}

// Row styles live next to the delegate that uses them.
var (
	caretStyle        = lipgloss.NewStyle().Foreground(accent).Bold(true)
	selectedItemStyle = lipgloss.NewStyle().Foreground(accent).Bold(true)
	itemStyle         = lipgloss.NewStyle().PaddingLeft(0)
)

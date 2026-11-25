package tui

import (
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// The left menu keeps its state (items, selection, filter) in a bubbles
// list.Model but does NOT use the list's own renderer: the list paginates by
// item count, so a page could start in the middle of a category with no header,
// and its blank spacers were not counted in its height. renderMenu draws the
// rows itself so the window always fits and always says which category the
// first visible row belongs to.

// stateDelegate satisfies list.ItemDelegate for the list's bookkeeping only;
// the model never calls list.View(), so Render writes nothing.
type stateDelegate struct{}

func (stateDelegate) Height() int                                  { return 1 }
func (stateDelegate) Spacing() int                                 { return 0 }
func (stateDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd      { return nil }
func (stateDelegate) Render(io.Writer, list.Model, int, list.Item) {}

// Row styles of the menu.
var (
	caretStyle        = lipgloss.NewStyle().Foreground(accent).Bold(true)
	selectedItemStyle = lipgloss.NewStyle().Foreground(accent).Bold(true)
	itemStyle         = lipgloss.NewStyle().PaddingLeft(0)
)

// menuRow is one drawn line: a category header, an item, or a blank spacer.
type menuRow struct {
	kind  rowKind
	group string // category of a header or item (used for the sticky header)
	label string
	index int // position in the visible items, for items
}

type rowKind int

const (
	rowBlank rowKind = iota
	rowHeader
	rowItem
)

// menuRows flattens the visible list items into rows. When spacers is true a
// blank row separates categories.
func menuRows(items []list.Item, spacers bool) []menuRow {
	var rows []menuRow
	for i, it := range items {
		li, ok := it.(listItem)
		if !ok {
			continue
		}
		if li.header {
			if spacers && len(rows) > 0 {
				rows = append(rows, menuRow{kind: rowBlank})
			}
			rows = append(rows, menuRow{kind: rowHeader, group: li.action.group})
			continue
		}
		rows = append(rows, menuRow{kind: rowItem, group: li.action.group, label: li.action.label, index: i})
	}
	return rows
}

// menuWindow picks the rows to draw in h lines for the cursor at item index
// cur. The full list is used when it fits, with category spacers only if they
// fit too; otherwise the window follows the cursor and the first line is
// replaced by the header of its category (a "sticky" header), so the user
// always knows where they are. A header stranded on the last line, with its
// items cut off, is blanked.
func menuWindow(items []list.Item, cur, h int) []menuRow {
	if h < 1 {
		return nil
	}
	if spaced := menuRows(items, true); len(spaced) <= h {
		return spaced
	}
	rows := menuRows(items, false)
	if len(rows) <= h {
		return rows
	}
	at := 0
	for i, r := range rows {
		if r.kind == rowItem && r.index == cur {
			at = i
		}
	}
	top := max(0, min(at-h/2, len(rows)-h))
	win := append([]menuRow(nil), rows[top:top+h]...)
	if win[0].kind == rowItem && h > 1 {
		win[0] = menuRow{kind: rowHeader, group: win[0].group}
	}
	if last := len(win) - 1; win[last].kind == rowHeader && last > 0 {
		win[last] = menuRow{kind: rowBlank}
	}
	return win
}

// renderMenu draws the menu in h lines of width w. filterLine, when non-empty,
// takes the first line (the live filter input or the applied-filter hint).
func renderMenu(items []list.Item, cur, w, h int, filterLine string) string {
	var lines []string
	if filterLine != "" {
		lines = append(lines, truncateToWidth(filterLine, w))
		h--
	}
	for _, r := range menuWindow(items, cur, h) {
		lines = append(lines, truncateToWidth(renderMenuRow(r, cur), w))
	}
	return strings.Join(lines, "\n")
}

func renderMenuRow(r menuRow, cur int) string {
	switch r.kind {
	case rowHeader:
		return groupHeaderStyle.Render(strings.ToUpper(r.group))
	case rowItem:
		if r.index == cur {
			return caretStyle.Render("▸ ") + selectedItemStyle.Render(r.label)
		}
		return "  " + itemStyle.Render(r.label)
	default:
		return ""
	}
}

// isHeaderAt reports whether the visible item at i is a category header.
func isHeaderAt(items []list.Item, i int) bool {
	if i < 0 || i >= len(items) {
		return true
	}
	li, ok := items[i].(listItem)
	return !ok || li.header
}

// stepMenu moves the selection by dir (+1/-1) to the next selectable item,
// skipping category headers. At either end it stays put.
func stepMenu(l list.Model, dir int) list.Model {
	items := l.VisibleItems()
	for i := l.Index() + dir; i >= 0 && i < len(items); i += dir {
		if !isHeaderAt(items, i) {
			l.Select(i)
			return l
		}
	}
	return l
}

// edgeMenu jumps to the first (dir<0) or last (dir>0) selectable item.
func edgeMenu(l list.Model, dir int) list.Model {
	items := l.VisibleItems()
	start := 0
	if dir > 0 {
		start = len(items) - 1
	}
	for i := start; i >= 0 && i < len(items); i -= dir {
		if !isHeaderAt(items, i) {
			l.Select(i)
			return l
		}
	}
	return l
}

// pageMenu moves n selectable items in direction dir.
func pageMenu(l list.Model, dir, n int) list.Model {
	for i := 0; i < n; i++ {
		next := stepMenu(l, dir)
		if next.Index() == l.Index() {
			break
		}
		l = next
	}
	return l
}

// menuFilterLine is the first menu line while filtering: the live input, or a
// hint showing the applied filter.
func menuFilterLine(l list.Model) string {
	switch l.FilterState() {
	case list.Filtering:
		return l.FilterInput.View()
	case list.FilterApplied:
		return hdrLabelStyle.Render("filtro: " + l.FilterValue())
	default:
		return ""
	}
}

package tui

import (
	"fmt"
	"strings"

	"github.com/arheanja-ops/mac-toolkit/internal/core"

	"github.com/charmbracelet/lipgloss"
)

// welcomeArt is a compact ASCII banner shown on open. It is ported from
// cmd/menu.go printBanner but kept plain (no ANSI) because the viewport renders
// raw text; lipgloss styling is applied by the surrounding pane border.
const welcomeArt = `   __  ___          ______          ____   _ __
  /  |/  /__ _ ____/_  __/__  ___  / / /__(_) /_
 / /|_/ / _ '/ __/ / / / _ \/ _ \/ /  '_/ / __/
/_/  /_/\_,_/\__/ /_/  \___/\___/_/_/\_\_/\__/`

// welcomeContent builds the initial detail-panel dashboard: a styled banner, a
// live system summary table, and a navigation hint. It takes the disk and
// header stats already held by the model so it reflects the same live values as
// the header without issuing extra reads (the header tick keeps them fresh).
// width is the detail pane's inner width: the banner is shown only when it fits,
// otherwise a compact title is used so the dashboard never overflows an 80x24
// terminal or wraps the art.
func welcomeContent(version string, disk core.DiskSnapshot, s headerStats, width int) string {
	w := usableWidth(width)
	var b strings.Builder
	if lipgloss.Width(welcomeArt) <= w {
		b.WriteString(bannerStyle.Render(welcomeArt))
		b.WriteString("\n\n")
	}
	b.WriteString(truncateToWidth(fmt.Sprintf("%s %s %s",
		lipgloss.NewStyle().Bold(true).Render("macOS DevOps Toolkit"),
		sepStyle.Render("·"), version), w) + "\n")
	ruleW := min(46, w)
	b.WriteString(rule(ruleW) + "\n\n")

	b.WriteString(tableHeaderStyle.Render("System summary") + "\n")
	welcomeSummary(&b, disk, s, w)

	b.WriteString("\nSelect an action on the left and press ")
	b.WriteString(footerKeyStyle.Render("⏎") + " to run it.\n")
	b.WriteString(footerDescStyle.Render(
		truncateToWidth("Tab switches focus · / filters · q exits.", w)) + "\n")
	return b.String()
}

// welcomeSummary writes the disk/memory/CPU lines, degrading field by field so
// a failed read never breaks the dashboard. Lines are truncated to w so a
// narrow pane never wraps or clips mid-word. Split out to keep welcomeContent
// under the length limit and the summary easy to test.
func welcomeSummary(b *strings.Builder, disk core.DiskSnapshot, s headerStats, w int) {
	if disk.OK {
		// Narrow the proportion bar on tight panes so the "free of TOTAL" tail
		// still fits on one line instead of being clipped by the viewport.
		barW := 20
		if w < 52 {
			barW = 10
		}
		bar := core.ProgressBar(disk.PctUsed, barW)
		line := fmt.Sprintf("%s %s  %d%%  %s free of %s",
			hdrLabelStyle.Render("Disk   "), bar, disk.PctUsed,
			core.FormatBytes(disk.AvailBytes), core.FormatBytes(disk.TotalBytes))
		b.WriteString(truncateToWidth(line, w) + "\n")
	} else {
		fmt.Fprintf(b, "%s (unavailable)\n", hdrLabelStyle.Render("Disk   "))
	}
	if s.ok {
		fmt.Fprintf(b, "%s %d%% used\n", hdrLabelStyle.Render("Memory "), s.memUsedPct)
		fmt.Fprintf(b, "%s %d%% used\n", hdrLabelStyle.Render("CPU    "), s.cpuUsedPct)
		if s.thermal != "" {
			fmt.Fprintf(b, "%s %s\n", hdrLabelStyle.Render("Thermal"), renderThermal(s.thermal))
		}
	} else {
		fmt.Fprintf(b, "%s (unavailable)\n", hdrLabelStyle.Render("System "))
	}
}

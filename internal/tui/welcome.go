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
func welcomeContent(version string, disk core.DiskSnapshot, s headerStats) string {
	var b strings.Builder
	b.WriteString(bannerStyle.Render(welcomeArt))
	b.WriteString("\n\n")
	fmt.Fprintf(&b, "  %s  %s  %s\n",
		lipgloss.NewStyle().Bold(true).Render("macOS DevOps Toolkit"),
		sepStyle.Render("·"), version)
	b.WriteString("  " + ruleStyle.Render(strings.Repeat("─", 46)) + "\n\n")

	b.WriteString("  " + tableHeaderStyle.Render("System summary") + "\n")
	if disk.OK {
		bar := core.ProgressBar(disk.PctUsed, 20)
		fmt.Fprintf(&b, "  %s %s  %d%%  %s free of %s\n",
			hdrLabelStyle.Render("Disk   "), bar, disk.PctUsed,
			core.FormatBytes(disk.AvailBytes), core.FormatBytes(disk.TotalBytes))
	} else {
		fmt.Fprintf(&b, "  %s (unavailable)\n", hdrLabelStyle.Render("Disk   "))
	}
	if s.ok {
		fmt.Fprintf(&b, "  %s %d%% used\n", hdrLabelStyle.Render("Memory "), s.memUsedPct)
		fmt.Fprintf(&b, "  %s %d%% used\n", hdrLabelStyle.Render("CPU    "), s.cpuUsedPct)
		if s.thermal != "" {
			fmt.Fprintf(&b, "  %s %s\n", hdrLabelStyle.Render("Thermal"), renderThermal(s.thermal))
		}
	} else {
		fmt.Fprintf(&b, "  %s (unavailable)\n", hdrLabelStyle.Render("System "))
	}

	b.WriteString("\n  Select an action on the left and press ")
	b.WriteString(footerKeyStyle.Render("⏎") + " to run it.\n")
	b.WriteString("  " + footerDescStyle.Render(
		"Tab switches focus · / filters · Quit or q exits.") + "\n")
	return b.String()
}

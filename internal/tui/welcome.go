package tui

import (
	"fmt"
	"strings"

	"github.com/arheanja-ops/mac-toolkit/internal/core"
)

// welcomeArt is a compact ASCII banner shown on open. It is ported from
// cmd/menu.go printBanner but kept plain (no ANSI) because the viewport renders
// raw text; lipgloss styling is applied by the surrounding pane border.
const welcomeArt = `   __  ___          ______          ____   _ __
  /  |/  /__ _ ____/_  __/__  ___  / / /__(_) /_
 / /|_/ / _ '/ __/ / / / _ \/ _ \/ /  '_/ / __/
/_/  /_/\_,_/\__/ /_/  \___/\___/_/_/\_\_/\__/`

// welcomeContent builds the initial detail-panel dashboard: a banner, a live
// system summary, and a navigation hint. It takes the disk and header stats
// already held by the model so it reflects the same live values as the header
// without issuing extra reads (the header tick keeps them fresh).
func welcomeContent(version string, disk core.DiskSnapshot, s headerStats) string {
	var b strings.Builder
	b.WriteString(welcomeArt)
	b.WriteString("\n\n")
	fmt.Fprintf(&b, "  macOS DevOps Toolkit  ·  %s\n", version)
	b.WriteString("  " + strings.Repeat("─", 46) + "\n\n")

	b.WriteString("  System summary\n")
	if disk.OK {
		bar := core.ProgressBar(disk.PctUsed, 20)
		fmt.Fprintf(&b, "  Disk    %s  %d%%  %s free of %s\n",
			bar, disk.PctUsed,
			core.FormatBytes(disk.AvailBytes), core.FormatBytes(disk.TotalBytes))
	} else {
		b.WriteString("  Disk    (unavailable)\n")
	}
	if s.ok {
		fmt.Fprintf(&b, "  Memory  %d%% used\n", s.memUsedPct)
		fmt.Fprintf(&b, "  CPU     %d%% used\n", s.cpuUsedPct)
		if s.thermal != "" {
			fmt.Fprintf(&b, "  Thermal %s\n", s.thermal)
		}
	} else {
		b.WriteString("  System  (unavailable)\n")
	}

	b.WriteString("\n  Select an action on the left and press ⏎ to run it.\n")
	b.WriteString("  Tab switches focus · / filters · Quit or q exits.\n")
	return b.String()
}

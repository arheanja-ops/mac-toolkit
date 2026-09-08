package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/arheanja-ops/mac-toolkit/internal/core"
	"github.com/manifoldco/promptui"
)

// menuOption is a single selectable action in the interactive menu.
// A nil fn with a label starting with a section glyph marks a non-selectable
// group header.
type menuOption struct {
	label    string
	fn       func()
	isHeader bool
}

// menuOptions is the ordered, grouped list shown in the arrow-key menu.
var menuOptions = []menuOption{
	{label: "DISK", isHeader: true},
	{label: "🔍  Analyze          full disk scan across all domains", fn: func() { analyzeCmd.Run(analyzeCmd, nil) }},
	{label: "🔎  Preview          dry-run cleanup, shows risks", fn: func() { runClean("", cleanMode, false) }},
	{label: "🧹  Clean            delete with interactive approval", fn: func() { runClean("", cleanMode, true) }},
	{label: "📊  Full flow        analyze + save report + clean", fn: func() { fullCmd.Run(fullCmd, nil) }},
	{label: "📋  Status           registered domains & risk levels", fn: func() { statusCmd.Run(statusCmd, nil) }},

	{label: "MONITORS", isHeader: true},
	{label: "🔋  Battery          health, cycles, temperature", fn: func() { batteryCmd.Run(batteryCmd, nil) }},
	{label: "💻  System           CPU, memory, swap, thermal", fn: func() { systemCmd.Run(systemCmd, nil) }},
	{label: "⚙️   Processes        top consumers by CPU / memory", fn: func() { processesCmd.Run(processesCmd, nil) }},
	{label: "🌐  Network          WiFi, traffic, connectivity", fn: func() { networkCmd.Run(networkCmd, nil) }},

	{label: "REPORTS", isHeader: true},
	{label: "📄  Last report      view most recent saved report", fn: func() { reportCmd.Run(reportCmd, nil) }},

	{label: "❌  Quit", fn: nil},
}

// printBanner renders the ASCII header plus a live disk snapshot line.
func printBanner() {
	art := `
   __  ___          ______          ____   _ __
  /  |/  /__ _ ____/_  __/__  ___  / / /__(_) /_
 / /|_/ / _ '/ __/ / / / _ \/ _ \/ /  '_/ / __/
/_/  /_/\_,_/\__/ /_/  \___/\___/_/_/\_\_/\__/`

	fmt.Fprintln(os.Stderr, core.Colorize(core.Cyan, art))

	sub := fmt.Sprintf("  macOS DevOps Toolkit  ·  %s", version)
	fmt.Fprintln(os.Stderr, core.Colorize(core.Gray, sub))

	if d := core.ReadDiskSnapshot(); d.OK {
		bar := core.ProgressBar(d.PctUsed, 24)
		line := fmt.Sprintf("  Disk  %s  %d%%   %s free of %s",
			bar, d.PctUsed,
			core.FormatBytes(d.AvailBytes), core.FormatBytes(d.TotalBytes))
		fmt.Fprintln(os.Stderr, line)
	}
	fmt.Fprintln(os.Stderr, core.Colorize(core.Gray, "  "+strings.Repeat("─", 46)))
	fmt.Fprintln(os.Stderr)
}

func runMenu() {
	printBanner()

	labels := make([]string, len(menuOptions))
	for i, o := range menuOptions {
		if o.isHeader {
			// Render headers as dim, bold section dividers.
			dashes := 44 - len(o.label)
			if dashes < 0 {
				dashes = 0
			}
			labels[i] = core.Bold_ + core.Cyan + "── " + o.label + " " +
				strings.Repeat("─", dashes) + core.Reset
		} else {
			labels[i] = o.label
		}
	}

	templates := &promptui.SelectTemplates{
		Label:    "{{ . }}",
		Active:   "  ▸ {{ . | cyan | bold }}",
		Inactive: "    {{ . }}",
		Selected: "  ▸ {{ . | green }}",
	}

	for {
		prompt := promptui.Select{
			Label:     "Select an action  (↑↓ to move · ⏎ to run · ^C to quit)",
			Items:     labels,
			Size:      len(labels),
			HideHelp:  true,
			Templates: templates,
		}

		idx, _, err := prompt.Run()
		if err != nil {
			fmt.Fprintln(os.Stderr, core.Colorize(core.Cyan, "👋 Bye!"))
			return
		}

		opt := menuOptions[idx]

		// Non-selectable header: ignore and re-prompt.
		if opt.isHeader {
			continue
		}
		if opt.fn == nil {
			fmt.Fprintln(os.Stderr, core.Colorize(core.Cyan, "👋 Bye!"))
			return
		}

		fmt.Fprintln(os.Stderr)
		opt.fn()
		fmt.Fprintln(os.Stderr)
	}
}

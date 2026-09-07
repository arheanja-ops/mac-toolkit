package cmd

import (
	"fmt"

	"github.com/manifoldco/promptui"
)

// menuOption is a single selectable action in the interactive menu.
type menuOption struct {
	label string
	fn    func()
}

// menuOptions is the ordered, grouped list shown in the arrow-key menu.
// Separators (fn == nil, label starts with "─") are non-selectable headers.
var menuOptions = []menuOption{
	{label: "── Disk ──"},
	{"🔍  Analyze disk (all domains)", func() { analyzeCmd.Run(analyzeCmd, nil) }},
	{"🔎  Cleanup preview (dry-run, shows risks)", func() { runClean("", cleanMode, false) }},
	{"🧹  Clean disk (interactive approval)", func() { runClean("", cleanMode, true) }},
	{"📊  Full flow (analyze + save + clean)", func() { fullCmd.Run(fullCmd, nil) }},
	{"📋  Domain status & risk levels", func() { statusCmd.Run(statusCmd, nil) }},
	{label: "── Monitors ──"},
	{"🔋  Battery health", func() { batteryCmd.Run(batteryCmd, nil) }},
	{"💻  System (CPU / memory)", func() { systemCmd.Run(systemCmd, nil) }},
	{"⚙️   Processes (top CPU / mem)", func() { processesCmd.Run(processesCmd, nil) }},
	{"🌐  Network", func() { networkCmd.Run(networkCmd, nil) }},
	{label: "── Reports ──"},
	{"📄  View last report", func() { reportCmd.Run(reportCmd, nil) }},
	{label: "──────────"},
	{"❌  Quit", nil},
}

func runMenu() {
	labels := make([]string, len(menuOptions))
	for i, o := range menuOptions {
		labels[i] = o.label
	}

	for {
		prompt := promptui.Select{
			Label:    "What would you like to do?",
			Items:    labels,
			Size:     len(labels),
			HideHelp: true,
			Templates: &promptui.SelectTemplates{
				Active:   "» {{ . | cyan }}",
				Inactive: "  {{ . }}",
				Selected: "» {{ . | green }}",
			},
		}

		idx, _, err := prompt.Run()
		if err != nil {
			// Ctrl-C / interrupt → exit cleanly.
			fmt.Println("\033[32m👋 Bye!\033[0m")
			return
		}

		opt := menuOptions[idx]

		// Non-selectable separator: ignore and re-prompt.
		if opt.fn == nil && opt.label != "❌  Quit" {
			continue
		}
		if opt.fn == nil {
			fmt.Println("\033[32m👋 Bye!\033[0m")
			return
		}
		opt.fn()
	}
}

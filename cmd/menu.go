package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

var menuOptions = []struct {
	label string
	fn    func()
}{
	{"Analyze disk", func() { analyzeCmd.Run(analyzeCmd, nil) }},
	{"Clean disk", func() { cleanCmd.Run(cleanCmd, nil) }},
	{"Full analysis + clean", func() { fullCmd.Run(fullCmd, nil) }},
	{"Domain status", func() { statusCmd.Run(statusCmd, nil) }},
	{"Battery monitor", func() { batteryCmd.Run(batteryCmd, nil) }},
	{"System monitor", func() { systemCmd.Run(systemCmd, nil) }},
	{"Process monitor", func() { processesCmd.Run(processesCmd, nil) }},
	{"Network monitor", func() { networkCmd.Run(networkCmd, nil) }},
	{"View last report", func() { reportCmd.Run(reportCmd, nil) }},
	{"Quit", nil},
}

func runMenu() {
	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Println()
		fmt.Println("\033[1m🖥  Mac DevOps Toolkit Pro\033[0m")
		fmt.Println(strings.Repeat("─", 35))

		for i, opt := range menuOptions {
			fmt.Printf("  \033[36m[%d]\033[0m %s\n", i+1, opt.label)
		}

		fmt.Print("\nSelect option: ")
		input, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		input = strings.TrimSpace(input)

		// Parse number
		var choice int
		if _, err := fmt.Sscanf(input, "%d", &choice); err != nil || choice < 1 || choice > len(menuOptions) {
			fmt.Println("\033[31mInvalid option\033[0m")
			continue
		}

		opt := menuOptions[choice-1]
		if opt.fn == nil {
			fmt.Println("\033[32m👋 Bye!\033[0m")
			return
		}
		opt.fn()
	}
}

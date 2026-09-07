---
name: add-monitor
description: Add a new system monitor to mac-toolkit
---

# Add Monitor

Add a new system monitor called `$ARGUMENTS` to the mac-toolkit project.

Steps:
1. Read mac-toolkit/internal/monitor/monitor.go for the Monitor interface
2. Read an existing monitor (e.g., mac-toolkit/internal/monitor/battery.go) as template
3. Create mac-toolkit/internal/monitor/$ARGUMENTS.go implementing:
   - Name() returning the monitor name
   - Snapshot() returning (map[string]any, error) with collected metrics
   - Display() formatting and printing the metrics with ANSI colors
4. Add a cobra command in mac-toolkit/cmd/monitors.go:
   - Create var ${ARGUMENTS}Cmd with Use, Short, and Run
   - Register with rootCmd.AddCommand in init()
5. Add the option to mac-toolkit/cmd/menu.go menuOptions
6. Run `go build ./...` and `go vet ./...` to verify

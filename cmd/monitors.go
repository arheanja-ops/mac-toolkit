package cmd

import (
	"github.com/arheanja-ops/mac-toolkit/internal/core"
	"github.com/arheanja-ops/mac-toolkit/internal/monitor"

	"github.com/spf13/cobra"
)

var batteryCmd = &cobra.Command{
	Use:   "battery",
	Short: "Battery health, cycles, temperature",
	Run: func(cmd *cobra.Command, args []string) {
		m := &monitor.BatteryMonitor{}
		if err := m.Display(); err != nil {
			core.Error("Battery: %v", err)
		}
	},
}

var systemCmd = &cobra.Command{
	Use:   "system",
	Short: "CPU, memory, swap, thermal state",
	Run: func(cmd *cobra.Command, args []string) {
		m := &monitor.SystemMonitor{}
		if err := m.Display(); err != nil {
			core.Error("System: %v", err)
		}
	},
}

var processesCmd = &cobra.Command{
	Use:   "processes",
	Short: "Top CPU and memory consuming processes",
	Run: func(cmd *cobra.Command, args []string) {
		m := &monitor.ProcessMonitor{}
		if err := m.Display(); err != nil {
			core.Error("Processes: %v", err)
		}
	},
}

var networkCmd = &cobra.Command{
	Use:   "network",
	Short: "WiFi, network stats, connectivity",
	Run: func(cmd *cobra.Command, args []string) {
		m := &monitor.NetworkMonitor{}
		if err := m.Display(); err != nil {
			core.Error("Network: %v", err)
		}
	},
}

func init() {
	rootCmd.AddCommand(batteryCmd)
	rootCmd.AddCommand(systemCmd)
	rootCmd.AddCommand(processesCmd)
	rootCmd.AddCommand(networkCmd)
}

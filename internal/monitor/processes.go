package monitor

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

type ProcessMonitor struct{}

func (p *ProcessMonitor) Name() string { return "processes" }

type procInfo struct {
	PID  int
	CPU  float64
	Mem  float64
	RSS  int64 // KB
	Name string
}

func (p *ProcessMonitor) Snapshot() (map[string]any, error) {
	out, err := cmdC("ps", "-arcwwwxo", "pid,pcpu,pmem,rss,comm").Output()
	if err != nil {
		return nil, fmt.Errorf("ps failed: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var procs []procInfo
	for _, line := range lines[1:] { // skip header
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		pid, _ := strconv.Atoi(fields[0])
		cpu, _ := strconv.ParseFloat(fields[1], 64)
		mem, _ := strconv.ParseFloat(fields[2], 64)
		rss, _ := strconv.ParseInt(fields[3], 10, 64)
		name := strings.Join(fields[4:], " ")
		procs = append(procs, procInfo{PID: pid, CPU: cpu, Mem: mem, RSS: rss, Name: name})
	}

	// Top 10 by CPU
	sort.Slice(procs, func(i, j int) bool { return procs[i].CPU > procs[j].CPU })
	topCPU := procs
	if len(topCPU) > 10 {
		topCPU = topCPU[:10]
	}

	// Top 10 by memory — need a copy since sort is in-place
	memProcs := make([]procInfo, len(procs))
	copy(memProcs, procs)
	sort.Slice(memProcs, func(i, j int) bool { return memProcs[i].Mem > memProcs[j].Mem })
	topMem := memProcs
	if len(topMem) > 10 {
		topMem = topMem[:10]
	}

	// Memory pressure
	pressure := "unknown"
	if mpOut, err := cmdC("memory_pressure").Output(); err == nil {
		for _, line := range strings.Split(string(mpOut), "\n") {
			if strings.Contains(line, "System-wide memory free percentage") {
				pressure = strings.TrimSpace(line)
				break
			}
		}
	}

	return map[string]any{
		"top_cpu":         topCPU,
		"top_memory":      topMem,
		"memory_pressure": pressure,
	}, nil
}

func (p *ProcessMonitor) Display() error {
	data, err := p.Snapshot()
	if err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("\033[1m⚡ Top Processes by CPU\033[0m")
	fmt.Println(strings.Repeat("─", 60))
	fmt.Printf("  %-8s %-8s %-8s %-10s %s\n", "PID", "CPU%", "MEM%", "RSS", "NAME")

	if topCPU, ok := data["top_cpu"].([]procInfo); ok {
		for _, proc := range topCPU {
			color := "\033[32m"
			if proc.CPU >= 50 {
				color = "\033[31m"
			} else if proc.CPU >= 20 {
				color = "\033[33m"
			}
			fmt.Printf("  %-8d %s%-8.1f\033[0m %-8.1f %-10s %s\n",
				proc.PID, color, proc.CPU, proc.Mem, formatKB(proc.RSS), proc.Name)
		}
	}

	fmt.Println()
	fmt.Println("\033[1m🧠 Top Processes by Memory\033[0m")
	fmt.Println(strings.Repeat("─", 60))
	fmt.Printf("  %-8s %-8s %-8s %-10s %s\n", "PID", "CPU%", "MEM%", "RSS", "NAME")

	if topMem, ok := data["top_memory"].([]procInfo); ok {
		for _, proc := range topMem {
			color := "\033[32m"
			if proc.Mem >= 10 {
				color = "\033[31m"
			} else if proc.Mem >= 5 {
				color = "\033[33m"
			}
			fmt.Printf("  %-8d %-8.1f %s%-8.1f\033[0m %-10s %s\n",
				proc.PID, proc.CPU, color, proc.Mem, formatKB(proc.RSS), proc.Name)
		}
	}

	if mp, ok := data["memory_pressure"]; ok {
		fmt.Printf("\n  %v\n", mp)
	}

	fmt.Println(strings.Repeat("─", 60))
	return nil
}

func formatKB(kb int64) string {
	if kb >= 1048576 {
		return fmt.Sprintf("%.1fG", float64(kb)/1048576)
	}
	if kb >= 1024 {
		return fmt.Sprintf("%.0fM", float64(kb)/1024)
	}
	return fmt.Sprintf("%dK", kb)
}

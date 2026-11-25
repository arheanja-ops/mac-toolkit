package monitor

import (
	"fmt"
	"sort"
	"strings"
)

// topProcsN is how many processes each Processes list keeps.
const topProcsN = 15

// ProcessMonitor is a read-only view of the busiest processes, their per-app
// totals and the process-state summary. It never signals a process.
type ProcessMonitor struct{}

func (p *ProcessMonitor) Name() string { return "processes" }

// ProcInfo is one process as the LOCAL views show it. The PID is meant for the
// user's own screen and CLI only: nothing that is serialized for a model or
// logged carries this type (the AI snapshot reads names and percentages only).
type ProcInfo struct {
	PID   int     `json:"pid"`
	CPU   float64 `json:"cpu_pct"`
	Mem   float64 `json:"mem_pct"`
	RSSKB int64   `json:"rss_kb"`
	Name  string  `json:"name"`
}

// ProcSummary counts processes by state. Threads are not counted: ps needs one
// extra full pass per thread to list them, which is not worth the cost here.
type ProcSummary struct {
	Total    int `json:"total"`
	Running  int `json:"running"`
	Sleeping int `json:"sleeping"`
	Stopped  int `json:"stopped"`
	Zombie   int `json:"zombie"`
	IOWait   int `json:"io_wait"` // uninterruptible wait, usually disk/memory
}

// Snapshot returns the lists as typed values (not formatted text), so each
// front-end renders its own table.
//
//	top_cpu, top_memory  []ProcInfo   top 15 by %CPU / by RSS
//	apps                 []AppUsage    per-app totals, largest RAM first
//	summary              ProcSummary   counts by state
//	pressure_level       string        kernel memory pressure label
//	memory_pressure      string        "free percentage" line, for the MCP tool
func (p *ProcessMonitor) Snapshot() (map[string]any, error) {
	entries, err := readProcTable()
	if err != nil {
		return nil, fmt.Errorf("ps failed: %w", err)
	}
	procs := make([]ProcInfo, len(entries))
	for i, e := range entries {
		procs[i] = ProcInfo{PID: e.pid, CPU: e.cpu, Mem: e.mem, RSSKB: e.rssKB, Name: procName(e.comm)}
	}
	data := map[string]any{
		"top_cpu":    topProcs(procs, func(a, b ProcInfo) bool { return a.CPU > b.CPU }),
		"top_memory": topProcs(procs, func(a, b ProcInfo) bool { return a.RSSKB > b.RSSKB }),
		"apps":       topApps(aggregateApps(rowsOf(entries)), topProcsN, true),
		"summary":    summarize(entries),
	}
	addProcPressure(data)
	return data, nil
}

// procName is the executable's basename: the views never show a path.
func procName(comm string) string {
	if i := strings.LastIndex(comm, "/"); i >= 0 {
		comm = comm[i+1:]
	}
	if comm == "" {
		return "?"
	}
	return comm
}

// topProcs returns the first topProcsN processes under less. Ties fall back to
// name then PID so the order is deterministic.
func topProcs(procs []ProcInfo, less func(a, b ProcInfo) bool) []ProcInfo {
	sorted := append([]ProcInfo(nil), procs...)
	sort.Slice(sorted, func(i, j int) bool {
		a, b := sorted[i], sorted[j]
		if less(a, b) {
			return true
		}
		if less(b, a) {
			return false
		}
		if a.Name != b.Name {
			return a.Name < b.Name
		}
		return a.PID < b.PID
	})
	if len(sorted) > topProcsN {
		sorted = sorted[:topProcsN]
	}
	return sorted
}

// summarize counts processes by the first letter of their ps state: R running,
// S/I sleeping (I is idle), T stopped, Z zombie, U uninterruptible wait.
func summarize(entries []procEntry) ProcSummary {
	s := ProcSummary{Total: len(entries)}
	for _, e := range entries {
		if e.state == "" {
			continue
		}
		switch e.state[0] {
		case 'R':
			s.Running++
		case 'S', 'I':
			s.Sleeping++
		case 'T':
			s.Stopped++
		case 'Z':
			s.Zombie++
		case 'U':
			s.IOWait++
		}
	}
	return s
}

// addProcPressure records the memory pressure next to the process counts.
func addProcPressure(data map[string]any) {
	data["pressure_level"] = ClassifyMemoryPressure(int(sysctlInt("kern.memorystatus_vm_pressure_level")))
	data["memory_pressure"] = "unknown"
	if out, err := runCmd("memory_pressure"); err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			if strings.Contains(line, "System-wide memory free percentage") {
				data["memory_pressure"] = strings.TrimSpace(line)
				break
			}
		}
	}
}

// Display prints the CLI tables with the top 10 of each list.
func (p *ProcessMonitor) Display() error {
	data, err := p.Snapshot()
	if err != nil {
		return err
	}
	fmt.Println()
	fmt.Println("\033[1m⚡ Top Processes by CPU\033[0m")
	fmt.Println(strings.Repeat("─", 60))
	fmt.Printf("  %-8s %-8s %-8s %-10s %s\n", "PID", "CPU%", "MEM%", "RSS", "NAME")
	printProcs(data["top_cpu"], func(c ProcInfo) (float64, float64) { return c.CPU, 50 }, true)

	fmt.Println()
	fmt.Println("\033[1m🧠 Top Processes by Memory\033[0m")
	fmt.Println(strings.Repeat("─", 60))
	fmt.Printf("  %-8s %-8s %-8s %-10s %s\n", "PID", "CPU%", "MEM%", "RSS", "NAME")
	printProcs(data["top_memory"], func(c ProcInfo) (float64, float64) { return c.Mem, 10 }, false)

	if s, ok := data["summary"].(ProcSummary); ok {
		fmt.Printf("\n  Procesos: %d · running %d · sleeping %d · stopped %d · zombie %d · I/O wait %d\n",
			s.Total, s.Running, s.Sleeping, s.Stopped, s.Zombie, s.IOWait)
	}
	if mp, ok := data["memory_pressure"]; ok {
		fmt.Printf("  %v\n", mp)
	}
	fmt.Println(strings.Repeat("─", 60))
	return nil
}

// printProcs prints up to 10 rows, coloring the metric returned by key (red at
// the given threshold, yellow at 40% of it).
func printProcs(v any, key func(ProcInfo) (val, red float64), byCPU bool) {
	procs, _ := v.([]ProcInfo)
	if len(procs) > 10 {
		procs = procs[:10]
	}
	for _, p := range procs {
		val, red := key(p)
		color := "\033[32m"
		if val >= red {
			color = "\033[31m"
		} else if val >= red*0.4 {
			color = "\033[33m"
		}
		cpu, mem := fmt.Sprintf("%-8.1f", p.CPU), fmt.Sprintf("%-8.1f", p.Mem)
		if byCPU {
			cpu = color + cpu + "\033[0m"
		} else {
			mem = color + mem + "\033[0m"
		}
		fmt.Printf("  %-8d %s %s %-10s %s\n", p.PID, cpu, mem, FormatKB(p.RSSKB), p.Name)
	}
}

// FormatKB renders a size in KB as 512K, 340M or 1.2G.
func FormatKB(kb int64) string {
	if kb >= 1048576 {
		return fmt.Sprintf("%.1fG", float64(kb)/1048576)
	}
	if kb >= 1024 {
		return fmt.Sprintf("%.0fM", float64(kb)/1024)
	}
	return fmt.Sprintf("%dK", kb)
}

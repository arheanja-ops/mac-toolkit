package monitor

import (
	"fmt"
	"strings"
)

// CPUMonitor is a read-only view of CPU time split, load and the busiest apps.
type CPUMonitor struct{}

func (c *CPUMonitor) Name() string { return "cpu" }

// Snapshot takes ~1 s (the iostat interval sample, which costs no CPU). Each
// source degrades independently; the snapshot never errors.
func (c *CPUMonitor) Snapshot() (map[string]any, error) {
	data := map[string]any{}
	if u, s, i, ok := readCPUSplit(); ok {
		data["user_pct"], data["sys_pct"], data["idle_pct"] = u, s, i
		data["kernel_high"] = HighKernelTime(s, u)
	}
	addLoad(data)
	if out, err := runCmd("pmset", "-g", "therm"); err == nil {
		data["thermal"] = parseThermal(string(out))
		if lim, ok := parseSpeedLimit(string(out)); ok {
			data["cpu_speed_limit"] = lim
		}
	} else {
		data["thermal"] = ThermalUnknown
	}
	if rows, err := readProcRows(); err == nil {
		data["top_cpu"] = topApps(aggregateApps(rows), topAppsN, false)
	}
	return data, nil
}

// addLoad records load averages, core counts and load per core. perflevel keys
// only exist on Apple Silicon, so P/E cores are optional.
func addLoad(data map[string]any) {
	cores := int(sysctlInt("hw.ncpu"))
	if cores > 0 {
		data["cores_total"] = cores
	}
	if p := int(sysctlInt("hw.perflevel0.logicalcpu")); p > 0 {
		data["perf_cores"] = p
	}
	if e := int(sysctlInt("hw.perflevel1.logicalcpu")); e > 0 {
		data["eff_cores"] = e
	}
	out, err := runCmd("sysctl", "-n", "vm.loadavg")
	if err != nil {
		return
	}
	l1, l5, l15, ok := parseLoadAvg(string(out))
	if !ok {
		return
	}
	data["load1"], data["load5"], data["load15"] = l1, l5, l15
	if cores > 0 {
		// load_per_core follows the 1-minute figure; the verdict and the doctor
		// use the 5-minute one (load5_per_core) to ignore short bursts.
		data["load_per_core"] = l1 / float64(cores)
		data["load5_per_core"] = l5 / float64(cores)
		data["load15_per_core"] = l15 / float64(cores)
		// Idle is passed so a high load with a free CPU is not called
		// "Saturada"; -1 means the split could not be read.
		idle := -1.0
		if v, ok := data["idle_pct"].(float64); ok {
			idle = v
		}
		data["load_verdict"] = ClassifyCPU(l5, cores, idle)
	}
}

// Display prints the CPU view for the CLI.
func (c *CPUMonitor) Display() error {
	d, err := c.Snapshot()
	if err != nil {
		return err
	}
	fmt.Println()
	fmt.Println("\033[1m⚡ CPU\033[0m")
	fmt.Println(strings.Repeat("─", 50))
	for _, k := range []string{"user", "sys", "idle"} {
		if v, ok := d[k+"_pct"].(float64); ok {
			fmt.Printf("  %-14s %s %5.1f%%\n", k+":", barFor(v), v)
		}
	}
	if _, ok := d["load1"]; ok {
		fmt.Printf("  %-14s %.2f · %.2f · %.2f (1/5/15 min)\n", "Load:",
			num(d, "load1"), num(d, "load5"), num(d, "load15"))
	}
	if v, ok := d["load_verdict"].(string); ok {
		fmt.Printf("  %-14s %.2f por núcleo (5 min) → %s\n", "Per core:", num(d, "load5_per_core"), v)
	}
	printCores(d)
	if t, ok := d["thermal"].(string); ok {
		fmt.Printf("  %-14s %s\n", "Thermal:", t)
	}
	if hi, _ := d["kernel_high"].(bool); hi {
		fmt.Printf("  \033[33m%s\033[0m\n", NoteKernelTime)
	}
	printTopApps("Top apps by CPU", d["top_cpu"], false)
	fmt.Println(strings.Repeat("─", 50))
	return nil
}

func printCores(d map[string]any) {
	total, ok := d["cores_total"].(int)
	if !ok {
		return
	}
	s := fmt.Sprintf("%d", total)
	p, hasP := d["perf_cores"].(int)
	e, hasE := d["eff_cores"].(int)
	if hasP && hasE {
		s += fmt.Sprintf(" (%d P + %d E)", p, e)
	}
	fmt.Printf("  %-14s %s\n", "Cores:", s)
}

// barFor draws a neutral 20-cell bar; it carries no color because for the
// idle share a full bar is good news, so a level-based color would mislead.
func barFor(pct float64) string {
	const w = 20
	n := int(pct * w / 100)
	if n < 0 {
		n = 0
	}
	if n > w {
		n = w
	}
	return strings.Repeat("█", n) + strings.Repeat("░", w-n)
}

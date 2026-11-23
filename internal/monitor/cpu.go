package monitor

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// CPUMonitor is a read-only view of CPU time split, load and the busiest apps.
type CPUMonitor struct{}

func (c *CPUMonitor) Name() string { return "cpu" }

var reCPUUsage = regexp.MustCompile(`CPU usage:\s+([\d.]+)% user,\s+([\d.]+)% sys,\s+([\d.]+)% idle`)

// parseTopCPU returns the user/sys/idle split of the LAST "CPU usage" line.
// `top -l 2` prints two samples: the first is the average since boot (or since
// the previous call) and says nothing about the present; only the second covers
// the interval between the samples, so that is the one we report.
func parseTopCPU(raw string) (user, sys, idle float64, ok bool) {
	all := reCPUUsage.FindAllStringSubmatch(raw, -1)
	if len(all) == 0 {
		return 0, 0, 0, false
	}
	m := all[len(all)-1]
	user, _ = strconv.ParseFloat(m[1], 64)
	sys, _ = strconv.ParseFloat(m[2], 64)
	idle, _ = strconv.ParseFloat(m[3], 64)
	return user, sys, idle, true
}

// Snapshot takes ~2 s because of the second `top` sample. Each source degrades
// independently; the snapshot never errors.
func (c *CPUMonitor) Snapshot() (map[string]any, error) {
	data := map[string]any{}
	if out, err := runCmd("top", "-l", "2", "-n", "0", "-s", "1"); err == nil {
		if u, s, i, ok := parseTopCPU(string(out)); ok {
			data["user_pct"], data["sys_pct"], data["idle_pct"] = u, s, i
			data["kernel_high"] = HighKernelTime(s, u)
		}
	}
	addLoad(data)
	if out, err := runCmd("pmset", "-g", "therm"); err == nil {
		data["thermal"] = parseThermal(string(out))
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
		data["load_verdict"] = ClassifyCPU(l5, cores)
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

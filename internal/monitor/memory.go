package monitor

import (
	"fmt"
	"strings"

	"github.com/arheanja-ops/mac-toolkit/internal/core"
)

// MemoryMonitor is a read-only view of RAM, pressure, swap and the biggest
// apps. It never signals or modifies any process.
type MemoryMonitor struct{}

func (m *MemoryMonitor) Name() string { return "memory" }

// Snapshot collects memory figures. Every optional source degrades on its own
// (a failing sysctl only drops its keys); only a failing vm_stat is an error,
// because without it there is no memory picture at all.
func (m *MemoryMonitor) Snapshot() (map[string]any, error) {
	u, err := readMemUsage()
	if err != nil {
		return nil, err
	}
	data := map[string]any{}
	addMemoryBreakdown(data, u)
	addPressure(data)
	addSwap(data, u)
	addUptime(data)
	addPaging(data, u)
	if rows, err := readProcRows(); err == nil {
		data["top_memory"] = topApps(aggregateApps(rows), topAppsN, true)
	}
	return data, nil
}

func addMemoryBreakdown(data map[string]any, u memUsage) {
	gb := func(b int64) float64 { return float64(b) / gibBytes }
	vm := u.vm
	data["ram_total_gb"] = gb(u.totalBytes)
	data["used_gb"] = gb(u.usedBytes)
	data["used_pct"] = u.pct()
	data["app_gb"] = gb(vm.bytes(vm.appPages()))
	data["wired_gb"] = gb(vm.bytes(vm.Wired))
	data["active_gb"] = gb(vm.bytes(vm.Active))
	data["inactive_gb"] = gb(vm.bytes(vm.Inactive))
	data["compressed_gb"] = gb(vm.bytes(vm.Compressed))
	data["free_gb"] = gb(vm.bytes(vm.Free))
	if vm.FileBacked > 0 {
		data["file_cache_gb"] = gb(vm.bytes(vm.FileBacked))
	}
}

// addPressure records the kernel pressure level and memory_pressure's free %.
// The numeric code is kept (pressure_code) so the doctor can use numbers only.
func addPressure(data map[string]any) {
	code := int(sysctlInt("kern.memorystatus_vm_pressure_level"))
	data["pressure_code"] = code
	data["pressure_level"] = ClassifyMemoryPressure(code)
	if out, err := runCmd("memory_pressure"); err == nil {
		if pct, ok := parseFreePct(string(out)); ok {
			data["free_pct"] = pct
		}
	}
}

// addSwap records swap usage and the cumulative swap traffic since boot. The
// swapins/swapouts counters are totals since boot, not a rate: a large number
// on a Mac that has been up for weeks is normal, hence the uptime next to it.
func addSwap(data map[string]any, u memUsage) {
	gb := func(pages int64) float64 { return float64(u.vm.bytes(pages)) / gibBytes }
	data["swapins_gb"] = gb(u.vm.Swapins)
	data["swapouts_gb"] = gb(u.vm.Swapouts)
	out, err := runCmd("sysctl", "vm.swapusage")
	if err != nil {
		return
	}
	total, used, ok := parseSwapUsage(string(out))
	if !ok {
		return
	}
	data["swap_total_mb"] = total
	data["swap_used_mb"] = used
	data["swap_thrash"] = HeavyPaging(used, total, gb(u.vm.Swapouts), float64(u.totalBytes)/gibBytes)
}

// addPaging records the page-in/out counters (cumulative since boot), their
// average hourly rate (needs uptime), the compressor size and its ratio, and a
// cautious verdict. Rates are averages since boot, not the current rate: the
// toolkit keeps no history, so it cannot say what is happening right now.
func addPaging(data map[string]any, u memUsage) {
	vm := u.vm
	gb := func(pages int64) float64 { return float64(vm.bytes(pages)) / gibBytes }
	data["pageins_gb"] = gb(vm.Pageins)
	data["pageouts_gb"] = gb(vm.Pageouts)
	data["compressor_stored_gb"] = gb(vm.Stored)
	if vm.Compressed > 0 && vm.Stored > 0 {
		data["compression_ratio"] = float64(vm.Stored) / float64(vm.Compressed)
	}
	data["paging_verdict"] = PagingOK
	if th, _ := data["swap_thrash"].(bool); th {
		data["paging_verdict"] = NoteHeavyPaging
	}
	if h, ok := data["uptime_hours"].(float64); ok && h > 0 {
		data["swapins_gb_per_h"] = gb(vm.Swapins) / h
		data["swapouts_gb_per_h"] = gb(vm.Swapouts) / h
		data["pageins_gb_per_h"] = gb(vm.Pageins) / h
		data["pageouts_gb_per_h"] = gb(vm.Pageouts) / h
	}
}

func addUptime(data map[string]any) {
	out, err := runCmd("sysctl", "-n", "kern.boottime")
	if err != nil {
		return
	}
	if boot, ok := parseBootSec(string(out)); ok {
		if h := float64(nowFn().Unix()-boot) / 3600; h >= 0 {
			data["uptime_hours"] = h
		}
	}
}

// Display prints the memory view for the CLI, in the same style as the other
// monitors.
func (m *MemoryMonitor) Display() error {
	d, err := m.Snapshot()
	if err != nil {
		return err
	}
	fmt.Println()
	fmt.Println("\033[1m🧠 Memory\033[0m")
	fmt.Println(strings.Repeat("─", 50))
	pct := num(d, "used_pct")
	fmt.Printf("  %-14s %s %3.0f%%  %.1f / %.1f GB\n", "Used:",
		core.ProgressBar(int(pct), 20), pct, num(d, "used_gb"), num(d, "ram_total_gb"))
	pr := func(l, v string) { fmt.Printf("  %-14s %s\n", l, v) }
	pr("Pressure:", pressureLine(d))
	pr("Apps:", fmt.Sprintf("%.1f GB", num(d, "app_gb")))
	pr("Wired:", fmt.Sprintf("%.1f GB", num(d, "wired_gb")))
	pr("Compressed:", fmt.Sprintf("%.1f GB", num(d, "compressed_gb")))
	pr("Active:", fmt.Sprintf("%.1f GB", num(d, "active_gb")))
	pr("Inactive:", fmt.Sprintf("%.1f GB", num(d, "inactive_gb")))
	pr("Free:", fmt.Sprintf("%.1f GB", num(d, "free_gb")))
	if _, ok := d["file_cache_gb"]; ok {
		pr("File cache:", fmt.Sprintf("%.1f GB", num(d, "file_cache_gb")))
	}
	printSwap(d)
	printTopApps("Top apps by memory", d["top_memory"], true)
	fmt.Println(strings.Repeat("─", 50))
	return nil
}

func pressureLine(d map[string]any) string {
	lvl, _ := d["pressure_level"].(string)
	color := "\033[32m"
	switch lvl {
	case PressureWarning:
		color = "\033[33m"
	case PressureCritical:
		color = "\033[31m"
	case PressureUnknown, "":
		color = "\033[90m"
	}
	s := color + lvl + "\033[0m"
	if f, ok := d["free_pct"].(int); ok {
		s += fmt.Sprintf(" (%d%% libre)", f)
	}
	return s
}

func printSwap(d map[string]any) {
	if total, ok := d["swap_total_mb"].(float64); ok && total > 0 {
		used := num(d, "swap_used_mb")
		fmt.Printf("  %-14s %s %3.0f%%  %.0f / %.0f MB\n", "Swap:",
			core.ProgressBar(int(used*100/total), 20), used*100/total, used, total)
	}
	fmt.Printf("  %-14s in %.1f GB · out %.1f GB (desde el arranque)\n", "Swap traffic:",
		num(d, "swapins_gb"), num(d, "swapouts_gb"))
	if th, _ := d["swap_thrash"].(bool); th {
		fmt.Printf("  \033[33m%s\033[0m\n", NoteHeavyPaging)
	}
	if h, ok := d["uptime_hours"].(float64); ok {
		fmt.Printf("  %-14s %.1f h\n", "Uptime:", h)
	}
}

// printTopApps prints the aggregated app list (names only, no PIDs or paths).
func printTopApps(title string, v any, byMem bool) {
	apps, _ := v.([]AppUsage)
	fmt.Println()
	fmt.Printf("\033[1m%s\033[0m\n", title)
	for i, a := range apps {
		val := fmt.Sprintf("%.0f MB", a.MemMB)
		if !byMem {
			val = fmt.Sprintf("%.1f%%", a.CPUPct)
		}
		fmt.Printf("  %d. %-28s %10s  (%d procs)\n", i+1, shortName(a.Name, 28), val, a.Procs)
	}
}

// num reads a float64 key, 0 when absent.
func num(d map[string]any, k string) float64 {
	v, _ := d[k].(float64)
	return v
}

// shortName cuts a name to n runes so one long bundle id cannot break columns.
func shortName(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

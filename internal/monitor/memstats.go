package monitor

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

const (
	gibBytes        = float64(1024 * 1024 * 1024)
	defaultPageSize = int64(16384) // Apple Silicon; vm_stat reports the real one
)

var (
	rePageSize  = regexp.MustCompile(`page size of (\d+) bytes`)
	reSwapUsage = regexp.MustCompile(`total\s*=\s*([\d.]+)M\s+used\s*=\s*([\d.]+)M`)
	reFreePct   = regexp.MustCompile(`free percentage:\s*(\d+)%`)
	reBootSec   = regexp.MustCompile(`sec\s*=\s*(\d+)`)
	reLoadAvg   = regexp.MustCompile(`([\d.]+)\s+([\d.]+)\s+([\d.]+)`)
)

// vmStats holds vm_stat counters, all in pages except PageSize.
type vmStats struct {
	PageSize                                     int64
	Free, Active, Inactive, Speculative, Wired   int64
	Compressed, Anonymous, Purgeable, FileBacked int64
	Swapins, Swapouts                            int64
}

// parseVMStat reads `vm_stat` output. Missing counters stay 0, so a trimmed or
// older output degrades instead of failing.
func parseVMStat(raw string) vmStats {
	v := vmStats{PageSize: defaultPageSize}
	if m := rePageSize.FindStringSubmatch(raw); len(m) > 1 {
		if ps, _ := strconv.ParseInt(m[1], 10, 64); ps > 0 {
			v.PageSize = ps
		}
	}
	v.Free = vmStatVal(raw, "Pages free")
	v.Active = vmStatVal(raw, "Pages active")
	v.Inactive = vmStatVal(raw, "Pages inactive")
	v.Speculative = vmStatVal(raw, "Pages speculative")
	v.Wired = vmStatVal(raw, "Pages wired down")
	v.Compressed = vmStatVal(raw, "Pages occupied by compressor")
	v.Anonymous = vmStatVal(raw, "Anonymous pages")
	v.Purgeable = vmStatVal(raw, "Pages purgeable")
	v.FileBacked = vmStatVal(raw, "File-backed pages")
	v.Swapins = vmStatVal(raw, "Swapins")
	v.Swapouts = vmStatVal(raw, "Swapouts")
	return v
}

func (v vmStats) bytes(pages int64) int64 { return pages * v.PageSize }

// appPages is "App Memory" as Activity Monitor defines it: anonymous pages
// minus purgeable ones. Without the anonymous counter we fall back to active
// pages, the closest older approximation.
func (v vmStats) appPages() int64 {
	if v.Anonymous > 0 {
		if app := v.Anonymous - v.Purgeable; app > 0 {
			return app
		}
		return 0
	}
	return v.Active
}

// usedBytes follows Activity Monitor's "Memory Used": App Memory + Wired +
// Compressed. File cache is excluded because the kernel gives it back on
// demand, which is why a naive "active+wired+compressed" figure reads higher.
func (v vmStats) usedBytes() int64 {
	return v.bytes(v.appPages() + v.Wired + v.Compressed)
}

// pageSum is the total of tracked pages, used only when hw.memsize is missing.
func (v vmStats) pageSum() int64 {
	return v.bytes(v.Free + v.Active + v.Inactive + v.Speculative + v.Wired + v.Compressed)
}

// memUsage is the single source of truth for "used memory". SystemMonitor,
// MemoryMonitor (and through them the TUI header and the doctor) all read it,
// so every screen shows the same number.
type memUsage struct {
	vm         vmStats
	totalBytes int64
	usedBytes  int64
}

func (u memUsage) pct() float64 {
	if u.totalBytes <= 0 {
		return 0
	}
	return float64(u.usedBytes) * 100 / float64(u.totalBytes)
}

// readMemUsage runs vm_stat (+ hw.memsize for the physical total).
func readMemUsage() (memUsage, error) {
	out, err := runCmd("vm_stat")
	if err != nil {
		return memUsage{}, fmt.Errorf("vm_stat failed: %w", err)
	}
	vm := parseVMStat(string(out))
	u := memUsage{vm: vm, usedBytes: vm.usedBytes(), totalBytes: vm.pageSum()}
	if t := sysctlInt("hw.memsize"); t > 0 {
		u.totalBytes = t
	}
	return u, nil
}

// sysctlInt reads an integer sysctl; 0 means missing or unreadable.
func sysctlInt(key string) int64 {
	out, err := runCmd("sysctl", "-n", key)
	if err != nil {
		return 0
	}
	n, _ := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
	return n
}

// parseSwapUsage parses `sysctl vm.swapusage` into total/used MB.
func parseSwapUsage(raw string) (totalMB, usedMB float64, ok bool) {
	m := reSwapUsage.FindStringSubmatch(raw)
	if len(m) < 3 {
		return 0, 0, false
	}
	totalMB, _ = strconv.ParseFloat(m[1], 64)
	usedMB, _ = strconv.ParseFloat(m[2], 64)
	return totalMB, usedMB, true
}

// parseFreePct extracts "System-wide memory free percentage: N%".
func parseFreePct(raw string) (int, bool) {
	m := reFreePct.FindStringSubmatch(raw)
	if len(m) < 2 {
		return 0, false
	}
	n, err := strconv.Atoi(m[1])
	return n, err == nil
}

// parseBootSec extracts the boot epoch from `sysctl -n kern.boottime`.
func parseBootSec(raw string) (int64, bool) {
	m := reBootSec.FindStringSubmatch(raw)
	if len(m) < 2 {
		return 0, false
	}
	n, err := strconv.ParseInt(m[1], 10, 64)
	return n, err == nil && n > 0
}

// parseLoadAvg parses `sysctl -n vm.loadavg` ("{ 1.52 1.80 2.01 }").
func parseLoadAvg(raw string) (l1, l5, l15 float64, ok bool) {
	m := reLoadAvg.FindStringSubmatch(raw)
	if len(m) < 4 {
		return 0, 0, 0, false
	}
	l1, _ = strconv.ParseFloat(m[1], 64)
	l5, _ = strconv.ParseFloat(m[2], 64)
	l15, _ = strconv.ParseFloat(m[3], 64)
	return l1, l5, l15, true
}

package monitor

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// SystemMonitor reports hardware info and live CPU/memory/swap/thermal figures.
// Light drops the static hardware lookup; the TUI header uses it on every tick.
type SystemMonitor struct{ Light bool }

func (s *SystemMonitor) Name() string { return "system" }

func (s *SystemMonitor) Snapshot() (map[string]any, error) {
	data := map[string]any{}

	// Hardware info from system_profiler. The header only needs live numbers,
	// so Light skips this static (and comparatively slow) lookup.
	if s.Light {
		return s.addLive(data), nil
	}
	if out, err := runCmd("system_profiler", "SPHardwareDataType", "-json"); err == nil {
		var prof map[string]any
		if json.Unmarshal(out, &prof) == nil {
			if items, ok := prof["SPHardwareDataType"].([]any); ok && len(items) > 0 {
				if hw, ok := items[0].(map[string]any); ok {
					data["model"] = hw["machine_model"]
					data["chip"] = hw["chip_type"]
					data["cores"] = hw["number_processors"]
					data["ram"] = hw["physical_memory"]
				}
			}
		}
	}

	// macOS version for the report header; absent when sw_vers fails.
	if out, err := runCmd("sw_vers", "-productVersion"); err == nil {
		if v := strings.TrimSpace(string(out)); v != "" {
			data["os_version"] = v
		}
	}

	return s.addLive(data), nil
}

// addLive adds the figures that change over time.
func (s *SystemMonitor) addLive(data map[string]any) map[string]any {
	// CPU usage over the last second (see readCPUSplit for why not `top`).
	if usr, sys, _, ok := readCPUSplit(); ok {
		data["cpu_percent"] = usr + sys
	}

	// Memory: same helper as MemoryMonitor so every screen shows one number
	// (Activity Monitor's definition: App + Wired + Compressed).
	if u, err := readMemUsage(); err == nil {
		data["memory_used_gb"] = float64(u.usedBytes) / gibBytes
		data["memory_total_gb"] = float64(u.totalBytes) / gibBytes
		if u.totalBytes > 0 {
			data["memory_percent"] = u.pct()
		}
	}

	// Swap from sysctl
	if out, err := runCmd("sysctl", "vm.swapusage"); err == nil {
		if total, used, ok := parseSwapUsage(string(out)); ok {
			data["swap_total_mb"], data["swap_used_mb"] = total, used
		}
	}

	// Thermal from pmset
	if out, err := runCmd("pmset", "-g", "therm"); err == nil {
		data["thermal"] = parseThermal(string(out))
	} else {
		data["thermal"] = ThermalUnknown
	}
	return data
}

func (s *SystemMonitor) Display() error {
	data, err := s.Snapshot()
	if err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("\033[1m💻 System Info\033[0m")
	fmt.Println(strings.Repeat("─", 50))

	pr := func(label string, val any) {
		fmt.Printf("  %-22s %v\n", label, val)
	}

	if v, ok := data["model"]; ok {
		pr("Model:", v)
	}
	if v, ok := data["chip"]; ok {
		pr("Chip:", v)
	}
	if v, ok := data["cores"]; ok {
		pr("Cores:", v)
	}
	if v, ok := data["ram"]; ok {
		pr("RAM:", v)
	}

	if cpu, ok := data["cpu_percent"].(float64); ok {
		color := "\033[32m"
		if cpu >= 85 {
			color = "\033[31m"
		} else if cpu >= 60 {
			color = "\033[33m"
		}
		fmt.Printf("  %-22s %s%.1f%%\033[0m\n", "CPU Usage:", color, cpu)
	}

	if mp, ok := data["memory_percent"].(float64); ok {
		color := "\033[32m"
		if mp >= 90 {
			color = "\033[31m"
		} else if mp >= 70 {
			color = "\033[33m"
		}
		used := data["memory_used_gb"].(float64)
		total := data["memory_total_gb"].(float64)
		fmt.Printf("  %-22s %s%.1f / %.1f GB (%.0f%%)\033[0m\n", "Memory:", color, used, total, mp)
	}

	if st, ok := data["swap_total_mb"]; ok {
		su := data["swap_used_mb"]
		pr("Swap:", fmt.Sprintf("%.0f / %.0f MB", su, st))
	}

	if th, ok := data["thermal"]; ok {
		pr("Thermal:", th)
	}

	fmt.Println(strings.Repeat("─", 50))
	return nil
}

func vmStatVal(raw, key string) int64 {
	re := regexp.MustCompile(fmt.Sprintf(`%s:\s+(\d+)`, regexp.QuoteMeta(key)))
	if m := re.FindStringSubmatch(raw); len(m) > 1 {
		v, _ := strconv.ParseInt(m[1], 10, 64)
		return v
	}
	return 0
}

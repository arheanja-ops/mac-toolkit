package monitor

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

type SystemMonitor struct{}

func (s *SystemMonitor) Name() string { return "system" }

func (s *SystemMonitor) Snapshot() (map[string]any, error) {
	data := map[string]any{}

	// Hardware info from system_profiler
	if out, err := cmdC("system_profiler", "SPHardwareDataType", "-json").Output(); err == nil {
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

	// CPU usage from top
	if out, err := cmdC("top", "-l", "1", "-n", "0", "-s", "0").Output(); err == nil {
		reCPU := regexp.MustCompile(`CPU usage:\s+([\d.]+)% user,\s+([\d.]+)% sys`)
		if m := reCPU.FindStringSubmatch(string(out)); len(m) > 2 {
			user, _ := strconv.ParseFloat(m[1], 64)
			sys, _ := strconv.ParseFloat(m[2], 64)
			data["cpu_percent"] = user + sys
		}
	}

	// Memory from vm_stat
	if out, err := cmdC("vm_stat").Output(); err == nil {
		raw := string(out)
		pageSize := int64(16384) // default for Apple Silicon
		rePS := regexp.MustCompile(`page size of (\d+) bytes`)
		if m := rePS.FindStringSubmatch(raw); len(m) > 1 {
			ps, _ := strconv.ParseInt(m[1], 10, 64)
			if ps > 0 {
				pageSize = ps
			}
		}

		active := vmStatVal(raw, "Pages active")
		inactive := vmStatVal(raw, "Pages inactive")
		wired := vmStatVal(raw, "Pages wired down")
		compressed := vmStatVal(raw, "Pages occupied by compressor")
		free := vmStatVal(raw, "Pages free")

		used := (active + wired + compressed) * pageSize
		total := (active + inactive + wired + compressed + free) * pageSize

		data["memory_used_gb"] = float64(used) / (1024 * 1024 * 1024)
		data["memory_total_gb"] = float64(total) / (1024 * 1024 * 1024)
		if total > 0 {
			data["memory_percent"] = float64(used) * 100 / float64(total)
		}
	}

	// Swap from sysctl
	if out, err := cmdC("sysctl", "vm.swapusage").Output(); err == nil {
		reSwap := regexp.MustCompile(`total\s*=\s*([\d.]+)M\s+used\s*=\s*([\d.]+)M`)
		if m := reSwap.FindStringSubmatch(string(out)); len(m) > 2 {
			data["swap_total_mb"], _ = strconv.ParseFloat(m[1], 64)
			data["swap_used_mb"], _ = strconv.ParseFloat(m[2], 64)
		}
	}

	// Thermal from pmset
	if out, err := cmdC("pmset", "-g", "therm").Output(); err == nil {
		if strings.Contains(string(out), "No thermal warnings") {
			data["thermal"] = "Normal"
		} else {
			data["thermal"] = "Warning"
		}
	}

	return data, nil
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

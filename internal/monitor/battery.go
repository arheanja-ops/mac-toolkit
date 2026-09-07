package monitor

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

type BatteryMonitor struct{}

func (b *BatteryMonitor) Name() string { return "battery" }

func (b *BatteryMonitor) Snapshot() (map[string]any, error) {
	out, err := cmdC("ioreg", "-rn", "AppleSmartBattery").Output()
	if err != nil {
		return nil, fmt.Errorf("ioreg failed: %w", err)
	}
	raw := string(out)

	data := map[string]any{}
	data["cycle_count"] = parseIoreg(raw, "CycleCount")
	data["max_capacity"] = parseIoreg(raw, "MaxCapacity")
	data["design_capacity"] = parseIoreg(raw, "DesignCapacity")
	data["current_capacity"] = parseIoreg(raw, "CurrentCapacity")
	data["raw_max_capacity"] = parseIoreg(raw, "AppleRawMaxCapacity")
	data["nominal_capacity"] = parseIoreg(raw, "NominalChargeCapacity")
	data["is_charging"] = strings.Contains(raw, "\"IsCharging\" = Yes")
	data["external"] = strings.Contains(raw, "\"ExternalConnected\" = Yes")

	// Temperature (in centi-degrees)
	tempRaw := parseIoreg(raw, "Temperature")
	if tempRaw > 0 {
		data["temperature_c"] = float64(tempRaw) / 100.0
	}

	// Voltage (in mV)
	voltage := parseIoreg(raw, "Voltage")
	if voltage > 0 {
		data["voltage_v"] = float64(voltage) / 1000.0
	}

	// Health percent.
	// On Apple Silicon, MaxCapacity is already a health percentage (0-100).
	// On Intel, MaxCapacity is in mAh, so derive health from raw capacity vs design.
	maxCap := data["max_capacity"].(int)
	designCap := data["design_capacity"].(int)
	rawMax := data["raw_max_capacity"].(int)
	if maxCap > 0 && maxCap <= 100 {
		data["health_percent"] = maxCap
	} else if rawMax > 0 && designCap > 0 {
		health := rawMax * 100 / designCap
		if health > 100 {
			health = 100
		}
		data["health_percent"] = health
	} else if maxCap > 0 && designCap > 0 {
		health := maxCap * 100 / designCap
		if health > 100 {
			health = 100
		}
		data["health_percent"] = health
	}

	// Current charge percent.
	// CurrentCapacity is a percentage on Apple Silicon; on Intel it is mAh
	// relative to MaxCapacity (mAh).
	curCap := data["current_capacity"].(int)
	if maxCap > 100 && maxCap > 0 {
		data["current_percent"] = curCap * 100 / maxCap
	} else if curCap >= 0 && curCap <= 100 {
		data["current_percent"] = curCap
	}

	// Time remaining from pmset
	if pmOut, err := cmdC("pmset", "-g", "batt").Output(); err == nil {
		pmRaw := string(pmOut)
		reTime := regexp.MustCompile(`(\d+:\d+) remaining`)
		if m := reTime.FindStringSubmatch(pmRaw); len(m) > 1 {
			data["time_remaining"] = m[1]
		}
	}

	return data, nil
}

func (b *BatteryMonitor) Display() error {
	data, err := b.Snapshot()
	if err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("\033[1m🔋 Battery Health\033[0m")
	fmt.Println(strings.Repeat("─", 45))

	printRow := func(label string, value any) {
		fmt.Printf("  %-22s %v\n", label, value)
	}

	if h, ok := data["health_percent"]; ok {
		hp := h.(int)
		color := "\033[32m" // green
		if hp < 60 {
			color = "\033[31m" // red
		} else if hp < 80 {
			color = "\033[33m" // yellow
		}
		fmt.Printf("  %-22s %s%d%%\033[0m\n", "Health:", color, hp)
	}

	if cc, ok := data["cycle_count"]; ok {
		printRow("Cycle Count:", cc)
	}
	if cp, ok := data["current_percent"]; ok {
		printRow("Current Charge:", fmt.Sprintf("%d%%", cp))
	}

	charging := "No"
	if ic, ok := data["is_charging"]; ok && ic.(bool) {
		charging = "Yes"
	}
	printRow("Charging:", charging)

	if ext, ok := data["external"]; ok && ext.(bool) {
		printRow("Power Adapter:", "Connected")
	}
	if t, ok := data["temperature_c"]; ok {
		printRow("Temperature:", fmt.Sprintf("%.1f°C", t))
	}
	if v, ok := data["voltage_v"]; ok {
		printRow("Voltage:", fmt.Sprintf("%.2fV", v))
	}
	if tr, ok := data["time_remaining"]; ok {
		printRow("Time Remaining:", tr)
	}

	fmt.Println(strings.Repeat("─", 45))
	return nil
}

func parseIoreg(raw, key string) int {
	re := regexp.MustCompile(fmt.Sprintf(`\"%s\"\s*=\s*(\d+)`, key))
	if m := re.FindStringSubmatch(raw); len(m) > 1 {
		v, _ := strconv.Atoi(m[1])
		return v
	}
	return 0
}

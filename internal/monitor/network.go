package monitor

import (
	"fmt"
	"net/http"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type NetworkMonitor struct{}

func (n *NetworkMonitor) Name() string { return "network" }

// defaultInterface returns the interface backing the default route
// (e.g. "en0", "en9"), or "" if it cannot be determined.
func defaultInterface() string {
	out, err := exec.Command("route", "-n", "get", "default").Output()
	if err != nil {
		return ""
	}
	re := regexp.MustCompile(`interface:\s*(\S+)`)
	if m := re.FindStringSubmatch(string(out)); len(m) > 1 {
		return m[1]
	}
	return ""
}

func (n *NetworkMonitor) Snapshot() (map[string]any, error) {
	data := map[string]any{}

	// Network stats from netstat for the default-route interface.
	// The <Link#> row carries cumulative byte counters (Ibytes at index 6,
	// Obytes at index 9). The active interface is not always en0 (e.g. USB
	// ethernet/dock adapters appear as en9), so resolve it from the route.
	iface := defaultInterface()
	if iface != "" {
		if out, err := exec.Command("netstat", "-ib").Output(); err == nil {
			lines := strings.Split(string(out), "\n")
			for _, line := range lines[1:] {
				fields := strings.Fields(line)
				if len(fields) < 10 || fields[0] != iface {
					continue
				}
				if !strings.Contains(fields[2], "Link") {
					continue
				}
				recv, _ := strconv.ParseInt(fields[6], 10, 64)
				sent, _ := strconv.ParseInt(fields[9], 10, 64)
				data["interface"] = iface
				data["bytes_recv"] = recv
				data["bytes_sent"] = sent
				data["bytes_recv_mb"] = float64(recv) / (1024 * 1024)
				data["bytes_sent_mb"] = float64(sent) / (1024 * 1024)
				break
			}
		}
	}

	// WiFi info from airport
	airportPath := "/System/Library/PrivateFrameworks/Apple80211.framework/Versions/Current/Resources/airport"
	if out, err := exec.Command(airportPath, "-I").Output(); err == nil {
		raw := string(out)
		reSSID := regexp.MustCompile(`\s+SSID:\s+(.+)`)
		reRSSI := regexp.MustCompile(`\s+agrCtlRSSI:\s+(-?\d+)`)
		reChan := regexp.MustCompile(`\s+channel:\s+(\S+)`)

		if m := reSSID.FindStringSubmatch(raw); len(m) > 1 {
			data["wifi_ssid"] = strings.TrimSpace(m[1])
		}
		if m := reRSSI.FindStringSubmatch(raw); len(m) > 1 {
			rssi, _ := strconv.Atoi(m[1])
			data["wifi_rssi"] = rssi
		}
		if m := reChan.FindStringSubmatch(raw); len(m) > 1 {
			data["wifi_channel"] = m[1]
		}
	}

	// Active connections count
	if out, err := exec.Command("netstat", "-an").Output(); err == nil {
		lines := strings.Split(string(out), "\n")
		active := 0
		total := 0
		for _, line := range lines {
			if strings.Contains(line, "ESTABLISHED") {
				active++
			}
			if strings.HasPrefix(line, "tcp") || strings.HasPrefix(line, "udp") {
				total++
			}
		}
		data["active_connections"] = active
		data["total_connections"] = total
	}

	// Connectivity check
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("https://www.apple.com")
	if err == nil {
		resp.Body.Close()
		data["online"] = true
	} else {
		data["online"] = false
	}

	return data, nil
}

func (n *NetworkMonitor) Display() error {
	data, err := n.Snapshot()
	if err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("\033[1m🌐 Network\033[0m")
	fmt.Println(strings.Repeat("─", 50))

	pr := func(label string, val any) {
		fmt.Printf("  %-22s %v\n", label, val)
	}

	if online, ok := data["online"].(bool); ok {
		if online {
			fmt.Printf("  %-22s \033[32m● Online\033[0m\n", "Status:")
		} else {
			fmt.Printf("  %-22s \033[31m● Offline\033[0m\n", "Status:")
		}
	}

	if ssid, ok := data["wifi_ssid"]; ok {
		pr("WiFi SSID:", ssid)
	}
	if rssi, ok := data["wifi_rssi"]; ok {
		pr("Signal (RSSI):", fmt.Sprintf("%v dBm", rssi))
	}
	if ch, ok := data["wifi_channel"]; ok {
		pr("Channel:", ch)
	}

	if iface, ok := data["interface"]; ok {
		pr("Interface:", iface)
	}
	if rm, ok := data["bytes_recv_mb"]; ok {
		pr("Received:", fmt.Sprintf("%.1f MB", rm))
	}
	if sm, ok := data["bytes_sent_mb"]; ok {
		pr("Sent:", fmt.Sprintf("%.1f MB", sm))
	}

	if ac, ok := data["active_connections"]; ok {
		pr("Active Connections:", ac)
	}
	if tc, ok := data["total_connections"]; ok {
		pr("Total Connections:", tc)
	}

	fmt.Println(strings.Repeat("─", 50))
	return nil
}

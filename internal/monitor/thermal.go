package monitor

import (
	"regexp"
	"strconv"
	"strings"
)

// Thermal states emitted by the system monitor. Only Throttled and Warning are
// alerts; Normal and Unknown must never be painted or counted as one.
const (
	ThermalNormal    = "Normal"
	ThermalThrottled = "Throttled"
	ThermalWarning   = "Warning"
	ThermalUnknown   = "Unknown"
)

var reSpeedLimit = regexp.MustCompile(`cpu_speed_limit\s*=\s*(\d+)`)

// parseThermal classifies the output of `pmset -g therm`. macOS words the
// "all clear" case in several ways ("No thermal warnings", "No thermal warning
// level has been recorded"), so matching one exact phrase made every healthy
// Mac look like Warning. Priority: a reduced CPU_Speed_Limit (Throttled) >
// a really recorded warning level (Warning) > an explicit all-clear (Normal).
// Anything we cannot determine is Unknown: ignorance must not raise an alarm.
func parseThermal(out string) string {
	lower := strings.ToLower(out)
	limit := -1
	if m := reSpeedLimit.FindStringSubmatch(lower); len(m) > 1 {
		if v, err := strconv.Atoi(m[1]); err == nil {
			limit = v
		}
	}
	switch {
	case limit >= 0 && limit < 100:
		return ThermalThrottled
	case hasRecordedWarning(lower):
		return ThermalWarning
	case strings.Contains(lower, "no thermal warning") || limit == 100:
		return ThermalNormal
	default:
		return ThermalUnknown
	}
}

// hasRecordedWarning is true when a "warning level" line reports a real level
// instead of the "No ... warning level has been recorded" note.
func hasRecordedWarning(lower string) bool {
	for _, line := range strings.Split(lower, "\n") {
		if !strings.Contains(line, "warning level") {
			continue
		}
		if strings.Contains(line, "no thermal") || strings.Contains(line, "no performance") {
			continue
		}
		return true
	}
	return false
}

// IsThermalAlert reports whether a thermal state deserves the user's attention.
// It is an allowlist on purpose: new or unrecognised states stay quiet.
func IsThermalAlert(state string) bool {
	switch strings.ToLower(strings.TrimSpace(state)) {
	case "throttled", "warning":
		return true
	default:
		return false
	}
}

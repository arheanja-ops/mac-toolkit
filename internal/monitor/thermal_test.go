package monitor

import "testing"

func TestParseThermal(t *testing.T) {
	cases := []struct {
		name string
		out  string
		want string
	}{
		{"legacy wording", "Note: No thermal warnings\n", ThermalNormal},
		{"modern wording (this Mac)", "Note: No thermal warning level has been recorded\n" +
			"Note: No performance warning level has been recorded\n" +
			"Note: No CPU power status has been recorded\n", ThermalNormal},
		{"case-insensitive", "NOTE: NO THERMAL WARNING LEVEL HAS BEEN RECORDED", ThermalNormal},
		{"speed limit 100", "CPU_Scheduler_Limit = 100\nCPU_Available_CPUs = 8\nCPU_Speed_Limit = 100\n", ThermalNormal},
		{"speed limit 70", "CPU_Scheduler_Limit = 100\nCPU_Speed_Limit = 70\n", ThermalThrottled},
		{"recorded thermal level", "Thermal warning level: 2\nNote: No performance warning level has been recorded\n", ThermalWarning},
		{"recorded performance level", "Note: No thermal warning level has been recorded\nPerformance warning level: 1\n", ThermalWarning},
		{"empty", "", ThermalUnknown},
		{"unrelated text", "pmset: something unexpected\n", ThermalUnknown},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := parseThermal(tc.out); got != tc.want {
				t.Fatalf("parseThermal = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestIsThermalAlert(t *testing.T) {
	for state, want := range map[string]bool{
		"Throttled": true, "Warning": true, " warning ": true,
		"Normal": false, "Unknown": false, "": false, "nominal": false,
	} {
		if got := IsThermalAlert(state); got != want {
			t.Errorf("IsThermalAlert(%q) = %v, want %v", state, got, want)
		}
	}
}

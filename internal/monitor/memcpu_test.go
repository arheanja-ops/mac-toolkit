package monitor

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"
)

// fakeCmds replaces runCmd with canned outputs keyed by "name arg arg". A key
// that is absent behaves like a command that fails, so degradation is covered
// without any real system call.
func fakeCmds(t *testing.T, outs map[string]string) {
	t.Helper()
	orig, origNow := runCmd, nowFn
	runCmd = func(name string, args ...string) ([]byte, error) {
		if out, ok := outs[strings.Join(append([]string{name}, args...), " ")]; ok {
			return []byte(out), nil
		}
		return nil, errors.New("synthetic failure")
	}
	nowFn = func() time.Time { return time.Unix(1_700_000_000+3*3600, 0) }
	t.Cleanup(func() { runCmd, nowFn = orig, origNow })
}

// Synthetic 16 GiB machine with 16384-byte pages (1_048_576 pages in total).
const synthVMStat = `Mach Virtual Memory Statistics: (page size of 16384 bytes)
Pages free:                               100000.
Pages active:                             300000.
Pages inactive:                           250000.
Pages speculative:                         10000.
Pages wired down:                         150000.
Pages purgeable:                           20000.
File-backed pages:                        200000.
Anonymous pages:                          320000.
Pages occupied by compressor:             100000.
Swapins:                                   65536.
Swapouts:                                 131072.
`

const synthMemsize = "17179869184\n" // 16 GiB

func memoryFakes() map[string]string {
	return map[string]string{
		"vm_stat":              synthVMStat,
		"sysctl -n hw.memsize": synthMemsize,
		"sysctl -n kern.memorystatus_vm_pressure_level": "2\n",
		"memory_pressure":            "The system has 17179869184 bytes\nSystem-wide memory free percentage: 41%\n",
		"sysctl vm.swapusage":        "vm.swapusage: total = 2048.00M  used = 1024.50M  free = 1023.50M  (encrypted)\n",
		"sysctl -n kern.boottime":    "{ sec = 1700000000, usec = 5 } Tue Nov 14 22:13:20 2023\n",
		"ps -axwwo rss=,pcpu=,comm=": synthPS,
	}
}

const synthPS = `  1048576  10.0 /Applications/Alpha.app/Contents/MacOS/Alpha
   524288   5.5 /Applications/Alpha.app/Contents/Frameworks/Alpha Helper.app/Contents/MacOS/Alpha Helper
   786432  30.0 /usr/libexec/beta
     1024   0.1 /Applications/Gamma.app/Contents/MacOS/Gamma
   not-a-row
`

func TestParseVMStatPageSizes(t *testing.T) {
	v := parseVMStat(synthVMStat)
	if v.PageSize != 16384 || v.Wired != 150000 || v.Swapouts != 131072 {
		t.Fatalf("unexpected parse: %+v", v)
	}
	v4k := parseVMStat("Mach Virtual Memory Statistics: (page size of 4096 bytes)\nPages wired down: 10.\n")
	if v4k.PageSize != 4096 || v4k.bytes(v4k.Wired) != 40960 {
		t.Fatalf("4k page size not honoured: %+v", v4k)
	}
	if def := parseVMStat(""); def.PageSize != 16384 {
		t.Fatalf("empty output should default the page size, got %d", def.PageSize)
	}
}

func TestMemoryUsedFollowsActivityMonitor(t *testing.T) {
	fakeCmds(t, memoryFakes())
	d, err := (&MemoryMonitor{}).Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	// App = 320000-20000 = 300000 pages; used = 300000+150000+100000 = 550000
	// pages * 16384 B = 9_011_200_000 B ≈ 8.39 GiB of 16 GiB.
	wantUsed := 550000.0 * 16384 / gibBytes
	if got := d["used_gb"].(float64); math.Abs(got-wantUsed) > 1e-9 {
		t.Fatalf("used_gb = %v, want %v", got, wantUsed)
	}
	if got := d["used_pct"].(float64); math.Abs(got-wantUsed/16*100) > 1e-6 {
		t.Fatalf("used_pct = %v", got)
	}
	if d["ram_total_gb"].(float64) != 16 || d["pressure_level"] != PressureWarning || d["free_pct"] != 41 {
		t.Fatalf("unexpected pressure/ram: %v", d)
	}
	if d["swap_total_mb"].(float64) != 2048 || d["swapouts_gb"].(float64) != 2 {
		t.Fatalf("unexpected swap: %v", d)
	}
	if got := d["uptime_hours"].(float64); got != 3 {
		t.Fatalf("uptime_hours = %v, want 3", got)
	}
}

// One number everywhere: the system monitor (header, doctor) must equal the
// memory monitor's figure for the same vm_stat output.
func TestSystemMemoryMatchesMemoryMonitor(t *testing.T) {
	fakeCmds(t, memoryFakes())
	mem, _ := (&MemoryMonitor{}).Snapshot()
	sys, _ := (&SystemMonitor{}).Snapshot()
	if sys["memory_percent"] != mem["used_pct"] || sys["memory_used_gb"] != mem["used_gb"] {
		t.Fatalf("system %v/%v != memory %v/%v",
			sys["memory_percent"], sys["memory_used_gb"], mem["used_pct"], mem["used_gb"])
	}
}

func TestMemoryDegradesWithoutOptionalSources(t *testing.T) {
	fakeCmds(t, map[string]string{"vm_stat": synthVMStat})
	d, err := (&MemoryMonitor{}).Snapshot()
	if err != nil {
		t.Fatalf("optional sources failing must not error: %v", err)
	}
	if d["pressure_level"] != PressureUnknown {
		t.Fatalf("pressure should be Unknown, got %v", d["pressure_level"])
	}
	for _, k := range []string{"swap_total_mb", "uptime_hours", "free_pct", "top_memory"} {
		if _, ok := d[k]; ok {
			t.Fatalf("%s should be absent when its source fails", k)
		}
	}
	// hw.memsize missing: the total falls back to the sum of tracked pages.
	if d["ram_total_gb"].(float64) <= 0 {
		t.Fatal("ram_total_gb should fall back to the page sum")
	}
}

func TestMemoryFailsOnlyWithoutVMStat(t *testing.T) {
	fakeCmds(t, map[string]string{})
	if _, err := (&MemoryMonitor{}).Snapshot(); err == nil {
		t.Fatal("expected an error when vm_stat fails")
	}
}

func TestAggregateByApp(t *testing.T) {
	apps := aggregateApps(parseProcRows(synthPS))
	top := topApps(apps, 5, true)
	if len(top) != 3 {
		t.Fatalf("want 3 apps (malformed line skipped), got %d: %+v", len(top), top)
	}
	if top[0].Name != "Alpha" || top[0].Procs != 2 || top[0].MemMB != 1536 {
		t.Fatalf("helper bundle must fold into its app: %+v", top[0])
	}
	if top[1].Name != "beta" {
		t.Fatalf("non-bundle falls back to the basename: %+v", top[1])
	}
	if byCPU := topApps(apps, 1, false); byCPU[0].Name != "beta" {
		t.Fatalf("top by CPU = %+v", byCPU)
	}
}

func TestAppNameCases(t *testing.T) {
	cases := map[string]string{
		"/Applications/Foo Bar.app/Contents/MacOS/Foo Bar":          "Foo Bar",
		"/System/Library/CoreServices/Dock.app/Contents/MacOS/Dock": "Dock",
		"/usr/sbin/syslogd": "syslogd",
		"kernel_task":       "kernel_task",
	}
	for in, want := range cases {
		if got := appName(in); got != want {
			t.Errorf("appName(%q) = %q, want %q", in, got, want)
		}
	}
}

// Privacy: neither PIDs nor paths may reach the snapshot output.
func TestSnapshotsCarryNoPathsOrPIDs(t *testing.T) {
	fakeCmds(t, memoryFakes())
	d, _ := (&MemoryMonitor{}).Snapshot()
	out := fmt.Sprintf("%+v", d["top_memory"])
	for _, bad := range []string{"/", "Contents", "pid"} {
		if strings.Contains(out, bad) {
			t.Fatalf("top_memory leaks %q: %s", bad, out)
		}
	}
}

func TestClassifyMemoryPressure(t *testing.T) {
	cases := map[int]string{1: PressureNormal, 2: PressureWarning, 4: PressureCritical, 0: PressureUnknown, 3: PressureUnknown, -1: PressureUnknown}
	for code, want := range cases {
		if got := ClassifyMemoryPressure(code); got != want {
			t.Errorf("code %d = %s, want %s", code, got, want)
		}
	}
}

func TestHeavyPagingThresholds(t *testing.T) {
	cases := []struct {
		name                       string
		used, total, swapouts, ram float64
		want                       bool
	}{
		{"both exceeded", 1700, 2048, 33, 16, true},
		{"swap exactly 80% is not enough", 1638.4, 2048, 33, 16, false},
		{"swapouts exactly 2x RAM is not enough", 2000, 2048, 32, 16, false},
		{"swap high, little traffic", 2000, 2048, 5, 16, false},
		{"traffic high, swap idle", 100, 2048, 100, 16, false},
		{"no swap file", 0, 0, 100, 16, false},
		{"unknown ram", 2000, 2048, 100, 0, false},
	}
	for _, c := range cases {
		if got := HeavyPaging(c.used, c.total, c.swapouts, c.ram); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

func TestClassifyCPUThresholds(t *testing.T) {
	cases := []struct {
		load  float64
		cores int
		want  string
	}{
		{0.99, 1, LoadNormal}, {1.0, 1, LoadHigh}, {2.0, 1, LoadHigh},
		{2.01, 1, LoadSaturated}, {5, 10, LoadNormal}, {10, 10, LoadHigh},
		{20, 10, LoadHigh}, {20.1, 10, LoadSaturated}, {50, 0, LoadNormal},
	}
	for _, c := range cases {
		if got := ClassifyCPU(c.load, c.cores); got != c.want {
			t.Errorf("load %.2f/%d = %s, want %s", c.load, c.cores, got, c.want)
		}
	}
}

func TestHighKernelTimeThresholds(t *testing.T) {
	cases := []struct {
		sys, user float64
		want      bool
	}{
		{40.1, 10, true}, {40, 10, false}, {60, 60, false}, {60, 59.9, true}, {10, 5, false},
	}
	for _, c := range cases {
		if got := HighKernelTime(c.sys, c.user); got != c.want {
			t.Errorf("sys %.1f user %.1f = %v, want %v", c.sys, c.user, got, c.want)
		}
	}
}

const synthTop = `Processes: 400 total
CPU usage: 50.00% user, 30.00% sys, 20.00% idle
Processes: 400 total
CPU usage: 12.50% user, 45.25% sys, 42.25% idle
`

func TestParseTopCPUUsesSecondSample(t *testing.T) {
	u, s, i, ok := parseTopCPU(synthTop)
	if !ok || u != 12.5 || s != 45.25 || i != 42.25 {
		t.Fatalf("got %v %v %v %v; the first (cumulative) sample must be ignored", u, s, i, ok)
	}
	if _, _, _, ok := parseTopCPU("garbage"); ok {
		t.Fatal("garbage must not parse")
	}
}

func cpuFakes() map[string]string {
	return map[string]string{
		"top -l 2 -n 0 -s 1":                 synthTop,
		"sysctl -n vm.loadavg":               "{ 4.00 3.00 2.00 }\n",
		"sysctl -n hw.ncpu":                  "10\n",
		"sysctl -n hw.perflevel0.logicalcpu": "6\n",
		"sysctl -n hw.perflevel1.logicalcpu": "4\n",
		"pmset -g therm":                     "No thermal warning level has been recorded\n",
		"ps -axwwo rss=,pcpu=,comm=":         synthPS,
	}
}

func TestCPUSnapshot(t *testing.T) {
	fakeCmds(t, cpuFakes())
	d, err := (&CPUMonitor{}).Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if d["user_pct"] != 12.5 || d["sys_pct"] != 45.25 || d["kernel_high"] != true {
		t.Fatalf("cpu split: %v", d)
	}
	if d["load_per_core"].(float64) != 0.4 || d["load5_per_core"].(float64) != 0.3 ||
		d["load_verdict"] != LoadNormal {
		t.Fatalf("load: %v", d)
	}
	if d["cores_total"] != 10 || d["perf_cores"] != 6 || d["eff_cores"] != 4 || d["thermal"] != ThermalNormal {
		t.Fatalf("cores/thermal: %v", d)
	}
}

func TestCPUDegradesToIntelAndFailures(t *testing.T) {
	fakeCmds(t, map[string]string{"sysctl -n hw.ncpu": "8\n"})
	d, err := (&CPUMonitor{}).Snapshot()
	if err != nil {
		t.Fatalf("must never error: %v", err)
	}
	if _, ok := d["perf_cores"]; ok {
		t.Fatal("perf_cores must be absent without perflevel keys")
	}
	if d["thermal"] != ThermalUnknown || d["cores_total"] != 8 {
		t.Fatalf("degradation: %v", d)
	}
	if _, ok := d["user_pct"]; ok {
		t.Fatal("no top output, no split")
	}
}

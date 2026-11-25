package monitor

import (
	"math"
	"strings"
	"testing"
)

func processFakes() map[string]string {
	return map[string]string{
		"ps -axwwo pid=,state=,rss=,pcpu=,pmem=,comm=":  synthPS,
		"sysctl -n kern.memorystatus_vm_pressure_level": "1\n",
		"memory_pressure": "System-wide memory free percentage: 41%\n",
	}
}

func TestProcessSnapshotTypedAndSorted(t *testing.T) {
	fakeCmds(t, processFakes())
	d, err := (&ProcessMonitor{}).Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	cpu := d["top_cpu"].([]ProcInfo)
	if cpu[0].Name != "beta" || cpu[0].PID != 102 || cpu[0].CPU != 30 {
		t.Fatalf("top by CPU: %+v", cpu[0])
	}
	mem := d["top_memory"].([]ProcInfo)
	if mem[0].Name != "Alpha" || mem[0].RSSKB != 1048576 {
		t.Fatalf("top by memory: %+v", mem[0])
	}
	for _, p := range append(cpu, mem...) {
		if strings.Contains(p.Name, "/") {
			t.Fatalf("name must not carry a path: %q", p.Name)
		}
	}
	apps := d["apps"].([]AppUsage)
	if apps[0].Name != "Alpha" || apps[0].Procs != 2 {
		t.Fatalf("apps: %+v", apps)
	}
	if d["pressure_level"] != PressureNormal {
		t.Fatalf("pressure: %v", d["pressure_level"])
	}
}

func TestProcessSummaryCountsStates(t *testing.T) {
	fakeCmds(t, processFakes())
	d, _ := (&ProcessMonitor{}).Snapshot()
	s := d["summary"].(ProcSummary)
	want := ProcSummary{Total: 7, Running: 1, Sleeping: 3, Stopped: 1, Zombie: 1, IOWait: 1}
	if s != want {
		t.Fatalf("summary = %+v, want %+v", s, want)
	}
}

func TestProcessSnapshotFailsWithoutPS(t *testing.T) {
	fakeCmds(t, map[string]string{})
	if _, err := (&ProcessMonitor{}).Snapshot(); err == nil {
		t.Fatal("expected an error when ps fails")
	}
}

func TestPagingKeysAndVerdict(t *testing.T) {
	fakeCmds(t, memoryFakes())
	d, err := (&MemoryMonitor{}).Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if d["paging_verdict"] != PagingOK {
		t.Fatalf("verdict = %v", d["paging_verdict"])
	}
	// Synthetic uptime is 3 h and swapouts are 2 GiB → 2/3 GiB per hour.
	if got := d["swapouts_gb_per_h"].(float64); math.Abs(got-2.0/3) > 1e-9 {
		t.Fatalf("swapouts_gb_per_h = %v", got)
	}
	if _, ok := d["compression_ratio"]; ok {
		t.Fatal("no 'stored in compressor' counter in the fixture: no ratio")
	}
}

func TestCompressionRatioFromStoredPages(t *testing.T) {
	fk := memoryFakes()
	fk["vm_stat"] = synthVMStat + "Pages stored in compressor:                  300000.\n"
	fakeCmds(t, fk)
	d, _ := (&MemoryMonitor{}).Snapshot()
	if got := d["compression_ratio"].(float64); got != 3 {
		t.Fatalf("ratio = %v, want 3", got)
	}
}

func TestSpeedLimitParsing(t *testing.T) {
	if v, ok := parseSpeedLimit("CPU_Scheduler_Limit = 100\nCPU_Speed_Limit = 80\n"); !ok || v != 80 {
		t.Fatalf("got %v %v", v, ok)
	}
	if _, ok := parseSpeedLimit("No thermal warning level has been recorded"); ok {
		t.Fatal("absent line must not parse")
	}
}

func TestCPUSnapshotCarriesSpeedLimit(t *testing.T) {
	fk := cpuFakes()
	fk["pmset -g therm"] = "CPU_Speed_Limit = 100\n"
	fakeCmds(t, fk)
	d, _ := (&CPUMonitor{}).Snapshot()
	if d["cpu_speed_limit"] != 100 || d["thermal"] != ThermalNormal {
		t.Fatalf("thermal keys: %v", d)
	}
}

// Real case: load5 per core 47 with 66.5% idle must not read as saturated.
func TestCPUVerdictUsesIdle(t *testing.T) {
	fk := cpuFakes()
	fk["sysctl -n vm.loadavg"] = "{ 400.00 376.00 300.00 }\n"
	fk["sysctl -n hw.ncpu"] = "8\n"
	fk["iostat -c 2 -w 1"] = "cpu\n us sy id\n 20 13 67\n"
	fakeCmds(t, fk)
	d, _ := (&CPUMonitor{}).Snapshot()
	if d["load_verdict"] != LoadWaiting {
		t.Fatalf("verdict = %v", d["load_verdict"])
	}
}

func TestSystemLightSkipsHardwareLookup(t *testing.T) {
	fk := memoryFakes()
	fk["iostat -c 2 -w 1"] = synthIostat
	fk["system_profiler SPHardwareDataType -json"] = `{"SPHardwareDataType":[{"machine_model":"X"}]}`
	fakeCmds(t, fk)
	light, _ := (&SystemMonitor{Light: true}).Snapshot()
	if _, ok := light["model"]; ok {
		t.Fatal("light snapshot must not run system_profiler")
	}
	if light["cpu_percent"] != 58.0 {
		t.Fatalf("cpu_percent = %v (user+sys of the interval line)", light["cpu_percent"])
	}
	full, _ := (&SystemMonitor{}).Snapshot()
	if full["model"] != "X" || full["cpu_percent"] != light["cpu_percent"] {
		t.Fatalf("full snapshot: %v", full)
	}
}

func TestSystemCPUDegradesWithoutIostat(t *testing.T) {
	fakeCmds(t, memoryFakes())
	d, err := (&SystemMonitor{Light: true}).Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := d["cpu_percent"]; ok {
		t.Fatal("no iostat: cpu_percent must be absent, not zero")
	}
}

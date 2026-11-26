package monitor

// LightReading is one cheap reading of the machine for the watch mode. It only
// uses sysctl, vm_stat, iostat, ps and (optionally) pmset: never top or
// system_profiler. Every field degrades on its own: a failed source leaves its
// fields at zero and its *OK flag false, and the rest of the reading is kept.
type LightReading struct {
	Load1, Load5, Load15 float64
	LoadOK               bool
	Cores                int

	UserPct, SysPct, IdlePct float64
	CPUOK                    bool

	MemUsedPct   float64
	MemOK        bool
	PressureCode int // 1 Normal, 2 Warning, 4 Critical, 0 unknown

	SwapUsedMB, SwapTotalMB float64
	SwapOK                  bool
	// Cumulative swap traffic since boot, in bytes. The watch engine turns the
	// difference between two readings into a rate.
	SwapinBytes, SwapoutBytes int64

	Thermal string // empty when not requested

	// Name of the app that uses the most CPU and memory (aggregated over its
	// processes; no PID, no path).
	TopCPUApp, TopMemApp string
}

// LightOptions picks the optional, costlier sources of a reading.
type LightOptions struct {
	Thermal bool // `pmset -g therm` (~20 ms of CPU; changes slowly)
	Apps    bool // one `ps` for the top apps (~40 ms of CPU, the costliest)
}

// ReadLight takes one light reading. Callers ask for the optional sources only
// every few readings to keep the average cost low.
func ReadLight(opt LightOptions) LightReading {
	withThermal := opt.Thermal
	var r LightReading
	r.Cores = int(sysctlInt("hw.ncpu"))
	if out, err := runCmd("sysctl", "-n", "vm.loadavg"); err == nil {
		r.Load1, r.Load5, r.Load15, r.LoadOK = parseLoadAvg(string(out))
	}
	r.UserPct, r.SysPct, r.IdlePct, r.CPUOK = readCPUSplit()
	if u, err := readMemUsage(); err == nil {
		r.MemUsedPct, r.MemOK = u.pct(), u.totalBytes > 0
		r.SwapinBytes = u.vm.bytes(u.vm.Swapins)
		r.SwapoutBytes = u.vm.bytes(u.vm.Swapouts)
	}
	r.PressureCode = int(sysctlInt("kern.memorystatus_vm_pressure_level"))
	if out, err := runCmd("sysctl", "vm.swapusage"); err == nil {
		r.SwapTotalMB, r.SwapUsedMB, r.SwapOK = parseSwapUsage(string(out))
	}
	if withThermal {
		r.Thermal = ThermalUnknown
		if out, err := runCmd("pmset", "-g", "therm"); err == nil {
			r.Thermal = parseThermal(string(out))
		}
	}
	if !opt.Apps {
		return r
	}
	if rows, err := readProcRows(); err == nil {
		apps := aggregateApps(rows)
		if top := topApps(apps, 1, false); len(top) > 0 {
			r.TopCPUApp = top[0].Name
		}
		if top := topApps(apps, 1, true); len(top) > 0 {
			r.TopMemApp = top[0].Name
		}
	}
	return r
}

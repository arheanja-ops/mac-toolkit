// Package watch is the listening mode: a light sampler that runs while the
// toolkit is open, sustained-threshold alert rules, anti-spam macOS
// notifications and a 7-day history. It is strictly read-only: it never kills
// processes nor changes anything; the only writes are its own history files.
package watch

import (
	"syscall"
	"time"

	"github.com/arheanja-ops/mac-toolkit/internal/monitor"
)

// Sampling cadence. Thermal state changes slowly, so it is read every
// thermalEvery samples (60 s at the default interval).
const (
	DefaultInterval = 15 * time.Second
	thermalEvery    = 4 // pmset every 60 s
	appsEvery       = 2 // ps (top apps) every 30 s: the costliest source
)

// Sample is one reading as the rules and the history see it: numbers plus the
// name of the top app by CPU and by memory. Never a path or a PID.
type Sample struct {
	Time time.Time `json:"t"`

	Load1   float64 `json:"load1,omitempty"`
	Load5   float64 `json:"load5,omitempty"`
	Load15  float64 `json:"load15,omitempty"`
	Cores   int     `json:"cores,omitempty"`
	LoadOK  bool    `json:"load_ok,omitempty"`
	IdlePct float64 `json:"idle_pct,omitempty"`
	UserPct float64 `json:"user_pct,omitempty"`
	SysPct  float64 `json:"sys_pct,omitempty"`
	CPUOK   bool    `json:"cpu_ok,omitempty"`

	MemUsedPct float64 `json:"mem_used_pct,omitempty"`
	MemOK      bool    `json:"mem_ok,omitempty"`
	Pressure   string  `json:"pressure,omitempty"`

	SwapUsedPct float64 `json:"swap_used_pct,omitempty"`
	SwapOK      bool    `json:"swap_ok,omitempty"`
	SwapOutMBps float64 `json:"swapout_mbps,omitempty"`
	SwapInMBps  float64 `json:"swapin_mbps,omitempty"`
	RateOK      bool    `json:"rate_ok,omitempty"` // needs a previous sample

	DiskFreeGB  float64 `json:"disk_free_gb,omitempty"`
	DiskFreePct float64 `json:"disk_free_pct,omitempty"`
	DiskOK      bool    `json:"disk_ok,omitempty"`

	Thermal   string `json:"thermal,omitempty"`
	TopCPUApp string `json:"top_cpu_app,omitempty"`
	TopMemApp string `json:"top_mem_app,omitempty"`
}

// LoadPerCore is the 5-minute load divided by the cores (0 when unknown).
func (s Sample) LoadPerCore() float64 {
	if !s.LoadOK || s.Cores <= 0 {
		return 0
	}
	return s.Load5 / float64(s.Cores)
}

// Sampler turns light readings into samples. It keeps the previous swap
// counters to derive the paging rates, and the last thermal state between its
// (less frequent) thermal reads. Read and DiskFree are injectable so tests never
// touch the machine.
type Sampler struct {
	Read     func(opt monitor.LightOptions) monitor.LightReading
	DiskFree func() (free, total int64, ok bool)

	n        int
	thermal  string
	topCPU   string
	topMem   string
	prev     monitor.LightReading
	prevTime time.Time
	hasPrev  bool
}

// NewSampler returns a sampler wired to the real machine.
func NewSampler() *Sampler {
	return &Sampler{Read: monitor.ReadLight, DiskFree: statfsDataVolume}
}

// statfsDataVolume reads the data volume with a direct statfs call (no exec).
func statfsDataVolume() (free, total int64, ok bool) {
	for _, p := range []string{"/System/Volumes/Data", "/"} {
		var st syscall.Statfs_t
		if err := syscall.Statfs(p, &st); err == nil && st.Blocks > 0 {
			bs := int64(st.Bsize)
			return int64(st.Bavail) * bs, int64(st.Blocks) * bs, true
		}
	}
	return 0, 0, false
}

const mib = 1024 * 1024

// Take reads the machine once and returns the sample for time now.
func (s *Sampler) Take(now time.Time) Sample {
	opt := monitor.LightOptions{Thermal: s.n%thermalEvery == 0, Apps: s.n%appsEvery == 0}
	s.n++
	r := s.Read(opt)
	if opt.Thermal {
		s.thermal = r.Thermal
	}
	if opt.Apps {
		s.topCPU, s.topMem = r.TopCPUApp, r.TopMemApp
	}
	out := Sample{
		Time: now, Load1: r.Load1, Load5: r.Load5, Load15: r.Load15, Cores: r.Cores, LoadOK: r.LoadOK,
		IdlePct: r.IdlePct, UserPct: r.UserPct, SysPct: r.SysPct, CPUOK: r.CPUOK,
		MemUsedPct: r.MemUsedPct, MemOK: r.MemOK, Pressure: monitor.ClassifyMemoryPressure(r.PressureCode),
		Thermal: s.thermal, TopCPUApp: s.topCPU, TopMemApp: s.topMem,
	}
	if r.SwapOK && r.SwapTotalMB > 0 {
		out.SwapUsedPct, out.SwapOK = r.SwapUsedMB*100/r.SwapTotalMB, true
	}
	if s.hasPrev && r.MemOK && s.prev.MemOK {
		if dt := now.Sub(s.prevTime).Seconds(); dt > 0 {
			out.SwapOutMBps = max(0, float64(r.SwapoutBytes-s.prev.SwapoutBytes)/mib/dt)
			out.SwapInMBps = max(0, float64(r.SwapinBytes-s.prev.SwapinBytes)/mib/dt)
			out.RateOK = true
		}
	}
	if s.DiskFree != nil {
		if free, total, ok := s.DiskFree(); ok && total > 0 {
			out.DiskFreeGB = float64(free) / (1 << 30)
			out.DiskFreePct = float64(free) * 100 / float64(total)
			out.DiskOK = true
		}
	}
	s.prev, s.prevTime, s.hasPrev = r, now, true
	return out
}

package watch

import (
	"sort"
	"time"
)

// Stat summarizes one metric over a range.
type Stat struct {
	Min float64 `json:"min"`
	Avg float64 `json:"avg"`
	P95 float64 `json:"p95"`
	Max float64 `json:"max"`
	N   int     `json:"n"`
}

func statOf(vals []float64) Stat {
	if len(vals) == 0 {
		return Stat{}
	}
	sorted := append([]float64(nil), vals...)
	sort.Float64s(sorted)
	var sum float64
	for _, v := range sorted {
		sum += v
	}
	idx := int(float64(len(sorted)-1)*0.95 + 0.5)
	return Stat{Min: sorted[0], Max: sorted[len(sorted)-1], Avg: sum / float64(len(sorted)),
		P95: sorted[idx], N: len(sorted)}
}

// AlertSpan is one alert from start to end (End zero = still open in range).
type AlertSpan struct {
	Rule     string    `json:"rule"`
	Title    string    `json:"title"`
	Severity string    `json:"severity"`
	Start    time.Time `json:"start"`
	End      time.Time `json:"end,omitempty"`
}

// NameCount is how many samples an app was the top one.
type NameCount struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// Summary is what `history` prints for a range.
type Summary struct {
	From        time.Time   `json:"from"`
	To          time.Time   `json:"to"`
	Samples     int         `json:"samples"`
	MemUsedPct  Stat        `json:"mem_used_pct"`
	SwapUsedPct Stat        `json:"swap_used_pct"`
	SwapOutMBps Stat        `json:"swapout_mbps"`
	LoadPerCore Stat        `json:"load5_per_core"`
	IdlePct     Stat        `json:"idle_pct"`
	DiskFreeGB  Stat        `json:"disk_free_gb"`
	Alerts      []AlertSpan `json:"alerts"`
	TopCPU      []NameCount `json:"top_cpu_apps"`
	TopMem      []NameCount `json:"top_mem_apps"`
}

// Summarize folds the records of a range.
func Summarize(recs []Record, from, to time.Time) Summary {
	sum := Summary{From: from, To: to, Alerts: []AlertSpan{}}
	var mem, swap, out, load, idle, disk []float64
	cpuApps, memApps := map[string]int{}, map[string]int{}
	open := map[string]int{} // rule → index in sum.Alerts
	for _, r := range recs {
		if s := r.Sample; s != nil {
			sum.Samples++
			if s.MemOK {
				mem = append(mem, s.MemUsedPct)
			}
			if s.SwapOK {
				swap = append(swap, s.SwapUsedPct)
			}
			if s.RateOK {
				out = append(out, s.SwapOutMBps)
			}
			if s.LoadOK && s.Cores > 0 {
				load = append(load, s.LoadPerCore())
			}
			if s.CPUOK {
				idle = append(idle, s.IdlePct)
			}
			if s.DiskOK {
				disk = append(disk, s.DiskFreeGB)
			}
			if s.TopCPUApp != "" {
				cpuApps[s.TopCPUApp]++
			}
			if s.TopMemApp != "" {
				memApps[s.TopMemApp]++
			}
		}
		if e := r.Event; e != nil {
			i, isOpen := open[e.Alert.Rule]
			switch e.Kind {
			case EventAlert:
				if !isOpen {
					open[e.Alert.Rule] = len(sum.Alerts)
					sum.Alerts = append(sum.Alerts, AlertSpan{Rule: e.Alert.Rule, Title: e.Alert.Title,
						Severity: e.Alert.Severity, Start: e.Alert.Since})
				}
			case EventEscalate:
				if isOpen {
					sum.Alerts[i].Severity = e.Alert.Severity
				}
			case EventResolve:
				if isOpen {
					sum.Alerts[i].End = e.Time
					delete(open, e.Alert.Rule)
				} else { // started before the range
					sum.Alerts = append(sum.Alerts, AlertSpan{Rule: e.Alert.Rule, Title: e.Alert.Title,
						Severity: e.Alert.Severity, Start: e.Alert.Since, End: e.Time})
				}
			}
		}
	}
	sum.MemUsedPct, sum.SwapUsedPct, sum.SwapOutMBps = statOf(mem), statOf(swap), statOf(out)
	sum.LoadPerCore, sum.IdlePct, sum.DiskFreeGB = statOf(load), statOf(idle), statOf(disk)
	sum.TopCPU, sum.TopMem = topNames(cpuApps, 3), topNames(memApps, 3)
	return sum
}

func topNames(m map[string]int, n int) []NameCount {
	out := make([]NameCount, 0, len(m))
	for k, v := range m {
		out = append(out, NameCount{k, v})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Name < out[j].Name
	})
	if len(out) > n {
		out = out[:n]
	}
	return out
}

// HourRow is one hour of the TUI History view.
type HourRow struct {
	Hour        time.Time
	Samples     int
	MemMaxPct   float64
	LoadMax     float64
	SwapOutMax  float64
	AlertsFired int
}

// Hourly groups records by local hour, newest first.
func Hourly(recs []Record) []HourRow {
	byHour := map[time.Time]*HourRow{}
	for _, r := range recs {
		t := r.Time().Local().Truncate(time.Hour)
		h := byHour[t]
		if h == nil {
			h = &HourRow{Hour: t}
			byHour[t] = h
		}
		if s := r.Sample; s != nil {
			h.Samples++
			h.MemMaxPct = max(h.MemMaxPct, s.MemUsedPct)
			h.LoadMax = max(h.LoadMax, s.LoadPerCore())
			h.SwapOutMax = max(h.SwapOutMax, s.SwapOutMBps)
		}
		if e := r.Event; e != nil && e.Kind == EventAlert {
			h.AlertsFired++
		}
	}
	out := make([]HourRow, 0, len(byHour))
	for _, h := range byHour {
		out = append(out, *h)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Hour.After(out[j].Hour) })
	return out
}

package reporter

import "time"

// SchemaVersion is written to report.json so readers can reject (or adapt to)
// a layout they do not know. Bump it on any incompatible change to Report.
const SchemaVersion = 1

// Report variants. "share" is the redacted copy that is safe to hand to others.
const (
	VariantFull  = "full"
	VariantShare = "share"
)

// Overall status of a report, from the worst current finding.
const (
	StatusOK        = "ok"
	StatusAttention = "attention"
	StatusCritical  = "critical"
)

// Report is the single source of truth of one run: it is built once, and the
// markdown, JSON and HTML renderers (and the TUI) only read it. Nothing is
// recomputed per format, so the same figure can never differ between them.
type Report struct {
	SchemaVersion int       `json:"schema_version"`
	Variant       string    `json:"variant"`
	GeneratedAt   time.Time `json:"generated_at"`
	ToolVersion   string    `json:"toolkit_version"`

	Host            Host            `json:"host"`
	Summary         Summary         `json:"summary"`
	System          System          `json:"system"`
	Disk            Disk            `json:"disk"`
	Details         []Detail        `json:"details,omitempty"`
	Processes       Processes       `json:"processes"`
	Recommendations Recommendations `json:"recommendations"`
}

// Host describes the machine. It never carries a serial number or a user name.
type Host struct {
	Model       string  `json:"model,omitempty"`
	Chip        string  `json:"chip,omitempty"`
	Cores       int     `json:"cores,omitempty"`
	PerfCores   int     `json:"perf_cores,omitempty"`
	EffCores    int     `json:"eff_cores,omitempty"`
	RAM         string  `json:"ram,omitempty"`
	OS          string  `json:"macos_version,omitempty"`
	UptimeHours float64 `json:"uptime_hours,omitempty"`
}

// Finding is one prioritized observation with its suggested action. The
// toolkit never executes the action.
type Finding struct {
	Title    string `json:"title"`
	Detail   string `json:"detail,omitempty"`
	Action   string `json:"action,omitempty"`
	Severity string `json:"severity"`
}

// Summary is the executive view: a traffic light and the three top findings.
type Summary struct {
	Status string    `json:"status"`
	Top    []Finding `json:"top_findings"`
}

// System groups the live readings. A nil section means its monitor failed.
type System struct {
	Memory  *Memory `json:"memory,omitempty"`
	CPU     *CPU    `json:"cpu,omitempty"`
	Thermal string  `json:"thermal,omitempty"`
	// SpeedLimitPct is CPU_Speed_Limit when macOS reports one (100 = no limit).
	SpeedLimitPct *int `json:"cpu_speed_limit_pct,omitempty"`
}

// Memory holds RAM, pressure, swap and compressor figures. Traffic counters are
// cumulative since boot; the hourly rates are averages over the uptime, not the
// current rate (the toolkit keeps no history).
type Memory struct {
	UsedPct          float64 `json:"used_pct"`
	UsedGB           float64 `json:"used_gb"`
	TotalGB          float64 `json:"total_gb"`
	Pressure         string  `json:"pressure"`
	SwapUsedMB       float64 `json:"swap_used_mb"`
	SwapTotalMB      float64 `json:"swap_total_mb"`
	SwapInGB         float64 `json:"swap_in_gb"`
	SwapOutGB        float64 `json:"swap_out_gb"`
	SwapInGBPerHour  float64 `json:"swap_in_gb_per_hour,omitempty"`
	SwapOutGBPerHour float64 `json:"swap_out_gb_per_hour,omitempty"`
	CompressedGB     float64 `json:"compressed_gb"`
	CompressionRatio float64 `json:"compression_ratio,omitempty"`
	PagingVerdict    string  `json:"paging_verdict,omitempty"`
}

// CPU holds the time split, load averages and the corrected load verdict.
type CPU struct {
	UserPct     float64 `json:"user_pct"`
	SysPct      float64 `json:"sys_pct"`
	IdlePct     float64 `json:"idle_pct"`
	Load1       float64 `json:"load_1"`
	Load5       float64 `json:"load_5"`
	Load15      float64 `json:"load_15"`
	LoadPerCore float64 `json:"load_5_per_core"`
	Verdict     string  `json:"load_verdict,omitempty"`
}

// Disk keeps the volume usage (context) apart from what can be reclaimed: the
// first is NEVER part of the second.
type Disk struct {
	Volume      Volume      `json:"volume"`
	Reclaimable Reclaimable `json:"reclaimable"`
	Domains     []DomainRow `json:"domains"`
	Failed      []Failure   `json:"failed,omitempty"`
}

// Volume is the usage of the data volume.
type Volume struct {
	TotalBytes int64 `json:"total_bytes"`
	UsedBytes  int64 `json:"used_bytes"`
	FreeBytes  int64 `json:"free_bytes"`
	UsedPct    int   `json:"used_pct"`
	Known      bool  `json:"known"`
}

// Reclaimable is the junk that analysis found. Total = Safe + Review, always.
type Reclaimable struct {
	TotalBytes  int64 `json:"total_bytes"`
	SafeBytes   int64 `json:"safe_bytes"`
	ReviewBytes int64 `json:"review_bytes"`
}

// DomainRow is one non-empty, non-failed domain of the summary table.
type DomainRow struct {
	Domain    string  `json:"domain"`
	Severity  string  `json:"severity"`
	SizeBytes int64   `json:"size_bytes"`
	Items     int     `json:"items"`
	SharePct  float64 `json:"share_pct"`
	Summary   string  `json:"summary,omitempty"`
}

// Failure is a domain whose analysis failed; it never contributes bytes.
type Failure struct {
	Domain string `json:"domain"`
	Error  string `json:"error,omitempty"`
}

// Detail lists the biggest items of one domain; the rest is aggregated.
type Detail struct {
	Domain    string `json:"domain"`
	SizeBytes int64  `json:"size_bytes"`
	Items     []Item `json:"items,omitempty"`
	More      int    `json:"more_items,omitempty"`
	MoreBytes int64  `json:"more_bytes,omitempty"`
}

// Item is one cleanable entry as the report shows it: label shortened, path
// with "~" instead of the home folder.
type Item struct {
	Label     string `json:"label"`
	Path      string `json:"path"`
	SizeBytes int64  `json:"size_bytes"`
	Risk      string `json:"risk"`
	Safe      bool   `json:"safe"`
}

// App is one application aggregated over all its processes (no PIDs).
type App struct {
	Name   string  `json:"name"`
	MemMB  float64 `json:"mem_mb"`
	CPUPct float64 `json:"cpu_pct"`
	Procs  int     `json:"procs"`
}

// Processes lists the top applications by memory and by CPU.
type Processes struct {
	TopMemory []App `json:"top_memory"`
	TopCPU    []App `json:"top_cpu"`
}

// Recommendations are the diagnosis findings, grouped as the doctor does.
type Recommendations struct {
	Engine      string    `json:"engine"`
	Performance []Finding `json:"performance"`
	QuickWins   []Finding `json:"quick_wins"`
	Review      []Finding `json:"review"`
}

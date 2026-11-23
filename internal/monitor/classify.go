package monitor

// Classification of memory/CPU readings. These are pure functions on purpose:
// the thresholds are the contract shared by the CLI, the TUI and the doctor, so
// they live in one place and are table-tested at their boundaries.

// Memory pressure levels, from kern.memorystatus_vm_pressure_level.
const (
	PressureNormal   = "Normal"
	PressureWarning  = "Warning"
	PressureCritical = "Critical"
	PressureUnknown  = "Unknown"
)

// CPU load verdicts, from the 5-minute load average per core.
const (
	LoadNormal    = "Normal"
	LoadHigh      = "Alta"
	LoadSaturated = "Saturada"
)

// Thresholds. Named so the rules read like the spec.
const (
	pagingSwapUsedRatio = 0.80 // swap used above 80% of its total...
	pagingSwapoutsRAMx  = 2.0  // ...and swapouts above 2x physical RAM
	kernelTimePct       = 40.0 // sys above 40% and above user
	loadHighPerCore     = 1.0  // load5/cores: <1 normal, 1-2 high, >2 saturated
	loadSaturatedCores  = 2.0
)

// Cautious wording: the toolkit observes symptoms, it cannot know the cause.
const (
	NoteHeavyPaging = "Paginación intensa: el swap está casi lleno y se ha escrito mucho " +
		"a disco desde el arranque. Puede indicar falta de RAM para la carga actual."
	NoteKernelTime = "Tiempo de kernel elevado: el sistema gasta más CPU que las apps. " +
		"Puede deberse a paginación o a agentes de seguridad."
)

// ClassifyMemoryPressure maps the kernel pressure code (1 Normal, 2 Warning,
// 4 Critical) to a label. Any other value, including 0 for a failed read, is
// Unknown: not knowing must not look like an alarm.
func ClassifyMemoryPressure(code int) string {
	switch code {
	case 1:
		return PressureNormal
	case 2:
		return PressureWarning
	case 4:
		return PressureCritical
	default:
		return PressureUnknown
	}
}

// HeavyPaging reports intense paging: swap used above 80% of its total AND
// cumulative swapouts above twice the physical RAM. Requiring both avoids
// flagging a Mac that merely keeps a large, mostly idle swap file.
func HeavyPaging(swapUsedMB, swapTotalMB, swapoutsGB, ramGB float64) bool {
	if swapTotalMB <= 0 || ramGB <= 0 {
		return false
	}
	return swapUsedMB/swapTotalMB > pagingSwapUsedRatio && swapoutsGB > pagingSwapoutsRAMx*ramGB
}

// ClassifyLoadRatio classifies a load-per-core ratio: <1 Normal, 1-2 Alta
// (both bounds inclusive), >2 Saturada.
func ClassifyLoadRatio(ratio float64) string {
	switch {
	case ratio > loadSaturatedCores:
		return LoadSaturated
	case ratio >= loadHighPerCore:
		return LoadHigh
	default:
		return LoadNormal
	}
}

// ClassifyCPU classifies the 5-minute load average against the core count.
// The 5-minute window is used instead of the 1-minute one so a short burst
// does not read as a saturated machine. Unknown core count yields Normal.
func ClassifyCPU(load5 float64, cores int) string {
	if cores <= 0 {
		return LoadNormal
	}
	return ClassifyLoadRatio(load5 / float64(cores))
}

// HighKernelTime reports elevated kernel time: sys above 40% and above user.
func HighKernelTime(sysPct, userPct float64) bool {
	return sysPct > kernelTimePct && sysPct > userPct
}

package ai

// Severity strings used across insights. These are stable wire values: the
// heuristic engine emits them and the Groq response is validated against them.
const (
	SevHigh = "high"
	SevMed  = "medium"
	SevLow  = "low"
)

// Engine identifiers reported to the user for transparency (AI vs heuristic).
const (
	EngineGroq      = "groq"
	EngineHeuristic = "heuristic"
)

// Insight is a single prioritized finding. Action is a suggested next step in
// human language — doctor never executes it (read-only, Fase 1).
type Insight struct {
	Title    string `json:"title"`
	Detail   string `json:"detail"`
	Action   string `json:"action"`
	Severity string `json:"severity"`
}

// DiagnosisContext carries the disk state as context, kept separate from any
// reclaimable total so the "system state vs cleanable junk" distinction holds.
type DiagnosisContext struct {
	DiskUsedPct    int   `json:"disk_used_pct"`
	DiskFreeBytes  int64 `json:"disk_free_bytes"`
	DiskTotalBytes int64 `json:"disk_total_bytes"`
}

// Diagnosis is the structured result shared by both engines. The heuristic
// engine is the source of truth for this schema; the LLM is validated against
// it.
type Diagnosis struct {
	Engine               string           `json:"engine"`
	Performance          []Insight        `json:"performance"`
	QuickWins            []Insight        `json:"quick_wins"`
	Review               []Insight        `json:"review"`
	ReclaimableSafeBytes int64            `json:"reclaimable_safe_bytes"`
	Context              DiagnosisContext `json:"context"`
}

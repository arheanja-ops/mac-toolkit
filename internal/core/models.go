package core

import (
	"fmt"
	"time"
)

// RiskLevel represents the risk of deleting an item
type RiskLevel string

const (
	RiskSafe   RiskLevel = "safe"
	RiskWarn   RiskLevel = "warn"
	RiskDanger RiskLevel = "danger"
)

// Severity represents analysis severity based on size
type Severity string

const (
	SeverityCritical Severity = "critical"
	SeverityHigh     Severity = "high"
	SeverityMedium   Severity = "medium"
	SeverityLow      Severity = "low"
)

// SeverityFromSize returns severity based on total bytes
func SeverityFromSize(bytes int64) Severity {
	switch {
	case bytes > 10*1024*1024*1024: // >10GB
		return SeverityCritical
	case bytes > 1*1024*1024*1024: // >1GB
		return SeverityHigh
	case bytes > 100*1024*1024: // >100MB
		return SeverityMedium
	default:
		return SeverityLow
	}
}

// ApprovalMode defines how the user approves deletions
type ApprovalMode string

const (
	ApprovalDeal      ApprovalMode = "deal"
	ApprovalCategory  ApprovalMode = "category"
	ApprovalItem      ApprovalMode = "item"
	ApprovalChecklist ApprovalMode = "checklist"
)

// CleanableItem represents a single file or directory that can be cleaned
type CleanableItem struct {
	Path         string    `json:"path"`
	SizeBytes    int64     `json:"size_bytes"`
	Label        string    `json:"label"`
	Domain       string    `json:"domain"`
	SafeToDelete bool      `json:"safe_to_delete"`
	Reason       string    `json:"reason"`
	AgeDays      int       `json:"age_days,omitempty"`
	Risk         RiskLevel `json:"risk"`
}

// AnalysisResult holds the result of analyzing one domain
type AnalysisResult struct {
	Domain    string          `json:"domain"`
	Severity  Severity        `json:"severity"`
	TotalSize int64           `json:"total_size_bytes"`
	Items     []CleanableItem `json:"items"`
	Summary   string          `json:"summary"`
	Timestamp time.Time       `json:"timestamp"`
	Error     string          `json:"error,omitempty"`
}

// DeleteResult records what happened when deleting an item
type DeleteResult struct {
	Path      string `json:"path"`
	Result    string `json:"result"` // success, skipped-blacklisted, skipped-dry-run, skipped-not-found, failure
	SizeBytes int64  `json:"size_bytes,omitempty"`
	Error     string `json:"error,omitempty"`
}

// FormatBytes returns a human-readable byte string (e.g. "1.5 GB")
func FormatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

package reporter

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"github.com/arheanja-ops/mac-toolkit/internal/core"
	"os"
	"path/filepath"
	"time"
)

type AuditReporter struct {
	OutputDir    string
	DryRun       bool
	ApprovalMode core.ApprovalMode
}

func (a *AuditReporter) Name() string { return "audit" }

type auditLog struct {
	SessionID    string              `json:"session_id"`
	Timestamp    string              `json:"timestamp"`
	ExecutedBy   string              `json:"executed_by"`
	DryRun       bool                `json:"dry_run"`
	ApprovalMode string              `json:"approval_mode"`
	Deletions    []core.DeleteResult `json:"deletions"`
}

// WriteAudit writes the cleanup audit log
func (a *AuditReporter) WriteAudit(results []core.DeleteResult) error {
	if a.OutputDir == "" {
		a.OutputDir = fmt.Sprintf("reports/cleanup_%s", time.Now().Format("20060102_150405"))
	}
	if err := os.MkdirAll(a.OutputDir, 0o755); err != nil {
		return fmt.Errorf("creating audit dir: %w", err)
	}

	log := auditLog{
		SessionID:    generateUUID(),
		Timestamp:    time.Now().Format(time.RFC3339),
		ExecutedBy:   os.Getenv("USER"),
		DryRun:       a.DryRun,
		ApprovalMode: string(a.ApprovalMode),
		Deletions:    results,
	}

	path := filepath.Join(a.OutputDir, "audit.json")
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("creating audit file: %w", err)
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(log); err != nil {
		return fmt.Errorf("encoding audit: %w", err)
	}

	core.Success("Audit log saved: %s", path)
	return nil
}

// Report satisfies the Reporter interface (no-op for audit; use WriteAudit directly)
func (a *AuditReporter) Report(results []core.AnalysisResult) error {
	return nil
}

// generateUUID generates a v4 UUID without external deps
func generateUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

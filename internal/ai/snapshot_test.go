package ai

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/arheanja-ops/mac-toolkit/internal/core"
)

// TestBuildSnapshot_NoPII is the privacy guarantee (F3): given results whose
// items carry real paths/labels, the serialized AISnapshot must not leak any
// path, file name, repo name or PID.
func TestBuildSnapshot_NoPII(t *testing.T) {
	results := []core.AnalysisResult{
		{
			Domain:    "repos",
			Severity:  core.SeverityHigh,
			TotalSize: 5 * gb,
			Items: []core.CleanableItem{
				{
					Path:         "/Users/jaime.henao/arheanja/secret-repo/node_modules",
					Label:        "node_modules (secret-repo)",
					SizeBytes:    5 * gb,
					SafeToDelete: true,
					Risk:         core.RiskSafe,
				},
			},
		},
		{
			Domain:    "downloads",
			Severity:  core.SeverityMedium,
			TotalSize: 2 * gb,
			Items: []core.CleanableItem{
				{Path: "/Users/jaime.henao/Downloads/private-tax-return.pdf", Label: "tax", SizeBytes: 2 * gb, Risk: core.RiskWarn},
			},
		},
	}
	sys := map[string]any{"top_cpu_name": "WindowServer", "top_cpu_pct": 12.0}
	disk := core.DiskSnapshot{TotalBytes: 500 * gb, AvailBytes: 100 * gb, PctUsed: 80, OK: true}

	snap := BuildSnapshot(results, sys, disk)

	raw, err := json.Marshal(snap)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	blob := string(raw)

	forbidden := []string{
		"/Users/jaime.henao",
		"secret-repo",
		"node_modules",
		"private-tax-return.pdf",
		"Downloads",
		".pdf",
	}
	for _, f := range forbidden {
		if strings.Contains(blob, f) {
			t.Fatalf("snapshot leaked PII %q in: %s", f, blob)
		}
	}
}

// TestBuildSnapshot_DiskIsContextNotReclaimable verifies the core bug fix: the
// `disk` domain must be context only, never summed into reclaimable, and never
// appear as a domain summary.
func TestBuildSnapshot_DiskIsContextNotReclaimable(t *testing.T) {
	results := []core.AnalysisResult{
		{
			Domain:    "disk",
			TotalSize: 346 * gb,
			Items: []core.CleanableItem{
				{Path: "/", Label: "volume", SizeBytes: 346 * gb, SafeToDelete: true, Risk: core.RiskSafe},
			},
		},
		{
			Domain:    "browser",
			TotalSize: 2 * gb,
			Items: []core.CleanableItem{
				{Path: "/x", Label: "cache", SizeBytes: 2 * gb, SafeToDelete: true, Risk: core.RiskSafe},
			},
		},
	}
	disk := core.DiskSnapshot{TotalBytes: 500 * gb, AvailBytes: 154 * gb, PctUsed: 69}

	snap := BuildSnapshot(results, nil, disk)

	if snap.ReclaimableSafeBytes != 2*gb {
		t.Fatalf("reclaimable = %d, want %d (disk must be excluded)", snap.ReclaimableSafeBytes, 2*gb)
	}
	for _, d := range snap.Domains {
		if d.Domain == "disk" {
			t.Fatalf("disk must not appear as a domain summary")
		}
	}
	if snap.DiskUsedPct != 69 || snap.DiskTotalBytes != 500*gb {
		t.Fatalf("disk context not carried through: %+v", snap)
	}
}

// TestBuildSnapshot_ErroredDomainExcludedFromReclaimable ensures a partial
// failure never inflates the reclaimable total.
func TestBuildSnapshot_ErroredDomainExcludedFromReclaimable(t *testing.T) {
	results := []core.AnalysisResult{
		{
			Domain:    "xcode",
			TotalSize: 8 * gb,
			Error:     "scan timed out",
			Items: []core.CleanableItem{
				{Path: "/x", SizeBytes: 8 * gb, SafeToDelete: true, Risk: core.RiskSafe},
			},
		},
		{
			Domain:    "logs",
			TotalSize: 1 * gb,
			Items: []core.CleanableItem{
				{Path: "/y", SizeBytes: 1 * gb, SafeToDelete: true, Risk: core.RiskSafe},
			},
		},
	}
	snap := BuildSnapshot(results, nil, core.DiskSnapshot{})

	if snap.ReclaimableSafeBytes != 1*gb {
		t.Fatalf("reclaimable = %d, want %d (errored domain excluded)", snap.ReclaimableSafeBytes, 1*gb)
	}
	// The errored domain is still summarized (for visibility), just not counted.
	if !hasDomain(snap.Domains, "xcode") {
		t.Fatalf("errored domain should still be summarized")
	}
}

func TestBuildSnapshot_TopRisk(t *testing.T) {
	results := []core.AnalysisResult{{
		Domain: "mixed",
		Items: []core.CleanableItem{
			{Risk: core.RiskSafe}, {Risk: core.RiskDanger}, {Risk: core.RiskWarn},
		},
	}}
	snap := BuildSnapshot(results, nil, core.DiskSnapshot{})
	if snap.Domains[0].TopRisk != string(core.RiskDanger) {
		t.Fatalf("top risk = %q, want danger", snap.Domains[0].TopRisk)
	}
}

func hasDomain(domains []DomainSummary, name string) bool {
	for _, d := range domains {
		if d.Domain == name {
			return true
		}
	}
	return false
}

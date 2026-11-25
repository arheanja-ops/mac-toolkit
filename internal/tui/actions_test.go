package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/arheanja-ops/mac-toolkit/internal/core"
)

func TestFormatSnapshotSortedAndSafe(t *testing.T) {
	out := formatSnapshot("system", map[string]any{
		"cpu_percent":    34.0,
		"memory_percent": 71.0,
		"thermal":        "Normal",
	}, 80)
	if !strings.Contains(out, "system monitor") {
		t.Fatalf("missing header, got: %q", out)
	}
	// Keys must be sorted deterministically (cpu before memory before thermal).
	iCPU := strings.Index(out, "cpu_percent")
	iMem := strings.Index(out, "memory_percent")
	iTherm := strings.Index(out, "thermal")
	if !(iCPU < iMem && iMem < iTherm) {
		t.Fatalf("keys not sorted: cpu=%d mem=%d thermal=%d", iCPU, iMem, iTherm)
	}
}

func TestFormatSnapshotEmpty(t *testing.T) {
	out := formatSnapshot("system", map[string]any{}, 80)
	if !strings.Contains(out, "no data available") {
		t.Fatalf("expected empty marker, got: %q", out)
	}
}

func TestMonitorRunnerUsesSnapshot(t *testing.T) {
	run := monitorRunner(fakeMonitor{name: "fake", data: map[string]any{"k": "v"}})
	out, err := run(80)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "k:") || !strings.Contains(out, "v") {
		t.Fatalf("snapshot not formatted: %q", out)
	}
}

func TestMonitorRunnerPropagatesError(t *testing.T) {
	run := monitorRunner(fakeMonitor{name: "fake", err: errBoom})
	if _, err := run(80); err == nil {
		t.Fatal("expected error from snapshot")
	}
}

func TestRunLastReportReadsNewest(t *testing.T) {
	dir := t.TempDir()
	withReportsDir(t, dir)
	for name, body := range map[string]string{
		"analysis_20200101_000000": "# Old\n\nOLDEST",
		"analysis_20260921_101500": "# New\n\nNEWEST",
	} {
		d := filepath.Join(dir, name)
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(d, "report.md"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	out, err := runLastReport(80)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "NEWEST") || strings.Contains(out, "OLDEST") {
		t.Fatalf("expected newest report only, got: %q", out)
	}
}

func TestRunStatusListsHeader(t *testing.T) {
	out, err := runStatus(80)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Domain") || !strings.Contains(out, "Risk") {
		t.Fatalf("status header missing: %q", out)
	}
}

// TestBuildActionsCleanIsGatedNotDirect verifies the TUI-2 safety contract:
// the Clean group exists, but "Clean (select & delete)" is an openClean control
// action (it enters the interactive select flow) — never a direct run() that
// could delete on selection. Preview is a plain read-only run(). No action both
// claims to clean AND carries a run() that could bypass the F2/F3 gate.
func TestBuildActionsCleanIsGatedNotDirect(t *testing.T) {
	var sawPreview, sawClean bool
	for _, a := range buildActions() {
		switch a.label {
		case "Preview":
			sawPreview = true
			if a.openClean {
				t.Fatal("Preview must not open the delete flow; it is read-only")
			}
			if a.run == nil {
				t.Fatal("Preview must have a read-only run()")
			}
		case "Select & clean":
			sawClean = true
			if !a.openClean {
				t.Fatal("Clean must be an openClean action (enters select mode)")
			}
			if a.run != nil {
				t.Fatal("Clean must NOT have a run(); deletion only via F2/F3 gate")
			}
			if a.group != "Clean" {
				t.Fatalf("Clean action should be in Clean group, got %q", a.group)
			}
		}
	}
	if !sawPreview || !sawClean {
		t.Fatalf("expected both Preview and Clean actions; preview=%v clean=%v", sawPreview, sawClean)
	}
}

func TestBuildActionsHasQuit(t *testing.T) {
	var found bool
	for _, a := range buildActions() {
		if a.quit {
			found = true
			if a.group != "Session" {
				t.Fatalf("quit action should be in Session group, got %q", a.group)
			}
		}
	}
	if !found {
		t.Fatal("expected a quit action in buildActions()")
	}
}

// TestFormatAnalysisExcludesDiskAndErrors verifies the reclaimable total counts
// only genuine junk: the "disk" domain (volume usage) and errored domains must
// not inflate it.
func TestFormatAnalysisExcludesDiskAndErrors(t *testing.T) {
	results := []core.AnalysisResult{
		{Domain: "disk", TotalSize: 365 * 1024 * 1024 * 1024, Severity: core.SeverityLow},
		{Domain: "dev_caches", TotalSize: 2 * 1024 * 1024 * 1024, Severity: core.SeverityLow},
		{Domain: "docker", TotalSize: 8 * 1024 * 1024 * 1024, Severity: core.SeverityHigh},
		{Domain: "ollama", Error: "read failed"},
	}
	out := formatAnalysis(results, 80)

	// Reclaimable = dev_caches + docker = 10 GB. Disk (365 GB) and the errored
	// ollama domain must be excluded.
	reclaimable := core.FormatBytes(10 * 1024 * 1024 * 1024)
	if !strings.Contains(out, "Total reclaimable (excl. disk): "+reclaimable) {
		t.Fatalf("reclaimable total wrong; want %s, got:\n%s", reclaimable, out)
	}
	// Disk is reported as context, not inside the total.
	if !strings.Contains(out, "Disk usage (context): "+core.FormatBytes(365*1024*1024*1024)) {
		t.Fatalf("disk context line missing:\n%s", out)
	}
	// The errored domain still appears in the table as an error row.
	if !strings.Contains(out, "read failed") {
		t.Fatalf("errored domain should still be listed:\n%s", out)
	}
}

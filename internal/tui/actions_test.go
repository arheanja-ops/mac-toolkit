package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFormatSnapshotSortedAndSafe(t *testing.T) {
	out := formatSnapshot("system", map[string]any{
		"cpu_percent":    34.0,
		"memory_percent": 71.0,
		"thermal":        "Normal",
	})
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
	out := formatSnapshot("system", map[string]any{})
	if !strings.Contains(out, "no data available") {
		t.Fatalf("expected empty marker, got: %q", out)
	}
}

func TestMonitorRunnerUsesSnapshot(t *testing.T) {
	run := monitorRunner(fakeMonitor{name: "fake", data: map[string]any{"k": "v"}})
	out, err := run()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "k:") || !strings.Contains(out, "v") {
		t.Fatalf("snapshot not formatted: %q", out)
	}
}

func TestMonitorRunnerPropagatesError(t *testing.T) {
	run := monitorRunner(fakeMonitor{name: "fake", err: errBoom})
	if _, err := run(); err == nil {
		t.Fatal("expected error from snapshot")
	}
}

func TestRunLastReportReadsNewest(t *testing.T) {
	dir := t.TempDir()
	// Two report dirs; the lexicographically greater name is "newest".
	older := filepath.Join(dir, "2020-01-01")
	newer := filepath.Join(dir, "2026-09-21")
	for _, d := range []string{older, newer} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(newer, "report.md"), []byte("NEWEST"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(older, "report.md"), []byte("OLDEST"), 0o644); err != nil {
		t.Fatal(err)
	}

	orig := reportsDir
	reportsDir = dir
	defer func() { reportsDir = orig }()

	out, err := runLastReport()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "NEWEST" {
		t.Fatalf("expected newest report, got: %q", out)
	}
}

func TestRunLastReportNoDir(t *testing.T) {
	orig := reportsDir
	reportsDir = filepath.Join(t.TempDir(), "does-not-exist")
	defer func() { reportsDir = orig }()
	if _, err := runLastReport(); err == nil {
		t.Fatal("expected error when reports dir is missing")
	}
}

func TestRunStatusListsHeader(t *testing.T) {
	out, err := runStatus()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Domain") || !strings.Contains(out, "Risk") {
		t.Fatalf("status header missing: %q", out)
	}
}

func TestBuildActionsAreReadOnly(t *testing.T) {
	for _, a := range buildActions() {
		l := strings.ToLower(a.label)
		if strings.Contains(l, "clean") || strings.Contains(l, "preview") || strings.Contains(l, "full") {
			t.Fatalf("destructive action leaked into TUI: %q", a.label)
		}
	}
}

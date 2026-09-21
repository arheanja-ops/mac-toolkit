package tui

import (
	"strings"
	"testing"

	"github.com/arheanja-ops/mac-toolkit/internal/core"
)

func TestRenderHeaderAlwaysShowsVersion(t *testing.T) {
	out := renderHeader("v1.2.3", core.DiskSnapshot{OK: false}, headerStats{ok: false}, 100)
	if !strings.Contains(out, "v1.2.3") {
		t.Fatalf("version missing: %q", out)
	}
	// With disk OK=false, the header must omit the disk field without breaking.
	if strings.Contains(out, "Disk") {
		t.Fatalf("disk field should be omitted when snapshot not OK: %q", out)
	}
}

func TestRenderHeaderDiskOnlyWhenSystemFails(t *testing.T) {
	disk := core.DiskSnapshot{OK: true, PctUsed: 62, AvailBytes: 128 * 1024 * 1024 * 1024}
	out := renderHeader("v1", disk, headerStats{ok: false}, 100)
	if !strings.Contains(out, "Disk") {
		t.Fatalf("disk should be present: %q", out)
	}
	// system failed → mem/CPU/thermal omitted.
	if strings.Contains(out, "Mem") || strings.Contains(out, "CPU") || strings.Contains(out, "🌡") {
		t.Fatalf("system fields should be omitted when stats not ok: %q", out)
	}
}

func TestRenderHeaderNarrowCollapsesToDiskAndMem(t *testing.T) {
	disk := core.DiskSnapshot{OK: true, PctUsed: 50, AvailBytes: 10 * 1024 * 1024 * 1024}
	stats := headerStats{ok: true, memUsedPct: 40, cpuUsedPct: 30, thermal: "Normal"}
	out := renderHeader("v1", disk, stats, 60) // < 80 cols
	if !strings.Contains(out, "Mem") {
		t.Fatalf("memory expected on narrow header: %q", out)
	}
	if strings.Contains(out, "CPU") || strings.Contains(out, "🌡") {
		t.Fatalf("narrow header must drop CPU/thermal: %q", out)
	}
}

func TestReadHeaderStatsDegradesOnError(t *testing.T) {
	orig := systemSnapshotFn
	systemSnapshotFn = func() (map[string]any, error) { return nil, errBoom }
	defer func() { systemSnapshotFn = orig }()

	s := readHeaderStats()
	if s.ok {
		t.Fatal("expected ok=false when system snapshot fails")
	}
}

func TestReadHeaderStatsParsesFields(t *testing.T) {
	orig := systemSnapshotFn
	systemSnapshotFn = func() (map[string]any, error) {
		return map[string]any{
			"memory_percent": 71.5,
			"cpu_percent":    34.9,
			"thermal":        "Normal",
		}, nil
	}
	defer func() { systemSnapshotFn = orig }()

	s := readHeaderStats()
	if !s.ok || s.memUsedPct != 71 || s.cpuUsedPct != 34 || s.thermal != "Normal" {
		t.Fatalf("unexpected stats: %+v", s)
	}
}

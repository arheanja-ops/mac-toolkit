package reporter

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/arheanja-ops/mac-toolkit/internal/ai"
	"github.com/arheanja-ops/mac-toolkit/internal/core"
	"github.com/arheanja-ops/mac-toolkit/internal/monitor"
)

const (
	gb       = int64(1) << 30
	mb       = int64(1) << 20
	fakeHome = "/Users/someone"
	digest   = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
)

// sampleInputs is a synthetic machine: a 400 GB used volume (the disk domain),
// three reclaimable domains, one failed domain and a couple of apps.
func sampleInputs() Inputs {
	return Inputs{
		Now:  time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
		Home: fakeHome,
		Results: []core.AnalysisResult{
			{Domain: "disk", Severity: core.SeverityHigh, TotalSize: 400 * gb},
			{Domain: "logs", Severity: core.SeverityLow, TotalSize: 1 * gb, Summary: "logs in " + fakeHome + "/Library/Logs",
				Items: []core.CleanableItem{{Path: fakeHome + "/Library/Logs/app.log", Label: "app.log", SizeBytes: 1 * gb, SafeToDelete: true, Risk: core.RiskSafe}}},
			{Domain: "docker", Severity: core.SeverityCritical, TotalSize: 30 * gb,
				Items: []core.CleanableItem{
					{Path: fakeHome + "/Library/Containers/docker/Docker.raw", Label: "Docker.raw", SizeBytes: 20 * gb, Risk: core.RiskDanger},
					{Path: "image:" + digest, Label: "image sha256:" + digest, SizeBytes: 10 * gb, Risk: core.RiskWarn},
				}},
			{Domain: "caches", Severity: core.SeverityMedium, TotalSize: 5 * gb,
				Items: []core.CleanableItem{{Path: fakeHome + "/Library/Caches/<script>alert(1)</script>", Label: "<script>alert(1)</script>", SizeBytes: 5 * gb, SafeToDelete: true, Risk: core.RiskSafe}}},
			{Domain: "ollama", Error: "timeout reading " + fakeHome + "/.ollama"},
			{Domain: "trash", Severity: core.SeverityLow},
		},
		Metrics: ai.Metrics{
			System: map[string]any{"model": "MacBook Pro", "chip": "Apple M2", "ram": "16 GB", "os_version": "15.1"},
			Memory: map[string]any{"used_pct": 72.0, "used_gb": 11.5, "ram_total_gb": 16.0, "pressure_level": "normal",
				"swap_used_mb": 1024.0, "swap_total_mb": 2048.0, "uptime_hours": 50.0,
				"top_memory": []monitor.AppUsage{{Name: "SecretEditor", MemMB: 2048, Procs: 3}}},
			CPU: map[string]any{"user_pct": 20.0, "sys_pct": 10.0, "idle_pct": 70.0, "load5": 2.0, "cores_total": 10,
				"top_cpu": []monitor.AppUsage{{Name: "SecretEditor", CPUPct: 45, Procs: 3}}},
		},
		Disk: core.DiskSnapshot{TotalBytes: 500 * gb, UsedBytes: 400 * gb, AvailBytes: 100 * gb, PctUsed: 80, OK: true},
		Diagnosis: ai.Diagnosis{
			Performance: []ai.Insight{{Title: "SecretEditor usa mucha CPU", Detail: "revisa " + fakeHome + "/Projects/x", Action: "cierra \"SecretEditor\"", Severity: ai.SevMed}},
			QuickWins:   []ai.Insight{{Title: "Cachés", Action: "limpia ~/Library/Caches", Severity: ai.SevLow}},
			Review:      []ai.Insight{{Title: "Docker grande", Detail: "imagen " + digest, Severity: ai.SevHigh}},
		},
		Engine: "heurístico",
	}
}

func TestReclaimableExcludesVolumeAndAddsUp(t *testing.T) {
	r := Build(sampleInputs())
	rc := r.Disk.Reclaimable
	if rc.TotalBytes != 36*gb {
		t.Fatalf("total must be logs+docker+caches (36 GB) without the 400 GB volume, got %s", core.FormatBytes(rc.TotalBytes))
	}
	if rc.SafeBytes != 6*gb || rc.ReviewBytes != 30*gb || rc.SafeBytes+rc.ReviewBytes != rc.TotalBytes {
		t.Fatalf("safe=%d review=%d total=%d", rc.SafeBytes, rc.ReviewBytes, rc.TotalBytes)
	}
	if !r.Disk.Volume.Known || r.Disk.Volume.UsedBytes != 400*gb {
		t.Fatalf("volume must be kept as context: %+v", r.Disk.Volume)
	}
	for _, d := range r.Disk.Domains {
		if d.Domain == "disk" || d.Domain == "ollama" || d.Domain == "trash" {
			t.Fatalf("%s must not be a reclaimable row", d.Domain)
		}
	}
	if len(r.Disk.Failed) != 1 || r.Disk.Failed[0].Domain != "ollama" {
		t.Fatalf("failed domains: %+v", r.Disk.Failed)
	}
}

func TestDomainsAndItemsSortedBySize(t *testing.T) {
	r := Build(sampleInputs())
	var got []string
	for _, d := range r.Disk.Domains {
		got = append(got, d.Domain)
	}
	if strings.Join(got, ",") != "docker,caches,logs" {
		t.Fatalf("domains must be largest first, got %v", got)
	}
	for _, det := range r.Details {
		for i := 1; i < len(det.Items); i++ {
			if det.Items[i].SizeBytes > det.Items[i-1].SizeBytes {
				t.Fatalf("%s items not sorted", det.Domain)
			}
		}
	}
	if r.Summary.Status != StatusAttention {
		t.Fatalf("status comes from performance only (medium → attention), got %s", r.Summary.Status)
	}
	if len(r.Summary.Top) != 3 || r.Summary.Top[0].Severity != ai.SevMed {
		t.Fatalf("top findings: %+v", r.Summary.Top)
	}
}

func TestPathsUseTildeAndShortDigests(t *testing.T) {
	r := Build(sampleInputs())
	for _, det := range r.Details {
		for _, it := range det.Items {
			if strings.Contains(it.Path, fakeHome) || strings.Contains(it.Label, digest) {
				t.Fatalf("item not shortened: %+v", it)
			}
		}
	}
}

func TestHTMLIsSelfContainedAndEscaped(t *testing.T) {
	for _, r := range []Report{Build(sampleInputs()), Build(sampleInputs()).Share()} {
		b, err := r.HTML()
		if err != nil {
			t.Fatal(err)
		}
		page := string(b)
		lower := strings.ToLower(page)
		for _, bad := range []string{"<script", "<link", "src=", "http://", "https://", "@import", "url("} {
			if strings.Contains(lower, bad) {
				t.Fatalf("%s html must be self-contained, found %q", r.Variant, bad)
			}
		}
		if !strings.HasPrefix(page, "<!DOCTYPE html>") || !strings.Contains(page, "<style>") {
			t.Fatal("html must be a full page with inline css")
		}
	}
	page, _ := Build(sampleInputs()).HTML()
	if !strings.Contains(string(page), "&lt;script&gt;alert(1)&lt;/script&gt;") {
		t.Fatal("item labels must be html-escaped")
	}
}

func TestShareHasNoPathsHashesLabelsOrAppNames(t *testing.T) {
	full := Build(sampleInputs())
	s := full.Share()
	if s.Variant != VariantShare || len(s.Details) != 0 {
		t.Fatalf("share must drop the detail: %+v", s.Details)
	}
	if s.Disk.Reclaimable != full.Disk.Reclaimable {
		t.Fatal("share keeps the figures")
	}
	js, err := s.JSON()
	if err != nil {
		t.Fatal(err)
	}
	for name, out := range map[string]string{"md": s.Markdown(), "json": string(js), "html": mustHTML(t, s)} {
		for _, bad := range []string{"/", "~", "someone", "SecretEditor", "Docker.raw", "app.log", "alert(1)"} {
			text := out
			if bad == "/" {
				text = stripMarkup(name, out)
			}
			if strings.Contains(text, bad) {
				t.Fatalf("share %s leaks %q", name, bad)
			}
		}
		if strings.Contains(out, digest[:12]) {
			t.Fatalf("share %s leaks a hash", name)
		}
	}
	if s.Processes.TopMemory[0].Name != "App 1" || s.Processes.TopMemory[0].MemMB != 2048 {
		t.Fatalf("apps must be anonymized keeping figures: %+v", s.Processes.TopMemory)
	}
	// The full report is untouched by Share().
	if full.Processes.TopMemory[0].Name != "SecretEditor" || len(full.Details) == 0 {
		t.Fatal("Share must not mutate the full report")
	}
}

// stripMarkup drops the inline css and the html tags: their "/" is markup,
// not report data.
func stripMarkup(format, s string) string {
	if format != "html" {
		return s
	}
	s = regexp.MustCompile(`(?s)<style>.*</style>`).ReplaceAllString(s, "")
	return regexp.MustCompile(`<[^>]*>`).ReplaceAllString(s, "")
}

func mustHTML(t *testing.T, r Report) string {
	t.Helper()
	b, err := r.HTML()
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestFormatsRenderTheSameModel(t *testing.T) {
	r := Build(sampleInputs())
	js, err := r.JSON()
	if err != nil {
		t.Fatal(err)
	}
	var back Report
	if err := json.Unmarshal(js, &back); err != nil {
		t.Fatal(err)
	}
	if back.Disk.Reclaimable != r.Disk.Reclaimable || len(back.Disk.Domains) != len(r.Disk.Domains) {
		t.Fatal("json must be the model itself")
	}
	md, page := r.Markdown(), mustHTML(t, r)
	for _, want := range []string{
		core.FormatBytes(r.Disk.Reclaimable.TotalBytes),
		core.FormatBytes(r.Disk.Reclaimable.SafeBytes),
		core.FormatBytes(r.Disk.Reclaimable.ReviewBytes),
		core.FormatBytes(r.Disk.Volume.UsedBytes),
		StatusLabel(r.Summary.Status),
	} {
		if !strings.Contains(md, want) || !strings.Contains(page, want) {
			t.Fatalf("%q must appear in md and html", want)
		}
	}
	for _, d := range r.Disk.Domains {
		if !strings.Contains(md, d.Domain) || !strings.Contains(page, d.Domain) {
			t.Fatalf("domain %s missing in a format", d.Domain)
		}
	}
}

func TestOpenReportUsesInjectedRunnerOnly(t *testing.T) {
	root := withReportsDir(t)
	var opened []string
	orig := OpenFn
	OpenFn = func(p string) error { opened = append(opened, p); return nil }
	t.Cleanup(func() { OpenFn = orig })

	saved, err := SaveReport(Build(sampleInputs()))
	if err != nil {
		t.Fatal(err)
	}
	p, err := OpenReport(saved.Dir, false)
	if err != nil || p != filepath.Join(saved.Dir, FileHTML) {
		t.Fatalf("open full: %q %v", p, err)
	}
	if p, err = OpenReport(saved.Dir, true); err != nil || p != filepath.Join(saved.Dir, ShareDir, FileHTML) {
		t.Fatalf("open share: %q %v", p, err)
	}
	if len(opened) != 2 {
		t.Fatalf("runner calls: %v", opened)
	}
	// Outside the reports folder, or a missing file: refused, runner untouched.
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, FileHTML), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{outside, filepath.Join(root, "analysis_missing"), "relative"} {
		if _, err := OpenReport(dir, false); err == nil {
			t.Fatalf("%s must be refused", dir)
		}
	}
	if len(opened) != 2 {
		t.Fatalf("refused opens must not run: %v", opened)
	}
	OpenFn = func(string) error { return errors.New("no opener") }
	if _, err = OpenReport(saved.Dir, false); err == nil {
		t.Fatal("runner error must surface")
	}
}

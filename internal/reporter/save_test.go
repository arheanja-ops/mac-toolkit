package reporter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/arheanja-ops/mac-toolkit/internal/core"
)

func withReportsDir(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	core.ReportsDirOverride = root
	t.Cleanup(func() { core.ReportsDirOverride = "" })
	return root
}

func TestSaveReportWritesAllFormatsToCanonicalFolder(t *testing.T) {
	root := withReportsDir(t)
	// A different cwd must not change where reports land.
	t.Chdir(t.TempDir())

	saved, err := SaveReport(Build(sampleInputs()))
	if err != nil {
		t.Fatal(err)
	}
	dir := saved.Dir
	if !filepath.IsAbs(dir) || filepath.Dir(dir) != root || !strings.HasPrefix(filepath.Base(dir), core.AnalysisPrefix) {
		t.Fatalf("report folder %q is not under %q", dir, root)
	}
	if len(saved.Full) != 3 || len(saved.Share) != 3 {
		t.Fatalf("want 3 full + 3 share files, got %v / %v", saved.Full, saved.Share)
	}
	for _, p := range append(append([]string{}, saved.Full...), saved.Share...) {
		info, err := os.Stat(p)
		if err != nil || info.Size() == 0 {
			t.Fatalf("%s missing or empty: %v", p, err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("%s must be private, got %v", p, info.Mode().Perm())
		}
		if !strings.HasPrefix(p, dir+string(filepath.Separator)) {
			t.Fatalf("%s written outside the run folder", p)
		}
	}
	e, ok, err := core.LatestReport()
	if err != nil || !ok {
		t.Fatalf("saved report must be listable: ok=%v err=%v", ok, err)
	}
	if strings.Join(e.Formats, ",") != "md,json,html" || !e.HasShare {
		t.Fatalf("entry formats=%v share=%v", e.Formats, e.HasShare)
	}
	r, err := LoadJSON(filepath.Join(dir, FileJSON))
	if err != nil || r.Disk.Reclaimable.TotalBytes != Build(sampleInputs()).Disk.Reclaimable.TotalBytes {
		t.Fatalf("report.json must round-trip: %v", err)
	}
}

func TestLoadJSONRejectsUnknownSchema(t *testing.T) {
	p := filepath.Join(t.TempDir(), FileJSON)
	if err := os.WriteFile(p, []byte(`{"schema_version": 99}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadJSON(p); err == nil {
		t.Fatal("a newer schema must be refused")
	}
}

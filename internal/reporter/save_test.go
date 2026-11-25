package reporter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/arheanja-ops/mac-toolkit/internal/core"
)

func TestSaveAnalysisWritesToCanonicalFolder(t *testing.T) {
	root := t.TempDir()
	core.ReportsDirOverride = root
	t.Cleanup(func() { core.ReportsDirOverride = "" })

	// A different cwd must not change where reports land.
	t.Chdir(t.TempDir())

	results := []core.AnalysisResult{{Domain: "caches", TotalSize: 1024, Severity: core.SeverityLow}}
	dir, err := SaveAnalysis(results, true)
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(dir) || filepath.Dir(dir) != root || !strings.HasPrefix(filepath.Base(dir), core.AnalysisPrefix) {
		t.Fatalf("report folder %q is not under %q", dir, root)
	}
	for _, f := range []string{"report.md", "report.json"} {
		if info, err := os.Stat(filepath.Join(dir, f)); err != nil || info.Size() == 0 {
			t.Fatalf("%s missing or empty: %v", f, err)
		}
	}
	if _, ok, err := core.LatestReport(); err != nil || !ok {
		t.Fatalf("saved report must be listable: ok=%v err=%v", ok, err)
	}
}

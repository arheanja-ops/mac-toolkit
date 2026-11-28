package cmd

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/arheanja-ops/mac-toolkit/internal/core"
	"github.com/arheanja-ops/mac-toolkit/internal/reporter"
)

func TestReportOutExportsTheLastReport(t *testing.T) {
	core.ReportsDirOverride = t.TempDir()
	t.Cleanup(func() { core.ReportsDirOverride = "" })
	if _, err := reporter.SaveReport(reporter.Build(reporter.Inputs{Now: time.Now(), Home: "/Users/someone"})); err != nil {
		t.Fatal(err)
	}
	dest := t.TempDir()
	if err := runReport(false, true, false, dest); err != nil {
		t.Fatal(err)
	}
	dirs, _ := filepath.Glob(filepath.Join(dest, reporter.ExportPrefix+"*-share"))
	if len(dirs) != 1 {
		t.Fatalf("want one share export under dest, got %v", dirs)
	}
	if _, err := os.Stat(filepath.Join(dirs[0], reporter.FileHTML)); err != nil {
		t.Fatalf("exported html missing: %v", err)
	}
	if err := runReport(false, false, false, "relativa"); err == nil {
		t.Fatal("a relative --out must be refused")
	}
}

package reporter

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/arheanja-ops/mac-toolkit/internal/core"
)

// OpenFn launches the system opener for a file. It is a package var so tests
// never open anything on the machine. This is the ONLY place where the toolkit
// starts another program on the user's behalf: `open <path>` with arguments
// (no shell), and only for an existing report.html inside the reports folder.
var OpenFn = func(path string) error {
	return exec.Command("open", path).Run()
}

// OpenReport opens the report.html of a run folder (or its share variant).
// The path must be absolute and inside dir's tree; anything else is refused so
// the opener can never be pointed at an arbitrary file.
func OpenReport(runDir string, share bool) (string, error) {
	p := filepath.Join(runDir, FileHTML)
	if share {
		p = filepath.Join(runDir, ShareDir, FileHTML)
	}
	if !filepath.IsAbs(p) || !strings.HasPrefix(p, filepath.Clean(core.ReportsDir())+string(filepath.Separator)) {
		return "", fmt.Errorf("refusing to open %q: outside the reports folder", p)
	}
	if _, err := os.Stat(p); err != nil {
		return "", fmt.Errorf("no HTML report here: %w", err)
	}
	if err := OpenFn(p); err != nil {
		return p, fmt.Errorf("open: %w", err)
	}
	return p, nil
}

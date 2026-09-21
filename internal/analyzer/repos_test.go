package analyzer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/arheanja-ops/mac-toolkit/internal/core"
)

// writeFile creates a file of exactly size bytes, making parent dirs as needed.
func writeFile(t *testing.T, path string, size int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, make([]byte, size), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// buildRepoTree creates a source tree with several prunable targets and returns
// the temp root. Layout:
//
//	root/
//	  projA/node_modules/x.js            (1000)
//	  projA/node_modules/nested/node_modules/y.js  (5000) -> no separate item
//	  projB/.venv/lib.py                 (2000)
//	  projB/src/main.py                  (999)  -> not a target, ignored
//	  projC/__pycache__/m.pyc            (300)
//	  .git/objects/blob                  (7000) -> skipped dir, ignored
func buildRepoTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	writeFile(t, filepath.Join(root, "projA", "node_modules", "x.js"), 1000)
	writeFile(t, filepath.Join(root, "projA", "node_modules", "nested", "node_modules", "y.js"), 5000)
	writeFile(t, filepath.Join(root, "projB", ".venv", "lib.py"), 2000)
	writeFile(t, filepath.Join(root, "projB", "src", "main.py"), 999)
	writeFile(t, filepath.Join(root, "projC", "__pycache__", "m.pyc"), 300)
	writeFile(t, filepath.Join(root, ".git", "objects", "blob"), 7000)

	return root
}

func TestReposAnalyzeDetectsAllTargets(t *testing.T) {
	root := buildRepoTree(t)

	saved := core.RepoRoots
	defer func() { core.RepoRoots = saved }()
	core.RepoRoots = []string{root}

	result, err := (&ReposAnalyzer{}).Analyze()
	if err != nil {
		t.Fatalf("Analyze error: %v", err)
	}

	// Three top-level targets: node_modules, .venv, __pycache__.
	// The node_modules nested inside another node_modules must NOT appear.
	if len(result.Items) != 3 {
		t.Fatalf("Items = %d, want 3 (%+v)", len(result.Items), result.Items)
	}

	for _, it := range result.Items {
		if !it.SafeToDelete {
			t.Errorf("%s: SafeToDelete = false, want true", it.Path)
		}
		if it.Risk != core.RiskSafe {
			t.Errorf("%s: Risk = %v, want RiskSafe", it.Path, it.Risk)
		}
		if it.Domain != "repos" {
			t.Errorf("%s: Domain = %s, want repos", it.Path, it.Domain)
		}
	}
}

func TestReposAnalyzeSizesSumCorrectly(t *testing.T) {
	root := buildRepoTree(t)

	saved := core.RepoRoots
	defer func() { core.RepoRoots = saved }()
	core.RepoRoots = []string{root}

	result, err := (&ReposAnalyzer{}).Analyze()
	if err != nil {
		t.Fatalf("Analyze error: %v", err)
	}

	// The outer node_modules is measured recursively, so it already includes
	// the nested node_modules bytes: 1000 + 5000 = 6000. Plus .venv (2000) and
	// __pycache__ (300) = 8300. We never emit a *separate* item for the nested
	// target, but its bytes are counted once inside the outer one — exactly the
	// original behavior.
	const want = 6000 + 2000 + 300
	if result.TotalSize != want {
		t.Errorf("TotalSize = %d, want %d", result.TotalSize, want)
	}
}

func TestReposAnalyzeDoesNotDescendIntoTarget(t *testing.T) {
	root := buildRepoTree(t)

	saved := core.RepoRoots
	defer func() { core.RepoRoots = saved }()
	core.RepoRoots = []string{root}

	result, err := (&ReposAnalyzer{}).Analyze()
	if err != nil {
		t.Fatalf("Analyze error: %v", err)
	}

	outer := filepath.Join(root, "projA", "node_modules")
	nested := filepath.Join(outer, "nested", "node_modules")

	var sawOuter bool
	for _, it := range result.Items {
		if it.Path == nested {
			t.Errorf("nested target was counted separately: %s", nested)
		}
		if it.Path == outer {
			sawOuter = true
			// The outer node_modules size must include everything under it
			// (1000 + nested 5000) because DirSize is recursive; we just do
			// not emit a separate item for the nested one.
			if it.SizeBytes != 6000 {
				t.Errorf("outer node_modules size = %d, want 6000", it.SizeBytes)
			}
		}
	}
	if !sawOuter {
		t.Fatalf("outer node_modules %s not found in items", outer)
	}
}

func TestReposAnalyzeSkipsMissingRoots(t *testing.T) {
	saved := core.RepoRoots
	defer func() { core.RepoRoots = saved }()
	core.RepoRoots = []string{"/definitely/does/not/exist"}

	result, err := (&ReposAnalyzer{}).Analyze()
	if err != nil {
		t.Fatalf("Analyze error: %v", err)
	}
	if len(result.Items) != 0 || result.TotalSize != 0 {
		t.Errorf("expected empty result for missing root, got %d items / %d bytes",
			len(result.Items), result.TotalSize)
	}
}

package analyzer

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"runtime"
	"sort"
	"sync"

	"os"

	"github.com/arheanja-ops/mac-toolkit/internal/core"
)

type ReposAnalyzer struct{}

func init() { Register(&ReposAnalyzer{}) }

func (r *ReposAnalyzer) Domain() string       { return "repos" }
func (r *ReposAnalyzer) Risk() core.RiskLevel { return core.RiskSafe }

var repoPruneTargets = map[string]bool{
	"node_modules":  true,
	".venv":         true,
	"__pycache__":   true,
	".pytest_cache": true,
}

// repoSkipDirs are directories that never contain prunable targets we care
// about and are expensive to traverse. Skipping them keeps the walk bounded.
var repoSkipDirs = map[string]bool{
	".git":   true,
	".hg":    true,
	".svn":   true,
	".Trash": true,
}

// Analyze scans the configured repo roots for prunable build-artifact
// directories (node_modules, .venv, ...). It runs in two phases so a large
// number of targets does not saturate the analyzer timeout:
//  1. A cheap tree walk collects target paths without measuring their size
//     and never descends into a target (so nested targets are never
//     double-counted).
//  2. Target sizes are measured concurrently with a bounded worker pool,
//     because measuring 1000+ targets sequentially is what previously
//     exceeded the global timeout.
func (r *ReposAnalyzer) Analyze() (core.AnalysisResult, error) {
	targets := collectRepoTargets(core.RepoRoots)
	items := measureRepoTargets(targets)

	// Deterministic ordering keeps output stable regardless of the order in
	// which concurrent workers finish.
	sort.Slice(items, func(i, j int) bool { return items[i].Path < items[j].Path })

	var totalSize int64
	for _, it := range items {
		totalSize += it.SizeBytes
	}

	summary := fmt.Sprintf("Repos: %s in %d directories", core.FormatBytes(totalSize), len(items))
	return MakeResult("repos", items, totalSize, summary), nil
}

// collectRepoTargets walks each root once and returns the paths of every
// prunable target directory. It is intentionally cheap: it records the path
// and immediately skips descending into the target, so a node_modules nested
// inside another node_modules is never visited twice.
func collectRepoTargets(roots []string) []string {
	var targets []string
	for _, root := range roots {
		if _, err := os.Stat(root); os.IsNotExist(err) {
			continue
		}
		filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil || !d.IsDir() {
				return nil
			}
			if repoSkipDirs[d.Name()] {
				return fs.SkipDir
			}
			if !repoPruneTargets[d.Name()] {
				return nil
			}
			targets = append(targets, path)
			return fs.SkipDir // don't descend into the target
		})
	}
	return targets
}

// measureRepoTargets computes each target's size concurrently using a worker
// pool bounded by the CPU count, then builds the cleanable items. Zero-sized
// targets are dropped, matching the previous sequential behavior.
func measureRepoTargets(targets []string) []core.CleanableItem {
	sizes := make([]int64, len(targets))

	workers := runtime.NumCPU()
	if workers < 1 {
		workers = 1
	}

	var wg sync.WaitGroup
	jobs := make(chan int)

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				sizes[i] = DirSize(targets[i])
			}
		}()
	}
	for i := range targets {
		jobs <- i
	}
	close(jobs)
	wg.Wait()

	items := make([]core.CleanableItem, 0, len(targets))
	for i, path := range targets {
		if sizes[i] == 0 {
			continue
		}
		items = append(items, core.CleanableItem{
			Path:         path,
			SizeBytes:    sizes[i],
			Label:        fmt.Sprintf("%s in %s", filepath.Base(path), filepath.Dir(path)),
			Domain:       "repos",
			SafeToDelete: true,
			Reason:       "Build artifact — auto-regenerated",
			Risk:         core.RiskSafe,
		})
	}
	return items
}

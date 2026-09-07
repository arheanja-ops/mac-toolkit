package analyzer

import (
	"fmt"
	"io/fs"
	"github.com/arheanja-ops/mac-toolkit/internal/core"
	"os"
	"path/filepath"
)

type ReposAnalyzer struct{}

func init() { Register(&ReposAnalyzer{}) }

func (r *ReposAnalyzer) Domain() string      { return "repos" }
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
	".git":  true,
	".hg":   true,
	".svn":  true,
	".Trash": true,
}

func (r *ReposAnalyzer) Analyze() (core.AnalysisResult, error) {
	var items []core.CleanableItem
	var totalSize int64

	for _, root := range core.RepoRoots {
		if _, err := os.Stat(root); os.IsNotExist(err) {
			continue
		}
		filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if !d.IsDir() {
				return nil
			}
			if repoSkipDirs[d.Name()] {
				return fs.SkipDir
			}
			if !repoPruneTargets[d.Name()] {
				return nil
			}
			// Found a target directory
			size := DirSize(path)
			if size == 0 {
				return fs.SkipDir
			}
			totalSize += size
			items = append(items, core.CleanableItem{
				Path:         path,
				SizeBytes:    size,
				Label:        fmt.Sprintf("%s in %s", d.Name(), filepath.Dir(path)),
				Domain:       "repos",
				SafeToDelete: true,
				Reason:       "Build artifact — auto-regenerated",
				Risk:         core.RiskSafe,
			})
			return fs.SkipDir // don't descend into the target
		})
	}

	summary := fmt.Sprintf("Repos: %s in %d directories", core.FormatBytes(totalSize), len(items))
	return MakeResult("repos", items, totalSize, summary), nil
}

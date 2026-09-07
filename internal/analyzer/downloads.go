package analyzer

import (
	"fmt"
	"io/fs"
	"github.com/arheanja-ops/mac-toolkit/internal/core"
	"os"
	"path/filepath"
	"strings"
)

type DownloadsAnalyzer struct{}

func init() { Register(&DownloadsAnalyzer{}) }

func (d *DownloadsAnalyzer) Domain() string      { return "downloads" }
func (d *DownloadsAnalyzer) Risk() core.RiskLevel { return core.RiskWarn }

var archiveExts = map[string]bool{
	".zip": true, ".tar": true, ".gz": true, ".tgz": true,
	".dmg": true, ".pkg": true, ".rar": true, ".7z": true,
}

var duplicatePatterns = []string{" (1)", " (2)", " (3)", " copy", "-1", "-2"}

func (d *DownloadsAnalyzer) Analyze() (core.AnalysisResult, error) {
	var items []core.CleanableItem
	var totalSize int64
	minBytes := int64(core.DefaultMinSizeMB) * 1024 * 1024

	for _, dir := range core.DownloadDirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			continue
		}
		base := dir
		filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				// Only scan the top two directory levels of Downloads/Desktop;
				// deep app bundles and project trees would blow the timeout.
				rel, _ := filepath.Rel(base, path)
				if rel != "." && strings.Count(rel, string(filepath.Separator)) >= 2 {
					return fs.SkipDir
				}
				return nil
			}
			info, err := d.Info()
			if err != nil {
				return nil
			}
			size := info.Size()
			name := d.Name()
			ext := strings.ToLower(filepath.Ext(name))

			// Check duplicate patterns
			isDuplicate := false
			for _, pat := range duplicatePatterns {
				if strings.Contains(name, pat) {
					isDuplicate = true
					break
				}
			}

			if isDuplicate {
				totalSize += size
				items = append(items, core.CleanableItem{
					Path:         path,
					SizeBytes:    size,
					Label:        fmt.Sprintf("Duplicate: %s", name),
					Domain:       "downloads",
					SafeToDelete: true,
					Reason:       "Probable duplicate file",
					Risk:         core.RiskSafe,
				})
				return nil
			}

			if archiveExts[ext] {
				totalSize += size
				items = append(items, core.CleanableItem{
					Path:         path,
					SizeBytes:    size,
					Label:        fmt.Sprintf("Archive: %s", name),
					Domain:       "downloads",
					SafeToDelete: false,
					Reason:       "Archive file — verify before deleting",
					Risk:         core.RiskWarn,
				})
				return nil
			}

			if size >= minBytes {
				totalSize += size
				items = append(items, core.CleanableItem{
					Path:         path,
					SizeBytes:    size,
					Label:        fmt.Sprintf("Large file: %s", name),
					Domain:       "downloads",
					SafeToDelete: false,
					Reason:       fmt.Sprintf("Large file (>%dMB)", core.DefaultMinSizeMB),
					Risk:         core.RiskWarn,
				})
			}
			return nil
		})
	}

	summary := fmt.Sprintf("Downloads: %s in %d items", core.FormatBytes(totalSize), len(items))
	return MakeResult("downloads", items, totalSize, summary), nil
}

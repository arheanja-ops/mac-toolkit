package analyzer

import (
	"fmt"
	"io/fs"
	"github.com/arheanja-ops/mac-toolkit/internal/core"
	"os"
	"path/filepath"
)

type OllamaAnalyzer struct{}

func init() { Register(&OllamaAnalyzer{}) }

func (o *OllamaAnalyzer) Domain() string      { return "ollama" }
func (o *OllamaAnalyzer) Risk() core.RiskLevel { return core.RiskDanger }

func (o *OllamaAnalyzer) Analyze() (core.AnalysisResult, error) {
	blobsDir := filepath.Join(core.OllamaModelsDir, "blobs")
	if _, err := os.Stat(blobsDir); os.IsNotExist(err) {
		return MakeResult("ollama", nil, 0, "Ollama not installed or no models"), nil
	}

	var items []core.CleanableItem
	var totalSize int64

	filepath.WalkDir(blobsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		size := info.Size()
		totalSize += size
		items = append(items, core.CleanableItem{
			Path:         path,
			SizeBytes:    size,
			Label:        fmt.Sprintf("Model blob: %s", d.Name()),
			Domain:       "ollama",
			SafeToDelete: false,
			Reason:       "LLM model — re-download required",
			Risk:         core.RiskDanger,
		})
		return nil
	})

	summary := fmt.Sprintf("Ollama models: %s in %d blobs", core.FormatBytes(totalSize), len(items))
	return MakeResult("ollama", items, totalSize, summary), nil
}

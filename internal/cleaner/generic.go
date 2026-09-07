package cleaner

import (
	"github.com/arheanja-ops/mac-toolkit/internal/core"
)

// GenericCleaner iterates items and deletes them one by one
type GenericCleaner struct {
	DryRun  bool
	Execute bool
}

// NewGenericCleaner creates a new cleaner
func NewGenericCleaner(dryRun, execute bool) *GenericCleaner {
	return &GenericCleaner{
		DryRun:  dryRun || !execute,
		Execute: execute && !dryRun,
	}
}

// Clean processes all items and returns deletion results
func (g *GenericCleaner) Clean(items []core.CleanableItem) []core.DeleteResult {
	results := make([]core.DeleteResult, 0, len(items))

	for _, item := range items {
		result := Delete(item.Path, g.DryRun)
		results = append(results, result)
	}

	return results
}

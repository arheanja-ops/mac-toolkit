package cleaner

import (
	"github.com/arheanja-ops/mac-toolkit/internal/core"
	"os"
	"path/filepath"
	"strings"
)

// Cleaner is the interface for cleanup implementations
type Cleaner interface {
	Clean(items []core.CleanableItem) []core.DeleteResult
}

// IsBlacklisted checks if a path is in the system blacklist.
// Returns true if the path should NOT be deleted.
func IsBlacklisted(path string) bool {
	// Check blacklisted app bundle IDs
	base := filepath.Base(path)
	if core.BlacklistedPaths[base] {
		return true
	}

	// Check blacklisted path prefixes
	cleanPath := filepath.Clean(path)
	for _, prefix := range core.BlacklistedPrefixes {
		if strings.HasPrefix(cleanPath, prefix) {
			return true
		}
	}

	return false
}

// Delete performs the actual file/directory deletion.
// Returns a DeleteResult recording what happened.
func Delete(path string, dryRun bool) core.DeleteResult {
	// Check blacklist first
	if IsBlacklisted(path) {
		core.Warn("Blocked by blacklist: %s", path)
		return core.DeleteResult{
			Path:   path,
			Result: "skipped-blacklisted",
		}
	}

	// Dry-run mode
	if dryRun {
		return core.DeleteResult{
			Path:   path,
			Result: "skipped-dry-run",
		}
	}

	// Get size before deletion
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return core.DeleteResult{
			Path:   path,
			Result: "skipped-not-found",
		}
	}

	var size int64
	if info.IsDir() {
		// Calculate dir size before removal
		filepath.Walk(path, func(_ string, fi os.FileInfo, err error) error {
			if err == nil && !fi.IsDir() {
				size += fi.Size()
			}
			return nil
		})
		err = os.RemoveAll(path)
	} else {
		size = info.Size()
		err = os.Remove(path)
	}

	if err != nil {
		core.Error("Failed to delete %s: %v", path, err)
		return core.DeleteResult{
			Path:   path,
			Result: "failure",
			Error:  err.Error(),
		}
	}

	core.Success("Deleted: %s (%s)", path, core.FormatBytes(size))
	return core.DeleteResult{
		Path:      path,
		Result:    "success",
		SizeBytes: size,
	}
}

package core

import (
	"os"
	"path/filepath"
)

// AnalyzerTimeoutSeconds bounds each analyzer. Repo scans over large source
// trees are the slowest domain, so this leaves headroom above their typical
// runtime while still guarding against a runaway walk.
const AnalyzerTimeoutSeconds = 180

// DefaultMinSizeMB is the minimum file size (in MB) to flag in downloads
const DefaultMinSizeMB = 50

var (
	// HomeDir is the current user's home directory
	HomeDir string

	// RepoRoots are directories to scan for node_modules, .venv, __pycache__
	RepoRoots []string

	// BrowserCachePaths maps browser name to its cache directory
	BrowserCachePaths map[string]string

	// LogDirs are directories containing log files
	LogDirs []string

	// DownloadDirs are directories to scan for large/old downloads
	DownloadDirs []string

	// AppSupportDir is ~/Library/Application Support
	AppSupportDir string

	// OllamaModelsDir is where Ollama stores downloaded LLM models
	OllamaModelsDir string

	// DevCachePaths maps tool name to its cache directory
	DevCachePaths map[string]string

	// XcodeDerivedData is the Xcode DerivedData path
	XcodeDerivedData string

	// XcodeArchives is the Xcode Archives path
	XcodeArchives string

	// XcodeSimulators is the CoreSimulator Devices path
	XcodeSimulators string

	// DockerRawPath is the Docker Desktop virtual disk image
	DockerRawPath string

	// TrashDir is the user's Trash directory
	TrashDir string

	// BlacklistedPaths are app bundle IDs that must never be deleted
	BlacklistedPaths map[string]bool

	// BlacklistedPrefixes are path prefixes under which nothing is deleted
	BlacklistedPrefixes []string
)

func init() {
	HomeDir, _ = os.UserHomeDir()

	RepoRoots = []string{
		h("arqueanja"),
		h("arheanja"),
	}

	BrowserCachePaths = map[string]string{
		"Chrome":  h("Library", "Caches", "Google", "Chrome"),
		"Safari":  h("Library", "Caches", "com.apple.Safari"),
		"Firefox": h("Library", "Caches", "Firefox"),
		"Edge":    h("Library", "Caches", "Microsoft Edge"),
	}

	LogDirs = []string{
		h("Library", "Logs"),
		"/Library/Logs",
		"/var/log",
	}

	DownloadDirs = []string{
		h("Downloads"),
		h("Desktop"),
	}

	AppSupportDir = h("Library", "Application Support")

	OllamaModelsDir = h(".ollama", "models")

	DevCachePaths = map[string]string{
		"npm":  h(".npm", "_cacache"),
		"pip":  h("Library", "Caches", "pip"),
		"brew": h("Library", "Caches", "Homebrew"),
	}

	XcodeDerivedData = h("Library", "Developer", "Xcode", "DerivedData")
	XcodeArchives = h("Library", "Developer", "Xcode", "Archives")
	XcodeSimulators = h("Library", "Developer", "CoreSimulator", "Devices")

	DockerRawPath = h("Library", "Containers", "com.docker.docker", "Data", "vms", "0", "data", "Docker.raw")

	TrashDir = h(".Trash")

	BlacklistedPaths = map[string]bool{
		"com.apple.dock":      true,
		"com.apple.finder":    true,
		"com.apple.spotlight": true,
	}

	BlacklistedPrefixes = []string{
		"/System",
		"/usr",
		"/bin",
		"/sbin",
		"/private/var/db",
	}
}

// h joins path parts relative to HomeDir.
func h(parts ...string) string {
	return filepath.Join(append([]string{HomeDir}, parts...)...)
}

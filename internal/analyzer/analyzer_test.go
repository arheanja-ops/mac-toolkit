package analyzer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/arheanja-ops/mac-toolkit/internal/core"
)

func TestDirSize(t *testing.T) {
	dir := t.TempDir()

	// Create files with known sizes
	os.WriteFile(filepath.Join(dir, "a.txt"), make([]byte, 1000), 0o644)
	os.WriteFile(filepath.Join(dir, "b.txt"), make([]byte, 2000), 0o644)

	sub := filepath.Join(dir, "sub")
	os.MkdirAll(sub, 0o755)
	os.WriteFile(filepath.Join(sub, "c.txt"), make([]byte, 500), 0o644)

	size := DirSize(dir)
	if size != 3500 {
		t.Errorf("DirSize = %d, want 3500", size)
	}
}

func TestDirSizeEmpty(t *testing.T) {
	dir := t.TempDir()
	size := DirSize(dir)
	if size != 0 {
		t.Errorf("DirSize empty = %d, want 0", size)
	}
}

func TestDirSizeNonExistent(t *testing.T) {
	size := DirSize("/nonexistent/path")
	if size != 0 {
		t.Errorf("DirSize nonexistent = %d, want 0", size)
	}
}

func TestMakeResult(t *testing.T) {
	items := []core.CleanableItem{
		{Path: "/tmp/a", SizeBytes: 1024, Domain: "test"},
	}
	result := MakeResult("test", items, 1024, "test summary")

	if result.Domain != "test" {
		t.Errorf("Domain = %s, want test", result.Domain)
	}
	if result.Severity != core.SeverityLow {
		t.Errorf("Severity = %s, want low", result.Severity)
	}
	if result.TotalSize != 1024 {
		t.Errorf("TotalSize = %d, want 1024", result.TotalSize)
	}
	if len(result.Items) != 1 {
		t.Errorf("Items count = %d, want 1", len(result.Items))
	}
}

func TestRegistryAll(t *testing.T) {
	all := All()
	if len(all) == 0 {
		t.Fatal("registry is empty, expected at least 1 analyzer")
	}

	// Verify we have the expected domains
	expected := map[string]bool{
		"disk": false, "ollama": false, "docker": false, "browser": false,
		"logs": false, "downloads": false, "appsupport": false, "repos": false,
		"dev_caches": false, "xcode": false, "trash": false,
	}

	for _, a := range all {
		if _, ok := expected[a.Domain()]; ok {
			expected[a.Domain()] = true
		}
	}

	for domain, found := range expected {
		if !found {
			t.Errorf("missing analyzer for domain: %s", domain)
		}
	}
}

func TestRegistryByDomain(t *testing.T) {
	a := ByDomain("disk")
	if a == nil {
		t.Fatal("ByDomain('disk') returned nil")
	}
	if a.Domain() != "disk" {
		t.Errorf("Domain() = %s, want disk", a.Domain())
	}

	none := ByDomain("nonexistent")
	if none != nil {
		t.Error("ByDomain('nonexistent') should return nil")
	}
}

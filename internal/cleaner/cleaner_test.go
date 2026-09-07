package cleaner

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsBlacklisted(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"/System/Library/something", true},
		{"/usr/local/bin", true},
		{"/bin/bash", true},
		{"/sbin/mount", true},
		{"/private/var/db/something", true},
		{"/Users/test/Library/Caches/Chrome", false},
		{"/tmp/safe-file", false},
		{filepath.Join(os.TempDir(), "test"), false},
		{"/some/path/com.apple.dock", true},
		{"/some/path/com.apple.finder", true},
		{"/some/path/com.apple.spotlight", true},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := IsBlacklisted(tt.path)
			if got != tt.want {
				t.Errorf("IsBlacklisted(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestDeleteDryRun(t *testing.T) {
	// Create a temp file
	tmpFile := filepath.Join(t.TempDir(), "test.txt")
	if err := os.WriteFile(tmpFile, []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}

	result := Delete(tmpFile, true) // dry-run
	if result.Result != "skipped-dry-run" {
		t.Errorf("expected skipped-dry-run, got %s", result.Result)
	}

	// File should still exist
	if _, err := os.Stat(tmpFile); os.IsNotExist(err) {
		t.Error("file should still exist in dry-run")
	}
}

func TestDeleteNotFound(t *testing.T) {
	result := Delete("/nonexistent/path/file.txt", false)
	if result.Result != "skipped-not-found" {
		t.Errorf("expected skipped-not-found, got %s", result.Result)
	}
}

func TestDeleteSuccess(t *testing.T) {
	// Create a temp file
	tmpFile := filepath.Join(t.TempDir(), "deleteme.txt")
	data := []byte("hello world")
	if err := os.WriteFile(tmpFile, data, 0o644); err != nil {
		t.Fatal(err)
	}

	result := Delete(tmpFile, false)
	if result.Result != "success" {
		t.Errorf("expected success, got %s (error: %s)", result.Result, result.Error)
	}
	if result.SizeBytes != int64(len(data)) {
		t.Errorf("expected size %d, got %d", len(data), result.SizeBytes)
	}

	// File should be gone
	if _, err := os.Stat(tmpFile); !os.IsNotExist(err) {
		t.Error("file should have been deleted")
	}
}

func TestDeleteBlacklisted(t *testing.T) {
	result := Delete("/System/Library/test", false)
	if result.Result != "skipped-blacklisted" {
		t.Errorf("expected skipped-blacklisted, got %s", result.Result)
	}
}

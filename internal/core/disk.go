package core

import (
	"os/exec"
	"strconv"
	"strings"
)

// DiskSnapshot is a lightweight view of the data volume usage, used by the
// banner and any caller that needs a quick free-space reading without running
// a full analysis.
type DiskSnapshot struct {
	TotalBytes int64
	UsedBytes  int64
	AvailBytes int64
	PctUsed    int
	OK         bool
}

// ReadDiskSnapshot returns current usage of the macOS data volume via `df`.
// On any parse/exec error it returns a snapshot with OK=false so callers can
// degrade gracefully instead of failing.
func ReadDiskSnapshot() DiskSnapshot {
	out, err := exec.Command("df", "-k", "/System/Volumes/Data").Output()
	if err != nil {
		return DiskSnapshot{}
	}

	lines := strings.Split(string(out), "\n")
	if len(lines) < 2 {
		return DiskSnapshot{}
	}
	fields := strings.Fields(lines[1])
	if len(fields) < 5 {
		return DiskSnapshot{}
	}

	totalKB, _ := strconv.ParseInt(fields[1], 10, 64)
	usedKB, _ := strconv.ParseInt(fields[2], 10, 64)
	availKB, _ := strconv.ParseInt(fields[3], 10, 64)

	total := totalKB * 1024
	used := usedKB * 1024
	avail := availKB * 1024

	pct := 0
	if total > 0 {
		pct = int(used * 100 / total)
	}

	return DiskSnapshot{
		TotalBytes: total,
		UsedBytes:  used,
		AvailBytes: avail,
		PctUsed:    pct,
		OK:         true,
	}
}

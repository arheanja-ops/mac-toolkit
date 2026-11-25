package monitor

import (
	"strconv"
	"strings"
)

// readCPUSplit returns the user/sys/idle split of the last second.
//
// Why iostat: `top -l 1` reports the average since boot (it understates a busy
// machine) and costs about 1.3 s of CPU per call; `top -l 2` is correct but
// twice as slow. `iostat -c 2 -w 1` prints a since-boot line and then one line
// for the 1-second interval, takes 1 s of wall time and ~0 CPU. `ps -o %cpu`
// is a decayed per-process average, not the current machine load.
func readCPUSplit() (user, sys, idle float64, ok bool) {
	out, err := runCmd("iostat", "-c", "2", "-w", "1")
	if err != nil {
		return 0, 0, 0, false
	}
	return parseIostatCPU(string(out))
}

// parseIostatCPU reads the last data line of `iostat -c 2 -w 1`. The disk
// columns before "us sy id" vary with the number of disks, so the columns are
// located from the END of the line using their distance from the end in the
// header ("us sy id 1m 5m 15m" → 6 from the end). A line that does not parse
// (header only, garbage) yields ok=false instead of a made-up number.
func parseIostatCPU(raw string) (user, sys, idle float64, ok bool) {
	lines := strings.Split(strings.TrimSpace(raw), "\n")
	fromEnd := 0
	for _, ln := range lines {
		f := strings.Fields(ln)
		for i, tok := range f {
			if tok == "us" {
				fromEnd = len(f) - i
			}
		}
	}
	if fromEnd < 3 || len(lines) < 2 {
		return 0, 0, 0, false
	}
	f := strings.Fields(lines[len(lines)-1])
	if len(f) < fromEnd {
		return 0, 0, 0, false
	}
	cols := f[len(f)-fromEnd : len(f)-fromEnd+3]
	vals := make([]float64, 3)
	for i, c := range cols {
		v, err := strconv.ParseFloat(c, 64)
		if err != nil {
			return 0, 0, 0, false
		}
		vals[i] = v
	}
	return vals[0], vals[1], vals[2], true
}

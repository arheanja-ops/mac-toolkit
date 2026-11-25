package monitor

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// topAppsN is how many applications the memory and CPU snapshots keep; the
// Overview views show the first 5 and the "Top apps" views all of them.
const topAppsN = 10

// AppUsage is one application's aggregated footprint. It deliberately has no
// PID and no path: only a display name and magnitudes, so it is safe to show
// and to log.
type AppUsage struct {
	Name   string  `json:"name"`
	MemMB  float64 `json:"mem_mb"`
	CPUPct float64 `json:"cpu_pct"`
	Procs  int     `json:"procs"`
}

// reProcEntry splits "pid state rss pcpu pmem command"; the command keeps its
// inner spaces because executable paths may contain them.
var reProcEntry = regexp.MustCompile(
	`^\s*(\d+)\s+(\S+)\s+(\d+)\s+(\d+(?:\.\d+)?)\s+(\d+(?:\.\d+)?)\s+(\S.*?)\s*$`)

// procEntry is one parsed `ps` line. The PID and state exist only for the
// local Processes views; the app aggregation and every snapshot that can leave
// the machine work on procRow, which has neither.
type procEntry struct {
	pid   int
	state string
	mem   float64
	procRow
}

// procRow is the PID-free subset used for per-app aggregation.
type procRow struct {
	rssKB int64
	cpu   float64
	comm  string
}

// readProcTable lists every process with ONE ps call, so the Processes, Memory
// and CPU views share the same command (and the same test fake).
func readProcTable() ([]procEntry, error) {
	out, err := runCmd("ps", "-axwwo", "pid=,state=,rss=,pcpu=,pmem=,comm=")
	if err != nil {
		return nil, err
	}
	return parseProcTable(string(out)), nil
}

// readProcRows is readProcTable without PID/state, for the app aggregation.
func readProcRows() ([]procRow, error) {
	entries, err := readProcTable()
	if err != nil {
		return nil, err
	}
	return rowsOf(entries), nil
}

// rowsOf drops the PID and state of every entry.
func rowsOf(entries []procEntry) []procRow {
	rows := make([]procRow, len(entries))
	for i, e := range entries {
		rows[i] = e.procRow
	}
	return rows
}

// parseProcTable parses `ps -o pid=,state=,rss=,pcpu=,pmem=,comm=`. Malformed
// lines are skipped rather than failing the whole snapshot.
func parseProcTable(raw string) []procEntry {
	var out []procEntry
	for _, line := range strings.Split(raw, "\n") {
		m := reProcEntry.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		pid, _ := strconv.Atoi(m[1])
		rss, _ := strconv.ParseInt(m[3], 10, 64)
		cpu, _ := strconv.ParseFloat(m[4], 64)
		mem, _ := strconv.ParseFloat(m[5], 64)
		out = append(out, procEntry{pid: pid, state: m[2], mem: mem,
			procRow: procRow{rssKB: rss, cpu: cpu, comm: m[6]}})
	}
	return out
}

// appName derives the display name of a process. A path inside `/X.app/` maps
// to `X` (the outermost bundle, so helper processes fold into their parent
// app); anything else falls back to the executable's basename.
func appName(comm string) string {
	if i := strings.Index(comm, ".app/"); i >= 0 {
		start := strings.LastIndex(comm[:i], "/") + 1
		if name := comm[start:i]; name != "" {
			return name
		}
	}
	if i := strings.LastIndex(comm, "/"); i >= 0 {
		comm = comm[i+1:]
	}
	if comm == "" {
		return "?"
	}
	return comm
}

// aggregateApps folds processes into per-application totals. RSS is summed per
// app, which can over-count pages shared between its processes; it is meant as
// a ranking aid, not an accounting figure.
func aggregateApps(rows []procRow) []AppUsage {
	byName := map[string]*AppUsage{}
	for _, r := range rows {
		n := appName(r.comm)
		a := byName[n]
		if a == nil {
			a = &AppUsage{Name: n}
			byName[n] = a
		}
		a.MemMB += float64(r.rssKB) / 1024
		a.CPUPct += r.cpu
		a.Procs++
	}
	out := make([]AppUsage, 0, len(byName))
	for _, a := range byName {
		out = append(out, *a)
	}
	return out
}

// topApps returns the n largest apps by the given key, ties broken by name so
// the output is deterministic.
func topApps(apps []AppUsage, n int, byMem bool) []AppUsage {
	sorted := append([]AppUsage(nil), apps...)
	sort.Slice(sorted, func(i, j int) bool {
		a, b := sorted[i], sorted[j]
		ka, kb := a.CPUPct, b.CPUPct
		if byMem {
			ka, kb = a.MemMB, b.MemMB
		}
		if ka != kb {
			return ka > kb
		}
		return a.Name < b.Name
	})
	if len(sorted) > n {
		sorted = sorted[:n]
	}
	return sorted
}

package monitor

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// topAppsN is how many applications the memory and CPU views list.
const topAppsN = 5

// AppUsage is one application's aggregated footprint. It deliberately has no
// PID and no path: only a display name and magnitudes, so it is safe to show
// and to log.
type AppUsage struct {
	Name   string  `json:"name"`
	MemMB  float64 `json:"mem_mb"`
	CPUPct float64 `json:"cpu_pct"`
	Procs  int     `json:"procs"`
}

// reProcRow splits "rss pcpu command"; the command keeps inner spaces.
var reProcRow = regexp.MustCompile(`^\s*(\d+)\s+(\d+(?:\.\d+)?)\s+(\S.*?)\s*$`)

// procRow is one parsed `ps` line.
type procRow struct {
	rssKB int64
	cpu   float64
	comm  string
}

// readProcRows lists every process as rss/%cpu/command. Fields use empty
// headers (`=`) so there is nothing to skip, and no PID is requested at all.
func readProcRows() ([]procRow, error) {
	out, err := runCmd("ps", "-axwwo", "rss=,pcpu=,comm=")
	if err != nil {
		return nil, err
	}
	return parseProcRows(string(out)), nil
}

// parseProcRows parses `ps -o rss=,pcpu=,comm=` output. The command is the
// remainder of the line because executable paths may contain spaces. Malformed
// lines are skipped rather than failing the whole snapshot.
func parseProcRows(raw string) []procRow {
	var rows []procRow
	for _, line := range strings.Split(raw, "\n") {
		m := reProcRow.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		rss, _ := strconv.ParseInt(m[1], 10, 64)
		cpu, _ := strconv.ParseFloat(m[2], 64)
		rows = append(rows, procRow{rssKB: rss, cpu: cpu, comm: m[3]})
	}
	return rows
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

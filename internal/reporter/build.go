package reporter

import (
	"sort"
	"time"

	"github.com/arheanja-ops/mac-toolkit/internal/ai"
	"github.com/arheanja-ops/mac-toolkit/internal/core"
	"github.com/arheanja-ops/mac-toolkit/internal/monitor"
)

// Limits of the report, named so the spec and the code agree.
const (
	topItemsPerDomain = 10 // items listed per domain before "… y N más"
	topAppsN          = 5  // apps per process ranking
	topFindingsN      = 3  // findings in the executive summary
	diskDomain        = "disk"
)

// ToolVersion is the toolkit version written to every report. cmd sets it at
// start-up (the reporter cannot import cmd).
var ToolVersion = "dev"

// Inputs is everything Build needs. Collecting it is separate (see Collect) so
// the model can be built from synthetic data in tests.
type Inputs struct {
	Now       time.Time
	Home      string // home folder to replace by "~" in paths; empty = none
	Results   []core.AnalysisResult
	Metrics   ai.Metrics
	Disk      core.DiskSnapshot
	Diagnosis ai.Diagnosis
	Engine    string // human label of the diagnosis engine
}

// Build assembles the report. It is the only place where figures are derived.
func Build(in Inputs) Report {
	r := Report{
		SchemaVersion: SchemaVersion,
		Variant:       VariantFull,
		GeneratedAt:   in.Now,
		ToolVersion:   ToolVersion,
		Host:          buildHost(in.Metrics),
		System:        buildSystem(in.Metrics),
		Processes:     buildProcesses(in.Metrics),
		Recommendations: Recommendations{
			Engine:      in.Engine,
			Performance: findings(in.Diagnosis.Performance),
			QuickWins:   findings(in.Diagnosis.QuickWins),
			Review:      findings(in.Diagnosis.Review),
		},
	}
	r.Disk, r.Details = buildDisk(in)
	r.Summary = buildSummary(r.Recommendations)
	return r
}

// buildDisk splits the analysis into the volume (context), the reclaimable
// total (safe + review), the sorted non-empty domains and the failed ones.
func buildDisk(in Inputs) (Disk, []Detail) {
	d := Disk{Volume: volumeOf(in)}
	var rows []core.AnalysisResult
	for _, res := range in.Results {
		switch {
		case res.Domain == diskDomain:
			// Volume usage is context; summing it as "reclaimable" was the bug
			// that made the old report claim hundreds of GB of junk.
			continue
		case res.Error != "":
			d.Failed = append(d.Failed, Failure{Domain: res.Domain, Error: res.Error})
		case res.TotalSize > 0:
			rows = append(rows, res)
		}
	}
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].TotalSize > rows[j].TotalSize })
	for _, res := range rows {
		d.Reclaimable.TotalBytes += res.TotalSize
		d.Reclaimable.SafeBytes += safeBytes(res)
	}
	// Safe never exceeds the total: a domain whose items sum above its own size
	// must not make the review share negative.
	d.Reclaimable.SafeBytes = min(d.Reclaimable.SafeBytes, d.Reclaimable.TotalBytes)
	d.Reclaimable.ReviewBytes = d.Reclaimable.TotalBytes - d.Reclaimable.SafeBytes

	var details []Detail
	for _, res := range rows {
		d.Domains = append(d.Domains, domainRow(res, d.Reclaimable.TotalBytes))
		if len(res.Items) > 0 {
			details = append(details, buildDetail(res, in.Home))
		}
	}
	return d, details
}

// safeBytes sums the items marked safe to delete, capped at the domain size.
func safeBytes(res core.AnalysisResult) int64 {
	var n int64
	for _, it := range res.Items {
		if it.SafeToDelete {
			n += it.SizeBytes
		}
	}
	return min(n, res.TotalSize)
}

func domainRow(res core.AnalysisResult, total int64) DomainRow {
	row := DomainRow{
		Domain: res.Domain, Severity: string(res.Severity), SizeBytes: res.TotalSize,
		Items: len(res.Items), Summary: res.Summary,
	}
	if total > 0 {
		row.SharePct = float64(res.TotalSize) * 100 / float64(total)
	}
	return row
}

// volumeOf reads the volume from the df snapshot; when df failed, the disk
// analyzer's used size is the only thing known.
func volumeOf(in Inputs) Volume {
	if in.Disk.OK {
		return Volume{TotalBytes: in.Disk.TotalBytes, UsedBytes: in.Disk.UsedBytes,
			FreeBytes: in.Disk.AvailBytes, UsedPct: in.Disk.PctUsed, Known: true}
	}
	for _, res := range in.Results {
		if res.Domain == diskDomain && res.Error == "" && res.TotalSize > 0 {
			return Volume{UsedBytes: res.TotalSize}
		}
	}
	return Volume{}
}

// buildDetail keeps the top items by size, with display-ready labels and paths,
// and folds the rest into a count and a byte total.
func buildDetail(res core.AnalysisResult, home string) Detail {
	items := append([]core.CleanableItem(nil), res.Items...)
	sort.SliceStable(items, func(i, j int) bool { return items[i].SizeBytes > items[j].SizeBytes })
	det := Detail{Domain: res.Domain, SizeBytes: res.TotalSize}
	for i, it := range items {
		if i >= topItemsPerDomain {
			det.More++
			det.MoreBytes += it.SizeBytes
			continue
		}
		det.Items = append(det.Items, Item{
			Label:     ShortLabel(it.Label, home),
			Path:      TildePath(it.Path, home),
			SizeBytes: it.SizeBytes, Risk: string(it.Risk), Safe: it.SafeToDelete,
		})
	}
	return det
}

func findings(in []ai.Insight) []Finding {
	out := make([]Finding, 0, len(in))
	for _, i := range in {
		out = append(out, Finding{Title: i.Title, Detail: i.Detail, Action: i.Action, Severity: i.Severity})
	}
	return out
}

// buildProcesses ranks the apps the monitors already aggregated (no PIDs).
func buildProcesses(m ai.Metrics) Processes {
	return Processes{
		TopMemory: appsOf(m.Memory["top_memory"]),
		TopCPU:    appsOf(m.CPU["top_cpu"]),
	}
}

func appsOf(v any) []App {
	list, _ := v.([]monitor.AppUsage)
	out := make([]App, 0, topAppsN)
	for i, a := range list {
		if i >= topAppsN {
			break
		}
		out = append(out, App{Name: a.Name, MemMB: a.MemMB, CPUPct: a.CPUPct, Procs: a.Procs})
	}
	return out
}

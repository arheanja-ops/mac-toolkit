package reporter

import (
	"fmt"
	"regexp"
	"strings"
)

// reLongHex catches digests that survive in free text.
var reLongHex = regexp.MustCompile(`[0-9a-fA-F]{32,}`)

// Share returns the redacted copy that is safe to hand to someone else. It
// keeps figures, domains, the executive summary, system readings and the
// recommendations; it drops everything that can identify what is on the disk
// or what runs on the machine:
//   - the per-domain detail (item labels, file names and paths),
//   - per-domain summaries and error messages (they can quote paths),
//   - app names (replaced by "App 1", "App 2"… keeping the figures),
//   - any word of a recommendation that looks like a path, a "~" or a digest,
//     and any known app name quoted in it.
func (r Report) Share() Report {
	s := r
	s.Variant = VariantShare
	s.Details = nil

	s.Disk.Domains = append([]DomainRow(nil), r.Disk.Domains...)
	for i := range s.Disk.Domains {
		s.Disk.Domains[i].Summary = ""
	}
	s.Disk.Failed = append([]Failure(nil), r.Disk.Failed...)
	for i := range s.Disk.Failed {
		s.Disk.Failed[i].Error = "el análisis falló"
	}

	names := appNames(r.Processes)
	s.Processes = Processes{
		TopMemory: anonymize(r.Processes.TopMemory),
		TopCPU:    anonymize(r.Processes.TopCPU),
	}
	s.Recommendations = Recommendations{
		Engine:      r.Recommendations.Engine,
		Performance: scrubAll(r.Recommendations.Performance, names),
		QuickWins:   scrubAll(r.Recommendations.QuickWins, names),
		Review:      scrubAll(r.Recommendations.Review, names),
	}
	s.Summary = buildSummary(s.Recommendations)
	return s
}

// anonymize replaces app names by their rank, keeping the magnitudes.
func anonymize(apps []App) []App {
	out := make([]App, len(apps))
	for i, a := range apps {
		a.Name = fmt.Sprintf("App %d", i+1)
		out[i] = a
	}
	return out
}

func appNames(p Processes) []string {
	var names []string
	for _, list := range [][]App{p.TopMemory, p.TopCPU} {
		for _, a := range list {
			if len(a.Name) >= 2 {
				names = append(names, a.Name)
			}
		}
	}
	return names
}

func scrubAll(in []Finding, names []string) []Finding {
	out := make([]Finding, len(in))
	for i, f := range in {
		out[i] = Finding{
			Title: scrubText(f.Title, names), Detail: scrubText(f.Detail, names),
			Action: scrubText(f.Action, names), Severity: f.Severity,
		}
	}
	return out
}

// scrubText removes app names, then drops every word that looks like a path
// (contains "/" or "~") or a long digest. Dropping the word is deliberate: a
// partial redaction could still reveal the rest of a path.
func scrubText(s string, names []string) string {
	for _, n := range names {
		q := regexp.QuoteMeta(n)
		s = regexp.MustCompile(`"`+q+`"`).ReplaceAllString(s, "ese proceso")
		s = regexp.MustCompile(`\b`+q+`\b`).ReplaceAllString(s, "un proceso")
	}
	words := strings.Fields(s)
	kept := words[:0]
	for _, w := range words {
		if strings.ContainsAny(w, "/~\\") || reLongHex.MatchString(w) {
			continue
		}
		kept = append(kept, w)
	}
	return strings.Join(kept, " ")
}

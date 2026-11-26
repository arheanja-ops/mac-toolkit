package reporter

import (
	"bytes"
	"fmt"
	"html/template"

	"github.com/arheanja-ops/mac-toolkit/internal/core"
)

// findingGroup titles a list of findings for the shared "findings" block.
type findingGroup struct {
	Title string
	List  []Finding
}

// levelClass picks the semantic colour of a usage percentage (same thresholds
// as the TUI: green below 70, yellow below 85, red from there).
func levelClass(pct float64) string {
	switch {
	case pct >= 85:
		return "crit"
	case pct >= 70:
		return "warn"
	default:
		return "ok"
	}
}

func statusClass(status string) string {
	switch status {
	case StatusCritical:
		return "crit"
	case StatusAttention:
		return "warn"
	default:
		return "ok"
	}
}

func sevClass(sev string) string {
	switch sev {
	case "high":
		return "crit"
	case "medium":
		return "warn"
	default:
		return "ok"
	}
}

// domainClass maps a core.Severity (critical/high/medium/low) to a colour,
// mirroring the CLI and the TUI.
func domainClass(sev string) string {
	switch sev {
	case "critical":
		return "crit"
	case "high", "medium":
		return "warn"
	default:
		return "ok"
	}
}

func riskClass(risk string) string {
	switch risk {
	case "danger":
		return "crit"
	case "warn":
		return "warn"
	default:
		return "ok"
	}
}

// htmlFuncs are the formatting helpers of the template. They return plain
// strings or numbers: html/template still escapes them on output.
var htmlFuncs = template.FuncMap{
	"bytes":       core.FormatBytes,
	"statusClass": statusClass, "statusLabel": StatusLabel,
	"sevClass": sevClass, "sevLabel": SeverityLabel,
	"domainClass": domainClass, "riskClass": riskClass, "levelClass": levelClass,
	"short": ShortPath,
	"inc":   func(i int) int { return i + 1 },
	"f0":    func(v float64) string { return fmt.Sprintf("%.0f", v) },
	"f1":    func(v float64) string { return fmt.Sprintf("%.1f", v) },
	"f2":    func(v float64) string { return fmt.Sprintf("%.2f", v) },
	"f64":   func(v int) float64 { return float64(v) },
	"pctw":  func(v float64) int { return max(0, min(int(v+0.5), 100)) },
	"yesNo": safeMark,
	"titled": func(title string, list []Finding) findingGroup {
		return findingGroup{Title: title, List: list}
	},
}

// reportTemplate is parsed once; a parse error is a programming error, caught
// by the tests, so Must is appropriate here.
var reportTemplate = template.Must(template.New("report").Funcs(htmlFuncs).Parse(htmlTemplate))

// HTML renders the self-contained page (inline CSS, no script, no remote
// resource, printable). Values are escaped by html/template.
func (r Report) HTML() ([]byte, error) {
	var buf bytes.Buffer
	if err := reportTemplate.Execute(&buf, r); err != nil {
		return nil, fmt.Errorf("rendering html report: %w", err)
	}
	return buf.Bytes(), nil
}

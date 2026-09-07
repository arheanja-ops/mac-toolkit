package analyzer

import (
	"fmt"
	"github.com/arheanja-ops/mac-toolkit/internal/core"
	"os/exec"
	"strconv"
	"strings"
)

type DiskAnalyzer struct{}

func init() { Register(&DiskAnalyzer{}) }

func (d *DiskAnalyzer) Domain() string      { return "disk" }
func (d *DiskAnalyzer) Risk() core.RiskLevel { return core.RiskSafe }

func (d *DiskAnalyzer) Analyze() (core.AnalysisResult, error) {
	out, err := exec.Command("df", "-k", "/System/Volumes/Data").Output()
	if err != nil {
		return core.AnalysisResult{Domain: "disk", Error: err.Error()}, err
	}

	lines := strings.Split(string(out), "\n")
	if len(lines) < 2 {
		return MakeResult("disk", nil, 0, "Could not parse df output"), nil
	}

	fields := strings.Fields(lines[1])
	if len(fields) < 5 {
		return MakeResult("disk", nil, 0, "Could not parse df output"), nil
	}

	totalKB, _ := strconv.ParseInt(fields[1], 10, 64)
	usedKB, _ := strconv.ParseInt(fields[2], 10, 64)
	availKB, _ := strconv.ParseInt(fields[3], 10, 64)

	totalBytes := totalKB * 1024
	usedBytes := usedKB * 1024
	availBytes := availKB * 1024

	pctUsed := 0
	if totalBytes > 0 {
		pctUsed = int(usedBytes * 100 / totalBytes)
	}

	var severity core.Severity
	switch {
	case pctUsed >= 85:
		severity = core.SeverityCritical
	case pctUsed >= 70:
		severity = core.SeverityHigh
	case pctUsed >= 50:
		severity = core.SeverityMedium
	default:
		severity = core.SeverityLow
	}

	summary := fmt.Sprintf("Disk: %s used / %s total (%d%%) — %s available",
		core.FormatBytes(usedBytes), core.FormatBytes(totalBytes), pctUsed, core.FormatBytes(availBytes))

	result := MakeResult("disk", nil, usedBytes, summary)
	result.Severity = severity
	return result, nil
}

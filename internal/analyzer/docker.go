package analyzer

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/arheanja-ops/mac-toolkit/internal/core"
)

type DockerAnalyzer struct{}

func init() { Register(&DockerAnalyzer{}) }

func (d *DockerAnalyzer) Domain() string       { return "docker" }
func (d *DockerAnalyzer) Risk() core.RiskLevel { return core.RiskDanger }

// dockerReclaimableFn asks the daemon for the RECLAIMABLE column of `docker
// system df`, one line per resource type. A package var so tests never talk to
// a real daemon.
var dockerReclaimableFn = func() (string, error) {
	out, err := exec.Command("docker", "system", "df", "--format", "{{.Reclaimable}}").Output()
	return string(out), err
}

func (d *DockerAnalyzer) Analyze() (core.AnalysisResult, error) {
	rawPath := core.DockerRawPath
	info, err := os.Stat(rawPath)
	if os.IsNotExist(err) {
		return MakeResult("docker", nil, 0, "Docker not installed"), nil
	}
	if err != nil {
		return MakeResult("docker", nil, 0, fmt.Sprintf("Error reading Docker path: %v", err)), nil
	}

	size := info.Size()
	items := []core.CleanableItem{{
		Path:         rawPath,
		SizeBytes:    size,
		Label:        "Docker.raw virtual disk",
		Domain:       "docker",
		SafeToDelete: false,
		Reason:       "Docker virtual disk — destroys all images/containers",
		Risk:         core.RiskDanger,
	}}

	out, dfErr := dockerReclaimableFn()
	return MakeResult("docker", items, size, dockerSummary(size, out, dfErr == nil)), nil
}

// dockerSummary writes one readable sentence instead of echoing the raw table
// of `docker system df`, whose columns made the report unreadable. When the
// daemon does not answer only the disk size is known, and it says so.
func dockerSummary(size int64, reclaimable string, daemonUp bool) string {
	base := "Disco virtual de Docker: " + core.FormatBytes(size)
	if !daemonUp {
		return base + " (daemon detenido: reclamable desconocido)"
	}
	return base + ", reclamable según Docker: " + core.FormatBytes(sumReclaimable(reclaimable))
}

// reReclaimable matches one value such as "800MB (66%)" or "1.2GB".
var reReclaimable = regexp.MustCompile(`^\s*(\d+(?:\.\d+)?)\s*([kKMGT]?B)\b`)

// dockerUnits are decimal, the way the docker CLI prints sizes.
var dockerUnits = map[string]float64{
	"B": 1, "KB": 1e3, "MB": 1e6, "GB": 1e9, "TB": 1e12,
}

// sumReclaimable adds the RECLAIMABLE values, one per line. Lines that do not
// parse are skipped: an unknown value must not turn into an invented number.
func sumReclaimable(raw string) int64 {
	var total float64
	for _, line := range strings.Split(raw, "\n") {
		m := reReclaimable.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		v, err := strconv.ParseFloat(m[1], 64)
		if err != nil {
			continue
		}
		total += v * dockerUnits[strings.ToUpper(m[2])]
	}
	return int64(total)
}

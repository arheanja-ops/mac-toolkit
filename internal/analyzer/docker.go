package analyzer

import (
	"fmt"
	"github.com/arheanja-ops/mac-toolkit/internal/core"
	"os"
	"os/exec"
	"strings"
)

type DockerAnalyzer struct{}

func init() { Register(&DockerAnalyzer{}) }

func (d *DockerAnalyzer) Domain() string      { return "docker" }
func (d *DockerAnalyzer) Risk() core.RiskLevel { return core.RiskDanger }

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

	// Check if daemon is running
	daemonStatus := "unknown"
	if out, err := exec.Command("docker", "system", "df").Output(); err == nil {
		daemonStatus = strings.TrimSpace(string(out))
	}

	items := []core.CleanableItem{{
		Path:         rawPath,
		SizeBytes:    size,
		Label:        "Docker.raw virtual disk",
		Domain:       "docker",
		SafeToDelete: false,
		Reason:       "Docker virtual disk — destroys all images/containers",
		Risk:         core.RiskDanger,
	}}

	summary := fmt.Sprintf("Docker: %s (daemon: %s)", core.FormatBytes(size), daemonStatus[:min(len(daemonStatus), 50)])
	return MakeResult("docker", items, size, summary), nil
}

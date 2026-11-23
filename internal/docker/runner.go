package docker

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// Runner executes a docker CLI command. It is an interface so tests can
// verify exactly which commands are issued without touching a real daemon.
type Runner interface {
	// Run executes `docker <args...>` and returns its stdout. On failure the
	// error carries the trimmed stderr so callers can surface the real cause.
	Run(args ...string) ([]byte, error)
}

// ExecRunner is the production Runner backed by exec.Command (never a shell).
type ExecRunner struct{}

// Run implements Runner.
func (ExecRunner) Run(args ...string) ([]byte, error) {
	cmd := exec.Command("docker", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return stdout.Bytes(), fmt.Errorf("%w: %s", err, msg)
		}
		return stdout.Bytes(), err
	}
	return stdout.Bytes(), nil
}

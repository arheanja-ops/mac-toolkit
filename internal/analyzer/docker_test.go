package analyzer

import (
	"strings"
	"testing"
)

func TestSumReclaimable(t *testing.T) {
	raw := "800MB (66%)\n0B\n1.5GB (40%)\ngarbage\n2kB\n"
	got := sumReclaimable(raw)
	want := int64(800e6 + 1.5e9 + 2e3)
	if got != want {
		t.Fatalf("sumReclaimable = %d, want %d", got, want)
	}
}

func TestDockerSummaryIsReadableSentence(t *testing.T) {
	s := dockerSummary(64<<30, "1GB (50%)\n0B\n", true)
	for _, bad := range []string{"TYPE", "TOTAL", "ACTIVE", "RECL", "daemon:"} {
		if strings.Contains(s, bad) {
			t.Fatalf("summary leaks raw docker output (%q): %s", bad, s)
		}
	}
	if !strings.Contains(s, "Disco virtual de Docker") || !strings.Contains(s, "reclamable") {
		t.Fatalf("unexpected summary: %s", s)
	}
}

func TestDockerSummaryDaemonDown(t *testing.T) {
	s := dockerSummary(1<<30, "", false)
	if !strings.Contains(s, "daemon detenido") {
		t.Fatalf("down daemon must be stated: %s", s)
	}
}

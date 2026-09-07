package core

import (
	"errors"
	"testing"
	"time"
)

func TestRunAnalyzersSuccess(t *testing.T) {
	tasks := []RunnerTask{
		{
			Domain: "test1",
			AnalyzeFn: func() (AnalysisResult, error) {
				return AnalysisResult{Domain: "test1", Severity: SeverityLow, Summary: "ok"}, nil
			},
		},
		{
			Domain: "test2",
			AnalyzeFn: func() (AnalysisResult, error) {
				return AnalysisResult{Domain: "test2", Severity: SeverityHigh, Summary: "big"}, nil
			},
		},
	}

	results := RunAnalyzers(tasks, 10)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	domains := map[string]bool{}
	for _, r := range results {
		domains[r.Domain] = true
		if r.Error != "" {
			t.Errorf("unexpected error for %s: %s", r.Domain, r.Error)
		}
	}
	if !domains["test1"] || !domains["test2"] {
		t.Error("missing expected domains")
	}
}

func TestRunAnalyzersError(t *testing.T) {
	tasks := []RunnerTask{
		{
			Domain: "failing",
			AnalyzeFn: func() (AnalysisResult, error) {
				return AnalysisResult{}, errors.New("analyzer crashed")
			},
		},
	}

	results := RunAnalyzers(tasks, 10)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Error == "" {
		t.Error("expected error result")
	}
}

func TestRunAnalyzersTimeout(t *testing.T) {
	tasks := []RunnerTask{
		{
			Domain: "slow",
			AnalyzeFn: func() (AnalysisResult, error) {
				time.Sleep(5 * time.Second)
				return AnalysisResult{Domain: "slow"}, nil
			},
		},
	}

	start := time.Now()
	results := RunAnalyzers(tasks, 1) // 1 second timeout
	elapsed := time.Since(start)

	if elapsed > 3*time.Second {
		t.Errorf("timeout took too long: %v", elapsed)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Error != "timeout" {
		t.Errorf("expected timeout error, got: %s", results[0].Error)
	}
}

func TestRunAnalyzersParallel(t *testing.T) {
	// 3 tasks that each sleep 500ms should complete in ~500ms if parallel
	tasks := make([]RunnerTask, 3)
	for i := range tasks {
		domain := string(rune('a' + i))
		tasks[i] = RunnerTask{
			Domain: domain,
			AnalyzeFn: func() (AnalysisResult, error) {
				time.Sleep(500 * time.Millisecond)
				return AnalysisResult{Domain: domain, Summary: "done"}, nil
			},
		}
	}

	start := time.Now()
	results := RunAnalyzers(tasks, 10)
	elapsed := time.Since(start)

	if elapsed > 2*time.Second {
		t.Errorf("tasks not parallel: took %v", elapsed)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
}

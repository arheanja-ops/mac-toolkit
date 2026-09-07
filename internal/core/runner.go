package core

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// AnalyzeFunc is the signature for any analyzer's Analyze method
type AnalyzeFunc func() (AnalysisResult, error)

// RunnerTask pairs a domain name with its analyze function
type RunnerTask struct {
	Domain    string
	AnalyzeFn AnalyzeFunc
}

// RunAnalyzers executes all analyzer tasks in parallel with a timeout.
// Returns results for all domains, including error results for timeouts/failures.
func RunAnalyzers(tasks []RunnerTask, timeoutSecs int) []AnalysisResult {
	if timeoutSecs <= 0 {
		timeoutSecs = AnalyzerTimeoutSeconds
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSecs)*time.Second)
	defer cancel()

	var mu sync.Mutex
	results := make([]AnalysisResult, 0, len(tasks))
	var wg sync.WaitGroup

	for _, task := range tasks {
		wg.Add(1)
		go func(t RunnerTask) {
			defer wg.Done()

			done := make(chan AnalysisResult, 1)
			errCh := make(chan error, 1)

			go func() {
				r, err := t.AnalyzeFn()
				if err != nil {
					errCh <- err
					return
				}
				done <- r
			}()

			select {
			case r := <-done:
				mu.Lock()
				results = append(results, r)
				mu.Unlock()
			case err := <-errCh:
				mu.Lock()
				results = append(results, AnalysisResult{
					Domain:    t.Domain,
					Severity:  SeverityLow,
					Timestamp: time.Now(),
					Error:     fmt.Sprintf("error: %v", err),
				})
				mu.Unlock()
			case <-ctx.Done():
				mu.Lock()
				results = append(results, AnalysisResult{
					Domain:    t.Domain,
					Severity:  SeverityLow,
					Timestamp: time.Now(),
					Error:     "timeout",
				})
				mu.Unlock()
			}
		}(task)
	}

	wg.Wait()
	return results
}

package ai

import (
	"context"
)

// Options controls engine selection for a single Diagnose call.
type Options struct {
	// NoAI forces the heuristic engine even when a key is present.
	NoAI bool
}

// Diagnose is the orchestrator. It returns a Groq-backed Diagnosis only when a
// key is present, --no-ai is off, and the call both succeeds and validates;
// otherwise it returns the deterministic heuristic. It never fails due to the
// absence of AI and never surfaces AI errors to the user.
func Diagnose(snap AISnapshot, opts Options) Diagnosis {
	if opts.NoAI {
		return Heuristic(snap)
	}
	client, ok := NewGroqClient()
	if !ok {
		return Heuristic(snap)
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	d, err := client.GroqDiagnose(ctx, snap)
	if err != nil {
		// Silent fallback: AI is best-effort, the heuristic is the contract.
		return Heuristic(snap)
	}
	return d
}

// ModelName reports the configured Groq model, for the engine label in the
// renderer. Empty when no key is set.
func ModelName() string {
	c, ok := NewGroqClient()
	if !ok {
		return ""
	}
	return c.model
}

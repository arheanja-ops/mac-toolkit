package core

import "testing"

// TestIsAffirmative is the authoritative table for the shared deletion gate.
// These cases were migrated from cmd/clean_test.go so the affirmative rule is
// tested once, at its source, and both the CLI gate and the TUI modal inherit
// the same guarantee.
func TestIsAffirmative(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		// Affirmatives — the only inputs that open the gate.
		{"si", true},
		{"sí", true},
		{"yes", true},
		{"y", true},
		{"  SI \n", true}, // trimmed + case-insensitive
		{"YES", true},
		{"Sí", true},
		// Negatives and ambiguity — must all fail to the safe default.
		{"n", false},
		{"no", false},
		{"", false}, // empty line never confirms
		{"maybe", false},
		{"s i", false}, // not a single affirmative token
		{"sim", false},
		{"yeah", false},
		{"0", false},
	}
	for _, c := range cases {
		if got := IsAffirmative(c.in); got != c.want {
			t.Errorf("IsAffirmative(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

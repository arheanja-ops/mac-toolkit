package core

import "strings"

// IsAffirmative reports whether s is an unambiguous human "yes". It is the
// single source of truth for the deletion gate shared by the CLI
// (confirmFinalDeletion) and the TUI confirmation modal, so both agree on
// exactly which words open the gate and neither can drift.
//
// It accepts the Spanish and English affirmatives "si"/"sí"/"yes"/"y",
// case-insensitively and after trimming surrounding whitespace. Everything
// else — including an empty string, a negative, or an ambiguous phrase like
// "s i" — returns false, so the safe default is always to NOT delete.
func IsAffirmative(s string) bool {
	switch strings.TrimSpace(strings.ToLower(s)) {
	case "si", "sí", "yes", "y":
		return true
	default:
		return false
	}
}

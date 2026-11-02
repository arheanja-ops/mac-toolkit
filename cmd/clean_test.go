package cmd

import (
	"strings"
	"testing"

	"github.com/arheanja-ops/mac-toolkit/internal/core"
)

// withStdin swaps the injectable final-confirmation reader for the duration of
// a test, restoring it afterward.
func withStdin(input string, fn func()) {
	orig := finalConfirmReader
	finalConfirmReader = strings.NewReader(input)
	defer func() { finalConfirmReader = orig }()
	fn()
}

// TestConfirmFinalDeletionAffirmatives verifies the gate opens ONLY for the
// explicit affirmatives — the final human confirmation before any deletion.
func TestConfirmFinalDeletionAffirmatives(t *testing.T) {
	items := []core.CleanableItem{{SizeBytes: 1024}}
	for _, in := range []string{"si\n", "sí\n", "yes\n", "y\n", "  SI \n"} {
		withStdin(in, func() {
			if !confirmFinalDeletion(items) {
				t.Fatalf("input %q should confirm deletion", in)
			}
		})
	}
}

// TestConfirmFinalDeletionRejections verifies the safe default: anything that
// is not an explicit affirmative — including empty input and EOF — must NOT
// delete. This is the core safety invariant.
func TestConfirmFinalDeletionRejections(t *testing.T) {
	items := []core.CleanableItem{{SizeBytes: 1024}}
	for _, in := range []string{"n\n", "no\n", "\n", "", "maybe\n", "s i\n"} {
		withStdin(in, func() {
			if confirmFinalDeletion(items) {
				t.Fatalf("input %q must NOT confirm deletion (safe default)", in)
			}
		})
	}
}

package core

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
)

// DigestStrings returns the SHA-256 (hex) of ids sorted and newline-joined.
// Sorting makes the digest independent of discovery order, so the same set of
// targets always yields the same value. It is the shared fingerprint behind the
// MCP preview → confirm_plan handshake (Docker cleanup and batch clean).
// The input slice is not modified.
func DigestStrings(ids []string) string {
	sorted := append([]string(nil), ids...)
	sort.Strings(sorted)
	sum := sha256.Sum256([]byte(strings.Join(sorted, "\n")))
	return hex.EncodeToString(sum[:])
}

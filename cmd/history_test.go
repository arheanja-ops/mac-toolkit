package cmd

import (
	"testing"
	"time"
)

func TestParseWhen(t *testing.T) {
	now := time.Date(2026, 1, 2, 12, 0, 0, 0, time.Local)
	got, err := parseWhen("08:30", now)
	if err != nil || !got.Equal(time.Date(2026, 1, 2, 8, 30, 0, 0, time.Local)) {
		t.Fatalf("HH:MM: %v %v", got, err)
	}
	got, err = parseWhen("2025-12-31T23:15", now)
	if err != nil || !got.Equal(time.Date(2025, 12, 31, 23, 15, 0, 0, time.Local)) {
		t.Fatalf("full: %v %v", got, err)
	}
	for _, bad := range []string{"", "8h", "25:99", "2025-12-31"} {
		if _, err := parseWhen(bad, now); err == nil {
			t.Fatalf("%q must be rejected", bad)
		}
	}
}

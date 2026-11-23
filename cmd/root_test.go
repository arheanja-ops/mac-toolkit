package cmd

import (
	"runtime/debug"
	"testing"
)

func TestResolveBuildInfo(t *testing.T) {
	full := &debug.BuildInfo{
		Main: debug.Module{Version: "v1.7.1"},
		Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: "0123456789abcdef"},
			{Key: "vcs.time", Value: "2026-11-03T10:00:00Z"},
		},
	}
	cases := []struct {
		name                string
		ver, com, dt        string
		info                *debug.BuildInfo
		wantV, wantC, wantD string
	}{
		{"go install", "dev", "none", "unknown", full, "v1.7.1", "0123456", "2026-11-03T10:00:00Z"},
		{"goreleaser wins", "v1.7.0", "abcdef0", "2026-01-01", full, "v1.7.0", "abcdef0", "2026-01-01"},
		{"devel keeps dev", "dev", "none", "unknown",
			&debug.BuildInfo{Main: debug.Module{Version: "(devel)"}}, "dev", "none", "unknown"},
		{"empty version keeps dev", "dev", "none", "unknown",
			&debug.BuildInfo{}, "dev", "none", "unknown"},
		{"nil info", "dev", "none", "unknown", nil, "dev", "none", "unknown"},
		{"short revision kept", "dev", "none", "unknown",
			&debug.BuildInfo{Settings: []debug.BuildSetting{{Key: "vcs.revision", Value: "abc"}}},
			"dev", "abc", "unknown"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v, c, d := resolveBuildInfo(tc.ver, tc.com, tc.dt, tc.info)
			if v != tc.wantV || c != tc.wantC || d != tc.wantD {
				t.Fatalf("got (%s,%s,%s) want (%s,%s,%s)", v, c, d, tc.wantV, tc.wantC, tc.wantD)
			}
		})
	}
}

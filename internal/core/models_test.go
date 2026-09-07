package core

import "testing"

func TestSeverityFromSize(t *testing.T) {
	tests := []struct {
		name  string
		bytes int64
		want  Severity
	}{
		{"low", 50 * 1024 * 1024, SeverityLow},
		{"medium", 200 * 1024 * 1024, SeverityMedium},
		{"high", 2 * 1024 * 1024 * 1024, SeverityHigh},
		{"critical", 15 * 1024 * 1024 * 1024, SeverityCritical},
		{"zero", 0, SeverityLow},
		{"boundary_100mb", 100*1024*1024 + 1, SeverityMedium},
		{"boundary_1gb", 1024*1024*1024 + 1, SeverityHigh},
		{"boundary_10gb", 10*1024*1024*1024 + 1, SeverityCritical},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SeverityFromSize(tt.bytes)
			if got != tt.want {
				t.Errorf("SeverityFromSize(%d) = %s, want %s", tt.bytes, got, tt.want)
			}
		})
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		bytes int64
		want  string
	}{
		{0, "0 B"},
		{500, "500 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{1073741824, "1.0 GB"},
		{1099511627776, "1.0 TB"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := FormatBytes(tt.bytes)
			if got != tt.want {
				t.Errorf("FormatBytes(%d) = %q, want %q", tt.bytes, got, tt.want)
			}
		})
	}
}

func TestCleanableItem(t *testing.T) {
	item := CleanableItem{
		Path:         "/tmp/test",
		SizeBytes:    1024,
		Label:        "Test",
		Domain:       "test",
		SafeToDelete: true,
		Reason:       "test reason",
		Risk:         RiskSafe,
	}

	if item.Path != "/tmp/test" {
		t.Error("Path mismatch")
	}
	if item.Risk != RiskSafe {
		t.Error("Risk mismatch")
	}
}

func TestAnalysisResult(t *testing.T) {
	result := AnalysisResult{
		Domain:   "test",
		Severity: SeverityLow,
		Items:    []CleanableItem{},
	}

	if result.Domain != "test" {
		t.Error("Domain mismatch")
	}
	if len(result.Items) != 0 {
		t.Error("Items should be empty")
	}
}

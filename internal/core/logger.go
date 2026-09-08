package core

import (
	"fmt"
	"os"
	"strings"
)

// ANSI color codes
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorCyan   = "\033[36m"
	colorGray   = "\033[90m"
	colorBold   = "\033[1m"
)

// Exported ANSI codes for callers that build their own colored output.
const (
	Reset = colorReset
	Cyan  = colorCyan
	Gray  = colorGray
	Bold_ = colorBold
)

// Colorize wraps s in the given ANSI code and a reset.
func Colorize(code, s string) string { return code + s + colorReset }

// ProgressBar renders a fixed-width unicode bar for a 0..100 percentage,
// colored green/yellow/red by fill level. width is the number of cells.
func ProgressBar(pct, width int) string {
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	filled := pct * width / 100
	color := colorGreen
	switch {
	case pct >= 85:
		color = colorRed
	case pct >= 70:
		color = colorYellow
	}
	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
	return color + bar + colorReset
}

// Info prints an informational message
func Info(msg string, args ...any) {
	fmt.Fprintf(os.Stderr, colorBlue+"ℹ "+colorReset+msg+"\n", args...)
}

// Success prints a success message
func Success(msg string, args ...any) {
	fmt.Fprintf(os.Stderr, colorGreen+"✓ "+colorReset+msg+"\n", args...)
}

// Warn prints a warning message
func Warn(msg string, args ...any) {
	fmt.Fprintf(os.Stderr, colorYellow+"⚠ "+colorReset+msg+"\n", args...)
}

// Error prints an error message
func Error(msg string, args ...any) {
	fmt.Fprintf(os.Stderr, colorRed+"✗ "+colorReset+msg+"\n", args...)
}

// Bold prints bold text
func Bold(msg string, args ...any) {
	fmt.Fprintf(os.Stderr, colorBold+msg+colorReset+"\n", args...)
}

// SeverityColor returns the ANSI color for a severity level
func SeverityColor(s Severity) string {
	switch s {
	case SeverityCritical:
		return colorRed
	case SeverityHigh:
		return colorYellow
	case SeverityMedium:
		return colorYellow
	case SeverityLow:
		return colorGreen
	default:
		return colorReset
	}
}

// RiskColor returns the ANSI color for a risk level
func RiskColor(r RiskLevel) string {
	switch r {
	case RiskDanger:
		return colorRed
	case RiskWarn:
		return colorYellow
	case RiskSafe:
		return colorGreen
	default:
		return colorReset
	}
}

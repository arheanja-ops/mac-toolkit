package core

import (
	"fmt"
	"os"
)

// ANSI color codes
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorCyan   = "\033[36m"
	colorBold   = "\033[1m"
)

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

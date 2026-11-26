package reporter

import (
	"fmt"
	"strings"

	"github.com/arheanja-ops/mac-toolkit/internal/core"
)

// StatusLabel is the user-facing name of a status.
func StatusLabel(status string) string {
	switch status {
	case StatusCritical:
		return "Crítico"
	case StatusAttention:
		return "Atención"
	default:
		return "OK"
	}
}

// SeverityLabel names a finding severity.
func SeverityLabel(sev string) string {
	switch sev {
	case "high":
		return "alta"
	case "medium":
		return "media"
	default:
		return "baja"
	}
}

// TextBar draws a proportion bar of width cells; pct is clamped to 0-100 and any
// non-zero value shows at least one cell, so a small domain is still visible.
func TextBar(pct float64, width int) string {
	if width < 1 {
		return ""
	}
	n := int(pct*float64(width)/100 + 0.5)
	if pct > 0 && n < 1 {
		n = 1
	}
	n = max(0, min(n, width))
	return strings.Repeat("█", n) + strings.Repeat("░", width-n)
}

// CoresLine is "10 (4 P + 6 E)" or just the total when the split is unknown.
func (h Host) CoresLine() string {
	switch {
	case h.Cores == 0:
		return ""
	case h.PerfCores > 0 && h.EffCores > 0:
		return fmt.Sprintf("%d (%d P + %d E)", h.Cores, h.PerfCores, h.EffCores)
	default:
		return fmt.Sprintf("%d", h.Cores)
	}
}

// UptimeLine renders the uptime as days and hours.
func (h Host) UptimeLine() string {
	if h.UptimeHours <= 0 {
		return ""
	}
	days := int(h.UptimeHours) / 24
	hours := int(h.UptimeHours) % 24
	if days > 0 {
		return fmt.Sprintf("%d d %d h", days, hours)
	}
	return fmt.Sprintf("%d h", hours)
}

// HostRows lists the non-empty header facts as label/value pairs, in order.
func (r Report) HostRows() [][2]string {
	h := r.Host
	rows := [][2]string{
		{"Fecha", r.GeneratedAt.Format("2006-01-02 15:04")},
		{"Modelo", h.Model}, {"Chip", h.Chip}, {"Núcleos", h.CoresLine()},
		{"Memoria RAM", h.RAM}, {"macOS", h.OS}, {"Uptime", h.UptimeLine()},
		{"mac-toolkit", r.ToolVersion},
	}
	var out [][2]string
	for _, row := range rows {
		if row[1] != "" {
			out = append(out, row)
		}
	}
	return out
}

// SwapLine is "1.0 GB usados de 2.0 GB" (empty when swap is unknown).
func (m Memory) SwapLine() string {
	if m.SwapTotalMB <= 0 {
		return ""
	}
	return fmt.Sprintf("%s usados de %s",
		core.FormatBytes(int64(m.SwapUsedMB*1024*1024)), core.FormatBytes(int64(m.SwapTotalMB*1024*1024)))
}

// SwapTrafficLine states the cumulative swap traffic and its hourly average.
func (m Memory) SwapTrafficLine() string {
	s := fmt.Sprintf("entrada %.1f GB · salida %.1f GB desde el arranque", m.SwapInGB, m.SwapOutGB)
	if m.SwapInGBPerHour > 0 || m.SwapOutGBPerHour > 0 {
		s += fmt.Sprintf(" (media %.2f GB por hora entrada, %.2f GB por hora salida)",
			m.SwapInGBPerHour, m.SwapOutGBPerHour)
	}
	return s
}

// CompressorLine states the compressor size and its ratio when known.
func (m Memory) CompressorLine() string {
	s := fmt.Sprintf("%.1f GB", m.CompressedGB)
	if m.CompressionRatio > 0 {
		s += fmt.Sprintf(" (ratio %.1f a 1)", m.CompressionRatio)
	}
	return s
}

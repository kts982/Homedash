package panels

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/kostas/homedash/internal/collector"
	"github.com/kostas/homedash/internal/ui/components"
	"github.com/kostas/homedash/internal/ui/styles"
)

func RenderSystem(data collector.SystemData, cpuHistory *components.RingBuffer, width, height int, focused bool) string {
	innerWidth := width - 4 // panel border + padding

	var lines []string

	// CPU gauge
	lines = append(lines, components.Gauge("CPU", data.CPUPercent, innerWidth))

	// CPU sparkline
	sparkWidth := innerWidth - 2
	if sparkWidth > 60 {
		sparkWidth = 60
	}
	spark := components.Sparkline(cpuHistory.Data(), sparkWidth, styles.Primary)
	sparkLabel := lipgloss.NewStyle().Foreground(styles.TextMuted).Render("  (2m)")
	lines = append(lines, "  "+spark+sparkLabel)

	// RAM gauge
	lines = append(lines, components.Gauge("RAM", data.MemPercent, innerWidth))

	// RAM details
	ramDetail := lipgloss.NewStyle().Foreground(styles.TextMuted).Render(
		fmt.Sprintf("       %s / %s",
			collector.FormatBytes(data.MemUsed),
			collector.FormatBytes(data.MemTotal)))
	lines = append(lines, ramDetail)

	// Disk gauges
	for _, d := range data.Disks {
		label := fmt.Sprintf("%-6s", d.Mount)
		lines = append(lines, components.Gauge(label, d.Percent, innerWidth))
	}

	// Load average
	loadStyle := lipgloss.NewStyle().Foreground(styles.TextSecondary)
	labelStyle := lipgloss.NewStyle().Foreground(styles.TextSecondary).Bold(true).Width(6)
	loadVals := fmt.Sprintf("%.1f  %.1f  %.1f", data.LoadAvg[0], data.LoadAvg[1], data.LoadAvg[2])
	lines = append(lines, labelStyle.Render("LOAD")+" "+loadStyle.Render(loadVals))

	// Network I/O rate.
	netDown := collector.FormatRate(data.NetRxRate)
	netUp := collector.FormatRate(data.NetTxRate)
	downStyled := lipgloss.NewStyle().Foreground(styles.Primary).Render("↓ " + netDown)
	upStyled := lipgloss.NewStyle().Foreground(styles.Secondary).Render("↑ " + netUp)
	lines = append(lines, labelStyle.Render("NET")+" "+downStyled+"  "+upStyled)

	content := strings.Join(lines, "\n")
	return components.Panel("SYSTEM", content, width, height, focused)
}

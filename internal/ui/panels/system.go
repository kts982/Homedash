package panels

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/kostas/homedash/internal/collector"
	"github.com/kostas/homedash/internal/ui/components"
	"github.com/kostas/homedash/internal/ui/styles"
)

func RenderSystem(data collector.SystemData, cpuHistory, ramHistory *components.RingBuffer, width, height int, focused bool) string {
	innerWidth := width - 4 // panel border + padding

	// Narrow: single-column fallback (matches isNarrow() threshold: width < 90)
	if width < 90 {
		return renderSystemSingleColumn(data, cpuHistory, ramHistory, innerWidth, width, height, focused)
	}

	leftWidth := innerWidth * 55 / 100
	if leftWidth < 30 {
		leftWidth = 30
	}
	rightWidth := innerWidth - leftWidth

	// Left column: sparklines + gauges
	var leftLines []string

	// CPU sparkline
	// Account for indent(2) + space(1) + label "(2m)"(4) = 7 chars of overhead
	sparkWidth := leftWidth - 7
	if sparkWidth > 60 {
		sparkWidth = 60
	}
	if sparkWidth < 1 {
		sparkWidth = 1
	}
	cpuSpark := components.Sparkline(cpuHistory.Data(), sparkWidth, styles.Primary)
	sparkLabel := lipgloss.NewStyle().Foreground(styles.TextMuted).Render("(2m)")
	leftLines = append(leftLines, "  "+cpuSpark+" "+sparkLabel)

	// CPU gauge
	leftLines = append(leftLines, components.Gauge("CPU", data.CPUPercent, leftWidth))

	// RAM sparkline
	ramSpark := components.Sparkline(ramHistory.Data(), sparkWidth, styles.Secondary)
	leftLines = append(leftLines, "  "+ramSpark+" "+sparkLabel)

	// RAM gauge
	leftLines = append(leftLines, components.Gauge("RAM", data.MemPercent, leftWidth))

	// Disk gauges
	for _, d := range data.Disks {
		label := fmt.Sprintf("%-6s", d.Mount)
		leftLines = append(leftLines, components.Gauge(label, d.Percent, leftWidth))
	}

	// Right column: text stats
	labelStyle := lipgloss.NewStyle().Foreground(styles.TextSecondary).Bold(true).Width(6)
	valStyle := lipgloss.NewStyle().Foreground(styles.TextSecondary)

	var rightLines []string

	// LOAD
	loadVals := fmt.Sprintf("%.1f  %.1f  %.1f", data.LoadAvg[0], data.LoadAvg[1], data.LoadAvg[2])
	loadStyle := valStyle
	if data.CPUCount > 0 && data.LoadAvg[0] > float64(data.CPUCount) {
		loadStyle = lipgloss.NewStyle().Foreground(styles.Warning)
	}
	rightLines = append(rightLines, labelStyle.Render("LOAD")+" "+loadStyle.Render(loadVals))

	// NET
	netDown := collector.FormatRate(data.NetRxRate)
	netUp := collector.FormatRate(data.NetTxRate)
	downStyled := lipgloss.NewStyle().Foreground(styles.Primary).Render("↓ " + netDown)
	upStyled := lipgloss.NewStyle().Foreground(styles.Secondary).Render("↑ " + netUp)
	rightLines = append(rightLines, labelStyle.Render("NET")+" "+downStyled+"  "+upStyled)

	// MEM absolute
	memText := fmt.Sprintf("%s / %s", collector.FormatBytes(data.MemUsed), collector.FormatBytes(data.MemTotal))
	rightLines = append(rightLines, labelStyle.Render("MEM")+" "+valStyle.Render(memText))

	// SWAP
	swapLine := renderSwapLine(data, labelStyle)
	rightLines = append(rightLines, swapLine)

	// Cap content lines
	maxContent := 12
	if len(leftLines) > maxContent {
		leftLines = leftLines[:maxContent]
	}
	if len(rightLines) > maxContent {
		rightLines = rightLines[:maxContent]
	}

	leftCol := strings.Join(leftLines, "\n")
	rightCol := strings.Join(rightLines, "\n")

	// Pad right column to rightWidth so JoinHorizontal aligns properly
	rightColStyled := lipgloss.NewStyle().Width(rightWidth).Render(rightCol)

	content := lipgloss.JoinHorizontal(lipgloss.Top, leftCol, rightColStyled)
	return components.Panel("SYSTEM", content, width, height, focused)
}

func renderSwapLine(data collector.SystemData, labelStyle lipgloss.Style) string {
	if data.SwapTotal == 0 {
		return labelStyle.Render("SWAP") + " " + lipgloss.NewStyle().Foreground(styles.TextMuted).Render("disabled")
	}
	swapStyle := lipgloss.NewStyle().Foreground(styles.TextSecondary)
	if data.SwapPercent > 25 {
		swapStyle = lipgloss.NewStyle().Foreground(styles.Warning)
	}
	swapText := fmt.Sprintf("%s / %s", collector.FormatBytes(data.SwapUsed), collector.FormatBytes(data.SwapTotal))
	return labelStyle.Render("SWAP") + " " + swapStyle.Render(swapText)
}

func renderSystemSingleColumn(data collector.SystemData, cpuHistory, ramHistory *components.RingBuffer, innerWidth, width, height int, focused bool) string {
	var lines []string

	// Account for indent(2) + space(1) + label "(2m)"(4) = 7 chars of overhead
	sparkWidth := innerWidth - 7
	if sparkWidth > 60 {
		sparkWidth = 60
	}
	if sparkWidth < 1 {
		sparkWidth = 1
	}
	sparkLabel := lipgloss.NewStyle().Foreground(styles.TextMuted).Render("(2m)")
	labelStyle := lipgloss.NewStyle().Foreground(styles.TextSecondary).Bold(true).Width(6)
	valStyle := lipgloss.NewStyle().Foreground(styles.TextSecondary)

	// CPU
	lines = append(lines, "  "+components.Sparkline(cpuHistory.Data(), sparkWidth, styles.Primary)+" "+sparkLabel)
	lines = append(lines, components.Gauge("CPU", data.CPUPercent, innerWidth))

	// RAM
	lines = append(lines, "  "+components.Sparkline(ramHistory.Data(), sparkWidth, styles.Secondary)+" "+sparkLabel)
	lines = append(lines, components.Gauge("RAM", data.MemPercent, innerWidth))

	// Disks
	for _, d := range data.Disks {
		label := fmt.Sprintf("%-6s", d.Mount)
		lines = append(lines, components.Gauge(label, d.Percent, innerWidth))
	}

	// LOAD
	loadVals := fmt.Sprintf("%.1f  %.1f  %.1f", data.LoadAvg[0], data.LoadAvg[1], data.LoadAvg[2])
	lines = append(lines, labelStyle.Render("LOAD")+" "+valStyle.Render(loadVals))

	// NET
	netDown := collector.FormatRate(data.NetRxRate)
	netUp := collector.FormatRate(data.NetTxRate)
	downStyled := lipgloss.NewStyle().Foreground(styles.Primary).Render("↓ " + netDown)
	upStyled := lipgloss.NewStyle().Foreground(styles.Secondary).Render("↑ " + netUp)
	lines = append(lines, labelStyle.Render("NET")+" "+downStyled+"  "+upStyled)

	// MEM absolute
	memText := fmt.Sprintf("%s / %s", collector.FormatBytes(data.MemUsed), collector.FormatBytes(data.MemTotal))
	lines = append(lines, labelStyle.Render("MEM")+" "+valStyle.Render(memText))

	// SWAP
	lines = append(lines, renderSwapLine(data, labelStyle))

	content := strings.Join(lines, "\n")
	return components.Panel("SYSTEM", content, width, height, focused)
}

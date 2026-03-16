package panels

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/kostas/homedash/internal/collector"
	"github.com/kostas/homedash/internal/ui/components"
	"github.com/kostas/homedash/internal/ui/styles"
)

func RenderSystem(data collector.SystemData, cpuHistory, ramHistory *components.RingBuffer, width, height int, focused bool, freshnessLabel string) string {
	innerWidth := width - 4 // panel border + padding

	// Narrow: single-column fallback (matches isNarrow() threshold: width < 90)
	if width < 90 {
		return renderSystemSingleColumn(data, cpuHistory, ramHistory, innerWidth, width, height, focused, freshnessLabel)
	}

	colGap := 2
	leftWidth := innerWidth * 48 / 100
	if leftWidth < 36 {
		leftWidth = 36
	}
	if leftWidth > 72 {
		leftWidth = 72
	}
	rightWidth := innerWidth - leftWidth - colGap

	// Left column: sparklines + gauges
	var leftLines []string

	// CPU sparkline
	// Account for indent(2) + space(1) + label "(2m)"(4) = 7 chars of overhead
	sparkWidth := leftWidth - 7
	if sparkWidth > 44 {
		sparkWidth = 44
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

	// Disk gauges with usage/capacity
	for _, d := range data.Disks {
		label := fmt.Sprintf("%-6s", d.Mount)
		detail := fmt.Sprintf("%s / %s", collector.FormatBytes(d.Used), collector.FormatBytes(d.Total))
		leftLines = append(leftLines, components.GaugeWithDetail(label, d.Percent, detail, leftWidth))
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
	loadLine := labelStyle.Render("LOAD") + " " + loadStyle.Render(loadVals)
	tasksLine := renderSystemTasksLine(data, labelStyle, valStyle)
	rightLines = append(rightLines, renderSystemStatRow(loadLine, tasksLine, rightWidth))

	// NET
	netDown := collector.FormatRate(data.NetRxRate)
	netUp := collector.FormatRate(data.NetTxRate)
	downStyled := lipgloss.NewStyle().Foreground(styles.Primary).Render("↓ " + netDown)
	upStyled := lipgloss.NewStyle().Foreground(styles.Secondary).Render("↑ " + netUp)
	netLine := labelStyle.Render("NET") + " " + downStyled + "  " + upStyled
	filesLine := renderSystemFilesLine(data, labelStyle, valStyle)
	rightLines = append(rightLines, renderSystemStatRow(netLine, filesLine, rightWidth))

	// MEM absolute
	memText := fmt.Sprintf("%s / %s", collector.FormatBytes(data.MemUsed), collector.FormatBytes(data.MemTotal))
	memLine := labelStyle.Render("MEM") + " " + valStyle.Render(memText)
	disksLine := labelStyle.Render("DISKS") + " " + valStyle.Render(systemDiskSummary(data.Disks))
	rightLines = append(rightLines, renderSystemStatRow(memLine, disksLine, rightWidth))

	// SWAP
	swapLine := renderSwapLine(data, labelStyle)
	hotLine := labelStyle.Render("HOT") + " " + renderHotMount(data.Disks)
	rightLines = append(rightLines, renderSystemStatRow(swapLine, hotLine, rightWidth))

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

	// Pad columns with gap between them
	rightColStyled := lipgloss.NewStyle().Width(rightWidth).Render(rightCol)
	gap := strings.Repeat(" ", colGap)

	content := lipgloss.JoinHorizontal(lipgloss.Top, leftCol, gap, rightColStyled)
	title := "SYSTEM"
	if freshnessLabel != "" {
		title += " · " + freshnessLabel
	}
	return components.Panel(title, content, width, height, focused)
}

func renderSystemStatRow(left, right string, width int) string {
	if width <= 0 {
		return ""
	}
	if right == "" {
		return lipgloss.NewStyle().Inline(true).MaxWidth(width).Render(left)
	}

	const colGap = 2
	leftWidth := width * 58 / 100
	if leftWidth < 18 {
		leftWidth = width / 2
	}
	if leftWidth >= width-colGap {
		leftWidth = width - colGap
	}
	rightWidth := width - leftWidth - colGap
	if rightWidth < 1 {
		rightWidth = 1
	}

	leftRendered := lipgloss.NewStyle().Inline(true).MaxWidth(leftWidth).Render(left)
	rightRendered := lipgloss.NewStyle().Inline(true).MaxWidth(rightWidth).Render(right)

	gap := leftWidth - lipgloss.Width(leftRendered) + colGap
	row := leftRendered + strings.Repeat(" ", gap) + rightRendered
	return lipgloss.NewStyle().Inline(true).MaxWidth(width).Render(row)
}

func renderSystemTasksLine(data collector.SystemData, labelStyle, valStyle lipgloss.Style) string {
	if data.TotalTasks <= 0 {
		return labelStyle.Render("TASKS") + " " + lipgloss.NewStyle().Foreground(styles.TextMuted).Render("-")
	}
	tasks := fmt.Sprintf("%d run / %d total", data.RunningTasks, data.TotalTasks)
	return labelStyle.Render("TASKS") + " " + valStyle.Render(tasks)
}

func renderSystemFilesLine(data collector.SystemData, labelStyle, valStyle lipgloss.Style) string {
	if data.MaxFiles == 0 {
		return labelStyle.Render("FILES") + " " + lipgloss.NewStyle().Foreground(styles.TextMuted).Render("-")
	}
	files := formatSystemCount(data.OpenFiles) + " / " + systemFileLimitLabel(data.MaxFiles)
	return labelStyle.Render("FILES") + " " + valStyle.Render(files)
}

func systemDiskSummary(disks []collector.DiskInfo) string {
	if len(disks) == 0 {
		return "0 mounts"
	}

	var free uint64
	for _, disk := range disks {
		if disk.Total > disk.Used {
			free += disk.Total - disk.Used
		}
	}

	summary := fmt.Sprintf("%d mounts", len(disks))
	if free == 0 {
		return summary
	}
	return summary + " · " + collector.FormatBytes(free) + " free"
}

func renderHotMount(disks []collector.DiskInfo) string {
	if len(disks) == 0 {
		return lipgloss.NewStyle().Foreground(styles.TextMuted).Render("-")
	}

	hot := disks[0]
	for _, disk := range disks[1:] {
		if disk.Percent > hot.Percent || (disk.Percent == hot.Percent && disk.Mount < hot.Mount) {
			hot = disk
		}
	}

	valueStyle := lipgloss.NewStyle().Foreground(styles.TextSecondary)
	if hot.Percent >= 80 {
		valueStyle = lipgloss.NewStyle().Foreground(styles.Warning)
	}
	return valueStyle.Render(fmt.Sprintf("%s %.0f%%", hot.Mount, hot.Percent))
}

func formatSystemCount(value uint64) string {
	const (
		thousand = 1000
		million  = thousand * 1000
		billion  = million * 1000
	)

	switch {
	case value >= billion:
		return fmt.Sprintf("%.1fB", float64(value)/float64(billion))
	case value >= million:
		return fmt.Sprintf("%.1fM", float64(value)/float64(million))
	case value >= thousand:
		return fmt.Sprintf("%.1fK", float64(value)/float64(thousand))
	default:
		return fmt.Sprintf("%d", value)
	}
}

func systemFileLimitLabel(value uint64) string {
	const unlimitedThreshold = uint64(1 << 60)
	if value >= unlimitedThreshold {
		return "unlimited"
	}
	return formatSystemCount(value)
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

func renderSystemSingleColumn(data collector.SystemData, cpuHistory, ramHistory *components.RingBuffer, innerWidth, width, height int, focused bool, freshnessLabel string) string {
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

	// Disks with usage/capacity
	for _, d := range data.Disks {
		label := fmt.Sprintf("%-6s", d.Mount)
		detail := fmt.Sprintf("%s / %s", collector.FormatBytes(d.Used), collector.FormatBytes(d.Total))
		lines = append(lines, components.GaugeWithDetail(label, d.Percent, detail, innerWidth))
	}

	// LOAD
	loadVals := fmt.Sprintf("%.1f  %.1f  %.1f", data.LoadAvg[0], data.LoadAvg[1], data.LoadAvg[2])
	lines = append(lines, renderSystemStatRow(labelStyle.Render("LOAD")+" "+valStyle.Render(loadVals), renderSystemTasksLine(data, labelStyle, valStyle), innerWidth))

	// NET
	netDown := collector.FormatRate(data.NetRxRate)
	netUp := collector.FormatRate(data.NetTxRate)
	downStyled := lipgloss.NewStyle().Foreground(styles.Primary).Render("↓ " + netDown)
	upStyled := lipgloss.NewStyle().Foreground(styles.Secondary).Render("↑ " + netUp)
	lines = append(lines, renderSystemStatRow(labelStyle.Render("NET")+" "+downStyled+"  "+upStyled, renderSystemFilesLine(data, labelStyle, valStyle), innerWidth))

	// MEM absolute
	memText := fmt.Sprintf("%s / %s", collector.FormatBytes(data.MemUsed), collector.FormatBytes(data.MemTotal))
	lines = append(lines, renderSystemStatRow(labelStyle.Render("MEM")+" "+valStyle.Render(memText), labelStyle.Render("DISKS")+" "+valStyle.Render(systemDiskSummary(data.Disks)), innerWidth))

	// SWAP
	lines = append(lines, renderSystemStatRow(renderSwapLine(data, labelStyle), labelStyle.Render("HOT")+" "+renderHotMount(data.Disks), innerWidth))

	content := strings.Join(lines, "\n")
	title := "SYSTEM"
	if freshnessLabel != "" {
		title += " · " + freshnessLabel
	}
	return components.Panel(title, content, width, height, focused)
}

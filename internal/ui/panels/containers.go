package panels

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
	"github.com/kostas/homedash/internal/collector"
	"github.com/kostas/homedash/internal/ui/components"
	"github.com/kostas/homedash/internal/ui/styles"
)

type ContainerDisplayItem struct {
	IsGroup        bool
	StackName      string
	ContainerCount int
	RunningCount   int
	UnhealthyCount int
	StartingCount  int
	StoppedCount   int
	Collapsed      bool
	Container      *collector.Container
}

type stackSummarySegment struct {
	text        string
	compactText string
	style       lipgloss.Style
}

func RenderContainers(items []ContainerDisplayItem, running, total, scrollOffset, selectedIndex, visibleRows, width int, focused bool, searchInput textinput.Model, filtering bool, testMode bool) string {
	innerWidth := width - 4

	// Adaptive columns based on available width
	showStack := innerWidth >= 75
	showHealth := innerWidth >= 85

	statusW := 10
	cpuW := 7
	memW := 10

	// Fixed columns total
	fixed := statusW + cpuW + memW + 3 // +3 for spaces between them
	if showStack {
		fixed += 14 + 1 // stackW + space
	}
	if showHealth {
		fixed += 10 + 1 // healthW + space
	}

	nameW := innerWidth - fixed - 1 // -1 trailing space
	if nameW < 12 {
		nameW = 12
	}
	stackW := 14
	healthW := 10

	headerStyle := lipgloss.NewStyle().
		Foreground(styles.TextMuted).
		Bold(true)

	// Build header dynamically
	header := fmt.Sprintf("%-*s", nameW, "NAME")
	if showStack {
		header += fmt.Sprintf(" %-*s", stackW, "STACK")
	}
	header += fmt.Sprintf(" %-*s", statusW, "STATUS")
	if showHealth {
		header += fmt.Sprintf(" %-*s", healthW, "HEALTH")
	}
	header += fmt.Sprintf(" %*s %*s", cpuW, "CPU%", memW, "MEMORY")
	headerLine := headerStyle.Render(truncate(header, innerWidth))

	// Container rows
	var rows []string
	end := scrollOffset + visibleRows
	if end > len(items) {
		end = len(items)
	}

	for i := scrollOffset; i < end; i++ {
		item := items[i]
		var row string
		if item.IsGroup {
			row = formatGroupHeader(
				item.StackName,
				item.RunningCount,
				item.ContainerCount,
				item.UnhealthyCount,
				item.StartingCount,
				item.StoppedCount,
				item.Collapsed,
				innerWidth,
			)
		} else if item.Container != nil {
			row = "  " + formatContainerRow(*item.Container, nameW-2, stackW, statusW, healthW, cpuW, memW, innerWidth-2, showStack, showHealth)
		}
		if focused && i == selectedIndex {
			row = lipgloss.NewStyle().Background(styles.BgFocus).Width(innerWidth).Render(row)
		}
		rows = append(rows, row)
	}

	// Status line
	refreshNoteText := "[5s refresh]"
	if testMode {
		refreshNoteText = "[test mode]"
	}
	refreshNote := lipgloss.NewStyle().Foreground(styles.TextMuted).Render(refreshNoteText)
	summary := lipgloss.NewStyle().Foreground(styles.TextSecondary).Render(
		fmt.Sprintf("(%d/%d running)", running, total))

	// 12 = visual width of " CONTAINERS " added by Panel(" "+title+" ")
	titleMaxWidth := innerWidth - 12
	spacer := max(0, titleMaxWidth-lipgloss.Width(summary)-lipgloss.Width(refreshNote)-1)
	titleExtra := " " + summary + strings.Repeat(" ", spacer) + refreshNote

	content := headerLine + "\n" + strings.Join(rows, "\n")

	// Prepend search input if filtering OR if there is a filter active
	if filtering || searchInput.Value() != "" {
		filterLine := searchInput.View()
		if !filtering {
			// If not active, but filter exists, show it grayed out
			filterLine = lipgloss.NewStyle().Foreground(styles.TextMuted).Render(" / " + searchInput.Value())
		}
		content = filterLine + "\n" + content
	}

	return components.Panel("CONTAINERS"+titleExtra, content, width, visibleRows+4, focused)
}

func formatGroupHeader(name string, running, total, unhealthy, starting, stopped int, collapsed bool, width int) string {
	if width <= 0 {
		return ""
	}

	arrow := "▼"
	if collapsed {
		arrow = "▶"
	}

	arrowStyle := lipgloss.NewStyle().Foreground(styles.Secondary).Bold(true)
	nameStyle := lipgloss.NewStyle().Foreground(styles.Secondary).Bold(true)

	summarySegments := buildStackSummarySegments(running, total, unhealthy, starting, stopped)
	summarySegments = fitStackSummarySegments(summarySegments, width-lipgloss.Width(arrow)-1)

	line := arrowStyle.Render(arrow) + " "
	if len(summarySegments) == 0 {
		return line + nameStyle.Render(truncate(name, width-2))
	}

	summary := renderStackSummarySegments(summarySegments)
	summaryWidth := stackSummaryWidth(summarySegments)
	nameWidth := width - lipgloss.Width(arrow) - 1 - 2 - summaryWidth
	if nameWidth < 1 {
		nameWidth = 1
	}

	line += nameStyle.Render(truncate(name, nameWidth))
	if summary != "" {
		line += strings.Repeat(" ", 2) + summary
	}

	return truncate(line, width)
}

func buildStackSummarySegments(running, total, unhealthy, starting, stopped int) []stackSummarySegment {
	segments := []stackSummarySegment{
		{
			text:        fmt.Sprintf("%d/%d up", running, total),
			compactText: fmt.Sprintf("%d/%d", running, total),
			style:       lipgloss.NewStyle().Foreground(styles.TextSecondary).Bold(true),
		},
	}
	if unhealthy > 0 {
		segments = append(segments, stackSummarySegment{
			text:  fmt.Sprintf("%d unhealthy", unhealthy),
			style: lipgloss.NewStyle().Foreground(styles.Error).Bold(true),
		})
	}
	if starting > 0 {
		segments = append(segments, stackSummarySegment{
			text:  fmt.Sprintf("%d starting", starting),
			style: lipgloss.NewStyle().Foreground(styles.Warning).Bold(true),
		})
	}
	if stopped > 0 {
		segments = append(segments, stackSummarySegment{
			text:  fmt.Sprintf("%d stopped", stopped),
			style: lipgloss.NewStyle().Foreground(styles.TextMuted).Bold(true),
		})
	}
	return segments
}

func fitStackSummarySegments(segments []stackSummarySegment, width int) []stackSummarySegment {
	if len(segments) == 0 || width <= 0 {
		return nil
	}

	fitted := append([]stackSummarySegment(nil), segments...)
	minNameWidth := 8
	if width < minNameWidth {
		minNameWidth = width
	}

	for len(fitted) > 1 && width-stackSummaryWidth(fitted)-2 < minNameWidth {
		fitted = fitted[:len(fitted)-1]
	}

	if width-stackSummaryWidth(fitted)-2 < minNameWidth && fitted[0].compactText != "" {
		fitted[0].text = fitted[0].compactText
	}

	if width-stackSummaryWidth(fitted)-2 < minNameWidth {
		return nil
	}

	return fitted
}

func stackSummaryWidth(segments []stackSummarySegment) int {
	if len(segments) == 0 {
		return 0
	}

	width := 0
	for i, segment := range segments {
		if i > 0 {
			width += 2
		}
		width += lipgloss.Width(segment.text)
	}
	return width
}

func renderStackSummarySegments(segments []stackSummarySegment) string {
	if len(segments) == 0 {
		return ""
	}

	rendered := make([]string, len(segments))
	for i, segment := range segments {
		rendered[i] = segment.style.Render(segment.text)
	}
	return strings.Join(rendered, "  ")
}

func formatContainerRow(c collector.Container, nameW, stackW, statusW, healthW, cpuW, memW, maxWidth int, showStack, showHealth bool) string {
	stateStyle := lipgloss.NewStyle().Foreground(styles.ContainerStateColor(c.State))

	healthStyle := lipgloss.NewStyle().Foreground(styles.TextMuted)
	switch c.Health {
	case "healthy":
		healthStyle = lipgloss.NewStyle().Foreground(styles.Success)
	case "unhealthy":
		healthStyle = lipgloss.NewStyle().Foreground(styles.Error)
	case "starting":
		healthStyle = lipgloss.NewStyle().Foreground(styles.Warning)
	}

	nameStyle := lipgloss.NewStyle().Foreground(styles.TextPrimary)
	stackStyle := lipgloss.NewStyle().Foreground(styles.Secondary)
	cpuStyle := lipgloss.NewStyle().Foreground(styles.TextSecondary)
	memStyle := lipgloss.NewStyle().Foreground(styles.TextSecondary)

	cpuStr := ""
	memStr := ""
	if c.State == "running" {
		cpuStr = fmt.Sprintf("%.1f%%", c.CPUPerc)
		memStr = collector.FormatBytes(c.MemUsed)
	}

	row := nameStyle.Render(pad(c.Name, nameW))
	if showStack {
		row += " " + stackStyle.Render(pad(c.Stack, stackW))
	}
	row += " " + stateStyle.Render(pad(c.State, statusW))
	if showHealth {
		row += " " + healthStyle.Render(pad(c.Health, healthW))
	}
	row += " " + cpuStyle.Render(lpad(cpuStr, cpuW))
	row += " " + memStyle.Render(lpad(memStr, memW))

	return truncate(row, maxWidth)
}

func pad(s string, w int) string {
	if w <= 0 {
		return ""
	}
	clamped := lipgloss.NewStyle().Inline(true).MaxWidth(w).Render(s)
	if padW := w - lipgloss.Width(clamped); padW > 0 {
		return clamped + strings.Repeat(" ", padW)
	}
	return clamped
}

func lpad(s string, w int) string {
	if w <= 0 {
		return ""
	}
	clamped := lipgloss.NewStyle().Inline(true).MaxWidth(w).Render(s)
	if padW := w - lipgloss.Width(clamped); padW > 0 {
		return strings.Repeat(" ", padW) + clamped
	}
	return clamped
}

func truncate(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= maxWidth {
		return s
	}
	return lipgloss.NewStyle().Inline(true).MaxWidth(maxWidth).Render(s)
}

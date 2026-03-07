package panels

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/kostas/homedash/internal/collector"
	"github.com/kostas/homedash/internal/ui/components"
	"github.com/kostas/homedash/internal/ui/styles"
)

type StackDetail struct {
	Name           string
	ContainerCount int
	RunningCount   int
	UnhealthyCount int
	StartingCount  int
	StoppedCount   int
	CPUPerc        float64
	MemUsed        uint64
	Containers     []StackDetailContainer
}

type StackDetailContainer struct {
	Name   string
	State  string
	Health string
	Image  string
	Ports  string
}

func StackDetailInfoPanelHeight(stack *StackDetail, width int) int {
	if stack == nil {
		return 7
	}
	innerWidth := width - 4
	if innerWidth < 1 {
		innerWidth = 1
	}
	return len(stackDetailInfoLines(stack, innerWidth)) + 3
}

func RenderStackDetail(
	stack *StackDetail,
	logs []string, logsErr error,
	confirmAction, actionResult string,
	scrollOffset, width, height int,
	logFollowing bool,
	logSearch LogSearch,
) string {
	if stack == nil {
		return "No stack selected"
	}

	innerWidth := width - 4
	titleAvail := width - 6

	nameStyled := lipgloss.NewStyle().Foreground(styles.TextPrimary).Render(stack.Name)
	titleLeft := nameStyled + lipgloss.NewStyle().Foreground(styles.TextMuted).Render("  stack")
	if stack.RunningCount > 0 {
		statsStyled := lipgloss.NewStyle().Foreground(styles.TextSecondary).Render(
			fmt.Sprintf("CPU %.1f%%  Mem %s", stack.CPUPerc, collector.FormatBytes(stack.MemUsed)),
		)
		if lipgloss.Width(titleLeft)+2+lipgloss.Width(statsStyled) <= titleAvail {
			titleLeft += "  " + statsStyled
		}
	}
	infoTitle := lipgloss.NewStyle().Inline(true).MaxWidth(titleAvail).Render(titleLeft)

	infoLines := stackDetailInfoLines(stack, innerWidth)
	infoPanelHeight := len(infoLines) + 3
	infoPanel := components.Panel(infoTitle, strings.Join(infoLines, "\n"), width, infoPanelHeight, false)

	logPanelHeight := height - infoPanelHeight - 1
	if logPanelHeight < 5 {
		logPanelHeight = 5
	}
	logContentHeight := logPanelHeight - 3
	if logContentHeight < 1 {
		logContentHeight = 1
	}

	logLines, logState := stackDetailLogLines(logs, logsErr, logFollowing, innerWidth)
	maxScroll := len(logLines) - logContentHeight
	if maxScroll < 0 {
		maxScroll = 0
	}
	if scrollOffset > maxScroll {
		scrollOffset = maxScroll
	}
	if scrollOffset < 0 {
		scrollOffset = 0
	}

	logTitleLeft := renderLogTitle(logState, scrollOffset, logContentHeight, len(logLines), titleAvail, logSearch)

	endIdx := scrollOffset + logContentHeight
	if endIdx > len(logLines) {
		endIdx = len(logLines)
	}
	visible := logLines[scrollOffset:endIdx]
	highlightSearchLines(visible, scrollOffset, logSearch, innerWidth)
	for len(visible) < logContentHeight {
		visible = append(visible, "")
	}
	logPanel := components.Panel(logTitleLeft, strings.Join(visible, "\n"), width, logPanelHeight, true)

	actionBar := renderStackDetailActionBar(stack, confirmAction, actionResult, width, logFollowing, logSearch)

	return lipgloss.JoinVertical(lipgloss.Left, infoPanel, logPanel, actionBar)
}

func stackDetailInfoLines(stack *StackDetail, innerWidth int) []string {
	labelStyle := lipgloss.NewStyle().Foreground(styles.TextMuted).Width(detailLabelWidth)
	valueStyle := lipgloss.NewStyle().Foreground(styles.TextPrimary)

	summary := stackDetailSummary(stack)
	members := stackDetailMembersLine(stack.Containers, innerWidth)
	images := stackDetailImagesLine(stack.Containers, innerWidth)
	ports := stackDetailPortsLine(stack.Containers, innerWidth)

	infoLines := []string{
		formatDetailLine(labelStyle, valueStyle, "Status", summary, innerWidth),
		formatDetailLine(labelStyle, valueStyle, "Members", members, innerWidth),
	}
	if images != "" {
		infoLines = append(infoLines, formatDetailLine(labelStyle, valueStyle, "Images", images, innerWidth))
	}
	if ports != "" {
		infoLines = append(infoLines, formatDetailLine(labelStyle, valueStyle, "Ports", ports, innerWidth))
	}
	return infoLines
}

func stackDetailSummary(stack *StackDetail) string {
	var parts []string
	parts = append(parts, fmt.Sprintf("%d/%d up", stack.RunningCount, stack.ContainerCount))
	if stack.UnhealthyCount > 0 {
		parts = append(parts, fmt.Sprintf("%d unhealthy", stack.UnhealthyCount))
	}
	if stack.StartingCount > 0 {
		parts = append(parts, fmt.Sprintf("%d starting", stack.StartingCount))
	}
	if stack.StoppedCount > 0 {
		parts = append(parts, fmt.Sprintf("%d stopped", stack.StoppedCount))
	}
	return strings.Join(parts, "  ")
}

func stackDetailMembersLine(containers []StackDetailContainer, innerWidth int) string {
	if len(containers) == 0 {
		return "No containers currently in stack"
	}

	parts := make([]string, 0, len(containers))
	for _, container := range containers {
		state := strings.TrimSpace(container.State)
		switch container.Health {
		case "unhealthy":
			state += " unhealthy"
		case "starting":
			state += " starting"
		}
		parts = append(parts, strings.TrimSpace(container.Name+" "+state))
	}
	return summarizeDetailItems(parts, ", ", detailValueWidth(innerWidth))
}

func stackDetailImagesLine(containers []StackDetailContainer, innerWidth int) string {
	seen := make(map[string]struct{})
	var images []string
	for _, container := range containers {
		image := strings.TrimSpace(container.Image)
		if image == "" {
			continue
		}
		if _, ok := seen[image]; ok {
			continue
		}
		seen[image] = struct{}{}
		images = append(images, image)
	}
	if len(images) == 0 {
		return ""
	}
	return summarizeDetailItems(images, ", ", detailValueWidth(innerWidth))
}

func stackDetailPortsLine(containers []StackDetailContainer, innerWidth int) string {
	var parts []string
	for _, container := range containers {
		ports := strings.TrimSpace(container.Ports)
		if ports == "" || ports == "-" {
			continue
		}
		parts = append(parts, container.Name+" "+ports)
	}
	if len(parts) == 0 {
		return ""
	}
	return summarizeDetailItems(parts, ", ", detailValueWidth(innerWidth))
}

func stackDetailLogLines(logs []string, logsErr error, logFollowing bool, innerWidth int) ([]string, detailLogState) {
	switch {
	case logs == nil && logsErr == nil && logFollowing:
		return []string{
			lipgloss.NewStyle().Foreground(styles.TextMuted).Render("Waiting for stack log output..."),
			lipgloss.NewStyle().Foreground(styles.TextSecondary).Render("Follow mode is active across running containers."),
		}, detailLogStateWaiting
	case logs == nil && logsErr == nil:
		return []string{
			lipgloss.NewStyle().Foreground(styles.TextMuted).Render("Loading stack logs..."),
			lipgloss.NewStyle().Foreground(styles.TextSecondary).Render("Fetching recent logs across stack containers."),
		}, detailLogStateLoading
	case logsErr != nil:
		return []string{
			lipgloss.NewStyle().Foreground(styles.Error).Render("Stack log refresh failed"),
			lipgloss.NewStyle().Foreground(styles.TextSecondary).Render(
				lipgloss.NewStyle().Inline(true).MaxWidth(innerWidth).Render(logsErr.Error())),
		}, detailLogStateError
	case len(logs) == 0 && logFollowing:
		return []string{
			lipgloss.NewStyle().Foreground(styles.TextMuted).Render("Following stack log streams..."),
			lipgloss.NewStyle().Foreground(styles.TextSecondary).Render("No log lines received yet."),
		}, detailLogStateWaiting
	case len(logs) == 0:
		return []string{
			lipgloss.NewStyle().Foreground(styles.TextMuted).Render("No stack logs available"),
			lipgloss.NewStyle().Foreground(styles.TextSecondary).Render("Docker returned no log lines for this stack."),
		}, detailLogStateEmpty
	default:
		rendered := make([]string, 0, len(logs))
		for _, line := range logs {
			rendered = append(rendered, formatLogLine(line, innerWidth))
		}
		return rendered, detailLogStateLoaded
	}
}

func renderStackDetailActionBar(stack *StackDetail, confirmAction, actionResult string, width int, logFollowing bool, logSearch LogSearch) string {
	keyStyle := lipgloss.NewStyle().Foreground(styles.Primary).Bold(true)
	descStyle := lipgloss.NewStyle().Foreground(styles.TextMuted)

	var content string
	if logSearch.Active {
		content = logSearch.InputView
		if logSearch.Total > 0 {
			content += "  " + lipgloss.NewStyle().Foreground(styles.TextSecondary).
				Render(fmt.Sprintf("%d/%d", logSearch.Current, logSearch.Total))
		} else if logSearch.Query != "" {
			content += "  " + lipgloss.NewStyle().Foreground(styles.Error).Render("no matches")
		}
		content += "   " + keyStyle.Render("enter") + descStyle.Render(" accept") +
			"   " + keyStyle.Render("esc") + descStyle.Render(" cancel")
	} else if confirmAction != "" {
		confirmStyle := lipgloss.NewStyle().Foreground(styles.Warning).Bold(true)
		content = confirmStyle.Render(fmt.Sprintf("%s %s?", capitalize(confirmAction), stack.Name)) +
			"  " + keyStyle.Render("y") + descStyle.Render(" confirm") +
			"  " + keyStyle.Render("n/esc") + descStyle.Render(" cancel")
	} else if actionResult != "" {
		resultColor := styles.Success
		if strings.HasPrefix(actionResult, "Error") {
			resultColor = styles.Error
		}
		content = lipgloss.NewStyle().Foreground(resultColor).Bold(true).Render(actionResult)
	} else {
		followLabel := "follow"
		if logFollowing {
			followLabel = "unfollow"
		}
		parts := []string{
			keyStyle.Render("esc") + descStyle.Render(" back"),
			keyStyle.Render("j/k") + descStyle.Render(" scroll"),
			keyStyle.Render("ctrl+u/d") + descStyle.Render(" page"),
			keyStyle.Render("g/G") + descStyle.Render(" top/end"),
			keyStyle.Render("f") + descStyle.Render(" "+followLabel),
			keyStyle.Render("l") + descStyle.Render(" refresh"),
			keyStyle.Render("/") + descStyle.Render(" search"),
		}
		if logSearch.Query != "" {
			parts = append(parts,
				keyStyle.Render("n/N")+descStyle.Render(" next/prev"))
		}
		if stack.StoppedCount > 0 {
			parts = append(parts, keyStyle.Render("S")+descStyle.Render(" start"))
		}
		if stack.RunningCount > 0 {
			parts = append(parts,
				keyStyle.Render("s")+descStyle.Render(" stop"),
				keyStyle.Render("R")+descStyle.Render(" restart"))
		}
		content = strings.Join(parts, "   ")
	}

	return lipgloss.NewStyle().
		Background(styles.BgPanel).
		Foreground(styles.TextPrimary).
		Width(width).
		Padding(0, 1).
		Render(content)
}

package panels

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/kts982/homedash/internal/collector"
	"github.com/kts982/homedash/internal/ui/styles"
)

type StackPreview struct {
	Name           string
	ContainerCount int
	RunningCount   int
	UnhealthyCount int
	StartingCount  int
	StoppedCount   int
	CPUPerc        float64
	MemUsed        uint64
}

// RenderPreview renders a single-line preview bar for the selected container or stack.
// When confirmAction or actionResult is set, it shows that instead of the normal preview.
func RenderPreview(c *collector.Container, stack *StackPreview, confirmAction, confirmName, actionResult string, width int) string {
	barStyle := lipgloss.NewStyle().
		Background(styles.BgPanel).
		Foreground(styles.TextPrimary).
		Width(width).
		Padding(0, 1)

	keyStyle := lipgloss.NewStyle().Foreground(styles.Primary).Bold(true)
	descStyle := lipgloss.NewStyle().Foreground(styles.TextSecondary)

	// Action confirmation takes priority
	if confirmAction != "" && confirmName != "" {
		confirmStyle := lipgloss.NewStyle().Foreground(styles.Warning).Bold(true)
		content := confirmStyle.Render(
			fmt.Sprintf("%s %s?", capitalize(confirmAction), confirmName)) +
			"  " + keyStyle.Render("y") + descStyle.Render(" confirm") +
			"  " + keyStyle.Render("n/esc") + descStyle.Render(" cancel")
		return barStyle.Render(content)
	}

	// Action result feedback
	if actionResult != "" {
		resultColor := styles.Success
		if strings.HasPrefix(actionResult, "Error") {
			resultColor = styles.Error
		}
		content := lipgloss.NewStyle().Foreground(resultColor).Bold(true).Render(actionResult)
		return barStyle.Render(content)
	}

	// Normal preview
	labelStyle := lipgloss.NewStyle().Foreground(styles.TextMuted)
	valueStyle := lipgloss.NewStyle().Foreground(styles.TextPrimary)

	if c == nil {
		if stack == nil {
			return barStyle.Render(labelStyle.Render("No container selected"))
		}
		maxContentW := width - 2 // padding(0,1)
		summarySegments := buildStackSummarySegments(
			stack.RunningCount,
			stack.ContainerCount,
			stack.UnhealthyCount,
			stack.StartingCount,
			stack.StoppedCount,
		)
		prefix := "stack "
		statusPrefix := "  status "
		summarySegments = fitStackSummarySegments(
			summarySegments,
			maxContentW-lipgloss.Width(prefix)-lipgloss.Width(statusPrefix),
		)
		summary := renderStackSummarySegments(summarySegments)
		resourceWidth := 0
		showResources := stack.RunningCount > 0
		resourceValueCPU := fmt.Sprintf("%.1f%%", stack.CPUPerc)
		resourceValueMem := collector.FormatBytes(stack.MemUsed)
		resourceText := ""
		if showResources {
			resourceText = labelStyle.Render("  cpu ") + valueStyle.Render(resourceValueCPU) +
				labelStyle.Render("  mem ") + valueStyle.Render(resourceValueMem)
			resourceWidth = lipgloss.Width("  cpu ") + lipgloss.Width(resourceValueCPU) +
				lipgloss.Width("  mem ") + lipgloss.Width(resourceValueMem)
		}
		nameWidth := maxContentW - lipgloss.Width(prefix)
		if summary != "" {
			nameWidth -= lipgloss.Width(statusPrefix) + stackSummaryWidth(summarySegments)
		}
		if showResources && nameWidth-resourceWidth >= 10 {
			nameWidth -= resourceWidth
		} else {
			showResources = false
		}
		if nameWidth < 1 {
			nameWidth = 1
		}
		name := lipgloss.NewStyle().Inline(true).MaxWidth(nameWidth).Render(stack.Name)
		row := labelStyle.Render("stack ") + valueStyle.Render(name)
		if summary != "" {
			row += labelStyle.Render("  status ") + summary
		}
		if showResources {
			row += resourceText
		}
		row = lipgloss.NewStyle().Inline(true).MaxWidth(maxContentW).Render(row)
		return barStyle.Render(row)
	}

	stateColor := styles.ContainerStateColor(c.State)
	stateStyled := lipgloss.NewStyle().Foreground(stateColor).Render(c.State)

	ports := collector.FormatPorts(c.Ports)

	// Truncate image to leave room for state and ports
	maxImageW := width - lipgloss.Width(stateStyled) - lipgloss.Width(ports) - 20
	if maxImageW < 15 {
		maxImageW = 15
	}
	image := lipgloss.NewStyle().Inline(true).MaxWidth(maxImageW).Render(c.Image)

	row := stateStyled +
		labelStyle.Render("  image ") + valueStyle.Render(image) +
		labelStyle.Render("  ports ") + valueStyle.Render(ports)

	return barStyle.Render(row)
}

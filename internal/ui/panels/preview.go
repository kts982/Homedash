package panels

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/kostas/homedash/internal/collector"
	"github.com/kostas/homedash/internal/ui/styles"
)

// RenderPreview renders a single-line preview bar for the selected container.
// When confirmAction or actionResult is set, it shows that instead of the normal preview.
func RenderPreview(c *collector.Container, confirmAction, confirmName, actionResult string, width int) string {
	barStyle := lipgloss.NewStyle().
		Background(styles.BgPanel).
		Foreground(styles.TextPrimary).
		Width(width).
		Padding(0, 1)

	keyStyle := lipgloss.NewStyle().Foreground(styles.Primary).Bold(true)
	descStyle := lipgloss.NewStyle().Foreground(styles.TextMuted)

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
		return barStyle.Render(labelStyle.Render("No container selected"))
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

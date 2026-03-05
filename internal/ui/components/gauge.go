package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/kostas/homedash/internal/ui/styles"
)

// Gauge renders a horizontal bar gauge: ██████░░░░ 60%
func Gauge(label string, percent float64, width int) string {
	labelStyle := lipgloss.NewStyle().
		Foreground(styles.TextSecondary).
		Bold(true).
		Width(6)

	pctStr := fmt.Sprintf("%3.0f%%", percent)
	// Bar width = total width - label(6) - space(1) - space(1) - pct(4)
	barWidth := width - 6 - 1 - 1 - 4
	if barWidth < 5 {
		barWidth = 5
	}

	filled := int(float64(barWidth) * percent / 100)
	if filled > barWidth {
		filled = barWidth
	}
	empty := barWidth - filled

	color := styles.GaugeColor(percent)
	filledStr := lipgloss.NewStyle().Foreground(color).Render(strings.Repeat("█", filled))
	emptyStr := lipgloss.NewStyle().Foreground(styles.BgFocus).Render(strings.Repeat("░", empty))
	pctStyle := lipgloss.NewStyle().Foreground(color).Bold(true)

	return labelStyle.Render(label) + " " + filledStr + emptyStr + " " + pctStyle.Render(pctStr)
}

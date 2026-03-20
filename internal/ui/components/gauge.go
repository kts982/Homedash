package components

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/kts982/homedash/internal/ui/styles"
)

// Gauge renders a horizontal bar gauge: ██████░░░░ 60%
func Gauge(label string, percent float64, width int) string {
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

	return renderGaugeLabel(label) + " " + filledStr + emptyStr + " " + pctStyle.Render(pctStr)
}

// GaugeWithDetail renders a gauge with usage/capacity text centered on the bar.
// Example: /data  ███ 105G/250G ░░░  40%
func GaugeWithDetail(label string, percent float64, detail string, width int) string {
	pctStr := fmt.Sprintf("%3.0f%%", percent)
	// Bar width = total width - label(6) - space(1) - space(1) - pct(4)
	barWidth := width - 6 - 1 - 1 - 4
	if barWidth < 5 {
		barWidth = 5
	}

	detailLen := len(detail)
	filled := int(float64(barWidth) * percent / 100)
	if filled > barWidth {
		filled = barWidth
	}

	color := styles.GaugeColor(percent)
	detailStyle := lipgloss.NewStyle().Foreground(styles.TextPrimary).Bold(true)

	// If the bar is wide enough, overlay the detail text centered on the bar
	if detailLen+2 <= barWidth { // +2 for spaces around detail
		// Center position for the detail text
		pad := (barWidth - detailLen) / 2
		padRight := barWidth - detailLen - pad

		// Build bar with detail overlaid
		var bar strings.Builder
		// Left portion of bar (before detail)
		if pad <= filled {
			bar.WriteString(lipgloss.NewStyle().Foreground(color).Render(strings.Repeat("█", pad)))
		} else {
			bar.WriteString(lipgloss.NewStyle().Foreground(color).Render(strings.Repeat("█", filled)))
			bar.WriteString(lipgloss.NewStyle().Foreground(styles.BgFocus).Render(strings.Repeat("░", pad-filled)))
		}
		// Detail text
		bar.WriteString(detailStyle.Render(detail))
		// Right portion of bar (after detail)
		rightStart := pad + detailLen
		if rightStart < filled {
			bar.WriteString(lipgloss.NewStyle().Foreground(color).Render(strings.Repeat("█", filled-rightStart)))
			bar.WriteString(lipgloss.NewStyle().Foreground(styles.BgFocus).Render(strings.Repeat("░", padRight-(filled-rightStart))))
		} else {
			remaining := barWidth - rightStart
			if remaining < 0 {
				remaining = 0
			}
			bar.WriteString(lipgloss.NewStyle().Foreground(styles.BgFocus).Render(strings.Repeat("░", remaining)))
		}

		pctStyle := lipgloss.NewStyle().Foreground(color).Bold(true)
		return renderGaugeLabel(label) + " " + bar.String() + " " + pctStyle.Render(pctStr)
	}

	// Fall back to regular gauge if detail doesn't fit
	return Gauge(label, percent, width)
}

func renderGaugeLabel(label string) string {
	return lipgloss.NewStyle().
		Foreground(styles.TextSecondary).
		Bold(true).
		Inline(true).
		MaxWidth(6).
		Width(6).
		Render(label)
}

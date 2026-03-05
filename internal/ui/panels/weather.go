package panels

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/kostas/homedash/internal/collector"
	"github.com/kostas/homedash/internal/ui/components"
	"github.com/kostas/homedash/internal/ui/styles"
)

func RenderWeather(data collector.WeatherData, weatherErr error, retries, width, height int, focused bool) string {
	if data.TempC == "" {
		msg := "Loading weather..."
		if weatherErr != nil && retries > 0 {
			msg = fmt.Sprintf("Retrying... (%d/3)", retries)
		} else if weatherErr != nil {
			msg = "Weather unavailable"
		}
		content := lipgloss.NewStyle().Foreground(styles.TextMuted).Render(msg)
		return components.Panel("WEATHER", content, width, height, focused)
	}

	var lines []string

	// Location in title area
	location := lipgloss.NewStyle().Foreground(styles.TextMuted).Render(data.Location)

	// Main weather line
	tempStyle := lipgloss.NewStyle().Foreground(styles.Info).Bold(true)
	condStyle := lipgloss.NewStyle().Foreground(styles.TextPrimary)
	mainLine := fmt.Sprintf("%s %s %s",
		data.Icon,
		tempStyle.Render(data.TempC+"°C"),
		condStyle.Render(data.Condition))
	lines = append(lines, mainLine)

	// Details
	detailStyle := lipgloss.NewStyle().Foreground(styles.TextSecondary)
	lines = append(lines, detailStyle.Render(
		fmt.Sprintf("Feels %s°C  Humidity %s%%", data.FeelsLikeC, data.Humidity)))
	lines = append(lines, detailStyle.Render(
		fmt.Sprintf("Wind %skm/h %s", data.WindSpeed, data.WindDir)))

	content := strings.Join(lines, "\n")
	titleSuffix := strings.Repeat(" ", max(0, width-18-lipgloss.Width(location))) + location
	return components.Panel("WEATHER"+titleSuffix, content, width, height, focused)
}

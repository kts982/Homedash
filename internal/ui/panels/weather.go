package panels

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/kostas/homedash/internal/collector"
	"github.com/kostas/homedash/internal/ui/components"
	"github.com/kostas/homedash/internal/ui/styles"
)

func RenderWeather(data collector.WeatherData, weatherErr error, retries, width, height int, focused bool) string {
	if data.TempC == "" {
		return renderWeatherNoData(weatherErr, retries, width, height, focused)
	}
	return renderWeatherWithData(data, weatherErr, retries, width, height, focused)
}

func renderWeatherNoData(weatherErr error, retries, width, height int, focused bool) string {
	mutedStyle := lipgloss.NewStyle().Foreground(styles.TextMuted)
	errStyle := lipgloss.NewStyle().Foreground(styles.Error)

	switch {
	case weatherErr == nil:
		content := mutedStyle.Render("Loading weather...")
		return components.Panel("WEATHER", content, width, height, focused)

	case retries > 0:
		msg := mutedStyle.Render(fmt.Sprintf("Retrying... (%d/3)", retries))
		detail := lipgloss.NewStyle().Foreground(styles.TextSecondary).Render(
			lipgloss.NewStyle().Inline(true).MaxWidth(width - 6).Render(weatherErr.Error()))
		content := msg + "\n" + detail
		return components.Panel("WEATHER", content, width, height, focused)

	default:
		msg := errStyle.Render("Weather unavailable")
		detail := lipgloss.NewStyle().Foreground(styles.TextSecondary).Render(
			lipgloss.NewStyle().Inline(true).MaxWidth(width - 6).Render(weatherErr.Error()))
		content := msg + "\n" + detail
		return components.Panel("WEATHER", content, width, height, focused)
	}
}

func renderWeatherWithData(data collector.WeatherData, weatherErr error, retries, width, height int, focused bool) string {
	// Choose styles based on error state
	tempStyle := lipgloss.NewStyle().Foreground(styles.Info).Bold(true)
	condStyle := lipgloss.NewStyle().Foreground(styles.TextPrimary)
	detailStyle := lipgloss.NewStyle().Foreground(styles.TextSecondary)

	// Dim styles when data is stale (error with retries exhausted)
	stale := weatherErr != nil && retries == 0
	if stale {
		tempStyle = lipgloss.NewStyle().Foreground(styles.TextMuted)
		condStyle = lipgloss.NewStyle().Foreground(styles.TextMuted)
		detailStyle = lipgloss.NewStyle().Foreground(styles.TextMuted)
	}

	var lines []string

	// Main weather line
	mainLine := fmt.Sprintf("%s %s %s",
		data.Icon,
		tempStyle.Render(data.TempC+"°C"),
		condStyle.Render(data.Condition))
	lines = append(lines, mainLine)

	// Details
	lines = append(lines, detailStyle.Render(
		fmt.Sprintf("Feels %s°C  Humidity %s%%", data.FeelsLikeC, data.Humidity)))
	lines = append(lines, detailStyle.Render(
		fmt.Sprintf("Wind %skm/h %s", data.WindSpeed, data.WindDir)))

	// Staleness / retry indicator
	if stale {
		warnStyle := lipgloss.NewStyle().Foreground(styles.Warning)
		staleMsg := "⚠ update failed"
		if !data.CollectedAt.IsZero() {
			staleMsg += fmt.Sprintf("  last ok %s", formatWeatherAge(data.CollectedAt))
		}
		lines = append(lines, warnStyle.Render(staleMsg))
	} else if weatherErr != nil && retries > 0 {
		retryStyle := lipgloss.NewStyle().Foreground(styles.TextMuted)
		lines = append(lines, retryStyle.Render(fmt.Sprintf("⟳ retrying (%d/3)", retries)))
	}

	// Build title with location and status
	location := lipgloss.NewStyle().Foreground(styles.TextMuted).Render(data.Location)
	title := "WEATHER"
	if stale {
		title = lipgloss.NewStyle().Foreground(styles.Warning).Bold(true).Render("WEATHER") +
			" " + lipgloss.NewStyle().Foreground(styles.Warning).Render("⚠")
	}
	titleSuffix := strings.Repeat(" ", max(0, width-18-lipgloss.Width(location))) + location

	content := strings.Join(lines, "\n")
	return components.Panel(title+titleSuffix, content, width, height, focused)
}

func formatWeatherAge(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	default:
		return fmt.Sprintf("%dh%dm ago", int(d.Hours()), int(d.Minutes())%60)
	}
}

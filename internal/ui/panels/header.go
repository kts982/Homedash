package panels

import (
	"fmt"
	"image/color"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/kostas/homedash/internal/collector"
	"github.com/kostas/homedash/internal/ui/styles"
)

func RenderHeader(data collector.SystemData, weather collector.WeatherData, weatherErr error, weatherRetries int, width int, testMode bool) string {
	bg := styles.BgPanel
	fg := styles.TextPrimary

	base := lipgloss.NewStyle().
		Background(bg).
		Foreground(fg).
		Width(width)

	titleStyle := lipgloss.NewStyle().
		Background(styles.Primary).
		Foreground(styles.TextInverse).
		Bold(true).
		Padding(0, 1)

	sepStyle := lipgloss.NewStyle().
		Background(bg).
		Foreground(styles.TextMuted)

	infoStyle := lipgloss.NewStyle().
		Background(bg).
		Foreground(styles.TextSecondary)

	timeStyle := lipgloss.NewStyle().
		Background(bg).
		Foreground(styles.Info).
		Bold(true)

	title := titleStyle.Render("HOMEDASH")
	sep := sepStyle.Render("  │  ")

	currentTime := time.Now()
	if testMode {
		// Use fixed time in test mode
		currentTime = time.Date(2026, 3, 7, 12, 0, 0, 0, time.UTC)
	}
	clock := timeStyle.Render(currentTime.Format("15:04:05"))
	clockWidth := lipgloss.Width(clock)
	clockReserved := clockWidth + 2 // minimum gap before clock

	// Build left side incrementally — stop adding when it would overflow
	left := title
	extras := []string{
		infoStyle.Render(data.Hostname),
		infoStyle.Render(fmt.Sprintf("up %s", formatUptime(data.Uptime))),
		infoStyle.Render(fmt.Sprintf("%d CPU / %s RAM",
			data.CPUCount,
			collector.FormatBytes(data.MemTotal))),
	}
	for _, extra := range extras {
		candidate := left + sep + extra
		if lipgloss.Width(candidate)+clockReserved > width {
			break
		}
		left = candidate
	}

	// Weather: try full variant, then compact, then omit
	for _, variant := range headerWeatherVariants(weather, weatherErr, weatherRetries, bg) {
		candidate := left + sep + variant
		if lipgloss.Width(candidate)+clockReserved <= width {
			left = candidate
			break
		}
	}

	// Right-align the clock
	leftWidth := lipgloss.Width(left)
	gap := width - leftWidth - clockWidth
	if gap < 1 {
		// Terminal too narrow even for title + clock — drop the clock
		row := lipgloss.NewStyle().Inline(true).MaxWidth(width).Render(left)
		return base.Render(row)
	}

	spacer := lipgloss.NewStyle().
		Background(bg).
		Width(gap).
		Render("")
	row := lipgloss.NewStyle().Inline(true).MaxWidth(width).Render(left + spacer + clock)

	return base.Render(row)
}

// headerWeatherVariants returns weather display variants from longest to shortest.
func headerWeatherVariants(weather collector.WeatherData, weatherErr error, weatherRetries int, bg color.Color) []string {
	stale := weatherErr != nil && weatherRetries == 0

	// No data yet
	if weather.TempC == "" {
		style := lipgloss.NewStyle().Background(bg).Foreground(styles.TextMuted)
		if stale {
			style = lipgloss.NewStyle().Background(bg).Foreground(styles.Warning)
		}
		return []string{style.Render("☁ --")}
	}

	// Have data — choose color based on staleness
	tempColor := styles.Info
	if stale {
		tempColor = styles.Warning
	}

	tempStyle := lipgloss.NewStyle().Background(bg).Foreground(tempColor).Bold(true)
	condStyle := lipgloss.NewStyle().Background(bg).Foreground(styles.TextSecondary)

	compact := weather.Icon + " " + tempStyle.Render(weather.TempC+"°C")
	full := compact + " " + condStyle.Render(weather.Condition) + " " + condStyle.Render("💧"+weather.Humidity)

	return []string{full, compact}
}

func formatUptime(d time.Duration) string {
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	mins := int(d.Minutes()) % 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh", days, hours)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, mins)
	}
	return fmt.Sprintf("%dm", mins)
}

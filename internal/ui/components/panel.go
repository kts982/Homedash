package components

import (
	"charm.land/lipgloss/v2"
	"github.com/kostas/homedash/internal/ui/styles"
)

// Panel renders content inside a bordered box with a title.
func Panel(title, content string, width, height int, focused bool) string {
	borderColor := styles.Border
	titleColor := styles.TextMuted
	if focused {
		borderColor = styles.BorderFocus
		titleColor = styles.Primary
	}

	titleStyle := lipgloss.NewStyle().
		Foreground(titleColor).
		Bold(true)

	s := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Foreground(styles.TextPrimary).
		Width(width - 2). // account for border
		Height(height - 2).
		Padding(0, 1)

	titleStr := titleStyle.Render(" " + title + " ")

	return s.BorderTop(true).
		BorderBottom(true).
		BorderLeft(true).
		BorderRight(true).
		Render(titleStr + "\n" + content)
}

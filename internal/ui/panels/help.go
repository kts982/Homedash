package panels

import (
	"charm.land/lipgloss/v2"
	"github.com/kostas/homedash/internal/ui/styles"
)

type KeyBinding struct {
	Key  string
	Desc string
}

var DefaultBindings = []KeyBinding{
	{"q", "quit"},
	{"tab", "focus"},
	{"j/k", "select"},
	{"/", "filter"},
	{"enter", "open/toggle"},
	{"space", "actions"},
	{"s/S/R", "stop/start/restart"},
	{"r", "refresh"},
}

func RenderHelp(bindings []KeyBinding, refreshing bool, width int) string {
	keyStyle := lipgloss.NewStyle().
		Foreground(styles.Primary).
		Bold(true)
	descStyle := lipgloss.NewStyle().
		Foreground(styles.TextMuted)
	sepStyle := lipgloss.NewStyle().
		Foreground(styles.TextMuted)

	row := ""
	for i, b := range bindings {
		if i > 0 {
			row += sepStyle.Render("  ")
		}
		row += keyStyle.Render(b.Key) + " " + descStyle.Render(b.Desc)
	}

	if refreshing {
		indicator := lipgloss.NewStyle().
			Foreground(styles.Warning).
			Bold(true).
			Render("  Refreshing...")
		row += indicator
	}

	return lipgloss.NewStyle().
		Background(styles.BgPanel).
		Foreground(styles.TextPrimary).
		Width(width).
		Padding(0, 1).
		Render(row)
}

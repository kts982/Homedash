package panels

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/kts982/homedash/internal/ui/styles"
)

type KeyBinding struct {
	Key     string
	Desc    string
	Compact string
}

var DefaultBindings = []KeyBinding{
	{"q", "quit", "quit"},
	{"tab", "focus", "focus"},
	{"j/k", "select", "nav"},
	{"PgUp/Dn", "page", "page"},
	{"Home/End", "jump", "jump"},
	{"a", "alerts", "alerts"},
	{"O", "options", "opts"},
	{"/", "filter", "find"},
	{"enter", "open/toggle", "open"},
	{"l", "logs", "logs"},
	{"o", "sort", "sort"},
	{"space", "actions", "menu"},
	{"s/S/R", "stop/start/restart", "life"},
	{"r", "refresh", "sync"},
}

func RenderHelp(bindings []KeyBinding, refreshing, paused bool, width int, status string) string {
	keyStyle := lipgloss.NewStyle().
		Foreground(styles.Primary).
		Bold(true)
	descStyle := lipgloss.NewStyle().
		Foreground(styles.TextMuted)
	sepStyle := lipgloss.NewStyle().
		Foreground(styles.TextMuted)

	badgeText := renderHelpBadges(refreshing, paused, status)
	contentWidth := max(0, width-2)
	bindingsWidth := contentWidth
	if badgeText != "" {
		bindingsWidth -= lipgloss.Width(badgeText) + 1
		if bindingsWidth < 0 {
			bindingsWidth = 0
		}
	}

	bindingsText := fitHelpBindings(bindings, bindingsWidth, keyStyle, descStyle, sepStyle)
	row := bindingsText
	if badgeText != "" {
		if row == "" {
			row = badgeText
		} else {
			gap := max(1, contentWidth-lipgloss.Width(bindingsText)-lipgloss.Width(badgeText))
			row += strings.Repeat(" ", gap) + badgeText
		}
	}

	content := lipgloss.NewStyle().
		Inline(true).
		MaxWidth(contentWidth).
		Render(row)

	return lipgloss.NewStyle().
		Background(styles.BgPanel).
		Foreground(styles.TextPrimary).
		Width(width).
		Padding(0, 1).
		Render(content)
}

func fitHelpBindings(bindings []KeyBinding, width int, keyStyle, descStyle, sepStyle lipgloss.Style) string {
	if width <= 0 || len(bindings) == 0 {
		return ""
	}
	if full := renderHelpBindings(bindings, false, keyStyle, descStyle, sepStyle); lipgloss.Width(full) <= width {
		return full
	}
	if compact := renderHelpBindings(bindings, true, keyStyle, descStyle, sepStyle); lipgloss.Width(compact) <= width {
		return compact
	}
	return renderHelpBindingsFitted(bindings, width, keyStyle, descStyle, sepStyle)
}

func renderHelpBindings(bindings []KeyBinding, compact bool, keyStyle, descStyle, sepStyle lipgloss.Style) string {
	sep := helpBindingSeparator(compact, sepStyle)
	parts := make([]string, 0, len(bindings))
	for _, binding := range bindings {
		parts = append(parts, renderHelpBinding(binding, compact, keyStyle, descStyle))
	}
	return strings.Join(parts, sep)
}

func renderHelpBindingsFitted(bindings []KeyBinding, width int, keyStyle, descStyle, sepStyle lipgloss.Style) string {
	sep := helpBindingSeparator(true, sepStyle)
	ellipsis := descStyle.Render("…")
	parts := make([]string, 0, len(bindings))
	currentWidth := 0

	for i, binding := range bindings {
		part := renderHelpBinding(binding, true, keyStyle, descStyle)
		partWidth := lipgloss.Width(part)
		nextWidth := currentWidth + partWidth
		if len(parts) > 0 {
			nextWidth += lipgloss.Width(sep)
		}
		if nextWidth > width {
			if len(parts) == 0 {
				return lipgloss.NewStyle().Inline(true).MaxWidth(width).Render(part)
			}
			if i < len(bindings) && currentWidth+lipgloss.Width(sep)+lipgloss.Width(ellipsis) <= width {
				return strings.Join(parts, sep) + sep + ellipsis
			}
			return strings.Join(parts, sep)
		}
		parts = append(parts, part)
		currentWidth = nextWidth
	}

	return strings.Join(parts, sep)
}

func renderHelpBinding(binding KeyBinding, compact bool, keyStyle, descStyle lipgloss.Style) string {
	desc := binding.Desc
	if compact && binding.Compact != "" {
		desc = binding.Compact
	}
	return keyStyle.Render(binding.Key) + " " + descStyle.Render(desc)
}

func helpBindingSeparator(compact bool, sepStyle lipgloss.Style) string {
	if compact {
		return sepStyle.Render(" ")
	}
	return sepStyle.Render("  ")
}

func renderHelpBadges(refreshing, paused bool, status string) string {
	var badges []string
	if status != "" {
		badges = append(badges, helpBadge(status, styles.Warning, styles.TextInverse))
	}
	if refreshing {
		badges = append(badges, helpBadge("refreshing", styles.Info, styles.TextInverse))
	} else if paused {
		badges = append(badges, helpBadge("paused", styles.BgFocus, styles.TextPrimary))
	}
	return strings.Join(badges, " ")
}

func helpBadge(label string, bg, fg color.Color) string {
	return lipgloss.NewStyle().
		Background(bg).
		Foreground(fg).
		Bold(true).
		Padding(0, 1).
		Render(label)
}

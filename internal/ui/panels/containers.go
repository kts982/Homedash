package panels

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
	"github.com/kostas/homedash/internal/collector"
	"github.com/kostas/homedash/internal/ui/components"
	"github.com/kostas/homedash/internal/ui/styles"
)

type ContainerDisplayItem struct {
	IsGroup        bool
	StackName      string
	ContainerCount int
	RunningCount   int
	Collapsed      bool
	Container      *collector.Container
}

func RenderContainers(items []ContainerDisplayItem, running, total, scrollOffset, selectedIndex, visibleRows, width int, focused bool, searchInput textinput.Model, filtering bool) string {
	innerWidth := width - 4

	// Adaptive columns based on available width
	showStack := innerWidth >= 75
	showHealth := innerWidth >= 85

	statusW := 10
	cpuW := 7
	memW := 10

	// Fixed columns total
	fixed := statusW + cpuW + memW + 3 // +3 for spaces between them
	if showStack {
		fixed += 14 + 1 // stackW + space
	}
	if showHealth {
		fixed += 10 + 1 // healthW + space
	}

	nameW := innerWidth - fixed - 1 // -1 trailing space
	if nameW < 12 {
		nameW = 12
	}
	stackW := 14
	healthW := 10

	headerStyle := lipgloss.NewStyle().
		Foreground(styles.TextMuted).
		Bold(true)

	// Build header dynamically
	header := fmt.Sprintf("%-*s", nameW, "NAME")
	if showStack {
		header += fmt.Sprintf(" %-*s", stackW, "STACK")
	}
	header += fmt.Sprintf(" %-*s", statusW, "STATUS")
	if showHealth {
		header += fmt.Sprintf(" %-*s", healthW, "HEALTH")
	}
	header += fmt.Sprintf(" %*s %*s", cpuW, "CPU%", memW, "MEMORY")
	headerLine := headerStyle.Render(truncate(header, innerWidth))

	// Container rows
	var rows []string
	end := scrollOffset + visibleRows
	if end > len(items) {
		end = len(items)
	}

	for i := scrollOffset; i < end; i++ {
		item := items[i]
		var row string
		if item.IsGroup {
			row = formatGroupHeader(item.StackName, item.RunningCount, item.ContainerCount, item.Collapsed, innerWidth)
		} else if item.Container != nil {
			row = "  " + formatContainerRow(*item.Container, nameW-2, stackW, statusW, healthW, cpuW, memW, innerWidth-2, showStack, showHealth)
		}
		if focused && i == selectedIndex {
			row = lipgloss.NewStyle().Background(styles.BgFocus).Width(innerWidth).Render(row)
		}
		rows = append(rows, row)
	}

	// Status line
	refreshNote := lipgloss.NewStyle().Foreground(styles.TextMuted).Render("[5s refresh]")
	summary := lipgloss.NewStyle().Foreground(styles.TextSecondary).Render(
		fmt.Sprintf("(%d/%d running)", running, total))

	// 12 = visual width of " CONTAINERS " added by Panel(" "+title+" ")
	titleMaxWidth := innerWidth - 12
	spacer := max(0, titleMaxWidth-lipgloss.Width(summary)-lipgloss.Width(refreshNote)-1)
	titleExtra := " " + summary + strings.Repeat(" ", spacer) + refreshNote

	content := headerLine + "\n" + strings.Join(rows, "\n")

	// Prepend search input if filtering OR if there is a filter active
	if filtering || searchInput.Value() != "" {
		filterLine := searchInput.View()
		if !filtering {
			// If not active, but filter exists, show it grayed out
			filterLine = lipgloss.NewStyle().Foreground(styles.TextMuted).Render(" / " + searchInput.Value())
		}
		content = filterLine + "\n" + content
	}

	return components.Panel("CONTAINERS"+titleExtra, content, width, visibleRows+4, focused)
}

func formatGroupHeader(name string, running, total int, collapsed bool, width int) string {
	arrow := "▼"
	if collapsed {
		arrow = "▶"
	}
	label := fmt.Sprintf("%s %s (%d/%d running)", arrow, name, running, total)
	return lipgloss.NewStyle().
		Foreground(styles.Secondary).
		Bold(true).
		Render(label)
}

func formatContainerRow(c collector.Container, nameW, stackW, statusW, healthW, cpuW, memW, maxWidth int, showStack, showHealth bool) string {
	stateStyle := lipgloss.NewStyle().Foreground(styles.ContainerStateColor(c.State))

	healthStyle := lipgloss.NewStyle().Foreground(styles.TextMuted)
	switch c.Health {
	case "healthy":
		healthStyle = lipgloss.NewStyle().Foreground(styles.Success)
	case "unhealthy":
		healthStyle = lipgloss.NewStyle().Foreground(styles.Error)
	case "starting":
		healthStyle = lipgloss.NewStyle().Foreground(styles.Warning)
	}

	nameStyle := lipgloss.NewStyle().Foreground(styles.TextPrimary)
	stackStyle := lipgloss.NewStyle().Foreground(styles.Secondary)
	cpuStyle := lipgloss.NewStyle().Foreground(styles.TextSecondary)
	memStyle := lipgloss.NewStyle().Foreground(styles.TextSecondary)

	cpuStr := ""
	memStr := ""
	if c.State == "running" {
		cpuStr = fmt.Sprintf("%.1f%%", c.CPUPerc)
		memStr = collector.FormatBytes(c.MemUsed)
	}

	row := nameStyle.Render(pad(c.Name, nameW))
	if showStack {
		row += " " + stackStyle.Render(pad(c.Stack, stackW))
	}
	row += " " + stateStyle.Render(pad(c.State, statusW))
	if showHealth {
		row += " " + healthStyle.Render(pad(c.Health, healthW))
	}
	row += " " + cpuStyle.Render(lpad(cpuStr, cpuW))
	row += " " + memStyle.Render(lpad(memStr, memW))

	return truncate(row, maxWidth)
}

func pad(s string, w int) string {
	if w <= 0 {
		return ""
	}
	clamped := lipgloss.NewStyle().Inline(true).MaxWidth(w).Render(s)
	if padW := w - lipgloss.Width(clamped); padW > 0 {
		return clamped + strings.Repeat(" ", padW)
	}
	return clamped
}

func lpad(s string, w int) string {
	if w <= 0 {
		return ""
	}
	clamped := lipgloss.NewStyle().Inline(true).MaxWidth(w).Render(s)
	if padW := w - lipgloss.Width(clamped); padW > 0 {
		return strings.Repeat(" ", padW) + clamped
	}
	return clamped
}

func truncate(s string, maxWidth int) string {
	if lipgloss.Width(s) <= maxWidth {
		return s
	}
	return lipgloss.NewStyle().Inline(true).MaxWidth(maxWidth).Render(s)
}

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

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func RenderDetail(
	c *collector.Container,
	meta *collector.ContainerDetail,
	logs []string, logsErr error,
	confirmAction, actionResult string,
	scrollOffset, width, height int,
	logFollowing bool,
) string {
	if c == nil {
		return "No container selected"
	}

	innerWidth := width - 4 // Panel content width (border + padding)
	titleAvail := width - 6 // Panel adds " " + title + " "

	// ── INFO PANEL ──────────────────────────────────────────
	stateColor := styles.ContainerStateColor(c.State)

	nameStyled := lipgloss.NewStyle().Foreground(styles.TextPrimary).Render(c.Name)
	stateStyled := lipgloss.NewStyle().Foreground(stateColor).Render(c.State)
	infoTitleLeft := nameStyled + "  " + stateStyled

	if c.State == "running" {
		cpuStr := fmt.Sprintf("%.1f%%", c.CPUPerc)
		memStr := collector.FormatBytes(c.MemUsed)
		statsStyled := lipgloss.NewStyle().Foreground(styles.TextSecondary).
			Render("CPU " + cpuStr + "  Mem " + memStr)
		if lipgloss.Width(infoTitleLeft)+2+lipgloss.Width(statsStyled) <= titleAvail {
			infoTitleLeft += "  " + statsStyled
		}
	}

	infoTitle := lipgloss.NewStyle().Inline(true).MaxWidth(titleAvail).Render(infoTitleLeft)

	// Info content: 4 lines of key-value pairs
	labelStyle := lipgloss.NewStyle().Foreground(styles.TextMuted).Width(8)
	valueStyle := lipgloss.NewStyle().Foreground(styles.TextPrimary)

	healthColor := styles.TextMuted
	healthText := c.Health
	if healthText == "" {
		healthText = "-"
	}
	switch c.Health {
	case "healthy":
		healthColor = styles.Success
	case "unhealthy":
		healthColor = styles.Error
	case "starting":
		healthColor = styles.Warning
	}
	healthStyled := lipgloss.NewStyle().Foreground(healthColor).Render(healthText)

	stackVal := c.Stack
	if stackVal == "" {
		stackVal = "-"
	}

	// Two-column row for Stack + Health
	stackPart := labelStyle.Render("Stack") + " " + valueStyle.Render(stackVal)
	healthLabel := lipgloss.NewStyle().Foreground(styles.TextMuted).Render("Health")
	stackPartW := lipgloss.Width(stackPart)
	midCol := innerWidth / 2
	if midCol < 25 {
		midCol = 25
	}
	stackHealthGap := midCol - stackPartW
	if stackHealthGap < 2 {
		stackHealthGap = 2
	}

	portsStr := collector.FormatPorts(c.Ports)

	infoLines := []string{
		labelStyle.Render("Image") + " " + valueStyle.Render(
			lipgloss.NewStyle().Inline(true).MaxWidth(innerWidth-9).Render(c.Image)),
		stackPart + strings.Repeat(" ", stackHealthGap) + healthLabel + " " + healthStyled,
		labelStyle.Render("Ports") + " " + valueStyle.Render(
			lipgloss.NewStyle().Inline(true).MaxWidth(innerWidth-9).Render(portsStr)),
		labelStyle.Render("ID") + " " + valueStyle.Render(c.ID[:12]),
	}

	// Network stats (for running containers with data).
	if c.State == "running" {
		netStr := collector.FormatBytes(c.NetRx) + " rx / " + collector.FormatBytes(c.NetTx) + " tx"
		infoLines = append(infoLines,
			labelStyle.Render("Net")+" "+valueStyle.Render(netStr))
	}

	if meta != nil {
		// Volumes line
		if len(meta.Mounts) > 0 {
			var volParts []string
			for _, mt := range meta.Mounts {
				src := mt.Source
				if len(src) > 25 {
					src = "..." + src[len(src)-22:]
				}
				volParts = append(volParts, src+" → "+mt.Destination)
			}
			volStr := strings.Join(volParts, ", ")
			infoLines = append(infoLines,
				labelStyle.Render("Vols")+" "+valueStyle.Render(
					lipgloss.NewStyle().Inline(true).MaxWidth(innerWidth-9).Render(volStr)))
		}

		// Compose labels line
		var labelParts []string
		for _, key := range []string{
			"com.docker.compose.project",
			"com.docker.compose.service",
			"com.docker.compose.version",
		} {
			if val, ok := meta.Labels[key]; ok {
				short := key[len("com.docker.compose."):]
				labelParts = append(labelParts, short+"="+val)
			}
		}
		if len(labelParts) > 0 {
			labelsStr := strings.Join(labelParts, " ")
			infoLines = append(infoLines,
				labelStyle.Render("Labels")+" "+valueStyle.Render(
					lipgloss.NewStyle().Inline(true).MaxWidth(innerWidth-9).Render(labelsStr)))
		}
	}

	infoPanelHeight := len(infoLines) + 3 // border(2) + title(1)
	infoContent := strings.Join(infoLines, "\n")
	infoPanel := components.Panel(infoTitle, infoContent, width, infoPanelHeight, false)

	// ── LOG PANEL ───────────────────────────────────────────
	logPanelHeight := height - infoPanelHeight - 1 // -1 for action bar
	if logPanelHeight < 5 {
		logPanelHeight = 5
	}
	logContentHeight := logPanelHeight - 3 // border(2) + title(1)
	if logContentHeight < 1 {
		logContentHeight = 1
	}

	// Build log lines
	var logLines []string
	if logs == nil && logsErr == nil {
		logLines = []string{lipgloss.NewStyle().Foreground(styles.TextMuted).Render("Loading logs...")}
	} else if logsErr != nil {
		logLines = []string{lipgloss.NewStyle().Foreground(styles.Error).Render(fmt.Sprintf("Error: %v", logsErr))}
	} else if len(logs) == 0 {
		logLines = []string{lipgloss.NewStyle().Foreground(styles.TextMuted).Render("No logs available")}
	} else {
		for _, line := range logs {
			logLines = append(logLines, formatLogLine(line, innerWidth))
		}
	}

	// Clamp scroll offset
	maxScroll := len(logLines) - logContentHeight
	if maxScroll < 0 {
		maxScroll = 0
	}
	if scrollOffset > maxScroll {
		scrollOffset = maxScroll
	}
	if scrollOffset < 0 {
		scrollOffset = 0
	}

	// Log title with optional scroll position
	logTitleLeft := "LOGS (last 50 lines)"
	if logFollowing {
		logTitleLeft = lipgloss.NewStyle().Foreground(styles.Success).Render("LOGS (following...)")
	}
	if len(logLines) > logContentHeight {
		endPos := min(scrollOffset+logContentHeight, len(logLines))
		scrollText := fmt.Sprintf("(%d-%d/%d)", scrollOffset+1, endPos, len(logLines))
		scrollStyled := lipgloss.NewStyle().Foreground(styles.TextSecondary).Render(scrollText)
		if lipgloss.Width(logTitleLeft)+2+lipgloss.Width(scrollStyled) <= titleAvail {
			logTitleLeft += "  " + scrollStyled
		}
	}

	// Slice visible log lines
	endIdx := scrollOffset + logContentHeight
	if endIdx > len(logLines) {
		endIdx = len(logLines)
	}
	visible := logLines[scrollOffset:endIdx]
	for len(visible) < logContentHeight {
		visible = append(visible, "")
	}

	logContent := strings.Join(visible, "\n")
	logPanel := components.Panel(logTitleLeft, logContent, width, logPanelHeight, true)

	// ── ACTION BAR ──────────────────────────────────────────
	actionBar := renderDetailActionBar(c, confirmAction, actionResult, width, logFollowing)

	return lipgloss.JoinVertical(lipgloss.Left, infoPanel, logPanel, actionBar)
}

func renderDetailActionBar(c *collector.Container, confirmAction, actionResult string, width int, logFollowing bool) string {
	keyStyle := lipgloss.NewStyle().Foreground(styles.Primary).Bold(true)
	descStyle := lipgloss.NewStyle().Foreground(styles.TextMuted)

	var content string
	if confirmAction != "" {
		confirmStyle := lipgloss.NewStyle().Foreground(styles.Warning).Bold(true)
		content = confirmStyle.Render(
			fmt.Sprintf("%s %s?", capitalize(confirmAction), c.Name)) +
			"  " + keyStyle.Render("y") + descStyle.Render(" confirm") +
			"  " + keyStyle.Render("n/esc") + descStyle.Render(" cancel")
	} else if actionResult != "" {
		resultColor := styles.Success
		if strings.HasPrefix(actionResult, "Error") {
			resultColor = styles.Error
		}
		content = lipgloss.NewStyle().Foreground(resultColor).Bold(true).Render(actionResult)
	} else {
		followLabel := "follow"
		if logFollowing {
			followLabel = "unfollow"
		}
		parts := []string{
			keyStyle.Render("esc") + descStyle.Render(" back"),
			keyStyle.Render("j/k") + descStyle.Render(" scroll"),
			keyStyle.Render("g/G") + descStyle.Render(" top/end"),
			keyStyle.Render("f") + descStyle.Render(" "+followLabel),
			keyStyle.Render("l") + descStyle.Render(" refresh"),
		}
		if c.State == "running" {
			parts = append(parts,
				keyStyle.Render("s")+descStyle.Render(" stop"),
				keyStyle.Render("R")+descStyle.Render(" restart"))
		} else {
			parts = append(parts, keyStyle.Render("S")+descStyle.Render(" start"))
		}
		content = strings.Join(parts, "   ")
	}

	return lipgloss.NewStyle().
		Background(styles.BgPanel).
		Foreground(styles.TextPrimary).
		Width(width).
		Padding(0, 1).
		Render(content)
}

// formatLogLine parses Docker timestamps and colorizes log levels.
func formatLogLine(line string, maxWidth int) string {
	timeStyle := lipgloss.NewStyle().Foreground(styles.TextMuted)
	msgStyle := lipgloss.NewStyle().Foreground(styles.TextSecondary)

	var ts, msg string

	// Docker timestamps: 2024-03-03T12:00:01.123456789Z <message>
	if len(line) > 30 && line[4] == '-' && line[7] == '-' && line[10] == 'T' {
		if spaceIdx := strings.IndexByte(line, ' '); spaceIdx > 19 {
			if t, err := time.Parse(time.RFC3339Nano, line[:spaceIdx]); err == nil {
				ts = t.Format("15:04:05")
				msg = line[spaceIdx+1:]
			}
		}
	}

	if ts == "" {
		// No Docker timestamp — use the raw line
		msg = line
	}

	// Detect log level and pick color for the message
	levelColor := detectLogLevel(msg)
	if levelColor != "" {
		msgStyle = lipgloss.NewStyle().Foreground(levelColor)
	}

	var rendered string
	if ts != "" {
		rendered = timeStyle.Render(ts) + " " + msgStyle.Render(msg)
	} else {
		rendered = msgStyle.Render(msg)
	}

	return lipgloss.NewStyle().Inline(true).MaxWidth(maxWidth).Render(rendered)
}

// detectLogLevel checks the first portion of a log message for level keywords.
func detectLogLevel(msg string) lipgloss.Color {
	check := msg
	if len(check) > 50 {
		check = check[:50]
	}
	upper := strings.ToUpper(check)

	for _, kw := range []string{"ERROR", "ERR ", "ERR]", "FATAL", "PANIC", "CRIT"} {
		if strings.Contains(upper, kw) {
			return styles.Error
		}
	}
	for _, kw := range []string{"WARN", "WRN ", "WRN]"} {
		if strings.Contains(upper, kw) {
			return styles.Warning
		}
	}

	return ""
}

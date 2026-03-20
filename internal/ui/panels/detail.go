package panels

import (
	"fmt"
	"image/color"
	"sort"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/kts982/homedash/internal/collector"
	"github.com/kts982/homedash/internal/ui/components"
	"github.com/kts982/homedash/internal/ui/styles"
)

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

const detailLabelWidth = 8

// DetailInfoPanelHeight returns the rendered info panel height for the detail view.
func DetailInfoPanelHeight(c *collector.Container, meta *collector.ContainerDetail, hostname string, width int) int {
	if c == nil {
		return 7 // 4 baseline rows + border/title chrome
	}
	innerWidth := width - 4
	if innerWidth < 1 {
		innerWidth = 1
	}
	return len(detailInfoLines(c, meta, hostname, innerWidth)) + 3
}

func detailInfoLines(c *collector.Container, meta *collector.ContainerDetail, hostname string, innerWidth int) []string {
	labelStyle := lipgloss.NewStyle().Foreground(styles.TextMuted).Width(detailLabelWidth)
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

	infoLines := []string{
		formatDetailLine(labelStyle, valueStyle, "Image", c.Image, innerWidth),
		formatStackHealthLine(labelStyle, valueStyle, stackVal, healthStyled, innerWidth),
		formatDetailLine(labelStyle, valueStyle, "Ports", collector.FormatPorts(c.Ports), innerWidth),
	}

	if meta != nil {
		if meta.RestartPolicy != "" && meta.RestartPolicy != "-" {
			infoLines = append(infoLines,
				formatDetailLine(labelStyle, valueStyle, "Policy", meta.RestartPolicy, innerWidth))
		}
		if timeLine := detailTimeLine(meta); timeLine != "" {
			infoLines = append(infoLines,
				formatDetailLine(labelStyle, valueStyle, "Time", timeLine, innerWidth))
		}
		if meta.Command != "" && meta.Command != "-" {
			infoLines = append(infoLines,
				formatDetailLine(labelStyle, valueStyle, "Cmd", meta.Command, innerWidth))
		}
		if addrLine := detailAddressLine(meta.Networks); addrLine != "" {
			infoLines = append(infoLines,
				formatDetailLine(labelStyle, valueStyle, "Network", addrLine, innerWidth))
		}
		if publishLine := detailPublishedPortLine(meta.PublishedPorts, innerWidth); publishLine != "" {
			infoLines = append(infoLines,
				formatDetailLine(labelStyle, valueStyle, "Publish", publishLine, innerWidth))
		}
		if urlLine := detailURLLine(meta.PublishedPorts, hostname, innerWidth); urlLine != "" {
			infoLines = append(infoLines,
				formatDetailLine(labelStyle, valueStyle, "URLs", urlLine, innerWidth))
		}
	}

	containerID := c.ID
	if len(containerID) > 12 {
		containerID = containerID[:12]
	}
	infoLines = append(infoLines, formatDetailLine(labelStyle, valueStyle, "ID", containerID, innerWidth))

	if c.State == "running" {
		netStr := collector.FormatBytes(c.NetRx) + " rx / " + collector.FormatBytes(c.NetTx) + " tx"
		infoLines = append(infoLines, formatDetailLine(labelStyle, valueStyle, "Net", netStr, innerWidth))
	}

	if meta != nil {
		if mountsLine := detailMountLine(meta.Mounts, innerWidth); mountsLine != "" {
			infoLines = append(infoLines,
				formatDetailLine(labelStyle, valueStyle, "Mounts", mountsLine, innerWidth))
		}
		if composeLine := detailComposeLine(meta.Labels); composeLine != "" {
			infoLines = append(infoLines,
				formatDetailLine(labelStyle, valueStyle, "Compose", composeLine, innerWidth))
		}
		if labelsLine := detailLabelLine(meta.Labels, innerWidth); labelsLine != "" {
			infoLines = append(infoLines,
				formatDetailLine(labelStyle, valueStyle, "Labels", labelsLine, innerWidth))
		}
	}

	return infoLines
}

func formatDetailLine(labelStyle, valueStyle lipgloss.Style, label, value string, innerWidth int) string {
	if value == "" {
		value = "-"
	}
	valueWidth := detailValueWidth(innerWidth)
	return labelStyle.Render(label) + " " +
		valueStyle.Render(lipgloss.NewStyle().Inline(true).MaxWidth(valueWidth).Render(value))
}

func detailValueWidth(innerWidth int) int {
	valueWidth := innerWidth - detailLabelWidth - 1
	if valueWidth < 1 {
		valueWidth = 1
	}
	return valueWidth
}

func formatStackHealthLine(labelStyle, valueStyle lipgloss.Style, stackVal, healthStyled string, innerWidth int) string {
	stackPart := labelStyle.Render("Stack") + " " + valueStyle.Render(
		lipgloss.NewStyle().Inline(true).MaxWidth(max(1, innerWidth/2)).Render(stackVal))
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
	line := stackPart + strings.Repeat(" ", stackHealthGap) + healthLabel + " " + healthStyled
	return lipgloss.NewStyle().Inline(true).MaxWidth(innerWidth).Render(line)
}

func detailComposeLine(labels map[string]string) string {
	if len(labels) == 0 {
		return ""
	}
	project := labels["com.docker.compose.project"]
	service := labels["com.docker.compose.service"]
	version := labels["com.docker.compose.version"]

	var parts []string
	if project != "" || service != "" {
		switch {
		case project != "" && service != "":
			parts = append(parts, project+"/"+service)
		case project != "":
			parts = append(parts, project)
		case service != "":
			parts = append(parts, service)
		}
	}
	if version != "" {
		parts = append(parts, "v"+version)
	}
	return strings.Join(parts, "  ")
}

func detailMountLine(mounts []collector.Mount, innerWidth int) string {
	parts := make([]string, 0, len(mounts))
	for _, mt := range mounts {
		src := strings.TrimSpace(mt.Source)
		if len(src) > 25 {
			src = "..." + src[len(src)-22:]
		}
		if src == "" {
			src = "-"
		}

		dest := strings.TrimSpace(mt.Destination)
		if dest == "" {
			dest = "-"
		}

		prefix := strings.TrimSpace(mt.Type)
		if prefix == "" {
			prefix = "mount"
		}
		mode := strings.TrimSpace(mt.Mode)
		if mode != "" && mode != "-" {
			prefix += ":" + mode
		}

		parts = append(parts, prefix+" "+src+" → "+dest)
	}
	return summarizeDetailItems(parts, ", ", detailValueWidth(innerWidth))
}

func detailLabelLine(labels map[string]string, innerWidth int) string {
	if len(labels) == 0 {
		return ""
	}

	keys := make([]string, 0, len(labels))
	for key := range labels {
		if strings.HasPrefix(key, "com.docker.compose.") {
			continue
		}
		keys = append(keys, key)
	}
	if len(keys) == 0 {
		return ""
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		value := strings.TrimSpace(labels[key])
		if value == "" {
			parts = append(parts, key)
			continue
		}
		parts = append(parts, key+"="+value)
	}
	return summarizeDetailItems(parts, ", ", detailValueWidth(innerWidth))
}

func detailAddressLine(networks []collector.NetworkAddress) string {
	var parts []string
	for _, network := range networks {
		var addrs []string
		if network.IPv4 != "" {
			addrs = append(addrs, network.IPv4)
		}
		if network.IPv6 != "" {
			addrs = append(addrs, network.IPv6)
		}
		if len(addrs) == 0 {
			continue
		}
		parts = append(parts, network.Name+" "+strings.Join(addrs, ","))
	}
	return strings.Join(parts, "  ")
}

func detailTimeLine(meta *collector.ContainerDetail) string {
	var parts []string
	if !meta.StartedAt.IsZero() {
		parts = append(parts, "start "+formatDetailTimestamp(meta.StartedAt))
	}
	if !meta.CreatedAt.IsZero() {
		parts = append(parts, "create "+formatDetailTimestamp(meta.CreatedAt))
	}
	return strings.Join(parts, "  ")
}

func formatDetailTimestamp(ts time.Time) string {
	return ts.UTC().Format("2006-01-02 15:04Z")
}

func detailPublishedPortLine(published []collector.PublishedPort, innerWidth int) string {
	value := collector.FormatPublishedPorts(published)
	if value == "" || value == "-" {
		return ""
	}
	return summarizeDetailItems(strings.Split(value, ", "), ", ", detailValueWidth(innerWidth))
}

func detailURLLine(published []collector.PublishedPort, hostname string, innerWidth int) string {
	urls := inferPublishedURLs(published, hostname)
	if len(urls) == 0 {
		return ""
	}
	return summarizeDetailItems(urls, "  ", detailValueWidth(innerWidth))
}

func inferPublishedURLs(published []collector.PublishedPort, hostname string) []string {
	var urls []string
	seen := make(map[string]struct{})
	hostname = strings.TrimSpace(hostname)

	for _, binding := range published {
		scheme := inferPublishedURLScheme(binding)
		if scheme == "" || binding.HostPort <= 0 {
			continue
		}
		for _, host := range detailURLHosts(binding.HostIP, hostname) {
			if host == "" {
				continue
			}
			url := scheme + "://" + formatURLHost(host, binding.HostPort, scheme)
			if _, ok := seen[url]; ok {
				continue
			}
			seen[url] = struct{}{}
			urls = append(urls, url)
		}
	}

	return urls
}

func inferPublishedURLScheme(binding collector.PublishedPort) string {
	if binding.Type != "tcp" {
		return ""
	}
	switch {
	case binding.ContainerPort == 443 || binding.HostPort == 443 || binding.HostPort == 8443:
		return "https"
	case isLikelyHTTPPort(binding.ContainerPort) || isLikelyHTTPPort(binding.HostPort):
		return "http"
	default:
		return ""
	}
}

func isLikelyHTTPPort(port int) bool {
	switch port {
	case 80, 81, 3000, 4000, 5000, 5173, 8000, 8080, 8081, 8088, 8090, 9000:
		return true
	default:
		return false
	}
}

func detailURLHosts(hostIP, hostname string) []string {
	hostIP = strings.TrimSpace(hostIP)
	switch hostIP {
	case "", "0.0.0.0", "::":
		hosts := []string{"localhost"}
		if hostname != "" && hostname != "localhost" {
			hosts = append(hosts, hostname)
		}
		return hosts
	case "127.0.0.1", "::1":
		return []string{"localhost"}
	default:
		return []string{hostIP}
	}
}

func formatURLHost(host string, port int, scheme string) string {
	if strings.Contains(host, ":") && !strings.HasPrefix(host, "[") {
		host = "[" + host + "]"
	}
	if (scheme == "http" && port == 80) || (scheme == "https" && port == 443) {
		return host
	}
	return fmt.Sprintf("%s:%d", host, port)
}

func summarizeDetailItems(items []string, separator string, width int) string {
	var cleaned []string
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item != "" {
			cleaned = append(cleaned, item)
		}
	}
	if len(cleaned) == 0 || width <= 0 {
		return ""
	}

	full := strings.Join(cleaned, separator)
	if lipgloss.Width(full) <= width {
		return full
	}

	selected := make([]string, 0, len(cleaned))
	current := ""
	for i, item := range cleaned {
		candidate := item
		if current != "" {
			candidate = current + separator + item
		}
		if lipgloss.Width(candidate) <= width {
			selected = append(selected, item)
			current = candidate
			continue
		}
		if len(selected) == 0 {
			return truncateDetailValue(item, width)
		}
		return summarizeDetailItemsWithRemainder(selected, len(cleaned)-i, separator, width)
	}

	return current
}

func summarizeDetailItemsWithRemainder(selected []string, hidden int, separator string, width int) string {
	if len(selected) == 0 {
		return ""
	}
	if hidden <= 0 {
		return strings.Join(selected, separator)
	}
	for keep := len(selected); keep >= 1; keep-- {
		prefix := strings.Join(selected[:keep], separator)
		suffix := fmt.Sprintf(" +%d more", hidden+len(selected)-keep)
		if lipgloss.Width(prefix)+lipgloss.Width(suffix) <= width {
			return prefix + suffix
		}
	}
	suffix := fmt.Sprintf(" +%d more", hidden+len(selected))
	if lipgloss.Width(suffix) <= width {
		return suffix
	}
	return truncateDetailValue(selected[0], width)
}

func truncateDetailValue(value string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(value) <= width {
		return value
	}
	if width == 1 {
		return "…"
	}

	var b strings.Builder
	for _, r := range value {
		next := b.String() + string(r)
		if lipgloss.Width(next)+lipgloss.Width("…") > width {
			break
		}
		b.WriteRune(r)
	}
	if b.Len() == 0 {
		return "…"
	}
	return b.String() + "…"
}

func RenderDetail(
	c *collector.Container,
	meta *collector.ContainerDetail,
	hostname string,
	logs []string, logsErr error,
	confirmAction, actionResult string,
	scrollOffset, width, height int,
	logFollowing bool,
	logSearch LogSearch,
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

	infoLines := detailInfoLines(c, meta, hostname, innerWidth)
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

	logLines, logState := detailLogLines(logs, logsErr, logFollowing, innerWidth)

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

	logTitleLeft := renderLogTitle(logState, scrollOffset, logContentHeight, len(logLines), titleAvail, logFollowing, logSearch)

	// Slice visible log lines
	endIdx := scrollOffset + logContentHeight
	if endIdx > len(logLines) {
		endIdx = len(logLines)
	}
	visible := logLines[scrollOffset:endIdx]
	highlightSearchLines(visible, scrollOffset, logSearch, innerWidth)
	for len(visible) < logContentHeight {
		visible = append(visible, "")
	}

	logContent := strings.Join(visible, "\n")
	logPanel := components.Panel(logTitleLeft, logContent, width, logPanelHeight, true)

	// ── ACTION BAR ──────────────────────────────────────────
	actionBar := renderDetailActionBar(c, confirmAction, actionResult, width, logFollowing, logSearch)

	return lipgloss.JoinVertical(lipgloss.Left, infoPanel, logPanel, actionBar)
}

func renderDetailActionBar(c *collector.Container, confirmAction, actionResult string, width int, logFollowing bool, logSearch LogSearch) string {
	keyStyle := lipgloss.NewStyle().Foreground(styles.Primary).Bold(true)
	descStyle := lipgloss.NewStyle().Foreground(styles.TextMuted)

	var content string
	if logSearch.Active {
		content = logSearch.InputView
		if logSearch.Total > 0 {
			content += "  " + lipgloss.NewStyle().Foreground(styles.TextSecondary).
				Render(fmt.Sprintf("%d/%d", logSearch.Current, logSearch.Total))
		} else if logSearch.Query != "" {
			content += "  " + lipgloss.NewStyle().Foreground(styles.Error).Render("no matches")
		}
		content += "   " + keyStyle.Render("enter") + descStyle.Render(" accept") +
			"   " + keyStyle.Render("esc") + descStyle.Render(" cancel")
	} else if confirmAction != "" {
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
			keyStyle.Render("ctrl+u/d") + descStyle.Render(" page"),
			keyStyle.Render("g/G") + descStyle.Render(" top/end"),
			keyStyle.Render("f") + descStyle.Render(" "+followLabel),
			keyStyle.Render("l") + descStyle.Render(" refresh"),
			keyStyle.Render("/") + descStyle.Render(" search"),
		}
		if logSearch.Query != "" {
			parts = append(parts,
				keyStyle.Render("n/N")+descStyle.Render(" next/prev"))
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

	// Docker timestamps: 2024-03-03T12:00:01Z <message> or RFC3339Nano variants.
	if len(line) > len("2006-01-02T15:04:05Z") && line[4] == '-' && line[7] == '-' && line[10] == 'T' {
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

	source := ""
	if prefixedSource, rest, ok := splitLogSourcePrefix(msg); ok {
		source = prefixedSource
		msg = rest
	}

	// Detect log level and pick color for the message
	levelColor := detectLogLevel(msg)
	if levelColor != nil {
		msgStyle = lipgloss.NewStyle().Foreground(levelColor)
	}

	msgRendered := msgStyle.Render(msg)
	if source != "" {
		sourceStyle := lipgloss.NewStyle().Foreground(stackLogSourceColor(source)).Bold(true)
		if msg == "" {
			msgRendered = sourceStyle.Render("[" + source + "]")
		} else {
			msgRendered = sourceStyle.Render("["+source+"]") + " " + msgRendered
		}
	}

	var rendered string
	if ts != "" {
		rendered = timeStyle.Render(ts) + " " + msgRendered
	} else {
		rendered = msgRendered
	}

	return lipgloss.NewStyle().Inline(true).MaxWidth(maxWidth).Render(rendered)
}

func splitLogSourcePrefix(msg string) (string, string, bool) {
	if !strings.HasPrefix(msg, "[") {
		return "", "", false
	}
	endIdx := strings.Index(msg, "] ")
	if endIdx <= 1 {
		return "", "", false
	}
	return msg[1:endIdx], msg[endIdx+2:], true
}

func stackLogSourceColor(name string) color.Color {
	palette := []color.Color{
		styles.Primary,
		styles.Secondary,
		styles.Success,
		styles.Warning,
	}
	var sum int
	for _, r := range name {
		sum += int(r)
	}
	return palette[sum%len(palette)]
}

type detailLogState int

const (
	detailLogStateLoaded detailLogState = iota
	detailLogStateLoading
	detailLogStateWaiting
	detailLogStateEmpty
	detailLogStateError
)

func detailLogLines(logs []string, logsErr error, logFollowing bool, innerWidth int) ([]string, detailLogState) {
	switch {
	case logs == nil && logsErr == nil && logFollowing:
		return []string{
			lipgloss.NewStyle().Foreground(styles.TextMuted).Render("Waiting for live log output..."),
			lipgloss.NewStyle().Foreground(styles.TextSecondary).Render("Follow mode is active."),
		}, detailLogStateWaiting
	case logs == nil && logsErr == nil:
		return []string{
			lipgloss.NewStyle().Foreground(styles.TextMuted).Render("Loading logs..."),
			lipgloss.NewStyle().Foreground(styles.TextSecondary).Render("Fetching the last 50 lines from Docker."),
		}, detailLogStateLoading
	case logsErr != nil:
		return []string{
			lipgloss.NewStyle().Foreground(styles.Error).Render("Log refresh failed"),
			lipgloss.NewStyle().Foreground(styles.TextSecondary).Render(
				lipgloss.NewStyle().Inline(true).MaxWidth(innerWidth).Render(logsErr.Error())),
		}, detailLogStateError
	case len(logs) == 0 && logFollowing:
		return []string{
			lipgloss.NewStyle().Foreground(styles.TextMuted).Render("Following log stream..."),
			lipgloss.NewStyle().Foreground(styles.TextSecondary).Render("No log lines received yet."),
		}, detailLogStateWaiting
	case len(logs) == 0:
		return []string{
			lipgloss.NewStyle().Foreground(styles.TextMuted).Render("No logs available"),
			lipgloss.NewStyle().Foreground(styles.TextSecondary).Render("Docker returned no log lines for this container."),
		}, detailLogStateEmpty
	default:
		rendered := make([]string, 0, len(logs))
		for _, line := range logs {
			rendered = append(rendered, formatLogLine(line, innerWidth))
		}
		return rendered, detailLogStateLoaded
	}
}

func renderLogTitle(state detailLogState, scrollOffset, logContentHeight, lineCount, titleAvail int, logFollowing bool, logSearch ...LogSearch) string {
	titleStyle := lipgloss.NewStyle().Foreground(styles.TextPrimary).Bold(true)
	statusStyle := lipgloss.NewStyle().Foreground(styles.TextSecondary)

	titleText := "LOGS"
	switch state {
	case detailLogStateLoading:
		titleText = "LOGS (loading)"
		statusStyle = lipgloss.NewStyle().Foreground(styles.TextMuted)
	case detailLogStateWaiting:
		titleText = "LOGS (live)"
		statusStyle = lipgloss.NewStyle().Foreground(styles.Success)
	case detailLogStateEmpty:
		titleText = "LOGS (empty)"
		statusStyle = lipgloss.NewStyle().Foreground(styles.TextMuted)
	case detailLogStateError:
		titleText = "LOGS (error)"
		statusStyle = lipgloss.NewStyle().Foreground(styles.Error)
	case detailLogStateLoaded:
		if logFollowing {
			titleText = "LOGS (live)"
			statusStyle = lipgloss.NewStyle().Foreground(styles.Success)
		}
	}
	title := titleStyle.Render(titleText)

	var statusParts []string
	switch state {
	case detailLogStateLoaded:
		if logFollowing {
			statusParts = append(statusParts, "following")
		}
		statusParts = append(statusParts, fmt.Sprintf("%d lines", lineCount))
	case detailLogStateLoading:
		statusParts = append(statusParts, "fetching tail")
	case detailLogStateWaiting:
		statusParts = append(statusParts, "following")
	case detailLogStateEmpty:
		statusParts = append(statusParts, "no output")
	case detailLogStateError:
		statusParts = append(statusParts, "refresh failed")
	}
	if lineCount > 0 {
		endPos := min(scrollOffset+logContentHeight, lineCount)
		statusParts = append(statusParts, fmt.Sprintf("%d-%d/%d", scrollOffset+1, endPos, lineCount))
	}
	// Append search match info
	statusText := strings.Join(statusParts, "  ")
	if len(logSearch) > 0 && logSearch[0].Query != "" {
		ls := logSearch[0]
		if ls.Total > 0 {
			statusText += fmt.Sprintf("  [%d/%d matches]", ls.Current, ls.Total)
		} else {
			statusText += "  [no matches]"
		}
	}

	if statusText == "" {
		return title
	}
	renderedStatus := statusStyle.Render(statusText)
	if lipgloss.Width(title)+2+lipgloss.Width(renderedStatus) <= titleAvail {
		return title + "  " + renderedStatus
	}
	return title
}

// detectLogLevel checks the first portion of a log message for level keywords.
// LogSearch holds the state for in-log search highlighting.
type LogSearch struct {
	Active      bool
	InputView   string
	Query       string
	MatchSet    map[int]bool // raw log line indices that match
	CurrentLine int          // raw log line index of current match, -1 = none
	Total       int
	Current     int // 1-based index into matches
}

func highlightSearchLines(visible []string, scrollOffset int, logSearch LogSearch, width int) {
	if logSearch.Query == "" || len(logSearch.MatchSet) == 0 {
		return
	}
	matchBg := lipgloss.Color("#3a3000")
	currentBg := lipgloss.Color("#5a4a00")
	for i := range visible {
		origIdx := scrollOffset + i
		if !logSearch.MatchSet[origIdx] {
			continue
		}
		bg := matchBg
		if origIdx == logSearch.CurrentLine {
			bg = currentBg
		}
		visible[i] = lipgloss.NewStyle().Background(bg).Width(width).Inline(true).Render(visible[i])
	}
}

func detectLogLevel(msg string) color.Color {
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

	return nil
}

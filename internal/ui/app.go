package ui

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/kostas/homedash/internal/collector"
	"github.com/kostas/homedash/internal/config"
	"github.com/kostas/homedash/internal/state"
	"github.com/kostas/homedash/internal/ui/components"
	"github.com/kostas/homedash/internal/ui/panels"
	"github.com/kostas/homedash/internal/ui/styles"
	overlay "github.com/rmhubbert/bubbletea-overlay"
)

type ViewMode int

const (
	ViewDashboard ViewMode = iota
	ViewDetail
)

type DisplayItemKind int

const (
	DisplayGroup DisplayItemKind = iota
	DisplayContainer
)

type DisplayItem struct {
	Kind           DisplayItemKind
	StackName      string
	ContainerCount int
	RunningCount   int
	Container      *collector.Container
	Collapsed      bool
}

type Model struct {
	width  int
	height int

	systemData  collector.SystemData
	dockerData  collector.DockerData
	weatherData collector.WeatherData
	disks       []config.Disk
	dockerHost  string

	cpuHistory      *components.RingBuffer
	focusedPanel    Panel
	scrollOffset    int
	containerRows   int
	detailLogRows   int
	containerStartY int // Y offset where container data rows begin
	detailLogStartY int
	selectedIndex   int

	systemErr  error
	dockerErr  error
	weatherErr error

	weatherRetries int
	refreshing     bool

	collapsedStacks map[string]bool
	displayItems    []DisplayItem

	viewMode                     ViewMode
	detailContainer              *collector.Container
	detailContainerID            string
	detailLogs                   []string
	detailLogsErr                error
	detailMeta                   *collector.ContainerDetail
	detailMetaErr                error
	detailScrollOffset           int
	confirmAction                string
	actionResult                 string
	dashboardActionContainerID   string
	dashboardActionContainerName string

	// Quick-action menu
	quickMenuOpen        bool
	quickMenuIndex       int
	quickMenuContainerID string
	quickMenuContainer   *collector.Container

	// Search/Filter
	searchInput textinput.Model
	filtering   bool

	// Log follow mode
	logFollowing    bool
	logFollowCancel context.CancelFunc
	logFollowCh     <-chan string
	logFollowSeq    uint64 // session counter to discard stale messages

	// Collapse persistence
	collapseSeq  uint64
	lastSavedSeq uint64

	// Notifications
	notifications     notificationQueue
	dockerBaselineSet bool
	diskWarned        map[string]bool
	weatherWasOK      bool
	shownWarnings     map[string]bool // collector warnings already surfaced

	// Double-click tracking
	lastClickTime  time.Time
	lastClickIndex int

	systemRefreshInterval  time.Duration
	dockerRefreshInterval  time.Duration
	weatherRefreshInterval time.Duration
}

type ModelOptions struct {
	Disks                  []config.Disk
	DockerHost             string
	SystemRefreshInterval  time.Duration
	DockerRefreshInterval  time.Duration
	WeatherRefreshInterval time.Duration
}

func NewModel(options ModelOptions) Model {
	defaults := config.Default()
	disks := options.Disks
	if len(disks) == 0 {
		disks = defaults.System.Disks
	}

	dockerHost := strings.TrimSpace(options.DockerHost)
	if dockerHost == "" {
		dockerHost = defaults.EffectiveDockerHost()
	}

	systemRefresh := options.SystemRefreshInterval
	if systemRefresh <= 0 {
		systemRefresh = defaults.Refresh.System
	}
	dockerRefresh := options.DockerRefreshInterval
	if dockerRefresh <= 0 {
		dockerRefresh = defaults.Refresh.Docker
	}
	weatherRefresh := options.WeatherRefreshInterval
	if weatherRefresh <= 0 {
		weatherRefresh = defaults.Refresh.Weather
	}

	ti := textinput.New()
	ti.Placeholder = "Filter containers..."
	ti.Prompt = " / "
	ti.PromptStyle = lipgloss.NewStyle().Foreground(styles.Secondary)
	ti.TextStyle = lipgloss.NewStyle().Foreground(styles.TextPrimary)

	return Model{
		cpuHistory:             components.NewRingBuffer(60),
		focusedPanel:           PanelContainers,
		containerRows:          10,
		collapsedStacks:        state.Load(),
		searchInput:            ti,
		disks:                  disks,
		dockerHost:             dockerHost,
		systemRefreshInterval:  systemRefresh,
		dockerRefreshInterval:  dockerRefresh,
		weatherRefreshInterval: weatherRefresh,
		diskWarned:             make(map[string]bool),
		shownWarnings:          make(map[string]bool),
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		// Initial data collection
		func() tea.Msg { return collectSystemCmd(m.disks) },
		func() tea.Msg { return collectDockerCmd() },
		func() tea.Msg { return collectWeatherCmd() },
		// Start tick timers
		systemTickCmd(m.disks, m.systemRefreshInterval),
		dockerTickCmd(m.dockerRefreshInterval),
		weatherTickCmd(m.weatherRefreshInterval),
	)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	// Handle key events while search input is focused
	if m.filtering {
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			switch keyMsg.String() {
			case "enter":
				m.filtering = false
				m.searchInput.Blur()
				m.recalcLayout()
				return m, nil
			case "esc":
				m.filtering = false
				m.searchInput.Blur()
				m.searchInput.SetValue("")
				m.rebuildDisplayItems()
				m.recalcLayout()
				return m, nil
			case "ctrl+c":
				if m.collapseSeq > m.lastSavedSeq {
					_ = state.Save(m.collapsedStacks)
				}
				return m, tea.Quit
			}
			prevFilter := m.searchInput.Value()
			m.searchInput, cmd = m.searchInput.Update(msg)
			if m.searchInput.Value() != prevFilter {
				m.rebuildDisplayItems()
			}
			return m, cmd
		}
		// Fall through for non-key messages (ticks, data, resize)
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.recalcLayout()
		return m, nil

	case tea.MouseMsg:
		if m.viewMode == ViewDetail {
			return handleDetailMouse(msg, &m)
		}
		if !m.quickMenuOpen {
			return handleMouse(msg, &m)
		}
		return m, nil

	case tea.KeyMsg:
		return handleKey(msg, &m)

	case SystemDataMsg:
		m.refreshing = false
		var notifCmds []tea.Cmd
		if msg.Err == nil {
			m.systemData = msg.Data
			m.cpuHistory.Push(msg.Data.CPUPercent)

			// Disk threshold warnings
			for _, d := range msg.Data.Disks {
				if d.Percent >= 90 && !m.diskWarned[d.Mount] {
					m.diskWarned[d.Mount] = true
					if cmd := m.pushNotify(
						fmt.Sprintf("Disk %s at %.0f%%", d.Mount, d.Percent),
						levelWarning,
					); cmd != nil {
						notifCmds = append(notifCmds, cmd)
					}
				} else if d.Percent < 90 && m.diskWarned[d.Mount] {
					delete(m.diskWarned, d.Mount)
				}
			}

			// Surface collector warnings (e.g. inaccessible disk mounts)
			for _, w := range msg.Data.Warnings {
				if !m.shownWarnings[w] {
					m.shownWarnings[w] = true
					if cmd := m.pushNotify(w, levelWarning); cmd != nil {
						notifCmds = append(notifCmds, cmd)
					}
				}
			}
		}
		m.systemErr = msg.Err
		cmds := append(notifCmds, systemTickCmd(m.disks, m.systemRefreshInterval))
		return m, tea.Batch(cmds...)

	case DockerDataMsg:
		m.refreshing = false
		// Detect container state changes
		var notifCmds []tea.Cmd
		if m.dockerBaselineSet && msg.Err == nil {
			oldStates := make(map[string]string, len(m.dockerData.Containers))
			for _, c := range m.dockerData.Containers {
				oldStates[c.ID] = c.State
			}
			for _, c := range msg.Data.Containers {
				oldState, existed := oldStates[c.ID]
				if !existed {
					continue // new container - skip, not a state change
				}
				if oldState != c.State {
					var level notificationLevel
					switch c.State {
					case "running":
						level = levelInfo
					case "exited":
						level = levelError
					default:
						level = levelWarning
					}
					if cmd := m.pushNotify(
						fmt.Sprintf("%s %s -> %s", c.Name, oldState, c.State),
						level,
					); cmd != nil {
						notifCmds = append(notifCmds, cmd)
					}
				}
			}
		}
		if msg.Err == nil {
			m.dockerBaselineSet = true
		}
		if msg.Err == nil {
			m.dockerData = msg.Data
		}
		m.dockerErr = msg.Err
		m.rebuildDisplayItems()
		// Refresh quick menu container pointer if open
		if m.quickMenuOpen {
			m.quickMenuContainer = nil
			for i := range m.dockerData.Containers {
				if m.dockerData.Containers[i].ID == m.quickMenuContainerID {
					m.quickMenuContainer = &m.dockerData.Containers[i]
					break
				}
			}
			if m.quickMenuContainer == nil {
				m.quickMenuOpen = false
			}
		}
		if m.viewMode == ViewDetail && m.detailContainerID != "" {
			m.detailContainer = nil
			for i := range m.dockerData.Containers {
				if m.dockerData.Containers[i].ID == m.detailContainerID {
					m.detailContainer = &m.dockerData.Containers[i]
					break
				}
			}
			m.recalcLayout()
		}
		cmds := append(notifCmds, dockerTickCmd(m.dockerRefreshInterval))
		return m, tea.Batch(cmds...)

	case ContainerLogsMsg:
		if m.viewMode == ViewDetail && msg.ContainerID == m.detailContainerID {
			m.detailLogs = msg.Lines
			m.detailLogsErr = msg.Err
		}
		return m, nil

	case ContainerDetailMsg:
		if msg.ContainerID == m.detailContainerID {
			if msg.Err != nil {
				m.detailMetaErr = msg.Err
			} else {
				m.detailMeta = &msg.Detail
				m.recalcLayout()
			}
		}
		return m, nil

	case ContainerActionMsg:
		containerID := msg.ContainerID
		if msg.Err != nil {
			m.actionResult = fmt.Sprintf("Error: %s failed: %v", msg.Action, msg.Err)
		} else {
			m.actionResult = fmt.Sprintf("Success: %s %s", msg.Action, containerID[:min(8, len(containerID))])
		}
		cmds := []tea.Cmd{
			func() tea.Msg { return collectDockerCmd() },
			clearActionResultCmd(),
		}
		if m.viewMode == ViewDetail {
			cmds = append(cmds, collectLogsCmd(m.detailContainerID, 50))
		}
		if m.viewMode == ViewDashboard {
			m.dashboardActionContainerID = ""
			m.dashboardActionContainerName = ""
		}
		return m, tea.Batch(cmds...)

	case ClearActionResultMsg:
		m.actionResult = ""
		return m, nil

	case DismissNotificationMsg:
		m.notifications.dismiss(msg.ID)
		m.recalcLayout()
		// Schedule dismiss for next visible notification
		if n := m.notifications.current(); n != nil {
			return m, dismissNotificationCmd(n.ID)
		}
		return m, nil

	case WeatherDataMsg:
		m.refreshing = false
		if msg.Err == nil {
			m.weatherData = msg.Data
			m.weatherErr = nil
			m.weatherWasOK = true
			m.weatherRetries = 0
			return m, weatherTickCmd(m.weatherRefreshInterval)
		}
		m.weatherErr = msg.Err
		var notifCmds []tea.Cmd
		if m.weatherWasOK {
			m.weatherWasOK = false
			if cmd := m.pushNotify("Weather update failed", levelWarning); cmd != nil {
				notifCmds = append(notifCmds, cmd)
			}
		}
		if m.weatherRetries < 3 {
			m.weatherRetries++
			cmds := append(notifCmds, weatherRetryCmd())
			return m, tea.Batch(cmds...)
		}
		m.weatherRetries = 0
		cmds := append(notifCmds, weatherTickCmd(m.weatherRefreshInterval))
		return m, tea.Batch(cmds...)
	case CollapseSaveTickMsg:
		if msg.Seq == m.collapseSeq {
			collapsed := make(map[string]bool, len(m.collapsedStacks))
			for k, v := range m.collapsedStacks {
				collapsed[k] = v
			}
			return m, collapseSaveCmd(collapsed, msg.Seq)
		}

	case CollapseSavedMsg:
		if msg.Err == nil && msg.Seq >= m.lastSavedSeq {
			m.lastSavedSeq = msg.Seq
		}

	case LogFollowLineMsg:
		if !m.logFollowing || msg.Seq != m.logFollowSeq {
			return m, nil
		}
		if msg.Done {
			m.logFollowing = false
			m.logFollowCancel = nil
			m.logFollowCh = nil
			return m, nil
		}
		wasAtBottom := m.isFollowAtBottom()
		m.detailLogs = append(m.detailLogs, msg.Line)
		// Cap at 1000 lines
		if len(m.detailLogs) > 1000 {
			m.detailLogs = m.detailLogs[len(m.detailLogs)-1000:]
		}
		// Auto-scroll to bottom if user was at bottom
		if wasAtBottom {
			maxScroll := len(m.detailLogs) - m.detailLogRows
			if maxScroll < 0 {
				maxScroll = 0
			}
			m.detailScrollOffset = maxScroll
		}
		return m, logFollowCmd(m.logFollowCh, m.logFollowSeq)
	}

	return m, nil
}

// stopFollowing cancels any active log follow stream.
func (m *Model) stopFollowing() {
	if m.logFollowing {
		if m.logFollowCancel != nil {
			m.logFollowCancel()
		}
		m.logFollowing = false
		m.logFollowCancel = nil
		m.logFollowCh = nil
	}
}

// startFollowing begins streaming logs for the current detail container.
func (m *Model) startFollowing() tea.Cmd {
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan string, 64)

	m.logFollowSeq++
	m.logFollowing = true
	m.logFollowCancel = cancel
	m.logFollowCh = ch

	containerID := m.detailContainerID

	go func() {
		defer close(ch)
		_ = collector.StreamContainerLogs(ctx, containerID, 50, ch)
	}()

	// Clear existing logs since the stream will provide tail + follow
	m.detailLogs = nil
	m.detailScrollOffset = 0

	return logFollowCmd(ch, m.logFollowSeq)
}

// isFollowAtBottom returns true if scroll is at or near the bottom of logs.
func (m *Model) isFollowAtBottom() bool {
	maxScroll := len(m.detailLogs) - m.detailLogRows
	if maxScroll < 0 {
		maxScroll = 0
	}
	return m.detailScrollOffset >= maxScroll
}

type quickMenuItem struct {
	label  string
	key    string
	action string // "logs", "stop", "start", "restart"
}

func quickMenuItems(state string) []quickMenuItem {
	items := []quickMenuItem{
		{"View Logs", "enter", "logs"},
	}
	if state == "running" {
		items = append(items,
			quickMenuItem{"Stop", "s", "stop"},
			quickMenuItem{"Restart", "R", "restart"},
		)
	} else {
		items = append(items, quickMenuItem{"Start", "S", "start"})
	}
	return items
}

// viewString wraps a pre-rendered string as a tea.Model for use with overlay.
type viewString struct{ s string }

func (v *viewString) Init() tea.Cmd                       { return nil }
func (v *viewString) Update(tea.Msg) (tea.Model, tea.Cmd) { return v, nil }
func (v *viewString) View() string                        { return v.s }

func (m Model) renderQuickMenu(base string) string {
	c := m.quickMenuContainer
	items := quickMenuItems(c.State)

	baseW := lipgloss.Width(base)

	keyStyle := lipgloss.NewStyle().Foreground(styles.Primary).Bold(true)
	labelStyle := lipgloss.NewStyle().Foreground(styles.TextPrimary)
	mutedStyle := lipgloss.NewStyle().Foreground(styles.TextMuted)

	// Menu width adapts to container name
	menuInner := len(c.Name) + 6
	if menuInner < 28 {
		menuInner = 28
	}
	if menuInner > baseW-8 {
		menuInner = baseW - 8
	}

	// Title bar: container name centered, state on the right
	name := c.Name
	if len(name) > menuInner-2 {
		name = name[:menuInner-2]
	}
	stateColor := styles.ContainerStateColor(c.State)
	stateStyled := lipgloss.NewStyle().Foreground(stateColor).Render(c.State)
	nameStyled := lipgloss.NewStyle().Foreground(styles.TextPrimary).Bold(true).Render(name)
	titleGap := menuInner - lipgloss.Width(nameStyled) - lipgloss.Width(stateStyled)
	if titleGap < 1 {
		titleGap = 1
	}
	titleLine := nameStyled + strings.Repeat(" ", titleGap) + stateStyled

	// Separator — account for 1-cell padding on each side inside the popup
	sep := mutedStyle.Render(strings.Repeat("─", menuInner))

	// Menu items
	var menuLines []string
	for i, item := range items {
		keyPart := keyStyle.Render(fmt.Sprintf("%-6s", item.key))
		labelPart := labelStyle.Render(item.label)
		line := " " + keyPart + " " + labelPart
		if i == m.quickMenuIndex {
			// Pad to full width, then apply background inline to avoid wrapping
			pad := menuInner - lipgloss.Width(line)
			if pad > 0 {
				line += strings.Repeat(" ", pad)
			}
			line = lipgloss.NewStyle().
				Background(styles.BgFocus).
				Inline(true).
				Render(line)
		}
		menuLines = append(menuLines, line)
	}

	// Hint
	hint := mutedStyle.Render("j/k navigate  enter confirm  esc close")

	body := titleLine + "\n" + sep + "\n" +
		strings.Join(menuLines, "\n") + "\n" + sep + "\n" + hint

	popup := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(styles.BorderFocus).
		Background(styles.BgPanel).
		Foreground(styles.TextPrimary).
		Width(menuInner+2).
		Padding(0, 1).
		Render(body)

	bg := &viewString{s: base}
	fg := &viewString{s: popup}
	ov := overlay.New(fg, bg, overlay.Center, overlay.Center, 0, 0)
	return ov.View()
}

func (m Model) isNarrow() bool {
	return m.width < 90
}

// pushNotify adds a notification and returns a dismiss command.
func (m *Model) pushNotify(message string, level notificationLevel) tea.Cmd {
	id := m.notifications.push(message, level)
	m.recalcLayout()
	// Only schedule dismiss if this is the only (now-visible) notification
	if m.notifications.len() == 1 {
		return dismissNotificationCmd(id)
	}
	return nil
}

func (m *Model) recalcLayout() {
	topHeight := 11
	if m.isNarrow() {
		topHeight = 22 // two stacked panels
	}
	chromeHeight := 7 // header(1) + container chrome(4) + preview(1) + help(1)

	// Add space for search prompt if active or filter is set
	if m.filtering || m.searchInput.Value() != "" {
		chromeHeight++
	}
	if m.notifications.len() > 0 {
		chromeHeight++
	}

	m.containerRows = m.height - topHeight - chromeHeight
	if m.containerRows < 0 {
		m.containerRows = 0
	}

	// Compute Y offset for mouse hit testing on container rows.
	// Layout: header(1) + topHeight + container panel border(1) + title(1) + header row(1)
	m.containerStartY = 1 + topHeight + 3
	if m.filtering || m.searchInput.Value() != "" {
		m.containerStartY++ // filter input line
	}
	// Note: notification bar is below the container panel (between preview
	// and help), so it does NOT affect containerStartY.

	// Detail view: dynamic info panel height
	infoLines := 4 // base: Image, Stack/Health, Ports, ID
	if m.detailContainer != nil && m.detailContainer.State == "running" {
		infoLines++ // Net row
	}
	if m.detailMeta != nil {
		if len(m.detailMeta.Mounts) > 0 {
			infoLines++
		}
		composeKeys := []string{
			"com.docker.compose.project",
			"com.docker.compose.service",
			"com.docker.compose.version",
		}
		hasComposeLabels := false
		for _, key := range composeKeys {
			if _, ok := m.detailMeta.Labels[key]; ok {
				hasComposeLabels = true
				break
			}
		}
		if hasComposeLabels {
			infoLines++
		}
	}
	infoPanelHeight := infoLines + 3 // border(2) + title(1)
	logPanel := m.height - infoPanelHeight - 1
	if logPanel < 5 {
		logPanel = 5
	}
	m.detailLogRows = logPanel - 3
	if m.detailLogRows < 1 {
		m.detailLogRows = 1
	}

	// Detail view: log content starts after info panel + log panel chrome
	m.detailLogStartY = infoPanelHeight + 2 // border(1) + title(1) of log panel
}

func (m *Model) ensureVisible() {
	if m.selectedIndex < m.scrollOffset {
		m.scrollOffset = m.selectedIndex
	}
	if m.selectedIndex >= m.scrollOffset+m.containerRows {
		m.scrollOffset = m.selectedIndex - m.containerRows + 1
	}
	maxOffset := len(m.displayItems) - m.containerRows
	if maxOffset < 0 {
		maxOffset = 0
	}
	if m.scrollOffset > maxOffset {
		m.scrollOffset = maxOffset
	}
}

func (m *Model) rebuildDisplayItems() {
	m.displayItems = m.displayItems[:0]
	filter := strings.ToLower(m.searchInput.Value())

	type stackGroup struct {
		name       string
		containers []*collector.Container
		running    int
	}
	groupMap := make(map[string]*stackGroup)
	var groupOrder []string
	var ungrouped []*collector.Container

	for i := range m.dockerData.Containers {
		c := &m.dockerData.Containers[i]

		// Filtering
		if filter != "" {
			nameMatch := strings.Contains(strings.ToLower(c.Name), filter)
			stackMatch := strings.Contains(strings.ToLower(c.Stack), filter)
			if !nameMatch && !stackMatch {
				continue
			}
		}

		if c.Stack == "" {
			ungrouped = append(ungrouped, c)
			continue
		}
		g, exists := groupMap[c.Stack]
		if !exists {
			g = &stackGroup{name: c.Stack}
			groupMap[c.Stack] = g
			groupOrder = append(groupOrder, c.Stack)
		}
		g.containers = append(g.containers, c)
		if c.State == "running" {
			g.running++
		}
	}

	sort.Strings(groupOrder)

	for _, name := range groupOrder {
		g := groupMap[name]
		collapsed := m.collapsedStacks[name]

		// Auto-expand if filtering
		if filter != "" {
			collapsed = false
		}

		m.displayItems = append(m.displayItems, DisplayItem{
			Kind:           DisplayGroup,
			StackName:      name,
			ContainerCount: len(g.containers),
			RunningCount:   g.running,
			Collapsed:      collapsed,
		})
		if !collapsed {
			for _, c := range g.containers {
				m.displayItems = append(m.displayItems, DisplayItem{
					Kind:      DisplayContainer,
					Container: c,
				})
			}
		}
	}

	for _, c := range ungrouped {
		m.displayItems = append(m.displayItems, DisplayItem{
			Kind:      DisplayContainer,
			Container: c,
		})
	}

	if m.selectedIndex >= len(m.displayItems) {
		m.selectedIndex = max(0, len(m.displayItems)-1)
	}
	m.ensureVisible()
}

func (m Model) View() string {
	if m.width == 0 {
		return "Loading..."
	}
	switch m.viewMode {
	case ViewDetail:
		return m.renderDetail()
	default:
		return m.renderDashboard()
	}
}

func (m Model) renderDetail() string {
	detail := panels.RenderDetail(
		m.detailContainer, m.detailMeta, m.detailLogs, m.detailLogsErr,
		m.confirmAction, m.actionResult,
		m.detailScrollOffset, m.width, m.height,
		m.logFollowing)
	return lipgloss.NewStyle().
		Background(styles.BgBase).
		Width(m.width).
		Height(m.height).
		Render(detail)
}

func (m Model) renderDashboard() string {
	// Header
	header := panels.RenderHeader(m.systemData, m.width)

	// Top section: side-by-side or stacked depending on width
	topHeight := 11
	var topRow string

	if m.isNarrow() {
		// Stack vertically at full width
		systemPanel := panels.RenderSystem(
			m.systemData, m.cpuHistory,
			m.width, topHeight,
			m.focusedPanel == PanelSystem)
		weatherPanel := panels.RenderWeather(
			m.weatherData, m.weatherErr, m.weatherRetries,
			m.width, topHeight,
			m.focusedPanel == PanelWeather)
		topRow = lipgloss.JoinVertical(lipgloss.Left, systemPanel, weatherPanel)
	} else {
		// Side-by-side: 40/60 split
		leftWidth := m.width * 40 / 100
		if leftWidth < 35 {
			leftWidth = 35
		}
		rightWidth := m.width - leftWidth
		systemPanel := panels.RenderSystem(
			m.systemData, m.cpuHistory,
			leftWidth, topHeight,
			m.focusedPanel == PanelSystem)
		weatherPanel := panels.RenderWeather(
			m.weatherData, m.weatherErr, m.weatherRetries,
			rightWidth, topHeight,
			m.focusedPanel == PanelWeather)
		topRow = lipgloss.JoinHorizontal(lipgloss.Top, systemPanel, weatherPanel)
	}

	// Preview bar — show selected container info
	var selectedContainer *collector.Container
	if m.selectedIndex >= 0 && m.selectedIndex < len(m.displayItems) {
		selectedContainer = m.displayItems[m.selectedIndex].Container
	}
	previewBar := panels.RenderPreview(selectedContainer, m.confirmAction, m.dashboardActionContainerName, m.actionResult, m.width)

	// Help bar
	helpBar := panels.RenderHelp(panels.DefaultBindings, m.refreshing, m.width)
	notifBar := renderNotificationBar(&m.notifications, m.width)

	// Bottom bars pinned — always visible
	var bottomBars []string
	bottomBars = append(bottomBars, previewBar)
	if notifBar != "" {
		bottomBars = append(bottomBars, notifBar)
	}
	bottomBars = append(bottomBars, helpBar)
	bottomSection := lipgloss.JoinVertical(lipgloss.Left, bottomBars...)

	// Measure actual rendered heights to compute container panel size dynamically
	headerLines := strings.Count(header, "\n") + 1
	topLines := strings.Count(topRow, "\n") + 1
	bottomLines := strings.Count(bottomSection, "\n") + 1
	containerChrome := 4 // border(2) + title(1) + column header(1)
	if m.filtering || m.searchInput.Value() != "" {
		containerChrome++
	}
	visibleRows := m.height - headerLines - topLines - bottomLines - containerChrome
	if visibleRows < 0 {
		visibleRows = 0
	}

	// Containers — sized to exactly fill remaining space
	panelItems := make([]panels.ContainerDisplayItem, len(m.displayItems))
	for i, item := range m.displayItems {
		panelItems[i] = panels.ContainerDisplayItem{
			IsGroup:        item.Kind == DisplayGroup,
			StackName:      item.StackName,
			ContainerCount: item.ContainerCount,
			RunningCount:   item.RunningCount,
			Collapsed:      item.Collapsed,
			Container:      item.Container,
		}
	}

	containersPanel := panels.RenderContainers(
		panelItems,
		m.dockerData.Running, m.dockerData.Total,
		m.scrollOffset, m.selectedIndex, visibleRows, m.width,
		m.focusedPanel == PanelContainers,
		m.searchInput, m.filtering)

	// Quick-action menu overlay
	if m.quickMenuOpen && m.quickMenuContainer != nil {
		containersPanel = m.renderQuickMenu(containersPanel)
	}

	view := lipgloss.JoinVertical(lipgloss.Left,
		header, topRow, containersPanel, bottomSection)

	// Safety truncation — should not be needed with dynamic sizing
	lines := strings.Split(view, "\n")
	if len(lines) > m.height {
		lines = lines[:m.height]
		view = strings.Join(lines, "\n")
	}

	return lipgloss.NewStyle().
		Background(styles.BgBase).
		Width(m.width).
		Height(m.height).
		Render(view)
}

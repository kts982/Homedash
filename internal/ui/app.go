package ui

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/kostas/homedash/internal/collector"
	"github.com/kostas/homedash/internal/config"
	"github.com/kostas/homedash/internal/state"
	"github.com/kostas/homedash/internal/ui/components"
	"github.com/kostas/homedash/internal/ui/panels"
	"github.com/kostas/homedash/internal/ui/styles"
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
	UnhealthyCount int
	StartingCount  int
	StoppedCount   int
	CPUPercTotal   float64
	MemUsedTotal   uint64
	Container      *collector.Container
	Collapsed      bool
}

type dashboardLayoutMetrics struct {
	header          string
	topRow          string
	bottomSection   string
	containerRows   int
	containerStartY int
}

type Model struct {
	width  int
	height int

	TestMode bool

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

	viewMode                   ViewMode
	detailContainer            *collector.Container
	detailContainerID          string
	detailStackName            string
	detailLogs                 []string
	detailLogsErr              error
	detailMeta                 *collector.ContainerDetail
	detailMetaErr              error
	detailScrollOffset         int
	confirmAction              string
	actionResult               string
	dashboardActionContainerID string
	dashboardActionStackName   string
	dashboardActionTargetName  string

	// Quick-action menu
	quickMenuOpen        bool
	quickMenuIndex       int
	quickMenuContainerID string
	quickMenuContainer   *collector.Container
	quickMenuStackName   string

	// Search/Filter
	searchInput      textinput.Model
	filtering        bool
	selectedTarget   string // semantic selection anchor: "c:<id>" or "s:<stack>"

	// Log search in detail view
	logSearchInput   textinput.Model
	logSearchActive  bool
	logSearchMatches []int // indices into detailLogs that match
	logSearchIndex   int   // current position in logSearchMatches

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
	TestMode               bool
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
	s := textinput.DefaultDarkStyles()
	s.Focused.Prompt = lipgloss.NewStyle().Foreground(styles.Secondary)
	s.Focused.Text = lipgloss.NewStyle().Foreground(styles.TextPrimary)
	ti.SetStyles(s)

	lsi := textinput.New()
	lsi.Placeholder = "Search logs..."
	lsi.Prompt = " / "
	ls := textinput.DefaultDarkStyles()
	ls.Focused.Prompt = lipgloss.NewStyle().Foreground(styles.Secondary)
	ls.Focused.Text = lipgloss.NewStyle().Foreground(styles.TextPrimary)
	lsi.SetStyles(ls)

	return Model{
		cpuHistory:             components.NewRingBuffer(60),
		focusedPanel:           PanelContainers,
		containerRows:          10,
		collapsedStacks:        state.Load(),
		searchInput:            ti,
		logSearchInput:         lsi,
		disks:                  disks,
		dockerHost:             dockerHost,
		systemRefreshInterval:  systemRefresh,
		dockerRefreshInterval:  dockerRefresh,
		weatherRefreshInterval: weatherRefresh,
		diskWarned:             make(map[string]bool),
		shownWarnings:          make(map[string]bool),
		TestMode:               options.TestMode,
	}
}

func (m Model) Init() tea.Cmd {
	if m.TestMode {
		return tea.Batch(
			func() tea.Msg { return collectMockSystemCmd() },
			func() tea.Msg { return collectMockDockerCmd() },
			func() tea.Msg { return collectMockWeatherCmd() },
		)
	}

	cmds := []tea.Cmd{
		// Initial data collection
		func() tea.Msg { return collectSystemCmd(m.disks) },
		func() tea.Msg { return collectDockerCmd() },
		func() tea.Msg { return collectWeatherCmd() },
	}

	if !m.TestMode {
		cmds = append(cmds,
			// Start tick timers
			systemTickCmd(m.disks, m.systemRefreshInterval),
			dockerTickCmd(m.dockerRefreshInterval),
			weatherTickCmd(m.weatherRefreshInterval),
		)
	}

	return tea.Batch(cmds...)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	// Handle key events while log search input is focused
	if m.logSearchActive {
		if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
			switch keyMsg.String() {
			case "enter":
				m.logSearchActive = false
				m.logSearchInput.Blur()
				return m, nil
			case "esc":
				m.logSearchActive = false
				m.logSearchInput.Blur()
				m.logSearchInput.SetValue("")
				m.logSearchMatches = nil
				m.logSearchIndex = 0
				return m, nil
			case "ctrl+c":
				if m.collapseSeq > m.lastSavedSeq {
					_ = state.Save(m.collapsedStacks)
				}
				return m, tea.Quit
			}
			prevQuery := m.logSearchInput.Value()
			m.logSearchInput, cmd = m.logSearchInput.Update(msg)
			if m.logSearchInput.Value() != prevQuery {
				m.recomputeLogSearchMatches()
			}
			return m, cmd
		}
	}

	// Handle key events while search input is focused
	if m.filtering {
		if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
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
		m.refreshing = false
		if m.viewMode == ViewDetail {
			return handleDetailMouse(msg, &m)
		}
		if !m.quickMenuOpen {
			return handleMouse(msg, &m)
		}
		return m, nil

	case tea.KeyPressMsg:
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
		if m.TestMode {
			return m, tea.Batch(notifCmds...)
		}
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
		// Refresh quick menu target if open
		if m.quickMenuOpen {
			if m.quickMenuStackName != "" {
				found := false
				for _, item := range m.displayItems {
					if item.Kind == DisplayGroup && item.StackName == m.quickMenuStackName {
						found = true
						break
					}
				}
				if !found {
					m.quickMenuOpen = false
					m.quickMenuStackName = ""
				}
			} else {
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
		}
		if m.viewMode == ViewDetail {
			if m.detailContainerID != "" {
				m.detailContainer = nil
				for i := range m.dockerData.Containers {
					if m.dockerData.Containers[i].ID == m.detailContainerID {
						m.detailContainer = &m.dockerData.Containers[i]
						break
					}
				}
			}
			m.recalcLayout()
		}
		if m.TestMode {
			return m, tea.Batch(notifCmds...)
		}
		cmds := append(notifCmds, dockerTickCmd(m.dockerRefreshInterval))
		return m, tea.Batch(cmds...)

	case ContainerLogsMsg:
		if m.viewMode == ViewDetail && msg.ContainerID == m.detailContainerID {
			m.detailLogs = msg.Lines
			m.detailLogsErr = msg.Err
			if m.logSearchInput.Value() != "" {
				m.recomputeLogSearchMatches()
			}
		}
		return m, nil

	case StackLogsMsg:
		if m.viewMode == ViewDetail && msg.StackName == m.detailStackName {
			m.detailLogs = msg.Lines
			m.detailLogsErr = msg.Err
			if m.logSearchInput.Value() != "" {
				m.recomputeLogSearchMatches()
			}
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
		if m.viewMode == ViewDetail && m.detailContainerID != "" {
			cmds = append(cmds, collectLogsCmd(m.detailContainerID, 50))
		}
		if m.viewMode == ViewDashboard {
			m.clearDashboardAction()
		}
		return m, tea.Batch(cmds...)

	case StackActionMsg:
		var notifCmd tea.Cmd
		switch {
		case msg.Attempted == 0:
			m.actionResult = fmt.Sprintf("Nothing to %s in stack %s", msg.Action, msg.StackName)
		case msg.Err != nil:
			m.actionResult = formatStackActionFailureResult(msg)
			notifCmd = m.pushNotify(formatStackActionFailureNotification(msg), levelError)
		default:
			m.actionResult = fmt.Sprintf(
				"Success: %s stack %s (%d containers)",
				msg.Action,
				msg.StackName,
				msg.Attempted,
			)
		}
		m.clearDashboardAction()
		cmds := []tea.Cmd{
			func() tea.Msg { return collectDockerCmd() },
			clearActionResultCmd(),
		}
		if m.viewMode == ViewDetail && msg.StackName == m.detailStackName {
			cmds = append(cmds, collectStackLogsCmd(m.dockerData.Containers, msg.StackName, 50))
		}
		if notifCmd != nil {
			cmds = append(cmds, notifCmd)
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
			if m.TestMode {
				return m, nil
			}
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
		if m.TestMode {
			return m, tea.Batch(notifCmds...)
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
			// Recompute search matches since indices shifted
			if m.logSearchInput.Value() != "" {
				m.recomputeLogSearchMatches()
			}
		} else if query := strings.ToLower(m.logSearchInput.Value()); query != "" {
			// Check if the new line matches
			if strings.Contains(strings.ToLower(msg.Line), query) {
				m.logSearchMatches = append(m.logSearchMatches, len(m.detailLogs)-1)
			}
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
	if m.TestMode {
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan string, 64)

	m.logFollowSeq++
	m.logFollowing = true
	m.logFollowCancel = cancel
	m.logFollowCh = ch

	tail := 0
	if m.detailLogs == nil {
		tail = 50
	}
	stackName := m.detailStackName
	containerID := m.detailContainerID
	containers := append([]collector.Container(nil), m.dockerData.Containers...)

	go func() {
		defer close(ch)
		if stackName != "" {
			_ = collector.StreamStackLogs(ctx, containers, stackName, tail, ch)
			return
		}
		_ = collector.StreamContainerLogs(ctx, containerID, tail, ch)
	}()

	// If logs are not loaded yet, let the follow stream provide the initial tail.
	if tail > 0 {
		m.detailLogs = nil
		m.detailScrollOffset = 0
	}

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

func stackQuickMenuItems(running, stopped int) []quickMenuItem {
	items := []quickMenuItem{
		{"View Stack Logs", "enter", "logs"},
	}
	if stopped > 0 {
		items = append(items, quickMenuItem{"Start Stack", "S", "start"})
	}
	if running > 0 {
		items = append(items,
			quickMenuItem{"Stop Stack", "s", "stop"},
			quickMenuItem{"Restart Stack", "R", "restart"},
		)
	}
	return items
}

func (m Model) currentQuickMenuItems() []quickMenuItem {
	if stack := m.quickMenuStackPreview(); stack != nil {
		return stackQuickMenuItems(stack.RunningCount, stack.StoppedCount)
	}
	if m.quickMenuContainer != nil {
		return quickMenuItems(m.quickMenuContainer.State)
	}
	return nil
}

func (m Model) renderQuickMenu(base string) string {
	items := m.currentQuickMenuItems()
	if len(items) == 0 {
		return base
	}

	baseW := lipgloss.Width(base)
	baseH := lipgloss.Height(base)

	keyStyle := lipgloss.NewStyle().Foreground(styles.Primary).Bold(true)
	labelStyle := lipgloss.NewStyle().Foreground(styles.TextPrimary)
	mutedStyle := lipgloss.NewStyle().Foreground(styles.TextMuted)

	titleText := ""
	statusText := ""
	if stack := m.quickMenuStackPreview(); stack != nil {
		titleText = stack.Name
		statusText = fmt.Sprintf("%d/%d up", stack.RunningCount, stack.ContainerCount)
	} else if c := m.quickMenuContainer; c != nil {
		titleText = c.Name
		statusText = c.State
	}

	// Menu width adapts to target name
	menuInner := len(titleText) + 6
	if menuInner < 28 {
		menuInner = 28
	}
	if menuInner > baseW-8 {
		menuInner = baseW - 8
	}

	// Title bar: target name centered, state/summary on the right
	name := titleText
	if len(name) > menuInner-2 {
		name = name[:menuInner-2]
	}
	nameStyled := lipgloss.NewStyle().Foreground(styles.TextPrimary).Bold(true).Render(name)
	stateStyled := lipgloss.NewStyle().Foreground(styles.TextSecondary).Render(statusText)
	if c := m.quickMenuContainer; c != nil {
		stateStyled = lipgloss.NewStyle().
			Foreground(styles.ContainerStateColor(c.State)).
			Render(c.State)
	}
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

	popupW := lipgloss.Width(popup)
	popupH := lipgloss.Height(popup)

	bgLayer := lipgloss.NewLayer(base).X(0).Y(0).Z(0)
	fgLayer := lipgloss.NewLayer(popup).
		X((baseW - popupW) / 2).
		Y((baseH - popupH) / 2).
		Z(1)

	canvas := lipgloss.NewCanvas(baseW, baseH)
	canvas.Compose(bgLayer).Compose(fgLayer)
	return canvas.Render()
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
	layout := m.measureDashboardLayout()
	m.containerRows = layout.containerRows
	m.containerStartY = layout.containerStartY

	var infoPanelHeight int
	switch {
	case m.detailStackName != "":
		infoPanelHeight = panels.StackDetailInfoPanelHeight(m.detailStackData(), m.width)
	default:
		infoPanelHeight = panels.DetailInfoPanelHeight(m.detailContainer, m.detailMeta, m.systemData.Hostname, m.width)
	}
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
	if m.width > 0 && m.height > 0 {
		m.containerRows = m.measureDashboardLayout().containerRows
	}
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
		unhealthy  int
		starting   int
		stopped    int
		cpuTotal   float64
		memTotal   uint64
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
		} else {
			g.stopped++
		}
		switch c.Health {
		case "unhealthy":
			g.unhealthy++
		case "starting":
			g.starting++
		}
		g.cpuTotal += c.CPUPerc
		g.memTotal += c.MemUsed
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
			UnhealthyCount: g.unhealthy,
			StartingCount:  g.starting,
			StoppedCount:   g.stopped,
			CPUPercTotal:   g.cpuTotal,
			MemUsedTotal:   g.memTotal,
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

	// Restore selection by semantic target
	if m.selectedTarget != "" {
		for i, item := range m.displayItems {
			if item.Kind == DisplayGroup && m.selectedTarget == "s:"+item.StackName {
				m.selectedIndex = i
				break
			}
			if item.Kind == DisplayContainer && item.Container != nil && m.selectedTarget == "c:"+item.Container.ID {
				m.selectedIndex = i
				break
			}
		}
	}

	if m.selectedIndex >= len(m.displayItems) {
		m.selectedIndex = max(0, len(m.displayItems)-1)
	}
	m.ensureVisible()
}

func (m Model) View() tea.View {
	if m.width == 0 {
		return tea.NewView("Loading...")
	}
	var s string
	switch m.viewMode {
	case ViewDetail:
		s = m.renderDetail()
	default:
		s = m.renderDashboard()
	}
	v := tea.NewView(s)
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	return v
}

func (m Model) renderDetail() string {
	logSearch := panels.LogSearch{
		Active:      m.logSearchActive,
		Query:       m.logSearchInput.Value(),
		Total:       len(m.logSearchMatches),
		CurrentLine: -1,
	}
	if m.logSearchActive {
		logSearch.InputView = m.logSearchInput.View()
	}
	if len(m.logSearchMatches) > 0 && m.logSearchIndex >= 0 && m.logSearchIndex < len(m.logSearchMatches) {
		logSearch.Current = m.logSearchIndex + 1
		logSearch.CurrentLine = m.logSearchMatches[m.logSearchIndex]
		logSearch.MatchSet = make(map[int]bool, len(m.logSearchMatches))
		for _, idx := range m.logSearchMatches {
			logSearch.MatchSet[idx] = true
		}
	}

	var detail string
	if stack := m.detailStackData(); stack != nil {
		detail = panels.RenderStackDetail(
			stack,
			m.detailLogs, m.detailLogsErr,
			m.confirmAction, m.actionResult,
			m.detailScrollOffset, m.width, m.height,
			m.logFollowing, logSearch)
	} else {
		detail = panels.RenderDetail(
			m.detailContainer, m.detailMeta, m.systemData.Hostname, m.detailLogs, m.detailLogsErr,
			m.confirmAction, m.actionResult,
			m.detailScrollOffset, m.width, m.height,
			m.logFollowing, logSearch)
	}
	return lipgloss.NewStyle().
		Background(styles.BgBase).
		Width(m.width).
		Height(m.height).
		Render(detail)
}

func (m Model) renderDashboard() string {
	layout := m.measureDashboardLayout()

	// Containers — sized to exactly fill remaining space
	panelItems := make([]panels.ContainerDisplayItem, len(m.displayItems))
	for i, item := range m.displayItems {
		panelItems[i] = panels.ContainerDisplayItem{
			IsGroup:        item.Kind == DisplayGroup,
			StackName:      item.StackName,
			ContainerCount: item.ContainerCount,
			RunningCount:   item.RunningCount,
			UnhealthyCount: item.UnhealthyCount,
			StartingCount:  item.StartingCount,
			StoppedCount:   item.StoppedCount,
			Collapsed:      item.Collapsed,
			Container:      item.Container,
		}
	}

	containersPanel := panels.RenderContainers(
		panelItems,
		m.dockerData.Running, m.dockerData.Total,
		m.scrollOffset, m.selectedIndex, layout.containerRows, m.width,
		m.focusedPanel == PanelContainers,
		m.searchInput, m.filtering,
		m.TestMode)

	// Quick-action menu overlay
	if m.quickMenuOpen {
		containersPanel = m.renderQuickMenu(containersPanel)
	}

	view := lipgloss.JoinVertical(lipgloss.Left,
		layout.header, layout.topRow, containersPanel, layout.bottomSection)

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

func (m Model) measureDashboardLayout() dashboardLayoutMetrics {
	if m.width <= 0 || m.height <= 0 {
		return dashboardLayoutMetrics{}
	}

	header := panels.RenderHeader(m.systemData, m.width, m.TestMode)

	topHeight := 11
	var topRow string
	if m.isNarrow() {
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

	previewBar := panels.RenderPreview(
		m.selectedContainer(),
		m.selectedStackPreview(),
		m.confirmAction,
		m.dashboardActionTargetName,
		m.actionResult,
		m.width,
	)
	helpBar := panels.RenderHelp(panels.DefaultBindings, m.refreshing, m.width)
	notifBar := renderNotificationBar(&m.notifications, m.width)

	bottomBars := []string{previewBar}
	if notifBar != "" {
		bottomBars = append(bottomBars, notifBar)
	}
	bottomBars = append(bottomBars, helpBar)
	bottomSection := lipgloss.JoinVertical(lipgloss.Left, bottomBars...)

	headerLines := renderedLineCount(header)
	topLines := renderedLineCount(topRow)
	bottomLines := renderedLineCount(bottomSection)
	containerChrome := 4 // border(2) + title(1) + column header(1)
	if m.filtering || m.searchInput.Value() != "" {
		containerChrome++
	}

	containerRows := m.height - headerLines - topLines - bottomLines - containerChrome
	if containerRows < 0 {
		containerRows = 0
	}

	containerStartY := headerLines + topLines + 3
	if m.filtering || m.searchInput.Value() != "" {
		containerStartY++
	}

	return dashboardLayoutMetrics{
		header:          header,
		topRow:          topRow,
		bottomSection:   bottomSection,
		containerRows:   containerRows,
		containerStartY: containerStartY,
	}
}

func (m Model) selectedContainer() *collector.Container {
	if m.selectedIndex >= 0 && m.selectedIndex < len(m.displayItems) {
		return m.displayItems[m.selectedIndex].Container
	}
	return nil
}

func (m Model) selectedStackPreview() *panels.StackPreview {
	if m.selectedIndex < 0 || m.selectedIndex >= len(m.displayItems) {
		return nil
	}

	item := m.displayItems[m.selectedIndex]
	if item.Kind != DisplayGroup {
		return nil
	}

	return m.stackPreviewByName(item.StackName)
}

func (m Model) quickMenuStackPreview() *panels.StackPreview {
	if m.quickMenuStackName == "" {
		return nil
	}

	return m.stackPreviewByName(m.quickMenuStackName)
}

func (m *Model) trackSelection() {
	if m.selectedIndex >= 0 && m.selectedIndex < len(m.displayItems) {
		item := m.displayItems[m.selectedIndex]
		if item.Kind == DisplayGroup {
			m.selectedTarget = "s:" + item.StackName
		} else if item.Kind == DisplayContainer && item.Container != nil {
			m.selectedTarget = "c:" + item.Container.ID
		}
	}
}

func (m *Model) clearDashboardAction() {
	m.dashboardActionContainerID = ""
	m.dashboardActionStackName = ""
	m.dashboardActionTargetName = ""
}

func (m *Model) clearDetailView() {
	m.stopFollowing()
	m.viewMode = ViewDashboard
	m.detailContainer = nil
	m.detailContainerID = ""
	m.detailStackName = ""
	m.detailLogs = nil
	m.detailLogsErr = nil
	m.detailMeta = nil
	m.detailMetaErr = nil
	m.detailScrollOffset = 0
	m.confirmAction = ""
	m.actionResult = ""
	m.logSearchActive = false
	m.logSearchInput.Blur()
	m.logSearchInput.SetValue("")
	m.logSearchMatches = nil
	m.logSearchIndex = 0
}

func (m *Model) recomputeLogSearchMatches() {
	query := strings.ToLower(m.logSearchInput.Value())
	m.logSearchMatches = nil
	m.logSearchIndex = 0
	if query == "" {
		return
	}
	for i, line := range m.detailLogs {
		if strings.Contains(strings.ToLower(line), query) {
			m.logSearchMatches = append(m.logSearchMatches, i)
		}
	}
	if len(m.logSearchMatches) > 0 {
		m.scrollToLogLine(m.logSearchMatches[0])
	}
}

func (m *Model) scrollToLogLine(lineIdx int) {
	target := lineIdx - m.detailLogRows/2
	if target < 0 {
		target = 0
	}
	maxScroll := len(m.detailLogs) - m.detailLogRows
	if maxScroll < 0 {
		maxScroll = 0
	}
	if target > maxScroll {
		target = maxScroll
	}
	m.detailScrollOffset = target
}

func (m Model) stackPreviewByName(stackName string) *panels.StackPreview {
	stackName = strings.TrimSpace(stackName)
	if stackName == "" {
		return nil
	}

	preview := &panels.StackPreview{Name: stackName}
	for _, c := range m.dockerData.Containers {
		if c.Stack != stackName {
			continue
		}
		preview.ContainerCount++
		if c.State == "running" {
			preview.RunningCount++
		} else {
			preview.StoppedCount++
		}
		switch c.Health {
		case "unhealthy":
			preview.UnhealthyCount++
		case "starting":
			preview.StartingCount++
		}
		preview.CPUPerc += c.CPUPerc
		preview.MemUsed += c.MemUsed
	}
	return preview
}

func (m Model) detailStackData() *panels.StackDetail {
	if m.detailStackName == "" {
		return nil
	}

	preview := m.stackPreviewByName(m.detailStackName)
	if preview == nil {
		return nil
	}

	detail := &panels.StackDetail{
		Name:           preview.Name,
		ContainerCount: preview.ContainerCount,
		RunningCount:   preview.RunningCount,
		UnhealthyCount: preview.UnhealthyCount,
		StartingCount:  preview.StartingCount,
		StoppedCount:   preview.StoppedCount,
		CPUPerc:        preview.CPUPerc,
		MemUsed:        preview.MemUsed,
	}

	for _, c := range m.dockerData.Containers {
		if c.Stack != m.detailStackName {
			continue
		}
		health := c.Health
		if health == "" {
			health = "-"
		}
		detail.Containers = append(detail.Containers, panels.StackDetailContainer{
			Name:   c.Name,
			State:  c.State,
			Health: health,
			Image:  c.Image,
			Ports:  collector.FormatPorts(c.Ports),
		})
	}
	sort.Slice(detail.Containers, func(i, j int) bool {
		return detail.Containers[i].Name < detail.Containers[j].Name
	})

	return detail
}

func summarizeStackActionTargets(names []string, limit int) string {
	if len(names) == 0 || limit <= 0 {
		return ""
	}
	if len(names) <= limit {
		return strings.Join(names, ", ")
	}
	return fmt.Sprintf("%s +%d more", strings.Join(names[:limit], ", "), len(names)-limit)
}

func formatStackActionFailureResult(msg StackActionMsg) string {
	targets := summarizeStackActionTargets(msg.Failed, 3)
	if targets == "" {
		return fmt.Sprintf("Error: %s stack %s failed", msg.Action, msg.StackName)
	}
	return fmt.Sprintf("Error: %s stack %s failed for %s", msg.Action, msg.StackName, targets)
}

func formatStackActionFailureNotification(msg StackActionMsg) string {
	targets := summarizeStackActionTargets(msg.Failed, 4)
	if targets == "" {
		return fmt.Sprintf("Stack %s %s failed", msg.StackName, msg.Action)
	}
	return fmt.Sprintf("Stack %s %s failed for %s", msg.StackName, msg.Action, targets)
}

func renderedLineCount(s string) int {
	if s == "" {
		return 0
	}
	return strings.Count(s, "\n") + 1
}

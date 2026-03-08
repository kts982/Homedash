package ui

import (
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/kostas/homedash/internal/collector"
	"github.com/kostas/homedash/internal/state"
)

type Panel int

const (
	PanelSystem Panel = iota
	PanelContainers
	PanelWeather
	panelCount
)

func (m *Model) enterDetailView(c *collector.Container) tea.Cmd {
	m.viewMode = ViewDetail
	m.detailContainer = c
	m.detailContainerID = c.ID
	m.detailStackName = ""
	m.detailScrollOffset = 0
	m.detailLogs = nil
	m.detailLogsErr = nil
	m.detailMeta = nil
	m.detailMetaErr = nil
	m.confirmAction = ""
	m.actionResult = ""
	m.recalcLayout()

	if m.TestMode {
		return tea.Batch(
			func() tea.Msg { return collectMockLogsCmd(c.ID, 50) },
			func() tea.Msg { return collectMockDetailCmd(c.ID) },
		)
	}

	return tea.Batch(
		m.startFollowing(),
		collectDetailCmd(c.ID),
	)
}

func (m *Model) enterStackDetailView(stackName string) tea.Cmd {
	m.viewMode = ViewDetail
	m.detailContainer = nil
	m.detailContainerID = ""
	m.detailStackName = stackName
	m.detailScrollOffset = 0
	m.detailLogs = nil
	m.detailLogsErr = nil
	m.detailMeta = nil
	m.detailMetaErr = nil
	m.confirmAction = ""
	m.actionResult = ""
	m.recalcLayout()

	if m.TestMode {
		return func() tea.Msg { return collectMockStackLogsCmd(stackName, 50) }
	}

	return m.startFollowing()
}

func handleKey(msg tea.KeyPressMsg, m *Model) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		if m.collapseSeq > m.lastSavedSeq {
			_ = state.Save(m.collapsedStacks)
		}
		return m, tea.Quit
	case "q":
		if m.viewMode == ViewDetail {
			m.clearDetailView()
			return m, nil
		}
		if m.collapseSeq > m.lastSavedSeq {
			_ = state.Save(m.collapsedStacks)
		}
		return m, tea.Quit
	}

	// Detail view keys
	if m.viewMode == ViewDetail {
		return handleDetailKey(msg, m)
	}

	// Quick-action menu intercepts all keys when open
	if m.quickMenuOpen {
		return handleQuickMenuKey(msg, m)
	}

	// Dashboard keys
	return handleDashboardKey(msg, m)
}

func handleQuickMenuKey(msg tea.KeyPressMsg, m *Model) (tea.Model, tea.Cmd) {
	items := m.currentQuickMenuItems()
	if len(items) == 0 {
		m.closeQuickMenu()
		return m, nil
	}

	if m.quickMenuIndex >= len(items) {
		m.quickMenuIndex = len(items) - 1
	}

	switch msg.String() {
	case "esc", " ", "space":
		m.closeQuickMenu()
	case "j", "down":
		if m.quickMenuIndex < len(items)-1 {
			m.quickMenuIndex++
		}
	case "k", "up":
		if m.quickMenuIndex > 0 {
			m.quickMenuIndex--
		}
	case "enter":
		return m.executeQuickMenuItem(items[m.quickMenuIndex])
	case "s":
		if quickMenuHasAction(items, "stop") {
			return m.executeQuickMenuAction("stop")
		}
	case "S":
		if quickMenuHasAction(items, "start") {
			return m.executeQuickMenuAction("start")
		}
	case "R":
		if quickMenuHasAction(items, "restart") {
			return m.executeQuickMenuAction("restart")
		}
	}
	return m, nil
}

func quickMenuHasAction(items []quickMenuItem, action string) bool {
	for _, item := range items {
		if item.action == action {
			return true
		}
	}
	return false
}

func (m *Model) closeQuickMenu() {
	m.quickMenuOpen = false
	m.quickMenuIndex = 0
	m.quickMenuContainerID = ""
	m.quickMenuContainer = nil
	m.quickMenuStackName = ""
}

func (m *Model) executeQuickMenuItem(item quickMenuItem) (tea.Model, tea.Cmd) {
	if item.action == "logs" {
		if m.quickMenuStackName != "" {
			stackName := m.quickMenuStackName
			m.closeQuickMenu()
			return m, m.enterStackDetailView(stackName)
		}
		if m.quickMenuContainer != nil {
			container := m.quickMenuContainer
			m.closeQuickMenu()
			cmd := m.enterDetailView(container)
			return m, cmd
		}
	}
	return m.executeQuickMenuAction(item.action)
}

func (m *Model) executeQuickMenuAction(action string) (tea.Model, tea.Cmd) {
	if m.quickMenuStackName != "" {
		stackName := m.quickMenuStackName
		m.closeQuickMenu()
		return m, stackActionCmd(m.dockerData.Containers, stackName, action)
	}

	containerID := m.quickMenuContainerID
	m.closeQuickMenu()
	return m, containerActionCmd(containerID, action)
}

func (m *Model) setDashboardContainerAction(action string, c *collector.Container) {
	m.confirmAction = action
	m.dashboardActionContainerID = c.ID
	m.dashboardActionStackName = ""
	m.dashboardActionTargetName = c.Name
}

func (m *Model) setDashboardStackAction(action, stackName string) {
	m.confirmAction = action
	m.dashboardActionContainerID = ""
	m.dashboardActionStackName = stackName
	m.dashboardActionTargetName = stackName
}

func stackActionAvailable(item DisplayItem, action string) bool {
	if item.Kind != DisplayGroup {
		return false
	}

	switch action {
	case "start":
		return item.StoppedCount > 0
	case "stop", "restart":
		return item.RunningCount > 0
	default:
		return false
	}
}

func handleDashboardKey(msg tea.KeyPressMsg, m *Model) (tea.Model, tea.Cmd) {
	// Handle confirmation first when an action is pending
	if m.confirmAction != "" && (m.dashboardActionContainerID != "" || m.dashboardActionStackName != "") {
		switch msg.String() {
		case "y":
			action := m.confirmAction
			containerID := m.dashboardActionContainerID
			stackName := m.dashboardActionStackName
			m.confirmAction = ""
			m.clearDashboardAction()
			if containerID != "" {
				return m, containerActionCmd(containerID, action)
			}
			if stackName != "" {
				return m, stackActionCmd(m.dockerData.Containers, stackName, action)
			}
		case "n", "esc":
			m.confirmAction = ""
			m.clearDashboardAction()
		}
		return m, nil
	}

	switch msg.String() {
	case "tab":
		m.focusedPanel = (m.focusedPanel + 1) % panelCount
	case "shift+tab":
		m.focusedPanel = (m.focusedPanel - 1 + panelCount) % panelCount
	case "j", "down":
		if m.focusedPanel == PanelContainers {
			maxIdx := len(m.displayItems) - 1
			if maxIdx < 0 {
				maxIdx = 0
			}
			if m.selectedIndex < maxIdx {
				m.selectedIndex++
			}
			m.trackSelection()
			m.ensureVisible()
		}
	case "k", "up":
		if m.focusedPanel == PanelContainers {
			if m.selectedIndex > 0 {
				m.selectedIndex--
			}
			m.trackSelection()
			m.ensureVisible()
		}
	case "enter":
		if m.focusedPanel == PanelContainers && len(m.displayItems) > 0 {
			item := m.displayItems[m.selectedIndex]
			if item.Kind == DisplayGroup {
				m.collapsedStacks[item.StackName] = !m.collapsedStacks[item.StackName]
				m.rebuildDisplayItems()
				m.ensureVisible()
				if m.searchInput.Value() == "" {
					m.collapseSeq++
					return m, collapseSaveTickCmd(m.collapseSeq)
				}
			} else if item.Kind == DisplayContainer && item.Container != nil {
				cmd := m.enterDetailView(item.Container)
				return m, cmd
			}
		}
	case " ", "space":
		if m.focusedPanel == PanelContainers && m.selectedIndex < len(m.displayItems) {
			item := m.displayItems[m.selectedIndex]
			if item.Kind == DisplayGroup {
				items := stackQuickMenuItems(item.RunningCount, item.StoppedCount)
				if len(items) > 0 {
					m.quickMenuOpen = true
					m.quickMenuIndex = 0
					m.quickMenuStackName = item.StackName
					m.quickMenuContainerID = ""
					m.quickMenuContainer = nil
				}
			} else if item.Kind == DisplayContainer && item.Container != nil {
				m.quickMenuOpen = true
				m.quickMenuIndex = 0
				m.quickMenuContainerID = item.Container.ID
				m.quickMenuContainer = item.Container
				m.quickMenuStackName = ""
			}
		}
	case "s":
		if m.focusedPanel == PanelContainers && m.selectedIndex < len(m.displayItems) {
			item := m.displayItems[m.selectedIndex]
			if item.Kind == DisplayGroup && stackActionAvailable(item, "stop") {
				m.setDashboardStackAction("stop", item.StackName)
			} else if item.Kind == DisplayContainer && item.Container != nil && item.Container.State == "running" {
				m.setDashboardContainerAction("stop", item.Container)
			}
		}
	case "S":
		if m.focusedPanel == PanelContainers && m.selectedIndex < len(m.displayItems) {
			item := m.displayItems[m.selectedIndex]
			if item.Kind == DisplayGroup && stackActionAvailable(item, "start") {
				m.setDashboardStackAction("start", item.StackName)
			} else if item.Kind == DisplayContainer && item.Container != nil && item.Container.State != "running" {
				m.setDashboardContainerAction("start", item.Container)
			}
		}
	case "R":
		if m.focusedPanel == PanelContainers && m.selectedIndex < len(m.displayItems) {
			item := m.displayItems[m.selectedIndex]
			if item.Kind == DisplayGroup && stackActionAvailable(item, "restart") {
				m.setDashboardStackAction("restart", item.StackName)
			} else if item.Kind == DisplayContainer && item.Container != nil && item.Container.State == "running" {
				m.setDashboardContainerAction("restart", item.Container)
			}
		}
	case "r":
		m.refreshing = true
		if m.TestMode {
			return m, tea.Batch(
				func() tea.Msg { return collectMockSystemCmd() },
				func() tea.Msg { return collectMockDockerCmd() },
				func() tea.Msg { return collectMockWeatherCmd() },
			)
		}
		return m, tea.Batch(
			func() tea.Msg { return collectSystemCmd(m.disks) },
			func() tea.Msg { return collectDockerCmd() },
			func() tea.Msg { return collectWeatherCmd() },
		)
	case "/":
		m.filtering = true
		m.focusedPanel = PanelContainers
		m.recalcLayout()
		return m, m.searchInput.Focus()
	case "esc":
		if m.searchInput.Value() != "" {
			m.searchInput.SetValue("")
			m.rebuildDisplayItems()
			m.recalcLayout()
		}
	}
	return m, nil
}

func handleMouse(msg tea.MouseMsg, m *Model) (tea.Model, tea.Cmd) {
	layout := m.measureDashboardLayout()
	m.containerRows = layout.containerRows
	m.containerStartY = layout.containerStartY

	mm := msg.Mouse()

	switch msg.(type) {
	case tea.MouseWheelMsg:
		switch mm.Button {
		case tea.MouseWheelUp:
			if m.focusedPanel == PanelContainers {
				m.scrollOffset -= 3
				if m.scrollOffset < 0 {
					m.scrollOffset = 0
				}
				// Keep selectedIndex within visible range
				if m.selectedIndex >= m.scrollOffset+m.containerRows {
					m.selectedIndex = m.scrollOffset + m.containerRows - 1
				}
				if m.selectedIndex < m.scrollOffset {
					m.selectedIndex = m.scrollOffset
				}
			}
		case tea.MouseWheelDown:
			if m.focusedPanel == PanelContainers {
				maxOffset := len(m.displayItems) - m.containerRows
				if maxOffset < 0 {
					maxOffset = 0
				}
				m.scrollOffset += 3
				if m.scrollOffset > maxOffset {
					m.scrollOffset = maxOffset
				}
				// Keep selectedIndex within visible range
				if m.selectedIndex < m.scrollOffset {
					m.selectedIndex = m.scrollOffset
				}
				if m.selectedIndex >= m.scrollOffset+m.containerRows && m.containerRows > 0 {
					m.selectedIndex = m.scrollOffset + m.containerRows - 1
				}
			}
		}
	case tea.MouseClickMsg:
		if mm.Button == tea.MouseLeft {
			// Click on container row
			row := mm.Y - m.containerStartY
			if row < 0 || row >= m.containerRows {
				return m, nil
			}
			idx := row + m.scrollOffset
			if idx < 0 || idx >= len(m.displayItems) {
				return m, nil
			}
			m.focusedPanel = PanelContainers
			item := m.displayItems[idx]
			now := time.Now()
			isDoubleClick := idx == m.lastClickIndex && now.Sub(m.lastClickTime) < 400*time.Millisecond
			m.lastClickTime = now
			m.lastClickIndex = idx
			if item.Kind == DisplayGroup {
				// Click on group header toggles collapse
				m.selectedIndex = idx
				m.trackSelection()
				m.collapsedStacks[item.StackName] = !m.collapsedStacks[item.StackName]
				m.rebuildDisplayItems()
				m.ensureVisible()
				if m.searchInput.Value() == "" {
					m.collapseSeq++
					return m, collapseSaveTickCmd(m.collapseSeq)
				}
			} else if item.Kind == DisplayContainer && item.Container != nil {
				m.selectedIndex = idx
				m.trackSelection()
				if isDoubleClick {
					// Double-click opens detail view
					return m, m.enterDetailView(item.Container)
				}
			} else {
				m.selectedIndex = idx
			}
		}
	}
	return m, nil
}

func handleDetailMouse(msg tea.MouseMsg, m *Model) (tea.Model, tea.Cmd) {
	mm := msg.Mouse()
	switch msg.(type) {
	case tea.MouseWheelMsg:
		switch mm.Button {
		case tea.MouseWheelUp:
			m.detailScrollOffset -= 3
			if m.detailScrollOffset < 0 {
				m.detailScrollOffset = 0
			}
		case tea.MouseWheelDown:
			maxScroll := len(m.detailLogs) - m.detailLogRows
			if maxScroll < 0 {
				maxScroll = 0
			}
			m.detailScrollOffset += 3
			if m.detailScrollOffset > maxScroll {
				m.detailScrollOffset = maxScroll
			}
		}
	}
	return m, nil
}

func detailMaxScroll(m *Model) int {
	maxScroll := len(m.detailLogs) - m.detailLogRows
	if maxScroll < 0 {
		maxScroll = 0
	}
	return maxScroll
}

func detailPageStep(m *Model) int {
	step := m.detailLogRows / 2
	if step < 1 {
		step = 1
	}
	return step
}

func handleDetailKey(msg tea.KeyPressMsg, m *Model) (tea.Model, tea.Cmd) {
	// Handle confirmation first
	if m.confirmAction != "" {
		switch msg.String() {
		case "y":
			action := m.confirmAction
			m.confirmAction = ""
			if m.detailStackName != "" {
				return m, stackActionCmd(m.dockerData.Containers, m.detailStackName, action)
			}
			return m, containerActionCmd(m.detailContainerID, action)
		case "n", "esc":
			m.confirmAction = ""
		}
		return m, nil
	}

	switch msg.String() {
	case "esc":
		if m.logSearchInput.Value() != "" {
			m.logSearchInput.SetValue("")
			m.logSearchMatches = nil
			m.logSearchIndex = 0
		} else {
			m.clearDetailView()
		}
	case "/":
		m.logSearchActive = true
		return m, m.logSearchInput.Focus()
	case "n":
		if len(m.logSearchMatches) > 0 {
			m.logSearchIndex = (m.logSearchIndex + 1) % len(m.logSearchMatches)
			m.scrollToLogLine(m.logSearchMatches[m.logSearchIndex])
		}
	case "N":
		if len(m.logSearchMatches) > 0 {
			m.logSearchIndex = (m.logSearchIndex - 1 + len(m.logSearchMatches)) % len(m.logSearchMatches)
			m.scrollToLogLine(m.logSearchMatches[m.logSearchIndex])
		}
	case "f":
		if m.logFollowing {
			m.stopFollowing()
		} else {
			return m, m.startFollowing()
		}
	case "j", "down":
		maxScroll := detailMaxScroll(m)
		if m.detailScrollOffset < maxScroll {
			m.detailScrollOffset++
		}
	case "k", "up":
		if m.detailScrollOffset > 0 {
			m.detailScrollOffset--
		}
	case "ctrl+d", "pgdown":
		maxScroll := detailMaxScroll(m)
		m.detailScrollOffset += detailPageStep(m)
		if m.detailScrollOffset > maxScroll {
			m.detailScrollOffset = maxScroll
		}
	case "ctrl+u", "pgup":
		m.detailScrollOffset -= detailPageStep(m)
		if m.detailScrollOffset < 0 {
			m.detailScrollOffset = 0
		}
	case "g", "home":
		m.detailScrollOffset = 0
	case "G", "end":
		m.detailScrollOffset = detailMaxScroll(m)
	case "l":
		m.stopFollowing()
		m.detailLogs = nil
		m.detailLogsErr = nil
		if m.TestMode {
			if m.detailStackName != "" {
				return m, func() tea.Msg { return collectMockStackLogsCmd(m.detailStackName, 50) }
			}
			return m, func() tea.Msg { return collectMockLogsCmd(m.detailContainerID, 50) }
		}
		if m.detailStackName != "" {
			return m, collectStackLogsCmd(m.dockerData.Containers, m.detailStackName, 50)
		}
		return m, collectLogsCmd(m.detailContainerID, 50)
	case "s":
		if stack := m.detailStackData(); stack != nil {
			if stack.RunningCount > 0 {
				m.stopFollowing()
				m.confirmAction = "stop"
			}
		} else if m.detailContainer != nil && m.detailContainer.State == "running" {
			m.stopFollowing()
			m.confirmAction = "stop"
		}
	case "S":
		if stack := m.detailStackData(); stack != nil {
			if stack.StoppedCount > 0 {
				m.stopFollowing()
				m.confirmAction = "start"
			}
		} else if m.detailContainer != nil && m.detailContainer.State != "running" {
			m.stopFollowing()
			m.confirmAction = "start"
		}
	case "R":
		if stack := m.detailStackData(); stack != nil {
			if stack.RunningCount > 0 {
				m.stopFollowing()
				m.confirmAction = "restart"
			}
		} else if m.detailContainer != nil && m.detailContainer.State == "running" {
			m.stopFollowing()
			m.confirmAction = "restart"
		}
	}
	return m, nil
}

package ui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
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
	m.detailScrollOffset = 0
	m.detailLogs = nil
	m.detailLogsErr = nil
	m.detailMeta = nil
	m.detailMetaErr = nil
	m.confirmAction = ""
	m.actionResult = ""
	return tea.Batch(
		collectLogsCmd(c.ID, 50),
		collectDetailCmd(c.ID),
	)
}

func handleKey(msg tea.KeyMsg, m *Model) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		if m.collapseSeq > m.lastSavedSeq {
			_ = state.Save(m.collapsedStacks)
		}
		return m, tea.Quit
	case "q":
		if m.viewMode == ViewDetail {
			m.stopFollowing()
			m.viewMode = ViewDashboard
			m.detailContainer = nil
			m.detailLogs = nil
			m.detailMeta = nil
			m.detailMetaErr = nil
			m.confirmAction = ""
			m.actionResult = ""
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

func handleQuickMenuKey(msg tea.KeyMsg, m *Model) (tea.Model, tea.Cmd) {
	c := m.quickMenuContainer
	if c == nil {
		m.quickMenuOpen = false
		return m, nil
	}

	items := quickMenuItems(c.State)
	if m.quickMenuIndex >= len(items) {
		m.quickMenuIndex = len(items) - 1
	}

	switch msg.String() {
	case "esc", " ":
		m.quickMenuOpen = false
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
		if c.State == "running" {
			return m.executeQuickMenuAction("stop")
		}
	case "S":
		if c.State != "running" {
			return m.executeQuickMenuAction("start")
		}
	case "R":
		if c.State == "running" {
			return m.executeQuickMenuAction("restart")
		}
	}
	return m, nil
}

func (m *Model) executeQuickMenuItem(item quickMenuItem) (tea.Model, tea.Cmd) {
	if item.action == "logs" {
		m.quickMenuOpen = false
		cmd := m.enterDetailView(m.quickMenuContainer)
		return m, cmd
	}
	return m.executeQuickMenuAction(item.action)
}

func (m *Model) executeQuickMenuAction(action string) (tea.Model, tea.Cmd) {
	containerID := m.quickMenuContainerID
	m.quickMenuOpen = false
	return m, containerActionCmd(containerID, action)
}

func handleDashboardKey(msg tea.KeyMsg, m *Model) (tea.Model, tea.Cmd) {
	// Handle confirmation first when an action is pending
	if m.confirmAction != "" && m.dashboardActionContainerID != "" {
		switch msg.String() {
		case "y":
			action := m.confirmAction
			containerID := m.dashboardActionContainerID
			m.confirmAction = ""
			m.dashboardActionContainerID = ""
			m.dashboardActionContainerName = ""
			return m, containerActionCmd(containerID, action)
		case "n", "esc":
			m.confirmAction = ""
			m.dashboardActionContainerID = ""
			m.dashboardActionContainerName = ""
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
			m.ensureVisible()
		}
	case "k", "up":
		if m.focusedPanel == PanelContainers {
			if m.selectedIndex > 0 {
				m.selectedIndex--
			}
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
	case " ":
		if m.focusedPanel == PanelContainers && m.selectedIndex < len(m.displayItems) {
			item := m.displayItems[m.selectedIndex]
			if item.Kind == DisplayContainer && item.Container != nil {
				m.quickMenuOpen = true
				m.quickMenuIndex = 0
				m.quickMenuContainerID = item.Container.ID
				m.quickMenuContainer = item.Container
			}
		}
	case "s":
		if m.focusedPanel == PanelContainers && m.selectedIndex < len(m.displayItems) {
			item := m.displayItems[m.selectedIndex]
			if item.Kind == DisplayContainer && item.Container != nil && item.Container.State == "running" {
				m.confirmAction = "stop"
				m.dashboardActionContainerID = item.Container.ID
				m.dashboardActionContainerName = item.Container.Name
			}
		}
	case "S":
		if m.focusedPanel == PanelContainers && m.selectedIndex < len(m.displayItems) {
			item := m.displayItems[m.selectedIndex]
			if item.Kind == DisplayContainer && item.Container != nil && item.Container.State != "running" {
				m.confirmAction = "start"
				m.dashboardActionContainerID = item.Container.ID
				m.dashboardActionContainerName = item.Container.Name
			}
		}
	case "R":
		if m.focusedPanel == PanelContainers && m.selectedIndex < len(m.displayItems) {
			item := m.displayItems[m.selectedIndex]
			if item.Kind == DisplayContainer && item.Container != nil && item.Container.State == "running" {
				m.confirmAction = "restart"
				m.dashboardActionContainerID = item.Container.ID
				m.dashboardActionContainerName = item.Container.Name
			}
		}
	case "r":
		m.refreshing = true
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
	}
	return m, nil
}

func handleMouse(msg tea.MouseMsg, m *Model) (tea.Model, tea.Cmd) {
	switch msg.Button {
	case tea.MouseButtonWheelUp:
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
	case tea.MouseButtonWheelDown:
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
	case tea.MouseButtonLeft:
		if msg.Action != tea.MouseActionPress {
			return m, nil
		}
		// Click on container row
		row := msg.Y - m.containerStartY
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
			m.collapsedStacks[item.StackName] = !m.collapsedStacks[item.StackName]
			m.rebuildDisplayItems()
			m.ensureVisible()
			if m.searchInput.Value() == "" {
				m.collapseSeq++
				return m, collapseSaveTickCmd(m.collapseSeq)
			}
		} else if item.Kind == DisplayContainer && item.Container != nil {
			m.selectedIndex = idx
			if isDoubleClick {
				// Double-click opens detail view
				return m, m.enterDetailView(item.Container)
			}
		} else {
			m.selectedIndex = idx
		}
	}
	return m, nil
}

func handleDetailMouse(msg tea.MouseMsg, m *Model) (tea.Model, tea.Cmd) {
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		m.detailScrollOffset -= 3
		if m.detailScrollOffset < 0 {
			m.detailScrollOffset = 0
		}
	case tea.MouseButtonWheelDown:
		maxScroll := len(m.detailLogs) - m.detailLogRows
		if maxScroll < 0 {
			maxScroll = 0
		}
		m.detailScrollOffset += 3
		if m.detailScrollOffset > maxScroll {
			m.detailScrollOffset = maxScroll
		}
	}
	return m, nil
}

func handleDetailKey(msg tea.KeyMsg, m *Model) (tea.Model, tea.Cmd) {
	// Handle confirmation first
	if m.confirmAction != "" {
		switch msg.String() {
		case "y":
			action := m.confirmAction
			m.confirmAction = ""
			return m, containerActionCmd(m.detailContainerID, action)
		case "n", "esc":
			m.confirmAction = ""
		}
		return m, nil
	}

	switch msg.String() {
	case "esc":
		m.stopFollowing()
		m.viewMode = ViewDashboard
		m.detailContainer = nil
		m.detailLogs = nil
		m.detailMeta = nil
		m.detailMetaErr = nil
	case "f":
		if m.logFollowing {
			m.stopFollowing()
		} else {
			return m, m.startFollowing()
		}
	case "j", "down":
		maxScroll := len(m.detailLogs) - m.detailLogRows
		if maxScroll < 0 {
			maxScroll = 0
		}
		if m.detailScrollOffset < maxScroll {
			m.detailScrollOffset++
		}
	case "k", "up":
		if m.detailScrollOffset > 0 {
			m.detailScrollOffset--
		}
	case "g", "home":
		m.detailScrollOffset = 0
	case "G", "end":
		maxScroll := len(m.detailLogs) - m.detailLogRows
		if maxScroll < 0 {
			maxScroll = 0
		}
		m.detailScrollOffset = maxScroll
	case "l":
		m.stopFollowing()
		m.detailLogs = nil
		m.detailLogsErr = nil
		return m, collectLogsCmd(m.detailContainerID, 50)
	case "s":
		if m.detailContainer != nil && m.detailContainer.State == "running" {
			m.stopFollowing()
			m.confirmAction = "stop"
		}
	case "S":
		if m.detailContainer != nil && m.detailContainer.State != "running" {
			m.stopFollowing()
			m.confirmAction = "start"
		}
	case "R":
		if m.detailContainer != nil && m.detailContainer.State == "running" {
			m.stopFollowing()
			m.confirmAction = "restart"
		}
	}
	return m, nil
}

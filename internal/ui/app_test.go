package ui

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/kostas/homedash/internal/collector"
	"github.com/kostas/homedash/internal/ui/components"
	"github.com/kostas/homedash/internal/ui/panels"
)

func newTestModel() Model {
	m := Model{
		collapsedStacks: make(map[string]bool),
		containerRows:   10,
		cpuHistory:      components.NewRingBuffer(60),
		diskWarned:      make(map[string]bool),
		shownWarnings:   make(map[string]bool),
	}
	return m
}

func TestRebuildDisplayItemsGrouping(t *testing.T) {
	m := newTestModel()
	m.dockerData = collector.DockerData{
		Containers: []collector.Container{
			{Name: "web", Stack: "myapp", State: "running"},
			{Name: "db", Stack: "myapp", State: "running"},
			{Name: "standalone", Stack: "", State: "running"},
		},
	}

	m.rebuildDisplayItems()

	// Expect: group header + 2 containers + 1 ungrouped
	if len(m.displayItems) != 4 {
		t.Fatalf("len(displayItems) = %d, want 4", len(m.displayItems))
	}
	if m.displayItems[0].Kind != DisplayGroup {
		t.Fatalf("item[0].Kind = %d, want DisplayGroup", m.displayItems[0].Kind)
	}
	if m.displayItems[0].StackName != "myapp" {
		t.Fatalf("item[0].StackName = %q, want %q", m.displayItems[0].StackName, "myapp")
	}
	if m.displayItems[0].ContainerCount != 2 {
		t.Fatalf("item[0].ContainerCount = %d, want 2", m.displayItems[0].ContainerCount)
	}
	if m.displayItems[0].RunningCount != 2 {
		t.Fatalf("item[0].RunningCount = %d, want 2", m.displayItems[0].RunningCount)
	}
	if m.displayItems[1].Kind != DisplayContainer || m.displayItems[1].Container.Name != "web" {
		t.Fatalf("item[1] should be container 'web'")
	}
	if m.displayItems[2].Kind != DisplayContainer || m.displayItems[2].Container.Name != "db" {
		t.Fatalf("item[2] should be container 'db'")
	}
	// Ungrouped at end
	if m.displayItems[3].Kind != DisplayContainer || m.displayItems[3].Container.Name != "standalone" {
		t.Fatalf("item[3] should be ungrouped container 'standalone'")
	}
}

func TestRebuildDisplayItemsCollapsed(t *testing.T) {
	m := newTestModel()
	m.collapsedStacks["myapp"] = true
	m.dockerData = collector.DockerData{
		Containers: []collector.Container{
			{Name: "web", Stack: "myapp", State: "running"},
			{Name: "db", Stack: "myapp", State: "running"},
		},
	}

	m.rebuildDisplayItems()

	// Collapsed: just the group header
	if len(m.displayItems) != 1 {
		t.Fatalf("len(displayItems) = %d, want 1", len(m.displayItems))
	}
	if m.displayItems[0].Kind != DisplayGroup {
		t.Fatal("item[0] should be DisplayGroup")
	}
	if !m.displayItems[0].Collapsed {
		t.Fatal("item[0].Collapsed should be true")
	}
}

func TestRebuildDisplayItemsFilter(t *testing.T) {
	m := newTestModel()
	m.dockerData = collector.DockerData{
		Containers: []collector.Container{
			{Name: "nginx", Stack: "web", State: "running"},
			{Name: "postgres", Stack: "db", State: "running"},
			{Name: "redis", Stack: "db", State: "running"},
		},
	}
	m.searchInput.SetValue("post")

	m.rebuildDisplayItems()

	// Only postgres matches
	if len(m.displayItems) != 2 {
		t.Fatalf("len(displayItems) = %d, want 2 (group + container)", len(m.displayItems))
	}
	if m.displayItems[1].Container.Name != "postgres" {
		t.Fatalf("filtered container = %q, want %q", m.displayItems[1].Container.Name, "postgres")
	}
}

func TestRebuildDisplayItemsFilterAutoExpands(t *testing.T) {
	m := newTestModel()
	m.collapsedStacks["db"] = true
	m.dockerData = collector.DockerData{
		Containers: []collector.Container{
			{Name: "postgres", Stack: "db", State: "running"},
		},
	}
	m.searchInput.SetValue("post")

	m.rebuildDisplayItems()

	// Even though "db" stack is collapsed, filter auto-expands it
	if len(m.displayItems) != 2 {
		t.Fatalf("len(displayItems) = %d, want 2 (group + container)", len(m.displayItems))
	}
}

func TestRebuildDisplayItemsSortedGroups(t *testing.T) {
	m := newTestModel()
	m.dockerData = collector.DockerData{
		Containers: []collector.Container{
			{Name: "c1", Stack: "zebra", State: "running"},
			{Name: "c2", Stack: "alpha", State: "running"},
		},
	}

	m.rebuildDisplayItems()

	// Groups should be sorted alphabetically
	if m.displayItems[0].StackName != "alpha" {
		t.Fatalf("first group = %q, want %q", m.displayItems[0].StackName, "alpha")
	}
	if m.displayItems[2].StackName != "zebra" {
		t.Fatalf("second group = %q, want %q", m.displayItems[2].StackName, "zebra")
	}
}

func TestRebuildDisplayItemsClampsSelectedIndex(t *testing.T) {
	m := newTestModel()
	m.selectedIndex = 10
	m.dockerData = collector.DockerData{
		Containers: []collector.Container{
			{Name: "only-one", Stack: "", State: "running"},
		},
	}

	m.rebuildDisplayItems()

	if m.selectedIndex != 0 {
		t.Fatalf("selectedIndex = %d, want 0 (clamped)", m.selectedIndex)
	}
}

func TestRebuildDisplayItemsEmpty(t *testing.T) {
	m := newTestModel()
	m.selectedIndex = 5

	m.rebuildDisplayItems()

	if len(m.displayItems) != 0 {
		t.Fatalf("len(displayItems) = %d, want 0", len(m.displayItems))
	}
	if m.selectedIndex != 0 {
		t.Fatalf("selectedIndex = %d, want 0", m.selectedIndex)
	}
}

func TestEnsureVisibleScrollsDown(t *testing.T) {
	m := newTestModel()
	m.containerRows = 3
	m.scrollOffset = 0
	m.selectedIndex = 5
	m.displayItems = make([]DisplayItem, 10)

	m.ensureVisible()

	// selectedIndex(5) should be visible: scrollOffset should be at least 3
	if m.selectedIndex < m.scrollOffset || m.selectedIndex >= m.scrollOffset+m.containerRows {
		t.Fatalf("selectedIndex %d not visible with scrollOffset %d and rows %d",
			m.selectedIndex, m.scrollOffset, m.containerRows)
	}
}

func TestEnsureVisibleScrollsUp(t *testing.T) {
	m := newTestModel()
	m.containerRows = 3
	m.scrollOffset = 5
	m.selectedIndex = 2
	m.displayItems = make([]DisplayItem, 10)

	m.ensureVisible()

	if m.scrollOffset != 2 {
		t.Fatalf("scrollOffset = %d, want 2", m.scrollOffset)
	}
}

func TestEnsureVisibleClampsMaxOffset(t *testing.T) {
	m := newTestModel()
	m.containerRows = 5
	m.scrollOffset = 20
	m.selectedIndex = 3
	m.displayItems = make([]DisplayItem, 6)

	m.ensureVisible()

	// maxOffset = 6 - 5 = 1, scrollOffset should be clamped
	maxOffset := len(m.displayItems) - m.containerRows
	if m.scrollOffset > maxOffset {
		t.Fatalf("scrollOffset = %d, want <= %d", m.scrollOffset, maxOffset)
	}
}

func TestEnsureVisibleEmptyList(t *testing.T) {
	m := newTestModel()
	m.containerRows = 5
	m.selectedIndex = 0
	m.scrollOffset = 0

	m.ensureVisible()

	if m.scrollOffset != 0 {
		t.Fatalf("scrollOffset = %d, want 0", m.scrollOffset)
	}
}

func TestRecalcLayoutCountsRunningDetailNetRow(t *testing.T) {
	m := newTestModel()
	m.width = 120
	m.height = 30
	m.detailContainer = &collector.Container{State: "running"}

	m.recalcLayout()

	if m.detailLogRows != 18 {
		t.Fatalf("detailLogRows = %d, want 18 for a running container detail view", m.detailLogRows)
	}
}

func TestEnterDetailViewRecalculatesDetailLayout(t *testing.T) {
	m := newTestModel()
	m.width = 120
	m.height = 30

	m.recalcLayout()
	if m.detailLogRows != 19 {
		t.Fatalf("baseline detailLogRows = %d, want 19 before selecting a container", m.detailLogRows)
	}

	m.enterDetailView(&collector.Container{ID: "abc123", State: "running"})

	if m.detailLogRows != 18 {
		t.Fatalf("detailLogRows = %d, want 18 after entering a running container detail view", m.detailLogRows)
	}
}

func TestDockerDataMsgRecalculatesDetailLayoutWhenContainerStateChanges(t *testing.T) {
	m := newTestModel()
	m.width = 120
	m.height = 30
	m.viewMode = ViewDetail
	m.detailContainerID = "abc123"
	m.detailContainer = &collector.Container{ID: "abc123", State: "running"}
	m.recalcLayout()

	updatedModel, _ := m.Update(DockerDataMsg{
		Data: collector.DockerData{
			Containers: []collector.Container{
				{ID: "abc123", Name: "svc", State: "exited"},
			},
		},
	})
	updated := updatedModel.(Model)

	if updated.detailContainer == nil {
		t.Fatal("detailContainer = nil, want refreshed container")
	}
	if updated.detailContainer.State != "exited" {
		t.Fatalf("detailContainer.State = %q, want %q", updated.detailContainer.State, "exited")
	}
	if updated.detailLogRows != 19 {
		t.Fatalf("detailLogRows = %d, want 19 after the container stops", updated.detailLogRows)
	}
}

func TestRecalcLayoutCountsPolishedDetailMetadataRows(t *testing.T) {
	m := newTestModel()
	m.width = 120
	m.height = 30
	m.detailContainer = &collector.Container{State: "running"}
	m.detailMeta = &collector.ContainerDetail{
		RestartPolicy: "unless-stopped",
		Command:       "/docker-entrypoint.sh postgres",
		CreatedAt:     time.Date(2026, 3, 1, 8, 0, 0, 0, time.UTC),
		StartedAt:     time.Date(2026, 3, 6, 12, 34, 0, 0, time.UTC),
		Networks: []collector.NetworkAddress{
			{Name: "app", IPv4: "172.20.0.5"},
		},
		Mounts: []collector.Mount{
			{Source: "/host/config", Destination: "/data"},
		},
		Labels: map[string]string{
			"com.docker.compose.project": "homedash",
			"com.docker.compose.service": "db",
		},
	}

	m.recalcLayout()

	if m.detailLogRows != 12 {
		t.Fatalf("detailLogRows = %d, want 12 with metadata-rich detail panel", m.detailLogRows)
	}
}

func TestHandleDetailKeyPagesDown(t *testing.T) {
	m := newTestModel()
	m.detailLogRows = 10
	m.detailLogs = make([]string, 40)

	updatedModel, _ := handleDetailKey(tea.KeyPressMsg{Text: "ctrl+d"}, &m)
	updated := updatedModel.(*Model)

	if updated.detailScrollOffset != 5 {
		t.Fatalf("detailScrollOffset = %d, want 5 after ctrl+d", updated.detailScrollOffset)
	}
}

func TestHandleDetailKeyPagesUp(t *testing.T) {
	m := newTestModel()
	m.detailLogRows = 10
	m.detailLogs = make([]string, 40)
	m.detailScrollOffset = 9

	updatedModel, _ := handleDetailKey(tea.KeyPressMsg{Text: "ctrl+u"}, &m)
	updated := updatedModel.(*Model)

	if updated.detailScrollOffset != 4 {
		t.Fatalf("detailScrollOffset = %d, want 4 after ctrl+u", updated.detailScrollOffset)
	}
}

func TestHandleDetailKeyRefreshRestartsFollowForStack(t *testing.T) {
	m := newTestModel()
	m.viewMode = ViewDetail
	m.detailStackName = "monitoring"
	m.detailLogs = []string{"old log line 1", "old log line 2"}
	m.detailScrollOffset = 5
	m.logFollowing = true
	// Not in test mode — the live path should restart following
	m.TestMode = false

	updatedModel, cmd := handleDetailKey(tea.KeyPressMsg{Text: "l"}, &m)
	updated := updatedModel.(*Model)

	if !updated.logFollowing {
		t.Fatal("logFollowing should be true after refresh (stream restart)")
	}
	if updated.detailLogs != nil {
		t.Fatal("detailLogs should be nil after refresh (cleared for stream)")
	}
	if updated.detailScrollOffset != 0 {
		t.Fatal("detailScrollOffset should be 0 after refresh")
	}
	if cmd == nil {
		t.Fatal("refresh should return a cmd (follow stream)")
	}
}

func TestHandleDetailKeyRefreshBatchInTestMode(t *testing.T) {
	m := newTestModel()
	m.viewMode = ViewDetail
	m.detailStackName = "monitoring"
	m.detailLogs = []string{"old log line"}
	m.TestMode = true

	updatedModel, cmd := handleDetailKey(tea.KeyPressMsg{Text: "l"}, &m)
	updated := updatedModel.(*Model)

	// Test mode uses batch fetch, not streaming
	if updated.logFollowing {
		t.Fatal("logFollowing should be false in test mode (batch path)")
	}
	if cmd == nil {
		t.Fatal("refresh should return a cmd (batch fetch)")
	}
}

func TestRecalcLayoutMatchesRenderedContainerRowsInNarrowLayout(t *testing.T) {
	m := newTestModel()
	m.width = 60
	m.height = 40
	m.refreshing = true
	m.searchInput.SetValue("postgres")

	m.recalcLayout()

	header := panels.RenderHeader(m.systemData, m.width, m.TestMode)
	systemPanel := panels.RenderSystem(m.systemData, m.cpuHistory, m.width, 11, m.focusedPanel == PanelSystem)
	weatherPanel := panels.RenderWeather(m.weatherData, m.weatherErr, m.weatherRetries, m.width, 11, m.focusedPanel == PanelWeather)
	topRow := lipgloss.JoinVertical(lipgloss.Left, systemPanel, weatherPanel)
	previewBar := panels.RenderPreview(nil, nil, m.confirmAction, m.dashboardActionTargetName, m.actionResult, m.width)
	helpBar := panels.RenderHelp(panels.DefaultBindings, m.refreshing, false, m.width)
	bottomSection := lipgloss.JoinVertical(lipgloss.Left, previewBar, helpBar)

	countLines := func(s string) int {
		if s == "" {
			return 0
		}
		return strings.Count(s, "\n") + 1
	}

	expectedRows := m.height - countLines(header) - countLines(topRow) - countLines(bottomSection) - 5
	if expectedRows < 0 {
		expectedRows = 0
	}

	if m.containerRows != expectedRows {
		t.Fatalf("containerRows = %d, want %d to match rendered narrow layout", m.containerRows, expectedRows)
	}
}

func TestHandleMouseIgnoresClicksBelowRenderedContainerRows(t *testing.T) {
	m := newTestModel()
	m.width = 60
	m.height = 40
	m.focusedPanel = PanelContainers
	m.refreshing = true
	m.searchInput.SetValue("postgres")
	for i := 0; i < 20; i++ {
		m.displayItems = append(m.displayItems, DisplayItem{
			Kind:      DisplayContainer,
			Container: &collector.Container{ID: "id", Name: "svc", State: "running"},
		})
	}
	m.selectedIndex = 1
	m.recalcLayout()

	header := panels.RenderHeader(m.systemData, m.width, m.TestMode)
	systemPanel := panels.RenderSystem(m.systemData, m.cpuHistory, m.width, 11, m.focusedPanel == PanelSystem)
	weatherPanel := panels.RenderWeather(m.weatherData, m.weatherErr, m.weatherRetries, m.width, 11, m.focusedPanel == PanelWeather)
	topRow := lipgloss.JoinVertical(lipgloss.Left, systemPanel, weatherPanel)
	previewBar := panels.RenderPreview(nil, nil, m.confirmAction, m.dashboardActionTargetName, m.actionResult, m.width)
	helpBar := panels.RenderHelp(panels.DefaultBindings, m.refreshing, false, m.width)
	bottomSection := lipgloss.JoinVertical(lipgloss.Left, previewBar, helpBar)
	expectedRows := m.height - (strings.Count(header, "\n") + 1) - (strings.Count(topRow, "\n") + 1) - (strings.Count(bottomSection, "\n") + 1) - 5
	if expectedRows < 0 {
		expectedRows = 0
	}
	if expectedRows == 0 {
		t.Fatal("expectedRows = 0, want a rendered container area for this regression test")
	}

	clickY := m.containerStartY + expectedRows
	updatedModel, _ := handleMouse(tea.MouseClickMsg{
		Button: tea.MouseLeft,
		Y:      clickY,
	}, &m)
	updated := updatedModel.(*Model)

	if updated.selectedIndex != 1 {
		t.Fatalf("selectedIndex = %d, want 1 when clicking below the rendered container rows", updated.selectedIndex)
	}
}

func TestQuickMenuItemsRunning(t *testing.T) {
	items := quickMenuItems("running")

	if len(items) != 3 {
		t.Fatalf("len(items) = %d, want 3", len(items))
	}
	if items[0].action != "logs" {
		t.Fatalf("items[0].action = %q, want %q", items[0].action, "logs")
	}
	if items[1].action != "stop" {
		t.Fatalf("items[1].action = %q, want %q", items[1].action, "stop")
	}
	if items[2].action != "restart" {
		t.Fatalf("items[2].action = %q, want %q", items[2].action, "restart")
	}
}

func TestQuickMenuItemsStopped(t *testing.T) {
	items := quickMenuItems("exited")

	if len(items) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(items))
	}
	if items[0].action != "logs" {
		t.Fatalf("items[0].action = %q, want %q", items[0].action, "logs")
	}
	if items[1].action != "start" {
		t.Fatalf("items[1].action = %q, want %q", items[1].action, "start")
	}
}

func TestStackQuickMenuItemsMixed(t *testing.T) {
	items := stackQuickMenuItems(2, 1)

	if len(items) != 4 {
		t.Fatalf("len(items) = %d, want 4", len(items))
	}
	if items[0].action != "logs" {
		t.Fatalf("items[0].action = %q, want %q", items[0].action, "logs")
	}
	if items[1].action != "start" {
		t.Fatalf("items[1].action = %q, want %q", items[1].action, "start")
	}
	if items[2].action != "stop" {
		t.Fatalf("items[2].action = %q, want %q", items[2].action, "stop")
	}
	if items[3].action != "restart" {
		t.Fatalf("items[3].action = %q, want %q", items[3].action, "restart")
	}
}

func TestHandleDashboardKeyOpensStackQuickMenu(t *testing.T) {
	m := newTestModel()
	m.focusedPanel = PanelContainers
	m.selectedIndex = 0
	m.displayItems = []DisplayItem{
		{
			Kind:         DisplayGroup,
			StackName:    "media",
			RunningCount: 2,
			StoppedCount: 1,
		},
	}

	updatedModel, _ := handleDashboardKey(tea.KeyPressMsg{Text: "space"}, &m)
	updated := updatedModel.(*Model)

	if !updated.quickMenuOpen {
		t.Fatal("quickMenuOpen = false, want true for stack row")
	}
	if updated.quickMenuStackName != "media" {
		t.Fatalf("quickMenuStackName = %q, want %q", updated.quickMenuStackName, "media")
	}
	if updated.quickMenuContainerID != "" {
		t.Fatalf("quickMenuContainerID = %q, want empty for stack row", updated.quickMenuContainerID)
	}
}

func TestHandleDashboardKeySetsStackConfirmAction(t *testing.T) {
	m := newTestModel()
	m.focusedPanel = PanelContainers
	m.displayItems = []DisplayItem{
		{
			Kind:         DisplayGroup,
			StackName:    "media",
			RunningCount: 2,
			StoppedCount: 1,
		},
	}

	updatedModel, _ := handleDashboardKey(tea.KeyPressMsg{Text: "s"}, &m)
	updated := updatedModel.(*Model)

	if updated.confirmAction != "stop" {
		t.Fatalf("confirmAction = %q, want %q", updated.confirmAction, "stop")
	}
	if updated.dashboardActionStackName != "media" {
		t.Fatalf("dashboardActionStackName = %q, want %q", updated.dashboardActionStackName, "media")
	}
	if updated.dashboardActionTargetName != "media" {
		t.Fatalf("dashboardActionTargetName = %q, want %q", updated.dashboardActionTargetName, "media")
	}
	if updated.dashboardActionContainerID != "" {
		t.Fatalf("dashboardActionContainerID = %q, want empty for stack action", updated.dashboardActionContainerID)
	}
}

func TestExecuteQuickMenuItemLogsOpensStackDetail(t *testing.T) {
	m := newTestModel()
	m.quickMenuOpen = true
	m.quickMenuStackName = "media"
	m.dockerData = collector.DockerData{
		Containers: []collector.Container{
			{ID: "web", Name: "web", Stack: "media", State: "running"},
		},
	}

	updatedModel, cmd := m.executeQuickMenuItem(quickMenuItem{
		label:  "View Stack Logs",
		key:    "enter",
		action: "logs",
	})
	updated := updatedModel.(*Model)

	if updated.viewMode != ViewDetail {
		t.Fatalf("viewMode = %d, want %d", updated.viewMode, ViewDetail)
	}
	if updated.detailStackName != "media" {
		t.Fatalf("detailStackName = %q, want %q", updated.detailStackName, "media")
	}
	if updated.detailContainerID != "" {
		t.Fatalf("detailContainerID = %q, want empty", updated.detailContainerID)
	}
	if updated.quickMenuOpen {
		t.Fatal("quickMenuOpen = true, want false after opening stack detail")
	}
	if cmd == nil {
		t.Fatal("cmd = nil, want stack log fetch command")
	}
}

func TestHandleDetailKeySetsStackConfirmAction(t *testing.T) {
	m := newTestModel()
	m.viewMode = ViewDetail
	m.detailStackName = "media"
	m.dockerData = collector.DockerData{
		Containers: []collector.Container{
			{ID: "web", Name: "web", Stack: "media", State: "running"},
			{ID: "worker", Name: "worker", Stack: "media", State: "exited"},
		},
	}

	updatedModel, _ := handleDetailKey(tea.KeyPressMsg{Text: "s"}, &m)
	updated := updatedModel.(*Model)

	if updated.confirmAction != "stop" {
		t.Fatalf("confirmAction = %q, want %q", updated.confirmAction, "stop")
	}
}

func TestUpdateStackLogsMsgUpdatesMatchingStackDetail(t *testing.T) {
	m := newTestModel()
	m.viewMode = ViewDetail
	m.detailStackName = "media"

	updatedModel, _ := m.Update(StackLogsMsg{
		StackName: "media",
		Lines:     []string{"2026-03-06T12:00:00Z [web] ready"},
	})
	updated := updatedModel.(Model)

	if len(updated.detailLogs) != 1 {
		t.Fatalf("len(detailLogs) = %d, want 1", len(updated.detailLogs))
	}
	if got, want := updated.detailLogs[0], "2026-03-06T12:00:00Z [web] ready"; got != want {
		t.Fatalf("detailLogs[0] = %q, want %q", got, want)
	}
}

func TestUpdateStackActionMsgClearsPendingDashboardAction(t *testing.T) {
	m := newTestModel()
	m.dashboardActionStackName = "media"
	m.dashboardActionTargetName = "media"

	updatedModel, _ := m.Update(StackActionMsg{
		StackName: "media",
		Action:    "restart",
		Attempted: 2,
	})
	updated := updatedModel.(Model)

	if updated.confirmAction != "" {
		t.Fatalf("confirmAction = %q, want empty", updated.confirmAction)
	}
	if updated.dashboardActionStackName != "" {
		t.Fatalf("dashboardActionStackName = %q, want cleared", updated.dashboardActionStackName)
	}
	if updated.dashboardActionTargetName != "" {
		t.Fatalf("dashboardActionTargetName = %q, want cleared", updated.dashboardActionTargetName)
	}
	if !strings.Contains(updated.actionResult, "Success: restart stack media (2 containers)") {
		t.Fatalf("actionResult = %q, want success stack summary", updated.actionResult)
	}
	if updated.notifications.current() != nil {
		t.Fatalf("notification = %#v, want nil on stack action success", updated.notifications.current())
	}
}

func TestUpdateStackActionMsgIncludesFailedContainerNames(t *testing.T) {
	m := newTestModel()

	updatedModel, _ := m.Update(StackActionMsg{
		StackName: "media",
		Action:    "restart",
		Attempted: 5,
		Failed:    []string{"web", "db", "worker", "cache"},
		Err:       errors.New("boom"),
	})
	updated := updatedModel.(Model)

	if got, want := updated.actionResult, "Error: restart stack media failed for web, db, worker +1 more"; got != want {
		t.Fatalf("actionResult = %q, want %q", got, want)
	}
	n := updated.notifications.current()
	if n == nil {
		t.Fatal("notification = nil, want failure notification")
	}
	if got, want := n.Message, "Stack media restart failed for web, db, worker, cache"; got != want {
		t.Fatalf("notification message = %q, want %q", got, want)
	}
	if n.Level != levelError {
		t.Fatalf("notification level = %d, want %d", n.Level, levelError)
	}
}

func TestUpdateStackActionMsgNoopStaysCompact(t *testing.T) {
	m := newTestModel()

	updatedModel, _ := m.Update(StackActionMsg{
		StackName: "media",
		Action:    "start",
		Attempted: 0,
	})
	updated := updatedModel.(Model)

	if got, want := updated.actionResult, "Nothing to start in stack media"; got != want {
		t.Fatalf("actionResult = %q, want %q", got, want)
	}
	if updated.notifications.current() != nil {
		t.Fatalf("notification = %#v, want nil on noop stack action", updated.notifications.current())
	}
}

func TestSelectionPreservedAcrossFilterChange(t *testing.T) {
	m := newTestModel()
	m.dockerData = collector.DockerData{
		Containers: []collector.Container{
			{ID: "a1", Name: "nginx", Stack: "web", State: "running"},
			{ID: "b2", Name: "postgres", Stack: "db", State: "running"},
			{ID: "c3", Name: "redis", Stack: "db", State: "running"},
		},
	}
	m.rebuildDisplayItems()

	// Items: db-group(0), postgres(1), redis(2), web-group(3), nginx(4)
	// Select postgres at index 1
	m.selectedIndex = 1
	m.trackSelection()

	if m.selectedTarget != "c:b2" {
		t.Fatalf("selectedTarget = %q, want %q", m.selectedTarget, "c:b2")
	}

	// Apply filter that still includes postgres
	m.searchInput.SetValue("post")
	m.rebuildDisplayItems()

	// Should find postgres again (db-group at 0, postgres at 1)
	if m.selectedIndex != 1 {
		t.Fatalf("selectedIndex = %d, want 1 after filter preserves selection", m.selectedIndex)
	}
	if m.displayItems[m.selectedIndex].Container == nil || m.displayItems[m.selectedIndex].Container.ID != "b2" {
		t.Fatal("selection should still point to postgres after filter")
	}
}

func TestSelectionPreservedForStackGroup(t *testing.T) {
	m := newTestModel()
	m.dockerData = collector.DockerData{
		Containers: []collector.Container{
			{ID: "a1", Name: "nginx", Stack: "web", State: "running"},
			{ID: "b2", Name: "postgres", Stack: "db", State: "running"},
		},
	}
	m.rebuildDisplayItems()

	// Select "web" group header (index 1, since db comes first alphabetically: db-group, postgres, web-group, nginx)
	m.selectedIndex = 2
	m.trackSelection()

	if m.selectedTarget != "s:web" {
		t.Fatalf("selectedTarget = %q, want %q", m.selectedTarget, "s:web")
	}
}

func TestEscClearsFilterOnDashboard(t *testing.T) {
	m := newTestModel()
	m.focusedPanel = PanelContainers
	m.dockerData = collector.DockerData{
		Containers: []collector.Container{
			{ID: "a1", Name: "nginx", Stack: "web", State: "running"},
			{ID: "b2", Name: "postgres", Stack: "db", State: "running"},
		},
	}
	m.searchInput.SetValue("nginx")
	m.rebuildDisplayItems()

	// Esc should clear filter
	handleDashboardKey(tea.KeyPressMsg{Text: "esc"}, &m)

	if m.searchInput.Value() != "" {
		t.Fatalf("searchInput value = %q, want empty after esc", m.searchInput.Value())
	}
	// All items should be visible again
	if len(m.displayItems) != 4 {
		t.Fatalf("len(displayItems) = %d, want 4 after clearing filter", len(m.displayItems))
	}
}

func TestEnterDetailViewStartsFollowing(t *testing.T) {
	m := newTestModel()
	m.dockerData = collector.DockerData{
		Containers: []collector.Container{
			{ID: "abc123", Name: "web", Stack: "app", State: "running"},
		},
	}

	m.enterDetailView(&m.dockerData.Containers[0])

	if !m.logFollowing {
		t.Fatal("logFollowing = false, want true after entering detail view")
	}
	if m.viewMode != ViewDetail {
		t.Fatalf("viewMode = %d, want %d", m.viewMode, ViewDetail)
	}
}

func TestEnterStackDetailViewStartsFollowing(t *testing.T) {
	m := newTestModel()
	m.dockerData = collector.DockerData{
		Containers: []collector.Container{
			{ID: "abc123", Name: "web", Stack: "app", State: "running"},
		},
	}

	m.enterStackDetailView("app")

	if !m.logFollowing {
		t.Fatal("logFollowing = false, want true after entering stack detail view")
	}
	if m.detailStackName != "app" {
		t.Fatalf("detailStackName = %q, want %q", m.detailStackName, "app")
	}
}

func TestLogSearchMatchesFound(t *testing.T) {
	m := newTestModel()
	m.viewMode = ViewDetail
	m.detailContainerID = "abc123"
	m.detailLogRows = 10
	m.detailLogs = []string{
		"2026-03-06T12:00:00Z starting server",
		"2026-03-06T12:00:01Z listening on port 8080",
		"2026-03-06T12:00:02Z error: connection refused",
		"2026-03-06T12:00:03Z retrying connection",
		"2026-03-06T12:00:04Z error: timeout",
	}
	m.logSearchInput.SetValue("error")
	m.recomputeLogSearchMatches()

	if len(m.logSearchMatches) != 2 {
		t.Fatalf("len(logSearchMatches) = %d, want 2", len(m.logSearchMatches))
	}
	if m.logSearchMatches[0] != 2 {
		t.Fatalf("logSearchMatches[0] = %d, want 2", m.logSearchMatches[0])
	}
	if m.logSearchMatches[1] != 4 {
		t.Fatalf("logSearchMatches[1] = %d, want 4", m.logSearchMatches[1])
	}
	if m.logSearchIndex != 0 {
		t.Fatalf("logSearchIndex = %d, want 0", m.logSearchIndex)
	}
}

func TestLogSearchNavigateNextPrev(t *testing.T) {
	m := newTestModel()
	m.viewMode = ViewDetail
	m.detailContainerID = "abc123"
	m.detailLogRows = 10
	m.detailLogs = []string{
		"line 0",
		"error line 1",
		"line 2",
		"error line 3",
		"error line 4",
	}
	m.logSearchInput.SetValue("error")
	m.recomputeLogSearchMatches()

	// Navigate forward
	handleDetailKey(tea.KeyPressMsg{Text: "n"}, &m)
	if m.logSearchIndex != 1 {
		t.Fatalf("after n: logSearchIndex = %d, want 1", m.logSearchIndex)
	}

	handleDetailKey(tea.KeyPressMsg{Text: "n"}, &m)
	if m.logSearchIndex != 2 {
		t.Fatalf("after n: logSearchIndex = %d, want 2", m.logSearchIndex)
	}

	// Wrap around
	handleDetailKey(tea.KeyPressMsg{Text: "n"}, &m)
	if m.logSearchIndex != 0 {
		t.Fatalf("after n wrap: logSearchIndex = %d, want 0", m.logSearchIndex)
	}

	// Navigate backward
	handleDetailKey(tea.KeyPressMsg{Text: "N"}, &m)
	if m.logSearchIndex != 2 {
		t.Fatalf("after N: logSearchIndex = %d, want 2", m.logSearchIndex)
	}
}

func TestLogSearchEscClearsSearch(t *testing.T) {
	m := newTestModel()
	m.viewMode = ViewDetail
	m.detailContainerID = "abc123"
	m.detailLogRows = 10
	m.detailLogs = []string{"error line"}
	m.logSearchInput.SetValue("error")
	m.recomputeLogSearchMatches()

	// Esc should clear search, not exit detail
	handleDetailKey(tea.KeyPressMsg{Text: "esc"}, &m)

	if m.logSearchInput.Value() != "" {
		t.Fatalf("logSearchInput = %q, want empty", m.logSearchInput.Value())
	}
	if m.logSearchMatches != nil {
		t.Fatalf("logSearchMatches = %v, want nil", m.logSearchMatches)
	}
	if m.viewMode != ViewDetail {
		t.Fatal("viewMode should still be ViewDetail after clearing search")
	}
}

func TestLogSearchEscWithoutSearchExitsDetail(t *testing.T) {
	m := newTestModel()
	m.viewMode = ViewDetail
	m.detailContainerID = "abc123"

	handleDetailKey(tea.KeyPressMsg{Text: "esc"}, &m)

	if m.viewMode != ViewDashboard {
		t.Fatal("viewMode should be ViewDashboard after esc with no search")
	}
}

func TestLogSearchClearedOnDetailExit(t *testing.T) {
	m := newTestModel()
	m.viewMode = ViewDetail
	m.detailContainerID = "abc123"
	m.detailLogs = []string{"error line"}
	m.logSearchInput.SetValue("error")
	m.recomputeLogSearchMatches()

	m.clearDetailView()

	if m.logSearchInput.Value() != "" {
		t.Fatalf("logSearchInput = %q, want empty after clearDetailView", m.logSearchInput.Value())
	}
	if m.logSearchMatches != nil {
		t.Fatalf("logSearchMatches should be nil after clearDetailView")
	}
}

func TestLogSearchNoMatches(t *testing.T) {
	m := newTestModel()
	m.viewMode = ViewDetail
	m.detailContainerID = "abc123"
	m.detailLogRows = 10
	m.detailLogs = []string{"starting server", "listening on port 8080"}
	m.logSearchInput.SetValue("nonexistent")
	m.recomputeLogSearchMatches()

	if len(m.logSearchMatches) != 0 {
		t.Fatalf("len(logSearchMatches) = %d, want 0", len(m.logSearchMatches))
	}
}

func TestRunningCount(t *testing.T) {
	m := newTestModel()
	m.dockerData = collector.DockerData{
		Containers: []collector.Container{
			{Name: "web", Stack: "app", State: "running", Health: "healthy"},
			{Name: "worker", Stack: "app", State: "exited"},
			{Name: "db", Stack: "app", State: "running", Health: "unhealthy"},
			{Name: "migrator", Stack: "app", State: "running", Health: "starting"},
		},
	}

	m.rebuildDisplayItems()

	if m.displayItems[0].RunningCount != 3 {
		t.Fatalf("RunningCount = %d, want 3", m.displayItems[0].RunningCount)
	}
	if m.displayItems[0].ContainerCount != 4 {
		t.Fatalf("ContainerCount = %d, want 4", m.displayItems[0].ContainerCount)
	}
	if m.displayItems[0].UnhealthyCount != 1 {
		t.Fatalf("UnhealthyCount = %d, want 1", m.displayItems[0].UnhealthyCount)
	}
	if m.displayItems[0].StartingCount != 1 {
		t.Fatalf("StartingCount = %d, want 1", m.displayItems[0].StartingCount)
	}
	if m.displayItems[0].StoppedCount != 1 {
		t.Fatalf("StoppedCount = %d, want 1", m.displayItems[0].StoppedCount)
	}
}

func TestOverlayCenterPreservesBackground(t *testing.T) {
	// Plain-text test (no ANSI) to verify overlay logic
	bgLines := []string{
		"AAAAAAAAAAAAAAAAAA",
		"BBBBBBBBBBBBBBBBBB",
		"CCCCCCCCCCCCCCCCCC",
		"DDDDDDDDDDDDDDDDDD",
		"EEEEEEEEEEEEEEEEEE",
	}
	bg := strings.Join(bgLines, "\n")
	fg := "XXX\nYYY"

	bgW := lipgloss.Width(bg)
	bgH := lipgloss.Height(bg)
	fgW := lipgloss.Width(fg)
	fgH := lipgloss.Height(fg)

	result := overlayCenter(bg, fg, bgW, bgH, fgW, fgH)
	lines := strings.Split(result, "\n")

	if len(lines) != bgH {
		t.Fatalf("overlay line count = %d, want %d", len(lines), bgH)
	}

	// First line should be unchanged (overlay starts at Y=1 for 5-line bg, 2-line fg)
	if lines[0] != bgLines[0] {
		t.Errorf("line 0 = %q, want %q (should be preserved)", lines[0], bgLines[0])
	}

	// Overlay lines should contain the foreground content
	startY := (bgH - fgH) / 2
	for i := startY; i < startY+fgH; i++ {
		if !strings.Contains(lines[i], strings.Split(fg, "\n")[i-startY]) {
			t.Errorf("line %d = %q, does not contain overlay content", i, lines[i])
		}
	}

	// Last line should be unchanged
	if lines[bgH-1] != bgLines[bgH-1] {
		t.Errorf("last line = %q, want %q (should be preserved)", lines[bgH-1], bgLines[bgH-1])
	}
}

func TestToggleFollowOnJumpsToBottom(t *testing.T) {
	m := newTestModel()
	m.viewMode = ViewDetail
	m.detailContainerID = "abc"
	m.detailLogRows = 10
	m.detailLogs = make([]string, 30)
	m.detailScrollOffset = 5 // scrolled up, not at bottom

	updatedModel, _ := handleDetailKey(tea.KeyPressMsg{Text: "f"}, &m)
	updated := updatedModel.(*Model)

	expectedScroll := 30 - 10 // maxScroll = len(logs) - logRows = 20
	if updated.detailScrollOffset != expectedScroll {
		t.Fatalf("detailScrollOffset = %d, want %d (should jump to bottom on follow toggle)", updated.detailScrollOffset, expectedScroll)
	}
	if !updated.logFollowing {
		t.Fatal("logFollowing should be true after toggling follow on")
	}
}

func TestFollowModeAutoScrollsToBottom(t *testing.T) {
	m := newTestModel()
	m.viewMode = ViewDetail
	m.detailContainerID = "abc"
	m.detailLogRows = 10
	m.logFollowing = true
	m.logFollowSeq = 1
	m.logFollowCh = make(chan string, 64)
	m.detailLogs = nil
	m.detailScrollOffset = 0

	// Simulate 25 log lines arriving via follow
	for i := 0; i < 25; i++ {
		msg := LogFollowLineMsg{Line: fmt.Sprintf("log line %d", i), Seq: 1}
		updated, _ := m.Update(msg)
		switch v := updated.(type) {
		case Model:
			m = v
		case *Model:
			m = *v
		}
	}

	if len(m.detailLogs) != 25 {
		t.Fatalf("detailLogs count = %d, want 25", len(m.detailLogs))
	}

	expectedScroll := 25 - m.detailLogRows // 15
	if m.detailScrollOffset != expectedScroll {
		t.Fatalf("detailScrollOffset = %d, want %d (should autoscroll to bottom)", m.detailScrollOffset, expectedScroll)
	}
}

func TestFollowStreamEndSchedulesRestart(t *testing.T) {
	m := newTestModel()
	m.viewMode = ViewDetail
	m.detailStackName = "monitoring" // stack detail — auto-restart applies
	m.logFollowing = true
	m.logFollowSeq = 1
	m.logFollowCh = make(chan string, 1)
	m.TestMode = false

	// Stream ends (container restart)
	msg := LogFollowLineMsg{Done: true, Seq: 1}
	updated, cmd := m.Update(msg)
	switch v := updated.(type) {
	case Model:
		m = v
	case *Model:
		m = *v
	}

	if m.logFollowing {
		t.Fatal("logFollowing should be false after stream end")
	}
	if cmd == nil {
		t.Fatal("should return a cmd to schedule restart")
	}
}

func TestFollowRestartMsgRestartsFollowing(t *testing.T) {
	m := newTestModel()
	m.viewMode = ViewDetail
	m.detailStackName = "monitoring" // stack detail
	m.detailLogs = []string{"existing log"}
	m.logFollowing = false
	m.TestMode = false

	updated, cmd := m.Update(followRestartMsg{})
	switch v := updated.(type) {
	case Model:
		m = v
	case *Model:
		m = *v
	}

	if !m.logFollowing {
		t.Fatal("logFollowing should be true after followRestartMsg")
	}
	if cmd == nil {
		t.Fatal("should return a cmd to start the follow stream")
	}
}

func TestFollowStreamEndNoRestartForSingleContainer(t *testing.T) {
	m := newTestModel()
	m.viewMode = ViewDetail
	m.detailContainerID = "abc"
	m.detailStackName = "" // single container, not stack
	m.logFollowing = true
	m.logFollowSeq = 1
	m.logFollowCh = make(chan string, 1)
	m.TestMode = false

	msg := LogFollowLineMsg{Done: true, Seq: 1}
	updated, cmd := m.Update(msg)
	switch v := updated.(type) {
	case Model:
		m = v
	case *Model:
		m = *v
	}

	if m.logFollowing {
		t.Fatal("logFollowing should be false after stream end")
	}
	if cmd != nil {
		t.Fatal("should NOT schedule restart for single-container detail view")
	}
}

func TestFollowRestartMsgIgnoredWhenNotInDetailView(t *testing.T) {
	m := newTestModel()
	m.viewMode = ViewDashboard
	m.logFollowing = false

	updated, cmd := m.Update(followRestartMsg{})
	switch v := updated.(type) {
	case Model:
		m = v
	case *Model:
		m = *v
	}

	if m.logFollowing {
		t.Fatal("logFollowing should remain false when not in detail view")
	}
	if cmd != nil {
		t.Fatal("should return nil cmd when not in detail view")
	}
}

func TestViewHardwareCursorWhenFiltering(t *testing.T) {
	m := NewModel(ModelOptions{TestMode: true})
	m.width = 120
	m.height = 40
	m.filtering = true
	m.searchInput.Focus()
	m.searchInput.SetValue("web")

	v := m.View()
	if v.Cursor == nil {
		t.Fatal("View().Cursor should be set when filtering")
	}
	// Cursor Y should be > 0 (below header + top panels)
	if v.Cursor.Position.Y <= 0 {
		t.Errorf("Cursor Y = %d, want > 0", v.Cursor.Position.Y)
	}
	// Cursor X should account for panel border(1) + padding(1) + prompt
	if v.Cursor.Position.X < 2 {
		t.Errorf("Cursor X = %d, want >= 2", v.Cursor.Position.X)
	}
}

func TestViewHardwareCursorWhenLogSearchActive(t *testing.T) {
	m := NewModel(ModelOptions{TestMode: true})
	m.width = 120
	m.height = 40
	m.viewMode = ViewDetail
	m.detailContainer = &collector.Container{Name: "test", ID: "abc123"}
	m.detailContainerID = "abc123"
	m.logSearchActive = true
	m.logSearchInput.Focus()
	m.logSearchInput.SetValue("error")

	v := m.View()
	if v.Cursor == nil {
		t.Fatal("View().Cursor should be set when log search is active")
	}
	// Cursor should be on the last line (action bar)
	if v.Cursor.Position.Y != m.height-1 {
		t.Errorf("Cursor Y = %d, want %d (last line)", v.Cursor.Position.Y, m.height-1)
	}
}

func TestViewNoCursorWhenNotEditing(t *testing.T) {
	m := NewModel(ModelOptions{TestMode: true})
	m.width = 120
	m.height = 40

	v := m.View()
	if v.Cursor != nil {
		t.Fatal("View().Cursor should be nil when no input is active")
	}
}

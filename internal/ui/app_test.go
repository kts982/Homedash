package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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

func TestRecalcLayoutMatchesRenderedContainerRowsInNarrowLayout(t *testing.T) {
	m := newTestModel()
	m.width = 60
	m.height = 40
	m.refreshing = true
	m.searchInput.SetValue("postgres")

	m.recalcLayout()

	header := panels.RenderHeader(m.systemData, m.width)
	systemPanel := panels.RenderSystem(m.systemData, m.cpuHistory, m.width, 11, m.focusedPanel == PanelSystem)
	weatherPanel := panels.RenderWeather(m.weatherData, m.weatherErr, m.weatherRetries, m.width, 11, m.focusedPanel == PanelWeather)
	topRow := lipgloss.JoinVertical(lipgloss.Left, systemPanel, weatherPanel)
	previewBar := panels.RenderPreview(nil, m.confirmAction, m.dashboardActionContainerName, m.actionResult, m.width)
	helpBar := panels.RenderHelp(panels.DefaultBindings, m.refreshing, m.width)
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

	header := panels.RenderHeader(m.systemData, m.width)
	systemPanel := panels.RenderSystem(m.systemData, m.cpuHistory, m.width, 11, m.focusedPanel == PanelSystem)
	weatherPanel := panels.RenderWeather(m.weatherData, m.weatherErr, m.weatherRetries, m.width, 11, m.focusedPanel == PanelWeather)
	topRow := lipgloss.JoinVertical(lipgloss.Left, systemPanel, weatherPanel)
	previewBar := panels.RenderPreview(nil, m.confirmAction, m.dashboardActionContainerName, m.actionResult, m.width)
	helpBar := panels.RenderHelp(panels.DefaultBindings, m.refreshing, m.width)
	bottomSection := lipgloss.JoinVertical(lipgloss.Left, previewBar, helpBar)
	expectedRows := m.height - (strings.Count(header, "\n") + 1) - (strings.Count(topRow, "\n") + 1) - (strings.Count(bottomSection, "\n") + 1) - 5
	if expectedRows < 0 {
		expectedRows = 0
	}
	if expectedRows == 0 {
		t.Fatal("expectedRows = 0, want a rendered container area for this regression test")
	}

	clickY := m.containerStartY + expectedRows
	updatedModel, _ := handleMouse(tea.MouseMsg{
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
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

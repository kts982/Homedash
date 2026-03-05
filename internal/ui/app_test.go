package ui

import (
	"testing"

	"github.com/kostas/homedash/internal/collector"
)

func newTestModel() Model {
	m := Model{
		collapsedStacks: make(map[string]bool),
		containerRows:   10,
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
			{Name: "web", Stack: "app", State: "running"},
			{Name: "worker", Stack: "app", State: "exited"},
			{Name: "db", Stack: "app", State: "running"},
		},
	}

	m.rebuildDisplayItems()

	if m.displayItems[0].RunningCount != 2 {
		t.Fatalf("RunningCount = %d, want 2", m.displayItems[0].RunningCount)
	}
	if m.displayItems[0].ContainerCount != 3 {
		t.Fatalf("ContainerCount = %d, want 3", m.displayItems[0].ContainerCount)
	}
}

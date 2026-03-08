package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// newTestModeModel creates a Model in test-mode with a realistic terminal size
// and feeds it the mock data, simulating a full cold-start.
func newTestModeModel(t *testing.T) Model {
	t.Helper()
	m := NewModel(ModelOptions{TestMode: true})
	// Simulate a terminal size
	m, _ = applyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 40})
	// Feed the mock data that Init() would produce
	m, _ = applyMsg(m, collectMockSystemCmd())
	m, _ = applyMsg(m, collectMockDockerCmd())
	m, _ = applyMsg(m, collectMockWeatherCmd())
	return m
}

// applyMsg sends a message through Update and returns the updated model.
func applyMsg(m Model, msg tea.Msg) (Model, tea.Cmd) {
	updated, cmd := m.Update(msg)
	switch v := updated.(type) {
	case Model:
		return v, cmd
	case *Model:
		return *v, cmd
	default:
		panic("unexpected model type from Update")
	}
}

// applyKey sends a key press through Update.
func applyKey(m Model, key string) (Model, tea.Cmd) {
	return applyMsg(m, keyMsg(key))
}

// keyMsg constructs a tea.KeyMsg from a string representation.
func keyMsg(key string) tea.KeyMsg {
	switch key {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case "shift+tab":
		return tea.KeyMsg{Type: tea.KeyShiftTab}
	case "ctrl+c":
		return tea.KeyMsg{Type: tea.KeyCtrlC}
	case "ctrl+d":
		return tea.KeyMsg{Type: tea.KeyCtrlD}
	case "ctrl+u":
		return tea.KeyMsg{Type: tea.KeyCtrlU}
	case " ":
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}}
	default:
		if len(key) == 1 {
			return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
		}
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
	}
}

// stripANSIForTest removes ANSI escape sequences for easier assertion.
func stripANSIForTest(s string) string {
	var out strings.Builder
	inEscape := false
	for _, r := range s {
		if r == '\x1b' {
			inEscape = true
			continue
		}
		if inEscape {
			if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || r == '~' {
				inEscape = false
			}
			continue
		}
		out.WriteRune(r)
	}
	return out.String()
}

// --- Test 1: Startup/render stability ---

func TestIntegration_StartupRenderStability(t *testing.T) {
	m := newTestModeModel(t)

	// Model should be in dashboard mode
	if m.viewMode != ViewDashboard {
		t.Fatalf("viewMode = %d, want ViewDashboard", m.viewMode)
	}

	// Should have system data loaded
	if m.systemData.Hostname != "synthetic-host" {
		t.Fatalf("hostname = %q, want %q", m.systemData.Hostname, "synthetic-host")
	}

	// Should have docker data loaded
	if m.dockerData.Total != 3 {
		t.Fatalf("docker total = %d, want 3", m.dockerData.Total)
	}
	if m.dockerData.Running != 2 {
		t.Fatalf("docker running = %d, want 2", m.dockerData.Running)
	}

	// Should have weather data loaded
	if m.weatherData.Location != "Synthetic Location" {
		t.Fatalf("weather location = %q, want %q", m.weatherData.Location, "Synthetic Location")
	}

	// Display items should be built (2 stacks with containers)
	if len(m.displayItems) == 0 {
		t.Fatal("displayItems is empty after startup")
	}

	// View should render without panic and produce non-empty output
	view := m.View()
	if view == "" {
		t.Fatal("View() returned empty string")
	}
	if view == "Loading..." {
		t.Fatal("View() still showing loading after data was fed")
	}

	// View should contain key content from the synthetic data
	plain := stripANSIForTest(view)
	for _, expected := range []string{"synthetic-host", "service-alpha", "service-beta"} {
		if !strings.Contains(plain, expected) {
			t.Errorf("View() missing expected content %q", expected)
		}
	}
}

func TestIntegration_StartupTestModeFlag(t *testing.T) {
	m := newTestModeModel(t)

	if !m.TestMode {
		t.Fatal("TestMode = false, want true")
	}

	// View should contain test-mode indicator
	view := stripANSIForTest(m.View())
	if !strings.Contains(strings.ToLower(view), "test") {
		t.Log("Warning: View() does not contain test-mode indicator (may be expected)")
	}
}

func TestIntegration_StartupDisplayItemsStructure(t *testing.T) {
	m := newTestModeModel(t)

	// Mock data has 3 containers in 2 stacks: infra (alpha, beta) and apps (gamma)
	// Expect: apps-group, gamma, infra-group, alpha, beta (alphabetical stacks)
	if len(m.displayItems) != 5 {
		t.Fatalf("len(displayItems) = %d, want 5", len(m.displayItems))
	}

	// First group should be "apps" (alphabetical)
	if m.displayItems[0].Kind != DisplayGroup || m.displayItems[0].StackName != "apps" {
		t.Fatalf("displayItems[0] = %+v, want apps group", m.displayItems[0])
	}
	// Second group should be "infra"
	if m.displayItems[2].Kind != DisplayGroup || m.displayItems[2].StackName != "infra" {
		t.Fatalf("displayItems[2] = %+v, want infra group", m.displayItems[2])
	}
}

// --- Test 2: Dashboard filtering ---

func TestIntegration_FilterNarrowsList(t *testing.T) {
	m := newTestModeModel(t)

	// Activate filter mode
	m, _ = applyKey(m, "/")
	if !m.filtering {
		t.Fatal("filtering = false after '/'")
	}

	// Type filter text by setting the input value directly
	// (the textinput model handles rune input internally)
	m.searchInput.SetValue("alpha")
	m.rebuildDisplayItems()

	// Should only show infra group + service-alpha
	if len(m.displayItems) != 2 {
		t.Fatalf("filtered displayItems = %d, want 2 (group + container)", len(m.displayItems))
	}
	if m.displayItems[0].StackName != "infra" {
		t.Fatalf("filtered group = %q, want %q", m.displayItems[0].StackName, "infra")
	}
	if m.displayItems[1].Container == nil || m.displayItems[1].Container.Name != "service-alpha" {
		t.Fatal("filtered container should be service-alpha")
	}
}

func TestIntegration_FilterEscClears(t *testing.T) {
	m := newTestModeModel(t)

	// Set up a filter
	m.searchInput.SetValue("alpha")
	m.rebuildDisplayItems()
	if len(m.displayItems) != 2 {
		t.Fatalf("pre-condition: filtered items = %d, want 2", len(m.displayItems))
	}

	// Press Esc to clear filter
	m.focusedPanel = PanelContainers
	m, _ = applyKey(m, "esc")

	if m.searchInput.Value() != "" {
		t.Fatalf("searchInput = %q after esc, want empty", m.searchInput.Value())
	}
	// All items should be visible again
	if len(m.displayItems) != 5 {
		t.Fatalf("displayItems after esc = %d, want 5", len(m.displayItems))
	}
}

func TestIntegration_FilterNoMatch(t *testing.T) {
	m := newTestModeModel(t)

	m.searchInput.SetValue("nonexistent-container-xyz")
	m.rebuildDisplayItems()

	if len(m.displayItems) != 0 {
		t.Fatalf("displayItems = %d, want 0 for no-match filter", len(m.displayItems))
	}

	// View should still render without panic
	view := m.View()
	if view == "" {
		t.Fatal("View() returned empty with no matches")
	}
}

func TestIntegration_FilterCaseInsensitive(t *testing.T) {
	m := newTestModeModel(t)

	m.searchInput.SetValue("ALPHA")
	m.rebuildDisplayItems()

	// Should match service-alpha (case-insensitive)
	found := false
	for _, item := range m.displayItems {
		if item.Container != nil && item.Container.Name == "service-alpha" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("case-insensitive filter should match service-alpha")
	}
}

// --- Test 3: Detail entry/exit ---

func TestIntegration_EnterContainerDetail(t *testing.T) {
	m := newTestModeModel(t)
	m.focusedPanel = PanelContainers

	// Navigate to a container (skip past first group header)
	m, _ = applyKey(m, "j") // move to apps container (gamma)

	// Find the first container in displayItems
	containerIdx := -1
	for i, item := range m.displayItems {
		if item.Kind == DisplayContainer {
			containerIdx = i
			break
		}
	}
	if containerIdx < 0 {
		t.Fatal("no container found in displayItems")
	}

	// Select that container
	m.selectedIndex = containerIdx

	// Press enter to open detail
	m, cmd := applyKey(m, "enter")

	if m.viewMode != ViewDetail {
		t.Fatalf("viewMode = %d after enter, want ViewDetail", m.viewMode)
	}
	if m.detailContainerID == "" {
		t.Fatal("detailContainerID is empty after entering detail")
	}

	// Execute the command to get mock logs/detail
	if cmd != nil {
		// In test mode, cmd returns mock data
		batchMsgs := executeBatchCmd(cmd)
		for _, msg := range batchMsgs {
			m, _ = applyMsg(m, msg)
		}
	}

	// View should render detail view
	view := stripANSIForTest(m.View())
	if !strings.Contains(view, m.detailContainer.Name) {
		t.Errorf("detail View() should contain container name %q", m.detailContainer.Name)
	}
}

func TestIntegration_ExitDetailWithEsc(t *testing.T) {
	m := newTestModeModel(t)

	// Enter detail view
	container := &m.dockerData.Containers[0]
	m.enterDetailView(container)

	// Feed mock logs
	m, _ = applyMsg(m, collectMockLogsCmd(container.ID, 50))
	m, _ = applyMsg(m, collectMockDetailCmd(container.ID))

	if m.viewMode != ViewDetail {
		t.Fatalf("viewMode = %d, want ViewDetail", m.viewMode)
	}

	// Press Esc to exit
	m, _ = applyKey(m, "esc")

	if m.viewMode != ViewDashboard {
		t.Fatalf("viewMode = %d after esc, want ViewDashboard", m.viewMode)
	}
	if m.detailContainerID != "" {
		t.Fatalf("detailContainerID = %q, want empty after exit", m.detailContainerID)
	}
}

func TestIntegration_ExitDetailWithQ(t *testing.T) {
	m := newTestModeModel(t)

	// Enter detail view
	container := &m.dockerData.Containers[0]
	m.enterDetailView(container)

	if m.viewMode != ViewDetail {
		t.Fatalf("viewMode = %d, want ViewDetail", m.viewMode)
	}

	// Press q to exit
	m, _ = applyKey(m, "q")

	if m.viewMode != ViewDashboard {
		t.Fatalf("viewMode = %d after q, want ViewDashboard", m.viewMode)
	}
}

func TestIntegration_EnterStackDetail(t *testing.T) {
	m := newTestModeModel(t)
	m.focusedPanel = PanelContainers

	// Select the first group header (apps stack at index 0)
	m.selectedIndex = 0
	if m.displayItems[0].Kind != DisplayGroup {
		t.Fatal("displayItems[0] should be a group header")
	}

	// Space to open quick menu on stack
	m, _ = applyKey(m, " ")
	if !m.quickMenuOpen {
		t.Fatal("quickMenuOpen = false after space on group")
	}

	// Enter to select first item (logs)
	m, cmd := applyKey(m, "enter")

	if m.viewMode != ViewDetail {
		t.Fatalf("viewMode = %d, want ViewDetail after entering stack logs", m.viewMode)
	}
	if m.detailStackName == "" {
		t.Fatal("detailStackName is empty after entering stack detail")
	}

	// Execute command
	if cmd != nil {
		batchMsgs := executeBatchCmd(cmd)
		for _, msg := range batchMsgs {
			m, _ = applyMsg(m, msg)
		}
	}
}

// --- Test 4: Quick-menu rendering ---

func TestIntegration_QuickMenuRunningContainer(t *testing.T) {
	m := newTestModeModel(t)
	m.focusedPanel = PanelContainers

	// Find a running container
	for i, item := range m.displayItems {
		if item.Kind == DisplayContainer && item.Container != nil && item.Container.State == "running" {
			m.selectedIndex = i
			break
		}
	}

	// Space opens quick menu
	m, _ = applyKey(m, " ")

	if !m.quickMenuOpen {
		t.Fatal("quickMenuOpen = false after space on running container")
	}
	if m.quickMenuContainerID == "" {
		t.Fatal("quickMenuContainerID is empty")
	}

	// Check items are correct for a running container
	items := m.currentQuickMenuItems()
	if len(items) != 3 {
		t.Fatalf("quick menu items = %d, want 3 for running container", len(items))
	}
	actions := make([]string, len(items))
	for i, item := range items {
		actions[i] = item.action
	}
	expected := []string{"logs", "stop", "restart"}
	for i, exp := range expected {
		if actions[i] != exp {
			t.Errorf("action[%d] = %q, want %q", i, actions[i], exp)
		}
	}

	// View should render without panic
	view := m.View()
	if view == "" {
		t.Fatal("View() empty with quick menu open")
	}
}

func TestIntegration_QuickMenuStoppedContainer(t *testing.T) {
	m := newTestModeModel(t)
	m.focusedPanel = PanelContainers

	// Find the stopped container (service-gamma)
	for i, item := range m.displayItems {
		if item.Kind == DisplayContainer && item.Container != nil && item.Container.State == "exited" {
			m.selectedIndex = i
			break
		}
	}

	// Space opens quick menu
	m, _ = applyKey(m, " ")

	if !m.quickMenuOpen {
		t.Fatal("quickMenuOpen = false after space on stopped container")
	}

	items := m.currentQuickMenuItems()
	if len(items) != 2 {
		t.Fatalf("quick menu items = %d, want 2 for stopped container", len(items))
	}
	if items[0].action != "logs" {
		t.Errorf("items[0] = %q, want logs", items[0].action)
	}
	if items[1].action != "start" {
		t.Errorf("items[1] = %q, want start", items[1].action)
	}
}

func TestIntegration_QuickMenuNavigateAndClose(t *testing.T) {
	m := newTestModeModel(t)
	m.focusedPanel = PanelContainers

	// Select a running container
	for i, item := range m.displayItems {
		if item.Kind == DisplayContainer && item.Container != nil && item.Container.State == "running" {
			m.selectedIndex = i
			break
		}
	}

	// Open quick menu
	m, _ = applyKey(m, " ")
	if !m.quickMenuOpen {
		t.Fatal("quickMenuOpen should be true")
	}
	if m.quickMenuIndex != 0 {
		t.Fatalf("quickMenuIndex = %d, want 0", m.quickMenuIndex)
	}

	// Navigate down
	m, _ = applyKey(m, "j")
	if m.quickMenuIndex != 1 {
		t.Fatalf("quickMenuIndex = %d after j, want 1", m.quickMenuIndex)
	}

	// Navigate up
	m, _ = applyKey(m, "k")
	if m.quickMenuIndex != 0 {
		t.Fatalf("quickMenuIndex = %d after k, want 0", m.quickMenuIndex)
	}

	// Close with Esc
	m, _ = applyKey(m, "esc")
	if m.quickMenuOpen {
		t.Fatal("quickMenuOpen should be false after esc")
	}
}

func TestIntegration_QuickMenuStackGroup(t *testing.T) {
	m := newTestModeModel(t)
	m.focusedPanel = PanelContainers

	// Select the infra group (has running containers)
	for i, item := range m.displayItems {
		if item.Kind == DisplayGroup && item.StackName == "infra" {
			m.selectedIndex = i
			break
		}
	}

	// Space opens stack quick menu
	m, _ = applyKey(m, " ")

	if !m.quickMenuOpen {
		t.Fatal("quickMenuOpen = false for stack group")
	}
	if m.quickMenuStackName != "infra" {
		t.Fatalf("quickMenuStackName = %q, want %q", m.quickMenuStackName, "infra")
	}

	items := m.currentQuickMenuItems()
	if len(items) == 0 {
		t.Fatal("stack quick menu has no items")
	}
	// First item should be logs
	if items[0].action != "logs" {
		t.Fatalf("stack menu items[0] = %q, want logs", items[0].action)
	}
}

// --- Test: Dashboard navigation ---

func TestIntegration_NavigationJK(t *testing.T) {
	m := newTestModeModel(t)
	m.focusedPanel = PanelContainers

	initialIdx := m.selectedIndex

	// Move down
	m, _ = applyKey(m, "j")
	if m.selectedIndex != initialIdx+1 {
		t.Fatalf("selectedIndex = %d after j, want %d", m.selectedIndex, initialIdx+1)
	}

	// Move up
	m, _ = applyKey(m, "k")
	if m.selectedIndex != initialIdx {
		t.Fatalf("selectedIndex = %d after k, want %d", m.selectedIndex, initialIdx)
	}
}

func TestIntegration_TabCyclesPanels(t *testing.T) {
	m := newTestModeModel(t)

	if m.focusedPanel != PanelContainers {
		t.Fatalf("initial panel = %d, want PanelContainers", m.focusedPanel)
	}

	m, _ = applyKey(m, "tab")
	if m.focusedPanel != PanelWeather {
		t.Fatalf("panel after tab = %d, want PanelWeather", m.focusedPanel)
	}

	m, _ = applyKey(m, "tab")
	if m.focusedPanel != PanelSystem {
		t.Fatalf("panel after 2nd tab = %d, want PanelSystem", m.focusedPanel)
	}

	m, _ = applyKey(m, "tab")
	if m.focusedPanel != PanelContainers {
		t.Fatalf("panel after 3rd tab = %d, want PanelContainers (cycle)", m.focusedPanel)
	}
}

// --- Test: Refresh in test mode ---

func TestIntegration_RefreshInTestMode(t *testing.T) {
	m := newTestModeModel(t)
	m.focusedPanel = PanelContainers

	// Press 'r' to refresh
	m, cmd := applyKey(m, "r")

	if !m.refreshing {
		t.Fatal("refreshing = false after 'r'")
	}

	// Should have produced commands
	if cmd == nil {
		t.Fatal("cmd = nil after refresh, want batch of mock collectors")
	}

	// Execute the batch
	msgs := executeBatchCmd(cmd)
	for _, msg := range msgs {
		m, _ = applyMsg(m, msg)
	}

	// Data should still be synthetic
	if m.systemData.Hostname != "synthetic-host" {
		t.Fatalf("hostname after refresh = %q, want synthetic-host", m.systemData.Hostname)
	}
}

// --- Test: Full user flow ---

func TestIntegration_FullDashboardToDetailAndBack(t *testing.T) {
	m := newTestModeModel(t)

	// 1. Verify dashboard renders
	view := stripANSIForTest(m.View())
	if !strings.Contains(view, "service-alpha") {
		t.Fatal("dashboard should show service-alpha")
	}

	// 2. Filter to narrow down
	m.searchInput.SetValue("beta")
	m.rebuildDisplayItems()
	if len(m.displayItems) != 2 {
		t.Fatalf("filtered to %d items, want 2", len(m.displayItems))
	}

	// 3. Clear filter
	m.focusedPanel = PanelContainers
	m, _ = applyKey(m, "esc")
	if len(m.displayItems) != 5 {
		t.Fatalf("items after clear = %d, want 5", len(m.displayItems))
	}

	// 4. Navigate to service-beta and enter detail
	for i, item := range m.displayItems {
		if item.Container != nil && item.Container.Name == "service-beta" {
			m.selectedIndex = i
			break
		}
	}
	m, cmd := applyKey(m, "enter")
	if m.viewMode != ViewDetail {
		t.Fatalf("viewMode = %d, want ViewDetail", m.viewMode)
	}

	// Feed mock data
	if cmd != nil {
		msgs := executeBatchCmd(cmd)
		for _, msg := range msgs {
			m, _ = applyMsg(m, msg)
		}
	}

	// 5. Detail view should show logs
	if len(m.detailLogs) == 0 {
		t.Fatal("detailLogs empty after feeding mock logs")
	}

	// 6. Exit back to dashboard
	m, _ = applyKey(m, "esc")
	if m.viewMode != ViewDashboard {
		t.Fatalf("viewMode = %d, want ViewDashboard", m.viewMode)
	}

	// 7. Dashboard should render normally
	view = stripANSIForTest(m.View())
	if !strings.Contains(view, "service-alpha") {
		t.Fatal("dashboard should show service-alpha after returning from detail")
	}
}

// --- Helpers ---

// executeBatchCmd tries to extract messages from a tea.Cmd by running it.
// For simple func() tea.Msg commands and tea.Batch, this collects results.
func executeBatchCmd(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	if msg == nil {
		return nil
	}
	// Check if it's a batch message (tea.BatchMsg is []tea.Cmd)
	if batch, ok := msg.(tea.BatchMsg); ok {
		var msgs []tea.Msg
		for _, c := range batch {
			if c != nil {
				if m := c(); m != nil {
					msgs = append(msgs, m)
				}
			}
		}
		return msgs
	}
	return []tea.Msg{msg}
}

// Verify the mock data containers for reference
func TestIntegration_MockDataConsistency(t *testing.T) {
	sysMsg := collectMockSystemCmd().(SystemDataMsg)
	if sysMsg.Data.Hostname != "synthetic-host" {
		t.Fatalf("mock hostname = %q, want synthetic-host", sysMsg.Data.Hostname)
	}

	dockerMsg := collectMockDockerCmd().(DockerDataMsg)
	if len(dockerMsg.Data.Containers) != 3 {
		t.Fatalf("mock containers = %d, want 3", len(dockerMsg.Data.Containers))
	}

	weatherMsg := collectMockWeatherCmd().(WeatherDataMsg)
	if weatherMsg.Data.Location != "Synthetic Location" {
		t.Fatalf("mock location = %q, want Synthetic Location", weatherMsg.Data.Location)
	}
}

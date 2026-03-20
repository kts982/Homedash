package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/kts982/homedash/internal/config"
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

// keyMsg constructs a tea.KeyPressMsg from a string representation.
func keyMsg(key string) tea.KeyPressMsg {
	switch key {
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	case "tab":
		return tea.KeyPressMsg{Code: tea.KeyTab}
	case "shift+tab":
		return tea.KeyPressMsg{Text: "shift+tab"}
	case "ctrl+c":
		return tea.KeyPressMsg{Text: "ctrl+c"}
	case "ctrl+d":
		return tea.KeyPressMsg{Text: "ctrl+d"}
	case "ctrl+u":
		return tea.KeyPressMsg{Text: "ctrl+u"}
	case "pgdown":
		return tea.KeyPressMsg{Code: tea.KeyPgDown}
	case "pgup":
		return tea.KeyPressMsg{Code: tea.KeyPgUp}
	case "home":
		return tea.KeyPressMsg{Code: tea.KeyHome}
	case "end":
		return tea.KeyPressMsg{Code: tea.KeyEnd}
	case " ":
		return tea.KeyPressMsg{Text: "space"}
	default:
		return tea.KeyPressMsg{Text: key}
	}
}

// stripANSIForTest removes ANSI escape sequences for easier assertion.
func stripANSIForTest(v tea.View) string {
	s := v.Content
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
	if view.Content == "" {
		t.Fatal("View() returned empty string")
	}
	if view.Content == "Loading..." {
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

func TestIntegration_SettingsOpenAndClose(t *testing.T) {
	m := newTestModeModel(t)

	m, _ = applyKey(m, "O")
	if !m.settingsOpen {
		t.Fatal("settingsOpen = false after pressing O")
	}
	plain := stripANSIForTest(m.View())
	if !strings.Contains(plain, "Options") {
		t.Fatalf("View() = %q, want settings overlay title", plain)
	}

	m, _ = applyKey(m, "esc")
	if m.settingsOpen {
		t.Fatal("settingsOpen = true after pressing esc")
	}
}

func TestIntegration_SettingsSaveAppliesConfig(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	m := newTestModeModel(t)
	m, _ = applyKey(m, "O")
	if !m.settingsOpen {
		t.Fatal("settingsOpen = false after pressing O")
	}

	m.settingsForm.cycleTheme(1)
	m.settingsForm.dockerHost.SetValue("tcp://127.0.0.1:2375")
	m.settingsForm.systemRefresh.SetValue("3s")
	m.settingsForm.dockerRefresh.SetValue("7s")
	m.settingsForm.weatherRefresh.SetValue("10m")
	m.settingsForm.disks = []settingsDiskRow{
		newSettingsDiskRow(config.Disk{Path: "/", Label: "System"}),
		newSettingsDiskRow(config.Disk{Path: "/mnt/archive", Label: "Archive"}),
	}
	_ = m.settingsForm.focusCurrent()

	updated, cmd := applyKey(m, "enter")
	if cmd == nil {
		t.Fatal("cmd = nil, want settings save command")
	}
	msg := cmd()
	saved, _ := applyMsg(updated, msg)

	if saved.settingsOpen {
		t.Fatal("settingsOpen = true after successful save")
	}
	if saved.themeName != "catppuccin" {
		t.Fatalf("themeName = %q, want catppuccin", saved.themeName)
	}
	if got := saved.dockerHost; got != "tcp://127.0.0.1:2375" {
		t.Fatalf("dockerHost = %q, want tcp://127.0.0.1:2375", got)
	}
	if got := saved.systemRefreshInterval; got != 3*time.Second {
		t.Fatalf("systemRefreshInterval = %v, want 3s", got)
	}
	if got := saved.dockerRefreshInterval; got != 7*time.Second {
		t.Fatalf("dockerRefreshInterval = %v, want 7s", got)
	}
	if got := saved.weatherRefreshInterval; got != 10*time.Minute {
		t.Fatalf("weatherRefreshInterval = %v, want 10m", got)
	}
	if len(saved.disks) != 2 {
		t.Fatalf("len(disks) = %d, want 2", len(saved.disks))
	}

	configPath := filepath.Join(tmpDir, "homedash", "config.yaml")
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("saved config file missing: %v", err)
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

// TestIntegration_FilterTypedInputPath exercises the real user-input wiring:
// '/' to focus → typed runes through Update() → textinput updates → rebuild →
// Enter to confirm → Esc to clear. This verifies the full filtering pipeline
// including the textinput component, not just direct SetValue.
func TestIntegration_FilterTypedInputPath(t *testing.T) {
	m := newTestModeModel(t)

	// All 5 items visible initially
	if len(m.displayItems) != 5 {
		t.Fatalf("initial displayItems = %d, want 5", len(m.displayItems))
	}

	// Press '/' to activate filter mode
	m, _ = applyKey(m, "/")
	if !m.filtering {
		t.Fatal("filtering = false after '/'")
	}

	// Type "beta" character by character through Update()
	for _, ch := range "beta" {
		m, _ = applyMsg(m, tea.KeyPressMsg{Text: string(ch)})
	}

	// The textinput should have the typed value
	if got := m.searchInput.Value(); got != "beta" {
		t.Fatalf("searchInput.Value() = %q, want %q", got, "beta")
	}

	// Display items should be filtered to infra group + service-beta
	if len(m.displayItems) != 2 {
		t.Fatalf("filtered displayItems = %d, want 2", len(m.displayItems))
	}
	if m.displayItems[1].Container == nil || m.displayItems[1].Container.Name != "service-beta" {
		t.Fatal("filtered container should be service-beta")
	}

	// Press Enter to confirm and exit filter mode
	m, _ = applyKey(m, "enter")
	if m.filtering {
		t.Fatal("filtering = true after Enter, want false")
	}
	// Filter value should persist after Enter
	if m.searchInput.Value() != "beta" {
		t.Fatalf("searchInput = %q after Enter, want %q", m.searchInput.Value(), "beta")
	}
	// Items should still be filtered
	if len(m.displayItems) != 2 {
		t.Fatalf("displayItems after Enter = %d, want 2 (filter persists)", len(m.displayItems))
	}

	// Press Esc on dashboard to clear the filter
	m.focusedPanel = PanelContainers
	m, _ = applyKey(m, "esc")
	if m.searchInput.Value() != "" {
		t.Fatalf("searchInput = %q after Esc, want empty", m.searchInput.Value())
	}
	if len(m.displayItems) != 5 {
		t.Fatalf("displayItems after Esc = %d, want 5 (all visible)", len(m.displayItems))
	}
}

// TestIntegration_FilterEscDuringTyping tests pressing Esc while still in
// filtering mode (textinput focused) — should clear and exit filter.
func TestIntegration_FilterEscDuringTyping(t *testing.T) {
	m := newTestModeModel(t)

	// Activate filter and type partial text
	m, _ = applyKey(m, "/")
	for _, ch := range "al" {
		m, _ = applyMsg(m, tea.KeyPressMsg{Text: string(ch)})
	}
	if m.searchInput.Value() != "al" {
		t.Fatalf("searchInput = %q, want %q", m.searchInput.Value(), "al")
	}

	// Esc while filtering should clear the input and restore all items
	m, _ = applyKey(m, "esc")
	if m.filtering {
		t.Fatal("filtering should be false after Esc")
	}
	if m.searchInput.Value() != "" {
		t.Fatalf("searchInput = %q after Esc, want empty", m.searchInput.Value())
	}
	if len(m.displayItems) != 5 {
		t.Fatalf("displayItems = %d after Esc, want 5", len(m.displayItems))
	}
}

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

func TestIntegration_FilterStateTokenNarrowsList(t *testing.T) {
	m := newTestModeModel(t)

	m.searchInput.SetValue("state:running")
	m.rebuildDisplayItems()

	if len(m.displayItems) != 3 {
		t.Fatalf("displayItems = %d, want 3 (group + 2 running containers)", len(m.displayItems))
	}
	if got := m.displayItems[0].StackName; got != "infra" {
		t.Fatalf("filtered group = %q, want %q", got, "infra")
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
	if view.Content == "" {
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

func TestIntegration_StackLogsShortcut(t *testing.T) {
	m := newTestModeModel(t)
	m.focusedPanel = PanelContainers

	m.selectedIndex = 0
	if m.displayItems[0].Kind != DisplayGroup {
		t.Fatal("displayItems[0] should be a group header")
	}

	m, cmd := applyKey(m, "l")

	if m.viewMode != ViewDetail {
		t.Fatalf("viewMode = %d, want ViewDetail after stack logs shortcut", m.viewMode)
	}
	if m.detailStackName == "" {
		t.Fatal("detailStackName is empty after stack logs shortcut")
	}

	if cmd != nil {
		batchMsgs := executeBatchCmd(cmd)
		for _, msg := range batchMsgs {
			m, _ = applyMsg(m, msg)
		}
	}
}

func TestIntegration_CycleSortModeReordersDashboard(t *testing.T) {
	m := newTestModeModel(t)
	m.focusedPanel = PanelContainers

	m, _ = applyKey(m, "o")

	if m.dashboardSort != DashboardSortCPU {
		t.Fatalf("dashboardSort = %v, want %v", m.dashboardSort, DashboardSortCPU)
	}
	if m.displayItems[0].Kind != DisplayGroup || m.displayItems[0].StackName != "infra" {
		t.Fatalf("displayItems[0] = %+v, want infra group first after cpu sort", m.displayItems[0])
	}
}

func TestIntegration_AlertsDrawerShowsActiveProblems(t *testing.T) {
	m := newTestModeModel(t)
	m.focusedPanel = PanelContainers
	m.dockerData.Containers[1].Health = "unhealthy"
	m.rebuildDisplayItems()

	m, _ = applyKey(m, "a")
	if !m.alertsOpen {
		t.Fatal("alertsOpen = false after pressing a")
	}

	view := m.View()
	plain := stripANSIForTest(view)
	for _, want := range []string{"ALERTS", "Active problems", "1 unhealthy containers"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("View() = %q, want substring %q", plain, want)
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
	if view.Content == "" {
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
	if m.focusedPanel != PanelSystem {
		t.Fatalf("panel after tab = %d, want PanelSystem", m.focusedPanel)
	}

	m, _ = applyKey(m, "tab")
	if m.focusedPanel != PanelContainers {
		t.Fatalf("panel after 2nd tab = %d, want PanelContainers (cycle)", m.focusedPanel)
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

func TestFocus_BlurPausesDashboardTicks(t *testing.T) {
	m := newTestModeModel(t)
	m.TestMode = false // Enable ticks for this test logic

	// 1. Lose focus
	m, _ = applyMsg(m, tea.BlurMsg{})
	if m.focused {
		t.Fatal("Expected focused = false after BlurMsg")
	}

	// 2. Simulate data arrival (System)
	// It should NOT produce a new tick command because we are blurred on dashboard
	_, cmd := m.Update(SystemDataMsg{Data: m.systemData})
	if cmd != nil {
		t.Error("Expected no tick command after SystemDataMsg while blurred on dashboard")
	}

	// 3. Simulate data arrival (Docker)
	_, cmd = m.Update(DockerDataMsg{Data: m.dockerData})
	if cmd != nil {
		t.Error("Expected no tick command after DockerDataMsg while blurred on dashboard")
	}

	// 4. Simulate data arrival (Weather)
	_, cmd = m.Update(WeatherDataMsg{Data: m.weatherData})
	if cmd != nil {
		t.Error("Expected no tick command after WeatherDataMsg while blurred on dashboard")
	}
}

func TestFocus_FocusResumesDashboardTicks(t *testing.T) {
	m := newTestModeModel(t)
	m.TestMode = false

	// 1. Lose focus
	m, _ = applyMsg(m, tea.BlurMsg{})

	// 2. Gain focus
	// It should return a batch command to refresh all data immediately
	m, cmd := applyMsg(m, tea.FocusMsg{})
	if !m.focused {
		t.Fatal("Expected focused = true after FocusMsg")
	}
	if cmd == nil {
		t.Fatal("Expected batch refresh command after FocusMsg")
	}

	// 3. Simulate data arrival (System) after focus
	// It should now produce a new tick command because we are focused
	_, cmd = m.Update(SystemDataMsg{Data: m.systemData})
	if cmd == nil {
		t.Error("Expected tick command after SystemDataMsg while focused")
	}
}

func TestFocus_BlurDoesNotPauseDetailTicks(t *testing.T) {
	m := newTestModeModel(t)
	m.TestMode = false

	// 1. Enter detail view
	m.viewMode = ViewDetail
	m.detailContainerID = "abc"

	// 2. Lose focus
	m, _ = applyMsg(m, tea.BlurMsg{})

	// 3. Simulate data arrival (Docker)
	// It should STILL produce a tick command because we are in detail view
	_, cmd := m.Update(DockerDataMsg{Data: m.dockerData})
	if cmd == nil {
		t.Error("Expected tick command after DockerDataMsg while blurred in detail view")
	}
}

func TestFocus_PendingTickDiscardedWhileBlurred(t *testing.T) {
	m := newTestModeModel(t)
	m.TestMode = false

	// 1. Lose focus
	m, _ = applyMsg(m, tea.BlurMsg{})

	// 2. Simulate a pending tick firing (SystemTickMsg arrives while blurred)
	// It should NOT trigger a collection command
	_, cmd := m.Update(SystemTickMsg{Epoch: 0})
	if cmd != nil {
		t.Error("Expected no collection command when SystemTickMsg arrives while blurred on dashboard")
	}

	// 3. Same for Docker and Weather
	_, cmd = m.Update(DockerTickMsg{Epoch: 0})
	if cmd != nil {
		t.Error("Expected no collection command when DockerTickMsg arrives while blurred on dashboard")
	}
	_, cmd = m.Update(WeatherTickMsg{Epoch: 0})
	if cmd != nil {
		t.Error("Expected no collection command when WeatherTickMsg arrives while blurred on dashboard")
	}
}

func TestFocus_OldPendingTickDiscardedAfterRefocus(t *testing.T) {
	m := newTestModeModel(t)
	m.TestMode = false

	// 1. Lose focus (epoch stays 0)
	m, _ = applyMsg(m, tea.BlurMsg{})

	// 2. Gain focus (epoch increments to 1)
	m, _ = applyMsg(m, tea.FocusMsg{})
	if m.tickEpoch != 1 {
		t.Fatalf("tickEpoch = %d, want 1 after refocus", m.tickEpoch)
	}

	// 3. Simulate an OLD pending tick arriving (Epoch 0)
	// It should be discarded even though we are focused now
	_, cmd := m.Update(SystemTickMsg{Epoch: 0})
	if cmd != nil {
		t.Error("Expected old SystemTickMsg (Epoch 0) to be discarded after refocus (Epoch 1)")
	}

	// 4. Simulate a NEW tick arriving (Epoch 1)
	// It should be processed
	_, cmd = m.Update(SystemTickMsg{Epoch: 1})
	if cmd == nil {
		t.Error("Expected new SystemTickMsg (Epoch 1) to be processed")
	}
}

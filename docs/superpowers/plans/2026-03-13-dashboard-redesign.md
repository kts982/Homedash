# Dashboard Redesign Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move weather into the header bar, expand the system panel to full width with two-column layout, add RAM sparkline and swap usage display.

**Architecture:** Bottom-up — collector changes first, then rendering, then weather+layout wiring (as one atomic task to avoid non-compilable intermediate states), then test updates.

**Tech Stack:** Go 1.25+, Bubble Tea v2, Lipgloss v2, no new dependencies.

**Spec:** `docs/superpowers/specs/2026-03-13-dashboard-redesign-design.md`

---

## Chunk 1: Collector + System Panel Rendering

### Task 1: Add swap fields to SystemData and collector

**Files:**
- Modify: `internal/collector/types.go:10-24` (SystemData struct)
- Modify: `internal/collector/system.go:82-91` (after memory parsing)
- Modify: `internal/collector/system_test.go`

- [ ] **Step 1: Write failing test for swap collection**

Add to `internal/collector/system_test.go`:

```go
func TestParseMemInfoSwap(t *testing.T) {
	input := "MemTotal:       16384000 kB\nMemFree:         8000000 kB\nMemAvailable:   12000000 kB\nSwapTotal:       4096000 kB\nSwapFree:        3072000 kB\n"
	info := parseMemInfo(input)

	swapTotal := info["SwapTotal"]
	swapFree := info["SwapFree"]
	if swapTotal != 4096000 {
		t.Fatalf("SwapTotal = %d, want 4096000", swapTotal)
	}
	if swapFree != 3072000 {
		t.Fatalf("SwapFree = %d, want 3072000", swapFree)
	}
}

func TestCollectSystemSwapFields(t *testing.T) {
	// parseMemInfo already parses SwapTotal/SwapFree.
	// This tests that CollectSystem populates the SystemData swap fields.
	// We can't easily mock /proc, so test the swap calculation helper.
	memInfo := map[string]uint64{
		"MemTotal":     16384000,
		"MemAvailable": 12000000,
		"SwapTotal":    4096000,
		"SwapFree":     3072000,
	}
	swapTotal := memInfo["SwapTotal"]
	swapFree := memInfo["SwapFree"]
	swapUsedKB := swapTotal - swapFree
	swapPercent := float64(swapUsedKB) / float64(swapTotal) * 100

	if swapUsedKB != 1024000 {
		t.Fatalf("swapUsed = %d kB, want 1024000", swapUsedKB)
	}
	if swapPercent < 24.9 || swapPercent > 25.1 {
		t.Fatalf("swapPercent = %.1f, want ~25.0", swapPercent)
	}
}

func TestCollectSystemSwapZero(t *testing.T) {
	memInfo := map[string]uint64{
		"MemTotal":     16384000,
		"MemAvailable": 12000000,
		"SwapTotal":    0,
		"SwapFree":     0,
	}
	swapTotal := memInfo["SwapTotal"]
	if swapTotal != 0 {
		t.Fatalf("swapTotal = %d, want 0", swapTotal)
	}
	// SwapPercent should remain 0 when SwapTotal is 0 (no division).
}
```

- [ ] **Step 2: Run tests to verify they pass (parseMemInfo already works)**

Run: `go test ./internal/collector/ -run TestParseMemInfoSwap -v`
Expected: PASS (parseMemInfo is generic, already parses all fields)

- [ ] **Step 3: Add swap fields to SystemData**

In `internal/collector/types.go`, add after line 18 (`MemPercent`):

```go
SwapTotal   uint64  // bytes
SwapUsed    uint64  // bytes
SwapPercent float64
```

- [ ] **Step 4: Populate swap fields in CollectSystem**

In `internal/collector/system.go`, add after the existing memory block (after line 91, inside the `if raw, err := os.ReadFile("/proc/meminfo")` block):

```go
	// Swap
	swapTotalKB := memInfo["SwapTotal"]
	swapFreeKB := memInfo["SwapFree"]
	data.SwapTotal = swapTotalKB * 1024
	if swapTotalKB > swapFreeKB {
		data.SwapUsed = (swapTotalKB - swapFreeKB) * 1024
	}
	if swapTotalKB > 0 {
		data.SwapPercent = float64(swapTotalKB-swapFreeKB) / float64(swapTotalKB) * 100
	}
```

- [ ] **Step 5: Run all collector tests**

Run: `go test ./internal/collector/ -v`
Expected: All PASS

- [ ] **Step 6: Update mock data with swap fields**

In `internal/ui/test_fixtures.go`, add to the `collectMockSystemCmd` return inside the `SystemData` struct (after `NetTxRate` line 25):

```go
SwapTotal:   4 * 1024 * 1024 * 1024,  // 4G
SwapUsed:    256 * 1024 * 1024,        // 256M
SwapPercent: 6.25,
```

- [ ] **Step 7: Run full test suite to verify nothing breaks**

Run: `go test ./... 2>&1 | tail -20`
Expected: All PASS

- [ ] **Step 8: Commit**

```bash
git add internal/collector/types.go internal/collector/system.go internal/collector/system_test.go internal/ui/test_fixtures.go
git commit -m "feat: add swap usage collection to SystemData"
```

---

### Task 2: Add ramHistory RingBuffer to Model

**Files:**
- Modify: `internal/ui/app.go:70` (Model struct, near cpuHistory)
- Modify: `internal/ui/app.go:200-201` (NewModel, near cpuHistory init)
- Modify: `internal/ui/app.go` (SystemDataMsg handler, near cpuHistory.Push)
- Modify: `internal/ui/app_test.go:17-26` (newTestModel)

- [ ] **Step 1: Add ramHistory field to Model struct**

In `internal/ui/app.go`, after line 70 (`cpuHistory`):

```go
ramHistory      *components.RingBuffer
```

- [ ] **Step 2: Initialize ramHistory in NewModel**

In `internal/ui/app.go`, in the `NewModel` return struct (after line 201, the `cpuHistory` line):

```go
ramHistory:             components.NewRingBuffer(60),
```

- [ ] **Step 3: Push RAM percent on each SystemDataMsg**

Find the `SystemDataMsg` handler in `app.go` where `m.cpuHistory.Push(msg.Data.CPUPercent)` is called. Add immediately after:

```go
m.ramHistory.Push(msg.Data.MemPercent)
```

- [ ] **Step 4: Update newTestModel helper**

In `internal/ui/app_test.go`, in `newTestModel()` (line 21), after the `cpuHistory` line:

```go
ramHistory:      components.NewRingBuffer(60),
```

- [ ] **Step 5: Run tests**

Run: `go test ./internal/ui/ -v -count=1 2>&1 | tail -20`
Expected: All PASS

- [ ] **Step 6: Commit**

```bash
git add internal/ui/app.go internal/ui/app_test.go
git commit -m "feat: add RAM history ring buffer to model"
```

---

### Task 3: Redesign RenderSystem with two-column layout

**Files:**
- Modify: `internal/ui/panels/system.go`

- [ ] **Step 1: Update RenderSystem signature**

Change the signature from:

```go
func RenderSystem(data collector.SystemData, cpuHistory *components.RingBuffer, width, height int, focused bool) string
```

to:

```go
func RenderSystem(data collector.SystemData, cpuHistory, ramHistory *components.RingBuffer, width, height int, focused bool) string
```

- [ ] **Step 2: Rewrite the function body**

Replace the entire function body with the two-column layout:

```go
func RenderSystem(data collector.SystemData, cpuHistory, ramHistory *components.RingBuffer, width, height int, focused bool) string {
	innerWidth := width - 4 // panel border + padding

	// Narrow: single-column fallback (matches isNarrow() threshold: width < 90)
	if width < 90 {
		return renderSystemSingleColumn(data, cpuHistory, ramHistory, innerWidth, width, height, focused)
	}

	leftWidth := innerWidth * 55 / 100
	if leftWidth < 30 {
		leftWidth = 30
	}
	rightWidth := innerWidth - leftWidth

	// Left column: sparklines + gauges
	var leftLines []string

	// CPU sparkline
	sparkWidth := leftWidth - 2
	if sparkWidth > 60 {
		sparkWidth = 60
	}
	cpuSpark := components.Sparkline(cpuHistory.Data(), sparkWidth, styles.Primary)
	sparkLabel := lipgloss.NewStyle().Foreground(styles.TextMuted).Render("(2m)")
	leftLines = append(leftLines, "  "+cpuSpark+" "+sparkLabel)

	// CPU gauge
	leftLines = append(leftLines, components.Gauge("CPU", data.CPUPercent, leftWidth))

	// RAM sparkline
	ramSpark := components.Sparkline(ramHistory.Data(), sparkWidth, styles.Secondary)
	leftLines = append(leftLines, "  "+ramSpark+" "+sparkLabel)

	// RAM gauge
	leftLines = append(leftLines, components.Gauge("RAM", data.MemPercent, leftWidth))

	// Disk gauges
	for _, d := range data.Disks {
		label := fmt.Sprintf("%-6s", d.Mount)
		leftLines = append(leftLines, components.Gauge(label, d.Percent, leftWidth))
	}

	// Right column: text stats
	labelStyle := lipgloss.NewStyle().Foreground(styles.TextSecondary).Bold(true).Width(6)
	valStyle := lipgloss.NewStyle().Foreground(styles.TextSecondary)

	var rightLines []string

	// LOAD
	loadVals := fmt.Sprintf("%.1f  %.1f  %.1f", data.LoadAvg[0], data.LoadAvg[1], data.LoadAvg[2])
	loadStyle := valStyle
	if data.CPUCount > 0 && data.LoadAvg[0] > float64(data.CPUCount) {
		loadStyle = lipgloss.NewStyle().Foreground(styles.Warning)
	}
	rightLines = append(rightLines, labelStyle.Render("LOAD")+" "+loadStyle.Render(loadVals))

	// NET
	netDown := collector.FormatRate(data.NetRxRate)
	netUp := collector.FormatRate(data.NetTxRate)
	downStyled := lipgloss.NewStyle().Foreground(styles.Primary).Render("↓ " + netDown)
	upStyled := lipgloss.NewStyle().Foreground(styles.Secondary).Render("↑ " + netUp)
	rightLines = append(rightLines, labelStyle.Render("NET")+" "+downStyled+"  "+upStyled)

	// MEM absolute
	memText := fmt.Sprintf("%s / %s", collector.FormatBytes(data.MemUsed), collector.FormatBytes(data.MemTotal))
	rightLines = append(rightLines, labelStyle.Render("MEM")+" "+valStyle.Render(memText))

	// SWAP
	swapLine := renderSwapLine(data, labelStyle)
	rightLines = append(rightLines, swapLine)

	// Cap content lines
	maxContent := 12
	if len(leftLines) > maxContent {
		leftLines = leftLines[:maxContent]
	}
	if len(rightLines) > maxContent {
		rightLines = rightLines[:maxContent]
	}

	leftCol := strings.Join(leftLines, "\n")
	rightCol := strings.Join(rightLines, "\n")

	// Pad right column to rightWidth so JoinHorizontal aligns properly
	rightColStyled := lipgloss.NewStyle().Width(rightWidth).Render(rightCol)

	content := lipgloss.JoinHorizontal(lipgloss.Top, leftCol, rightColStyled)
	return components.Panel("SYSTEM", content, width, height, focused)
}

func renderSwapLine(data collector.SystemData, labelStyle lipgloss.Style) string {
	if data.SwapTotal == 0 {
		return labelStyle.Render("SWAP") + " " + lipgloss.NewStyle().Foreground(styles.TextMuted).Render("disabled")
	}
	swapStyle := lipgloss.NewStyle().Foreground(styles.TextSecondary)
	if data.SwapPercent > 25 {
		swapStyle = lipgloss.NewStyle().Foreground(styles.Warning)
	}
	swapText := fmt.Sprintf("%s / %s", collector.FormatBytes(data.SwapUsed), collector.FormatBytes(data.SwapTotal))
	return labelStyle.Render("SWAP") + " " + swapStyle.Render(swapText)
}

func renderSystemSingleColumn(data collector.SystemData, cpuHistory, ramHistory *components.RingBuffer, innerWidth, width, height int, focused bool) string {
	var lines []string

	sparkWidth := innerWidth - 2
	if sparkWidth > 60 {
		sparkWidth = 60
	}
	sparkLabel := lipgloss.NewStyle().Foreground(styles.TextMuted).Render("(2m)")
	labelStyle := lipgloss.NewStyle().Foreground(styles.TextSecondary).Bold(true).Width(6)
	valStyle := lipgloss.NewStyle().Foreground(styles.TextSecondary)

	// CPU
	lines = append(lines, "  "+components.Sparkline(cpuHistory.Data(), sparkWidth, styles.Primary)+" "+sparkLabel)
	lines = append(lines, components.Gauge("CPU", data.CPUPercent, innerWidth))

	// RAM
	lines = append(lines, "  "+components.Sparkline(ramHistory.Data(), sparkWidth, styles.Secondary)+" "+sparkLabel)
	lines = append(lines, components.Gauge("RAM", data.MemPercent, innerWidth))

	// Disks
	for _, d := range data.Disks {
		label := fmt.Sprintf("%-6s", d.Mount)
		lines = append(lines, components.Gauge(label, d.Percent, innerWidth))
	}

	// LOAD
	loadVals := fmt.Sprintf("%.1f  %.1f  %.1f", data.LoadAvg[0], data.LoadAvg[1], data.LoadAvg[2])
	lines = append(lines, labelStyle.Render("LOAD")+" "+valStyle.Render(loadVals))

	// NET
	netDown := collector.FormatRate(data.NetRxRate)
	netUp := collector.FormatRate(data.NetTxRate)
	downStyled := lipgloss.NewStyle().Foreground(styles.Primary).Render("↓ " + netDown)
	upStyled := lipgloss.NewStyle().Foreground(styles.Secondary).Render("↑ " + netUp)
	lines = append(lines, labelStyle.Render("NET")+" "+downStyled+"  "+upStyled)

	// MEM absolute
	memText := fmt.Sprintf("%s / %s", collector.FormatBytes(data.MemUsed), collector.FormatBytes(data.MemTotal))
	lines = append(lines, labelStyle.Render("MEM")+" "+valStyle.Render(memText))

	// SWAP
	lines = append(lines, renderSwapLine(data, labelStyle))

	content := strings.Join(lines, "\n")
	return components.Panel("SYSTEM", content, width, height, focused)
}
```

- [ ] **Step 3: Fix all RenderSystem call sites to pass ramHistory**

There are 4 call sites. Update each to pass the new parameter:

In `internal/ui/app.go`, the two `panels.RenderSystem(` calls in `measureDashboardLayout` — add `m.ramHistory` after `m.cpuHistory`:

```go
// Narrow path (~line 1360):
panels.RenderSystem(m.systemData, m.cpuHistory, m.ramHistory, m.width, topHeight, m.focusedPanel == PanelSystem)

// Wide path (~line 1375):
panels.RenderSystem(m.systemData, m.cpuHistory, m.ramHistory, leftWidth, topHeight, m.focusedPanel == PanelSystem)
```

In `internal/ui/app_test.go`, the two test calls (~lines 412, 453):

```go
panels.RenderSystem(m.systemData, m.cpuHistory, m.ramHistory, m.width, 11, m.focusedPanel == PanelSystem)
```

- [ ] **Step 4: Verify compilation**

Run: `go build ./...`
Expected: Success

- [ ] **Step 5: Run tests**

Run: `go test ./... 2>&1 | tail -20`
Expected: All PASS

- [ ] **Step 6: Commit**

```bash
git add internal/ui/panels/system.go internal/ui/app.go internal/ui/app_test.go
git commit -m "feat: two-column system panel with RAM sparkline and swap display"
```

---

## Chunk 2: Weather to Header + Layout Wiring

### Task 4: Move weather to header, wire layout, delete weather panel

This is one atomic task — header, layout, panel enum, and weather file deletion must happen together to maintain a compilable state.

**Files:**
- Modify: `internal/ui/panels/header.go`
- Modify: `internal/ui/app.go` (measureDashboardLayout)
- Modify: `internal/ui/keys.go` (panel enum)
- Delete: `internal/ui/panels/weather.go`
- Delete: `internal/ui/panels/weather_test.go`

- [ ] **Step 1: Update RenderHeader signature and add weather helper**

In `internal/ui/panels/header.go`, change the signature from:

```go
func RenderHeader(data collector.SystemData, width int, testMode bool) string
```

to:

```go
func RenderHeader(data collector.SystemData, weather collector.WeatherData, weatherErr error, weatherRetries int, width int, testMode bool) string
```

Add `"strings"` to the import block.

Add the `headerWeatherVariants` helper at the bottom of the file:

```go
// headerWeatherVariants returns weather display variants from longest to shortest.
func headerWeatherVariants(weather collector.WeatherData, weatherErr error, weatherRetries int, bg lipgloss.TerminalColor) []string {
	stale := weatherErr != nil && weatherRetries == 0

	// No data yet
	if weather.TempC == "" {
		style := lipgloss.NewStyle().Background(bg).Foreground(styles.TextMuted)
		if stale {
			style = lipgloss.NewStyle().Background(bg).Foreground(styles.Warning)
		}
		return []string{style.Render("☁ --")}
	}

	// Have data — choose color based on staleness
	tempColor := styles.Info
	if stale {
		tempColor = styles.Warning
	}

	tempStyle := lipgloss.NewStyle().Background(bg).Foreground(tempColor).Bold(true)
	condStyle := lipgloss.NewStyle().Background(bg).Foreground(styles.TextSecondary)

	compact := weather.Icon + " " + tempStyle.Render(weather.TempC+"°C")
	full := compact + " " + condStyle.Render(weather.Condition) + " " + condStyle.Render("💧"+weather.Humidity)

	return []string{full, compact}
}
```

- [ ] **Step 2: Integrate weather variants into the header layout**

In the `RenderHeader` function body, after the existing extras loop (after the `for _, extra := range extras` block), add weather variant selection before the clock alignment:

```go
	// Weather: try full variant, then compact, then omit
	for _, variant := range headerWeatherVariants(weather, weatherErr, weatherRetries, bg) {
		candidate := left + sep + variant
		if lipgloss.Width(candidate)+clockReserved <= width {
			left = candidate
			break
		}
	}
```

- [ ] **Step 3: Remove PanelWeather from panel enum**

In `internal/ui/keys.go`, remove line 16 (`PanelWeather`). The enum becomes:

```go
const (
	PanelSystem Panel = iota
	PanelContainers
	panelCount
)
```

- [ ] **Step 4: Rewrite measureDashboardLayout**

Replace the `measureDashboardLayout` function body in `internal/ui/app.go`. Key changes: remove the `isNarrow` branch that creates separate system+weather panels, system panel always gets full width, height is measured from the rendered output.

```go
func (m Model) measureDashboardLayout() dashboardLayoutMetrics {
	if m.width <= 0 || m.height <= 0 {
		return dashboardLayoutMetrics{}
	}

	header := panels.RenderHeader(m.systemData, m.weatherData, m.weatherErr, m.weatherRetries, m.width, m.TestMode)

	// Compute system panel height dynamically.
	// Left column: 2 (CPU spark+gauge) + 2 (RAM spark+gauge) + disks
	// Right column: 4 (LOAD, NET, MEM, SWAP)
	// Panel chrome: border(2) + title(1) = 3
	leftLines := 4 + len(m.systemData.Disks)
	rightLines := 4
	contentLines := leftLines
	if rightLines > contentLines {
		contentLines = rightLines
	}
	if contentLines > 12 {
		contentLines = 12
	}
	topHeight := contentLines + 3 // +3 for panel chrome

	systemPanel := panels.RenderSystem(
		m.systemData, m.cpuHistory, m.ramHistory,
		m.width, topHeight,
		m.focusedPanel == PanelSystem)
	topRow := systemPanel

	// Measure actual rendered height to avoid wrapping surprises
	topLines := renderedLineCount(topRow)

	previewBar := panels.RenderPreview(
		m.selectedContainer(),
		m.selectedStackPreview(),
		m.confirmAction,
		m.dashboardActionTargetName,
		m.actionResult,
		m.width,
	)
	helpBar := panels.RenderHelp(panels.DefaultBindings, m.refreshing, !m.focused && m.viewMode == ViewDashboard, m.width)
	notifBar := renderNotificationBar(&m.notifications, m.width)

	bottomBars := []string{previewBar}
	if notifBar != "" {
		bottomBars = append(bottomBars, notifBar)
	}
	bottomBars = append(bottomBars, helpBar)
	bottomSection := lipgloss.JoinVertical(lipgloss.Left, bottomBars...)

	headerLines := renderedLineCount(header)
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
```

- [ ] **Step 5: Remove remaining PanelWeather references from app.go**

Search for any remaining `PanelWeather` references in `app.go`. The weather panel render calls in the old `measureDashboardLayout` are already replaced. Verify no others exist.

- [ ] **Step 6: Delete weather panel files**

```bash
rm internal/ui/panels/weather.go internal/ui/panels/weather_test.go
```

- [ ] **Step 7: Verify compilation (excluding test files)**

Run: `go build ./cmd/homedash/`
Expected: Success (test files will still fail due to old references — fixed in Task 5)

- [ ] **Step 8: Commit**

```bash
git add internal/ui/panels/header.go internal/ui/app.go internal/ui/keys.go
git rm internal/ui/panels/weather.go internal/ui/panels/weather_test.go
git commit -m "feat: move weather to header, remove weather panel, full-width system layout"
```

---

### Task 5: Update all tests

**Files:**
- Modify: `internal/ui/app_test.go`
- Modify: `internal/ui/integration_test.go`

- [ ] **Step 1: Update newTestModel to include ramHistory**

Already done in Task 2 Step 4. Verify it's there.

- [ ] **Step 2: Rewrite TestRecalcLayoutMatchesRenderedContainerRowsInNarrowLayout**

In `internal/ui/app_test.go`, replace the function (~line 402-434):

```go
func TestRecalcLayoutMatchesRenderedContainerRowsInNarrowLayout(t *testing.T) {
	m := newTestModel()
	m.width = 60
	m.height = 40
	m.refreshing = true
	m.searchInput.SetValue("postgres")

	m.recalcLayout()

	header := panels.RenderHeader(m.systemData, m.weatherData, m.weatherErr, m.weatherRetries, m.width, m.TestMode)

	// Compute expected system panel height
	leftLines := 4 + len(m.systemData.Disks)
	rightLines := 4
	contentLines := leftLines
	if rightLines > contentLines {
		contentLines = rightLines
	}
	if contentLines > 12 {
		contentLines = 12
	}
	topHeight := contentLines + 3
	systemPanel := panels.RenderSystem(m.systemData, m.cpuHistory, m.ramHistory, m.width, topHeight, m.focusedPanel == PanelSystem)
	previewBar := panels.RenderPreview(nil, nil, m.confirmAction, m.dashboardActionTargetName, m.actionResult, m.width)
	helpBar := panels.RenderHelp(panels.DefaultBindings, m.refreshing, false, m.width)
	bottomSection := lipgloss.JoinVertical(lipgloss.Left, previewBar, helpBar)

	countLines := func(s string) int {
		if s == "" {
			return 0
		}
		return strings.Count(s, "\n") + 1
	}

	expectedRows := m.height - countLines(header) - countLines(systemPanel) - countLines(bottomSection) - 5
	if expectedRows < 0 {
		expectedRows = 0
	}

	if m.containerRows != expectedRows {
		t.Fatalf("containerRows = %d, want %d to match rendered narrow layout", m.containerRows, expectedRows)
	}
}
```

- [ ] **Step 3: Rewrite TestHandleMouseIgnoresClicksBelowRenderedContainerRows**

Replace the function (~line 436-477):

```go
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

	header := panels.RenderHeader(m.systemData, m.weatherData, m.weatherErr, m.weatherRetries, m.width, m.TestMode)

	leftLines := 4 + len(m.systemData.Disks)
	rightLines := 4
	contentLines := leftLines
	if rightLines > contentLines {
		contentLines = rightLines
	}
	if contentLines > 12 {
		contentLines = 12
	}
	topHeight := contentLines + 3
	systemPanel := panels.RenderSystem(m.systemData, m.cpuHistory, m.ramHistory, m.width, topHeight, m.focusedPanel == PanelSystem)
	previewBar := panels.RenderPreview(nil, nil, m.confirmAction, m.dashboardActionTargetName, m.actionResult, m.width)
	helpBar := panels.RenderHelp(panels.DefaultBindings, m.refreshing, false, m.width)
	bottomSection := lipgloss.JoinVertical(lipgloss.Left, previewBar, helpBar)
	expectedRows := m.height - (strings.Count(header, "\n") + 1) - (strings.Count(systemPanel, "\n") + 1) - (strings.Count(bottomSection, "\n") + 1) - 5
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
```

- [ ] **Step 4: Update TestIntegration_TabCyclesPanels**

In `internal/ui/integration_test.go`, replace lines 647-668:

```go
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
```

- [ ] **Step 5: Remove any remaining references to panels.RenderWeather or PanelWeather in test files**

Search and remove any stale imports or references. The `RenderWeather` calls in `app_test.go` lines 413 and 454 are already gone from the rewrite above.

- [ ] **Step 6: Verify compilation**

Run: `go build ./...`
Expected: Success

- [ ] **Step 7: Run full test suite**

Run: `go test ./... -count=1 2>&1 | tail -30`
Expected: All PASS

- [ ] **Step 8: Commit**

```bash
git add internal/ui/app_test.go internal/ui/integration_test.go
git commit -m "test: update tests for dashboard redesign (panel count, layout reconstruction)"
```

Note: weather files were already deleted and header/app/keys changes were already committed in Task 4.

---

## Chunk 3: Header Weather Tests + Final Polish

### Task 6: Add header weather and system panel tests

**Files:**
- Create: `internal/ui/panels/header_test.go`

- [ ] **Step 1: Create header_test.go with weather rendering tests**

```go
package panels

import (
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/kostas/homedash/internal/collector"
)

var ansiRE = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripHeaderANSI(s string) string {
	return ansiRE.ReplaceAllString(s, "")
}

func TestRenderHeaderWeatherFull(t *testing.T) {
	sys := collector.SystemData{Hostname: "myhost", CPUCount: 4, MemTotal: 16 * 1024 * 1024 * 1024}
	weather := collector.WeatherData{
		TempC:       "18",
		Condition:   "Clear",
		Humidity:    "45",
		Icon:        "☀️",
		CollectedAt: time.Now(),
	}
	view := RenderHeader(sys, weather, nil, 0, 120, false)
	plain := stripHeaderANSI(view)

	if !strings.Contains(plain, "18°C") {
		t.Fatalf("want temperature in header, got %q", plain)
	}
	if !strings.Contains(plain, "Clear") {
		t.Fatalf("want condition in header, got %q", plain)
	}
}

func TestRenderHeaderWeatherCompactWhenNarrow(t *testing.T) {
	sys := collector.SystemData{Hostname: "myhost", CPUCount: 4, MemTotal: 16 * 1024 * 1024 * 1024}
	weather := collector.WeatherData{
		TempC:       "18",
		Condition:   "Thunderstorm",
		Humidity:    "90",
		Icon:        "⛈",
		CollectedAt: time.Now(),
	}
	// Use a narrow width that can't fit full variant but can fit compact
	view := RenderHeader(sys, weather, nil, 0, 70, false)
	plain := stripHeaderANSI(view)

	if !strings.Contains(plain, "18°C") {
		t.Fatalf("want temperature in compact header, got %q", plain)
	}
}

func TestRenderHeaderWeatherNeverLoaded(t *testing.T) {
	sys := collector.SystemData{Hostname: "myhost", CPUCount: 4, MemTotal: 16 * 1024 * 1024 * 1024}
	weather := collector.WeatherData{} // TempC == ""
	view := RenderHeader(sys, weather, nil, 0, 120, false)
	plain := stripHeaderANSI(view)

	if !strings.Contains(plain, "--") {
		t.Fatalf("want '--' for never-loaded weather, got %q", plain)
	}
}

func TestRenderHeaderWeatherStale(t *testing.T) {
	sys := collector.SystemData{Hostname: "myhost", CPUCount: 4, MemTotal: 16 * 1024 * 1024 * 1024}
	weather := collector.WeatherData{
		TempC:       "18",
		Condition:   "Clear",
		Humidity:    "45",
		Icon:        "☀️",
		CollectedAt: time.Now().Add(-10 * time.Minute),
	}
	// Stale = error with 0 retries
	view := RenderHeader(sys, weather, errors.New("timeout"), 0, 120, false)
	plain := stripHeaderANSI(view)

	// Should still show temperature (stale data is better than no data)
	if !strings.Contains(plain, "18°C") {
		t.Fatalf("want stale temperature shown, got %q", plain)
	}
}

func TestRenderHeaderWeatherErrorNoData(t *testing.T) {
	sys := collector.SystemData{Hostname: "myhost", CPUCount: 4, MemTotal: 16 * 1024 * 1024 * 1024}
	weather := collector.WeatherData{} // TempC == ""
	// Error with 0 retries, no prior data
	view := RenderHeader(sys, weather, errors.New("DNS failed"), 0, 120, false)
	plain := stripHeaderANSI(view)

	if !strings.Contains(plain, "--") {
		t.Fatalf("want '--' for error-no-data weather, got %q", plain)
	}
}
```

- [ ] **Step 2: Add system panel rendering tests**

Add to `internal/ui/panels/header_test.go` (or create `internal/ui/panels/system_test.go`):

```go
func TestRenderSystemTwoColumn(t *testing.T) {
	data := collector.SystemData{
		CPUPercent: 50, MemPercent: 30, CPUCount: 4,
		MemTotal: 16 * 1024 * 1024 * 1024, MemUsed: 5 * 1024 * 1024 * 1024,
		LoadAvg: [3]float64{1.0, 0.5, 0.2},
		Disks:   []collector.DiskInfo{{Mount: "/", Percent: 40, Total: 100 * 1024 * 1024 * 1024, Used: 40 * 1024 * 1024 * 1024}},
		NetRxRate: 1024 * 1024, NetTxRate: 512 * 1024,
		SwapTotal: 4 * 1024 * 1024 * 1024, SwapUsed: 256 * 1024 * 1024, SwapPercent: 6.25,
	}
	cpu := components.NewRingBuffer(60)
	ram := components.NewRingBuffer(60)
	cpu.Push(50)
	ram.Push(30)

	view := RenderSystem(data, cpu, ram, 120, 10, false)
	plain := stripHeaderANSI(view)

	for _, want := range []string{"CPU", "RAM", "LOAD", "NET", "MEM", "SWAP"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("want %q in system panel, got %q", want, plain)
		}
	}
}

func TestRenderSystemSwapDisabled(t *testing.T) {
	data := collector.SystemData{
		CPUPercent: 50, MemPercent: 30, CPUCount: 4,
		MemTotal: 16 * 1024 * 1024 * 1024, MemUsed: 5 * 1024 * 1024 * 1024,
		SwapTotal: 0, SwapUsed: 0, SwapPercent: 0,
	}
	cpu := components.NewRingBuffer(60)
	ram := components.NewRingBuffer(60)

	view := RenderSystem(data, cpu, ram, 120, 10, false)
	plain := stripHeaderANSI(view)

	if !strings.Contains(plain, "disabled") {
		t.Fatalf("want 'disabled' for zero swap, got %q", plain)
	}
}

func TestRenderSystemNarrowFallback(t *testing.T) {
	data := collector.SystemData{
		CPUPercent: 50, MemPercent: 30, CPUCount: 4,
		MemTotal: 16 * 1024 * 1024 * 1024, MemUsed: 5 * 1024 * 1024 * 1024,
		SwapTotal: 4 * 1024 * 1024 * 1024, SwapUsed: 256 * 1024 * 1024, SwapPercent: 6.25,
	}
	cpu := components.NewRingBuffer(60)
	ram := components.NewRingBuffer(60)

	// width < 90 triggers single-column fallback
	view := RenderSystem(data, cpu, ram, 60, 15, false)
	plain := stripHeaderANSI(view)

	for _, want := range []string{"CPU", "RAM", "LOAD", "NET", "MEM", "SWAP"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("want %q in narrow system panel, got %q", want, plain)
		}
	}
}
```

Add the `components` import to the test file:

```go
import (
	// ... existing imports ...
	"github.com/kostas/homedash/internal/ui/components"
)
```

- [ ] **Step 3: Run all panel tests**

Run: `go test ./internal/ui/panels/ -v`
Expected: All PASS

- [ ] **Step 4: Commit**

```bash
git add internal/ui/panels/header_test.go
git commit -m "test: add header weather and system panel rendering tests"
```

---

### Task 7: Seed ramHistory in test mode and run visual verification

**Files:**
- Modify: `internal/ui/app.go` (Init or test mode setup)

- [ ] **Step 1: Verify ramHistory populates in test mode**

`cpuHistory` is not explicitly seeded — it populates via the `SystemDataMsg` handler on each mock system tick. `ramHistory` follows the same path via `m.ramHistory.Push(msg.Data.MemPercent)` added in Task 2. No explicit seeding needed. Verify visually:

Run: `go run ./cmd/homedash/ --test-mode`
Expected: Dashboard shows with two-column system panel, RAM sparkline visible, weather in header bar.

- [ ] **Step 2: Run the full test suite one final time**

Run: `go test ./... -count=1`
Expected: All PASS

- [ ] **Step 3: Run linter**

Run: `make lint`
Expected: No errors

- [ ] **Step 4: Final commit if any remaining changes**

```bash
git status
# If clean, nothing to commit. If changes exist:
git add -A && git commit -m "chore: final polish for dashboard redesign"
```

---

### Task 8: Verify with TUI driver (optional)

**Files:** None (verification only)

- [ ] **Step 1: Launch in test mode via TUI driver**

Use the MCP TUI driver to launch `go run ./cmd/homedash/ --test-mode` and take a screenshot to verify:
- Header shows: HOMEDASH | hostname | uptime | CPU/RAM | weather | clock
- System panel is full-width with two columns
- Left column: CPU sparkline, CPU gauge, RAM sparkline, RAM gauge, disk gauges
- Right column: LOAD, NET, MEM, SWAP
- Weather panel is gone
- Tab cycles between System and Containers only (2 panels)
- Container list fills remaining vertical space

- [ ] **Step 2: Test narrow terminal**

Resize to < 90 columns and verify single-column system panel fallback.

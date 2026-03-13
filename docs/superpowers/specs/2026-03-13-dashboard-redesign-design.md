# Dashboard Redesign: Weather-to-Header + Full-Width System Panel

**Date**: 2026-03-13
**Branch**: feature/dashboard-redesign
**Scope**: Layout restructure, RAM sparkline, swap usage display

## Summary

Move weather from a standalone panel into a compact header element. Expand the system panel to full terminal width with a two-column layout. Add RAM sparkline and swap usage display to the system panel.

## Motivation

The weather panel currently takes 60% of the top row width to display 3 lines of data. The system panel is compressed into 40% width despite being the primary monitoring view. This restructure gives system metrics the space they deserve and makes room for future additions (temperatures, top processes) without further layout changes.

## Design

### 1. Weather in Header

The weather panel is removed as a standalone panel. Weather data is rendered as an inline element in the header bar, inserted after the existing extras (hostname, uptime, CPU/RAM summary) and before the clock.

```
HOMEDASH | myhost | up 3d 2h | 4 CPU / 16.0G RAM | sun 18C Cloudy droplet 72% | 15:04
```

**Responsive degradation** (follows the existing header pattern that drops extras when width is tight):
- Wide (>= ~25 chars available): icon + temp + condition + humidity (`sun 18C Cloudy droplet 72%`)
- Medium (>= ~10 chars available): icon + temp (`sun 18C`)
- Narrow: weather hidden entirely

**Stale/error handling**: Temperature text renders in warning color when weather data is stale (error with retries exhausted). No retry counters in the header. Weather fetch errors continue to produce notifications via the existing notification queue.

**Panel navigation**: `PanelWeather` is removed from the panel enum. `panelCount` drops from 3 to 2 (`PanelSystem`, `PanelContainers`). Tab cycling skips weather.

**Weather data flow**: No changes to the collector or tick schedule. `RenderHeader` signature expands to:

```go
func RenderHeader(data collector.SystemData, weather collector.WeatherData, weatherErr error, weatherRetries int, width int, testMode bool) string
```

Weather stale detection in test mode must use the same fixed reference time (`2026-03-07 12:00:00 UTC`) already used for the clock display, so `time.Since(weather.CollectedAt)` produces deterministic output.

### 2. System Panel: Full Width, Two Columns

The system panel takes the full terminal width (no longer shares a row with weather). The interior splits into two columns:

```
+-- SYSTEM --------------------------------------------------+
|      barchart history (2m)         LOAD  1.2  0.8  0.5     |
| CPU  gauge bar 62%                 NET   down 2.4M/s up 340K/s |
|      barchart history (2m)         MEM   7.2G / 16.0G      |
| RAM  gauge bar 38%                 SWAP  0B / 4.0G          |
| /    gauge bar 89%                 TEMP  CPU 52C             |
| /mnt gauge bar 31%                                          |
+------------------------------------------------------------+
```

**Left column** (~55% of inner width): Sparklines and gauges. Sparkline appears *above* its corresponding gauge so visual ownership is unambiguous. Order: CPU sparkline, CPU gauge, RAM sparkline, RAM gauge, disk gauges.

**Right column** (~45% of inner width): Text-based stats vertically aligned.
- LOAD: three load averages. Warning color when 1-min load exceeds CPU count.
- NET: download (Primary color, down arrow) and upload (Secondary color, up arrow) rates.
- MEM: absolute used/total in human-readable format (e.g., `7.2G / 16.0G`).
- SWAP: absolute used/total. Warning color when swap percent > 25%.
- TEMP: CPU temperature. Placeholder for branch 2; shows `--` or is hidden until temperature collection is implemented.

**Column split**: Left column gets `innerWidth * 55 / 100`, right column gets the remainder. Left minimum: 30 chars. The two-column-to-single-column fallback is governed by `isNarrow()` (triggers at `m.width < 90`), which is the sole gatekeeper — no separate inner-width threshold.

**Height**: Dynamic based on content, capped at 12 content lines. Left column line count = 2 (CPU sparkline + gauge) + 2 (RAM sparkline + gauge) + len(disks). Right column line count = 5 (LOAD, NET, MEM, SWAP, TEMP). Panel height = max(left, right) + panel chrome (border + title). If either column exceeds 12 content lines, it truncates from the bottom.

The current hardcoded `topHeight := 11` in `measureDashboardLayout` (app.go:1356) is replaced with a dynamic calculation: `topHeight := max(leftLines, rightLines) + panelChrome`, capped at `12 + panelChrome`. The `topRow` field in `dashboardLayoutMetrics` becomes the rendered system panel only (no longer a join of system + weather).

### 3. RAM Sparkline

A new `ramHistory *components.RingBuffer` field is added to the Model, initialized with the same size as `cpuHistory` (60 samples = 2 minutes at 2s refresh).

On each `SystemDataMsg`, `m.ramHistory.Push(data.MemPercent)` is called alongside the existing `m.cpuHistory.Push(data.CPUPercent)`.

The sparkline is rendered using the existing `components.Sparkline()` function with `styles.Secondary` color (to visually distinguish from the CPU sparkline which uses `styles.Primary`).

`RenderSystem` signature expands to accept the second ring buffer:

```go
func RenderSystem(data collector.SystemData, cpuHistory, ramHistory *components.RingBuffer, width, height int, focused bool) string
```

### 4. Swap Usage

**Collection**: `CollectSystem` in `internal/collector/system.go` already calls `parseMemInfo` which parses all `/proc/meminfo` fields including `SwapTotal` and `SwapFree`. Three new fields are added to `SystemData`:

```go
SwapTotal   uint64  // bytes
SwapUsed    uint64  // bytes
SwapPercent float64
```

Populated as:
```go
swapTotal := memInfo["SwapTotal"] // already in the map
swapFree := memInfo["SwapFree"]   // already in the map
data.SwapTotal = swapTotal * 1024
data.SwapUsed = (swapTotal - swapFree) * 1024
if swapTotal > 0 {
    data.SwapPercent = float64(swapTotal - swapFree) / float64(swapTotal) * 100
}
```

**Display**: Compact text in the right column: `SWAP  0B / 4.0G`. Uses `collector.FormatBytes` for both values. Text color is `TextSecondary` normally, `Warning` when `SwapPercent > 25`.

If `SwapTotal` is 0 (no swap configured), the line reads `SWAP  disabled` in `TextMuted` color.

### 5. Test Mode / Mock Data

Existing mock data generators in `test_fixtures.go` are updated:
- `collectMockSystemCmd` sets `SwapTotal`, `SwapUsed`, `SwapPercent` to fixed values.
- Mock weather data continues to work; it feeds into the header now instead of a panel.
- `ramHistory` gets seeded with synthetic data in test mode initialization (same pattern as `cpuHistory`).

### 6. Narrow Terminal Behavior

Narrow is defined by `m.isNarrow()` (existing function). In narrow mode:
- System panel renders single-column: left column content stacked above right column content.
- Weather in header follows the responsive rules (medium or hidden).
- No other layout changes; container panel and bottom bars are unaffected.

## Files Changed

| File | Change |
|------|--------|
| `internal/collector/types.go` | Add `SwapTotal`, `SwapUsed`, `SwapPercent` to `SystemData` |
| `internal/collector/system.go` | Populate swap fields from parsed meminfo |
| `internal/collector/system_test.go` | Test swap extraction |
| `internal/ui/panels/header.go` | Accept `WeatherData`, error state, retries; render inline weather with responsive degradation |
| `internal/ui/panels/weather.go` | Delete file |
| `internal/ui/panels/weather_test.go` | Delete file (weather rendering tested via header tests) |
| `internal/ui/panels/system.go` | Two-column layout, accept `ramHistory`, render RAM sparkline, swap, TEMP placeholder |
| `internal/ui/app.go` | Remove `PanelWeather` references, add `ramHistory` field, update `measureDashboardLayout` (single full-width system panel, dynamic height with cap), update `renderDashboard`, pass weather data to header |
| `internal/ui/keys.go` | Remove `PanelWeather` from panel enum, update `panelCount` |
| `internal/ui/messages.go` | No changes expected (weather messages unchanged) |
| `internal/ui/notifications.go` | No changes |
| `internal/ui/test_fixtures.go` | Update mock system data with swap fields, seed `ramHistory` |
| `internal/ui/app_test.go` | Substantial rework: `TestRecalcLayoutMatchesRenderedContainerRowsInNarrowLayout` and `TestHandleMouseIgnoresClicksBelowRenderedContainerRows` manually reconstruct the top row from system+weather — must be rewritten for single-panel layout. Update panel count assertions. |
| `internal/ui/integration_test.go` | `TestIntegration_TabCyclesPanels` asserts `PanelWeather` in the tab cycle — must be updated for 2-panel cycle (System, Containers) |
| `internal/ui/panels/containers_test.go` | No changes expected |
| `internal/ui/panels/preview_test.go` | No changes expected |

## Out of Scope

- Network I/O sparklines (branch 2: host-observability)
- CPU temperature collection (branch 2: host-observability; TEMP line shows placeholder)
- Top processes (branch 3)
- Docker pull/recreate commands (branch 4)

## Risks

- **Header overflow**: Weather text could push the clock off-screen on medium terminals. Mitigated by the existing overflow-checking loop in `RenderHeader` that drops extras when they don't fit.
- **Dynamic height instability**: If disk count changes between refreshes (unlikely but possible with hot-plug), panel height would shift. The cap at 12 lines bounds the worst case.
- **Weather staleness visibility**: Condensing weather to a header element loses the detailed retry/stale UI. Acceptable because: (a) stale marker on the temp text is visible, (b) weather errors already produce notifications, (c) weather is not the primary purpose of the dashboard.

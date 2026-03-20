package panels

import (
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/kts982/homedash/internal/collector"
	"github.com/kts982/homedash/internal/ui/components"
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
	// Use a short hostname and minimal data so weather still fits at reduced width
	sys := collector.SystemData{Hostname: "box", CPUCount: 2, MemTotal: 8 * 1024 * 1024 * 1024}
	weather := collector.WeatherData{
		TempC:       "18",
		Condition:   "Thunderstorm",
		Humidity:    "90",
		Icon:        "⛈",
		CollectedAt: time.Now(),
	}
	// Width 90 is wide enough for compact but may drop the full condition+humidity
	view := RenderHeader(sys, weather, nil, 0, 90, false)
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

// --- System panel tests ---

func TestRenderSystemTwoColumn(t *testing.T) {
	data := collector.SystemData{
		Uptime:     49 * time.Hour,
		CPUPercent: 50, MemPercent: 30, CPUCount: 4,
		RunningTasks: 2, TotalTasks: 214,
		OpenFiles: 2100, MaxFiles: 1000000,
		MemTotal: 16 * 1024 * 1024 * 1024, MemUsed: 5 * 1024 * 1024 * 1024,
		LoadAvg: [3]float64{1.0, 0.5, 0.2},
		Disks: []collector.DiskInfo{
			{Mount: "/", Percent: 40, Total: 100 * 1024 * 1024 * 1024, Used: 40 * 1024 * 1024 * 1024},
			{Mount: "/data", Percent: 60, Total: 200 * 1024 * 1024 * 1024, Used: 120 * 1024 * 1024 * 1024},
		},
		NetRxRate: 1024 * 1024, NetTxRate: 512 * 1024,
		SwapTotal: 4 * 1024 * 1024 * 1024, SwapUsed: 256 * 1024 * 1024, SwapPercent: 6.25,
	}
	cpu := components.NewRingBuffer(60)
	ram := components.NewRingBuffer(60)
	cpu.Push(50)
	ram.Push(30)

	view := RenderSystem(data, cpu, ram, 120, 10, false, "")
	plain := stripHeaderANSI(view)

	for _, want := range []string{"CPU", "RAM", "LOAD", "TASKS", "NET", "FILES", "MEM", "DISKS", "SWAP", "PEAK"} {
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

	view := RenderSystem(data, cpu, ram, 120, 10, false, "")
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
	view := RenderSystem(data, cpu, ram, 60, 15, false, "")
	plain := stripHeaderANSI(view)

	for _, want := range []string{"CPU", "RAM", "LOAD", "TASKS", "NET", "FILES", "MEM", "DISKS", "SWAP", "PEAK"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("want %q in narrow system panel, got %q", want, plain)
		}
	}
}

func TestRenderSystemShowsFreshnessInTitle(t *testing.T) {
	data := collector.SystemData{
		CPUPercent: 50, MemPercent: 30, CPUCount: 4,
		MemTotal: 16 * 1024 * 1024 * 1024, MemUsed: 5 * 1024 * 1024 * 1024,
	}
	cpu := components.NewRingBuffer(60)
	ram := components.NewRingBuffer(60)

	view := RenderSystem(data, cpu, ram, 120, 10, false, "2s ago")
	plain := stripHeaderANSI(view)

	if !strings.Contains(plain, "SYSTEM · 2s ago") {
		t.Fatalf("want freshness label in system title, got %q", plain)
	}
}

func TestRenderSystemShowsUnlimitedFileLimit(t *testing.T) {
	data := collector.SystemData{
		CPUPercent:   50,
		MemPercent:   30,
		CPUCount:     4,
		OpenFiles:    4500,
		MaxFiles:     9223372036854775807,
		RunningTasks: 2,
		TotalTasks:   214,
		MemTotal:     16 * 1024 * 1024 * 1024,
		MemUsed:      5 * 1024 * 1024 * 1024,
		LoadAvg:      [3]float64{1.0, 0.5, 0.2},
	}
	cpu := components.NewRingBuffer(60)
	ram := components.NewRingBuffer(60)

	view := RenderSystem(data, cpu, ram, 120, 10, false, "")
	plain := stripHeaderANSI(view)

	if !strings.Contains(plain, "FILES") || !strings.Contains(plain, "unlimited") {
		t.Fatalf("want unlimited file limit in system panel, got %q", plain)
	}
}

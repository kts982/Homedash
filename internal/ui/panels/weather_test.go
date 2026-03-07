package panels

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/kostas/homedash/internal/collector"
)

func sampleWeatherData() collector.WeatherData {
	return collector.WeatherData{
		Location:    "Test City",
		TempC:       "22",
		FeelsLikeC:  "20",
		Condition:   "Sunny",
		Humidity:    "50",
		WindSpeed:   "15",
		WindDir:     "NE",
		Icon:        "☀️",
		CollectedAt: time.Now().Add(-2 * time.Minute),
	}
}

func TestRenderWeatherLoading(t *testing.T) {
	view := RenderWeather(collector.WeatherData{}, nil, 0, 60, 11, false)
	plain := stripANSI(view)

	if !strings.Contains(plain, "Loading weather...") {
		t.Fatalf("want 'Loading weather...', got %q", plain)
	}
}

func TestRenderWeatherRetryingNoData(t *testing.T) {
	view := RenderWeather(collector.WeatherData{}, errors.New("connection refused"), 2, 60, 11, false)
	plain := stripANSI(view)

	if !strings.Contains(plain, "Retrying... (2/3)") {
		t.Fatalf("want retry indicator, got %q", plain)
	}
	if !strings.Contains(plain, "connection refused") {
		t.Fatalf("want error detail, got %q", plain)
	}
}

func TestRenderWeatherUnavailable(t *testing.T) {
	view := RenderWeather(collector.WeatherData{}, errors.New("DNS lookup failed"), 0, 60, 11, false)
	plain := stripANSI(view)

	if !strings.Contains(plain, "Weather unavailable") {
		t.Fatalf("want 'Weather unavailable', got %q", plain)
	}
	if !strings.Contains(plain, "DNS lookup failed") {
		t.Fatalf("want error detail, got %q", plain)
	}
}

func TestRenderWeatherNormal(t *testing.T) {
	data := sampleWeatherData()
	view := RenderWeather(data, nil, 0, 60, 11, false)
	plain := stripANSI(view)

	for _, want := range []string{"22°C", "Sunny", "Feels 20°C", "Wind 15km/h NE", "Test City"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("want %q, got %q", want, plain)
		}
	}
	// Should not show any error indicators
	if strings.Contains(plain, "update failed") || strings.Contains(plain, "retrying") {
		t.Fatalf("normal state should have no error indicators, got %q", plain)
	}
}

func TestRenderWeatherRetryingWithData(t *testing.T) {
	data := sampleWeatherData()
	view := RenderWeather(data, errors.New("timeout"), 1, 60, 11, false)
	plain := stripANSI(view)

	// Should still show weather data
	if !strings.Contains(plain, "22°C") {
		t.Fatalf("want temperature shown during retry, got %q", plain)
	}
	// Should show retry indicator
	if !strings.Contains(plain, "retrying (1/3)") {
		t.Fatalf("want retry indicator, got %q", plain)
	}
}

func TestRenderWeatherStaleWithData(t *testing.T) {
	data := sampleWeatherData()
	view := RenderWeather(data, errors.New("timeout"), 0, 60, 11, false)
	plain := stripANSI(view)

	// Should still show weather data (readable)
	if !strings.Contains(plain, "22°C") {
		t.Fatalf("want temperature shown when stale, got %q", plain)
	}
	// Should show failure indicator
	if !strings.Contains(plain, "update failed") {
		t.Fatalf("want 'update failed' indicator, got %q", plain)
	}
	// Should show staleness age
	if !strings.Contains(plain, "last ok") {
		t.Fatalf("want 'last ok' age indicator, got %q", plain)
	}
}

func TestRenderWeatherStaleShowsWarningInTitle(t *testing.T) {
	data := sampleWeatherData()
	view := RenderWeather(data, errors.New("timeout"), 0, 60, 11, false)
	plain := stripANSI(view)

	// Title should contain warning indicator
	if !strings.Contains(plain, "WEATHER ⚠") {
		t.Fatalf("want warning in title when stale, got %q", plain)
	}
}

func TestFormatWeatherAge(t *testing.T) {
	tests := []struct {
		ago  time.Duration
		want string
	}{
		{30 * time.Second, "just now"},
		{5 * time.Minute, "5m ago"},
		{90 * time.Minute, "1h30m ago"},
	}
	for _, tt := range tests {
		got := formatWeatherAge(time.Now().Add(-tt.ago))
		if got != tt.want {
			t.Errorf("formatWeatherAge(-%v) = %q, want %q", tt.ago, got, tt.want)
		}
	}
}

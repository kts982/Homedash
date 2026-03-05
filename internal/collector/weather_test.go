package collector

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWeatherIcon(t *testing.T) {
	tests := []struct {
		name      string
		condition string
		want      string
	}{
		{name: "sunny", condition: "Sunny", want: "☀"},
		{name: "cloudy", condition: "Cloudy", want: "☁"},
		{name: "partly cloudy", condition: "Partly Cloudy", want: "⛅"},
		{name: "light rain", condition: "Light rain", want: "🌧"},
		{name: "unknown", condition: "Volcanic Ash", want: "?"},
		{name: "empty", condition: "", want: "?"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := weatherIcon(tc.condition)
			if got != tc.want {
				t.Fatalf("weatherIcon(%q) = %q, want %q", tc.condition, got, tc.want)
			}
		})
	}
}

func TestCollectWeather(t *testing.T) {
	t.Run("successful response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			_, _ = w.Write([]byte(`{
				"current_condition": [{
					"temp_C": "23",
					"FeelsLikeC": "25",
					"humidity": "61",
					"windspeedKmph": "18",
					"winddir16Point": "NW",
					"weatherDesc": [{"value": "Sunny"}]
				}],
				"nearest_area": [{
					"areaName": [{"value": "Athens"}]
				}]
			}`))
		}))
		defer server.Close()

		oldURL := weatherURL
		oldClient := weatherClient
		t.Cleanup(func() {
			weatherURL = oldURL
			weatherClient = oldClient
		})

		weatherURL = server.URL
		weatherClient = server.Client()

		got, err := CollectWeather()
		if err != nil {
			t.Fatalf("CollectWeather() returned error: %v", err)
		}

		if got.Location != "Athens" {
			t.Fatalf("Location = %q, want %q", got.Location, "Athens")
		}
		if got.TempC != "23" {
			t.Fatalf("TempC = %q, want %q", got.TempC, "23")
		}
		if got.FeelsLikeC != "25" {
			t.Fatalf("FeelsLikeC = %q, want %q", got.FeelsLikeC, "25")
		}
		if got.Condition != "Sunny" {
			t.Fatalf("Condition = %q, want %q", got.Condition, "Sunny")
		}
		if got.Humidity != "61" {
			t.Fatalf("Humidity = %q, want %q", got.Humidity, "61")
		}
		if got.WindSpeed != "18" {
			t.Fatalf("WindSpeed = %q, want %q", got.WindSpeed, "18")
		}
		if got.WindDir != "NW" {
			t.Fatalf("WindDir = %q, want %q", got.WindDir, "NW")
		}
		if got.Icon != "☀" {
			t.Fatalf("Icon = %q, want %q", got.Icon, "☀")
		}
	})

	t.Run("malformed json", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"current_condition":`))
		}))
		defer server.Close()

		oldURL := weatherURL
		oldClient := weatherClient
		t.Cleanup(func() {
			weatherURL = oldURL
			weatherClient = oldClient
		})

		weatherURL = server.URL
		weatherClient = server.Client()

		_, err := CollectWeather()
		if err == nil {
			t.Fatal("CollectWeather() expected error for malformed JSON, got nil")
		}
		if !strings.Contains(err.Error(), "weather parse:") {
			t.Fatalf("CollectWeather() error = %q, want parse error", err.Error())
		}
	})

	t.Run("non-200 status", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer server.Close()

		oldURL := weatherURL
		oldClient := weatherClient
		t.Cleanup(func() {
			weatherURL = oldURL
			weatherClient = oldClient
		})

		weatherURL = server.URL
		weatherClient = server.Client()

		_, err := CollectWeather()
		if err == nil {
			t.Fatal("CollectWeather() expected error for 503, got nil")
		}
		if !strings.Contains(err.Error(), "status 503") {
			t.Fatalf("CollectWeather() error = %q, want status error", err.Error())
		}
	})

	t.Run("network error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
		serverURL := server.URL
		server.Close()

		oldURL := weatherURL
		oldClient := weatherClient
		t.Cleanup(func() {
			weatherURL = oldURL
			weatherClient = oldClient
		})

		weatherURL = serverURL
		weatherClient = &http.Client{}

		_, err := CollectWeather()
		if err == nil {
			t.Fatal("CollectWeather() expected network error, got nil")
		}
		if !strings.Contains(err.Error(), "weather fetch:") {
			t.Fatalf("CollectWeather() error = %q, want fetch error", err.Error())
		}
	})
}

package collector

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type wttrResponse struct {
	CurrentCondition []struct {
		TempC       string `json:"temp_C"`
		FeelsLikeC  string `json:"FeelsLikeC"`
		Humidity    string `json:"humidity"`
		WindspeedKm string `json:"windspeedKmph"`
		WindDir16   string `json:"winddir16Point"`
		WeatherDesc []struct {
			Value string `json:"value"`
		} `json:"weatherDesc"`
	} `json:"current_condition"`
	NearestArea []struct {
		AreaName []struct {
			Value string `json:"value"`
		} `json:"areaName"`
	} `json:"nearest_area"`
}

var weatherConditionIcons = map[string]string{
	"Sunny":                       "☀",
	"Clear":                       "☀",
	"Partly Cloudy":               "⛅",
	"Partly cloudy":               "⛅",
	"Cloudy":                      "☁",
	"Overcast":                    "☁",
	"Mist":                        "🌫",
	"Fog":                         "🌫",
	"Patchy rain possible":        "🌦",
	"Patchy rain nearby":          "🌦",
	"Light rain":                  "🌧",
	"Light Rain":                  "🌧",
	"Moderate rain":               "🌧",
	"Heavy rain":                  "🌧",
	"Light drizzle":               "🌧",
	"Patchy light drizzle":        "🌧",
	"Thundery outbreaks possible": "⛈",
	"Light snow":                  "🌨",
	"Moderate snow":               "🌨",
	"Heavy snow":                  "🌨",
	"Patchy light snow":           "🌨",
}

var weatherURL = "https://wttr.in/?format=j1"
var weatherClient = &http.Client{Timeout: 30 * time.Second}

func CollectWeather() (WeatherData, error) {
	var data WeatherData
	data.CollectedAt = time.Now()

	resp, err := weatherClient.Get(weatherURL)
	if err != nil {
		return data, fmt.Errorf("weather fetch: %w", err)
	}
	defer closeQuietly(resp.Body)

	if resp.StatusCode != 200 {
		return data, fmt.Errorf("weather fetch: status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1*1024*1024))
	if err != nil {
		return data, fmt.Errorf("weather read: %w", err)
	}

	var wttr wttrResponse
	if err := json.Unmarshal(body, &wttr); err != nil {
		return data, fmt.Errorf("weather parse: %w", err)
	}

	if len(wttr.CurrentCondition) > 0 {
		cc := wttr.CurrentCondition[0]
		data.TempC = cc.TempC
		data.FeelsLikeC = cc.FeelsLikeC
		data.Humidity = cc.Humidity
		data.WindSpeed = cc.WindspeedKm
		data.WindDir = cc.WindDir16
		if len(cc.WeatherDesc) > 0 {
			data.Condition = cc.WeatherDesc[0].Value
		}
	}

	if len(wttr.NearestArea) > 0 && len(wttr.NearestArea[0].AreaName) > 0 {
		data.Location = wttr.NearestArea[0].AreaName[0].Value
	}

	data.Icon = weatherIcon(data.Condition)

	return data, nil
}

func weatherIcon(condition string) string {
	if icon, ok := weatherConditionIcons[condition]; ok {
		return icon
	}
	return "?"
}

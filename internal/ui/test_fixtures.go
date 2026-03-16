package ui

import (
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/kostas/homedash/internal/collector"
)

func collectMockSystemCmd() tea.Msg {
	return SystemDataMsg{
		Data: collector.SystemData{
			Hostname:     "synthetic-host",
			Uptime:       48 * time.Hour,
			CPUPercent:   12.5,
			CPUCount:     8,
			RunningTasks: 2,
			TotalTasks:   214,
			MemTotal:     16 * 1024 * 1024 * 1024,
			MemUsed:      4 * 1024 * 1024 * 1024,
			MemPercent:   25.0,
			OpenFiles:    4500,
			MaxFiles:     1000000,
			LoadAvg:      [3]float64{1.0, 0.5, 0.2},
			Disks: []collector.DiskInfo{
				{Mount: "System", Total: 512 * 1024 * 1024 * 1024, Used: 64 * 1024 * 1024 * 1024, Percent: 12.5},
				{Mount: "T1-Ext", Total: 2 * 1024 * 1024 * 1024 * 1024, Used: 1024 * 1024 * 1024 * 1024, Percent: 50.0},
			},
			SwapTotal:   4 * 1024 * 1024 * 1024, // 4G
			SwapUsed:    256 * 1024 * 1024,      // 256M
			SwapPercent: 6.25,
			NetRxRate:   1024 * 1024,
			NetTxRate:   512 * 1024,
			CollectedAt: time.Date(2026, 3, 7, 12, 0, 0, 0, time.UTC),
		},
	}
}

func collectMockDockerCmd() tea.Msg {
	return DockerDataMsg{
		Data: collector.DockerData{
			Containers: []collector.Container{
				{
					ID:      "id-1",
					Name:    "service-alpha",
					Stack:   "infra",
					Image:   "image-alpha:latest",
					State:   "running",
					Health:  "healthy",
					CPUPerc: 0.5,
					MemUsed: 128 * 1024 * 1024,
				},
				{
					ID:      "id-2",
					Name:    "service-beta",
					Stack:   "infra",
					Image:   "image-beta:latest",
					State:   "running",
					Health:  "healthy",
					CPUPerc: 1.2,
					MemUsed: 512 * 1024 * 1024,
				},
				{
					ID:      "id-3",
					Name:    "service-gamma",
					Stack:   "apps",
					Image:   "image-gamma:latest",
					State:   "exited",
					Health:  "-",
					CPUPerc: 0,
					MemUsed: 0,
				},
			},
			Running:     2,
			Total:       3,
			CollectedAt: time.Date(2026, 3, 7, 12, 0, 0, 0, time.UTC),
		},
	}
}

func collectMockWeatherCmd() tea.Msg {
	return WeatherDataMsg{
		Data: collector.WeatherData{
			Location:    "Synthetic Location",
			TempC:       "20",
			FeelsLikeC:  "21",
			Condition:   "Clear",
			Humidity:    "45%",
			WindSpeed:   "10",
			WindDir:     "N",
			Icon:        "☀️",
			CollectedAt: time.Date(2026, 3, 7, 12, 0, 0, 0, time.UTC),
		},
	}
}

func collectMockLogsCmd(containerID string, tail int) tea.Msg {
	return ContainerLogsMsg{
		ContainerID: containerID,
		Lines: []string{
			"2026-03-07T12:00:00Z [info] Starting mock service...",
			"2026-03-07T12:00:01Z [info] Listening on port 8080",
			"2026-03-07T12:00:05Z [debug] Connection established",
		},
	}
}

func collectMockStackLogsCmd(stackName string, tail int) tea.Msg {
	return StackLogsMsg{
		StackName: stackName,
		Lines: []string{
			"2026-03-07T12:00:00Z [service-alpha] Initializing...",
			"2026-03-07T12:00:00Z [service-beta] Ready for requests",
		},
	}
}

func collectMockDetailCmd(containerID string) tea.Msg {
	return ContainerDetailMsg{
		ContainerID: containerID,
		Detail: collector.ContainerDetail{
			CreatedAt:     time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC),
			StartedAt:     time.Date(2026, 3, 5, 14, 0, 0, 0, time.UTC),
			RestartPolicy: "always",
			Command:       "/usr/bin/mock-entrypoint.sh",
			Mounts: []collector.Mount{
				{Source: "/mock/source", Destination: "/mock/dest", Mode: "ro", Type: "bind"},
			},
			Networks: []collector.NetworkAddress{
				{Name: "mock-net", IPv4: "10.0.0.2"},
			},
			PublishedPorts: []collector.PublishedPort{
				{HostIP: "0.0.0.0", HostPort: 8080, ContainerPort: 8080, Type: "tcp"},
			},
		},
	}
}

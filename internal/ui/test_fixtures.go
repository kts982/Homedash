package ui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kostas/homedash/internal/collector"
)

func collectMockSystemCmd() tea.Msg {
	return SystemDataMsg{
		Data: collector.SystemData{
			Hostname:   "test-host",
			Uptime:     48 * time.Hour,
			CPUPercent: 12.5,
			CPUCount:   8,
			MemTotal:   16 * 1024 * 1024 * 1024,
			MemUsed:    4 * 1024 * 1024 * 1024,
			MemPercent: 25.0,
			LoadAvg:    [3]float64{1.0, 0.5, 0.2},
			Disks: []collector.DiskInfo{
				{Mount: "/", Total: 100 * 1024 * 1024 * 1024, Used: 40 * 1024 * 1024 * 1024, Percent: 40.0},
			},
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
					ID:      "cont-1",
					Name:    "nginx-proxy",
					Stack:   "core",
					Image:   "nginx:latest",
					State:   "running",
					Health:  "healthy",
					CPUPerc: 0.5,
					MemUsed: 128 * 1024 * 1024,
				},
				{
					ID:      "cont-2",
					Name:    "postgres-db",
					Stack:   "core",
					Image:   "postgres:15",
					State:   "running",
					Health:  "healthy",
					CPUPerc: 1.2,
					MemUsed: 512 * 1024 * 1024,
				},
				{
					ID:      "cont-3",
					Name:    "minecraft-server",
					Stack:   "games",
					Image:   "itzg/minecraft-server",
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
			Location:    "Test City",
			TempC:       "20",
			FeelsLikeC:  "21",
			Condition:   "Sunny",
			Humidity:    "45%",
			WindSpeed:   "10",
			WindDir:     "NW",
			Icon:        "☀️",
			CollectedAt: time.Date(2026, 3, 7, 12, 0, 0, 0, time.UTC),
		},
	}
}

func collectMockLogsCmd(containerID string, tail int) tea.Msg {
	return ContainerLogsMsg{
		ContainerID: containerID,
		Lines: []string{
			"2026-03-07T12:00:00Z [info] Starting service...",
			"2026-03-07T12:00:01Z [info] Listening on :80",
			"2026-03-07T12:00:05Z [debug] Connection from 127.0.0.1",
		},
	}
}

func collectMockStackLogsCmd(stackName string, tail int) tea.Msg {
	return StackLogsMsg{
		StackName: stackName,
		Lines: []string{
			"2026-03-07T12:00:00Z [nginx-proxy] Starting nginx...",
			"2026-03-07T12:00:00Z [postgres-db] Database ready",
		},
	}
}

func collectMockDetailCmd(containerID string) tea.Msg {
	return ContainerDetailMsg{
		ContainerID: containerID,
		Detail: collector.ContainerDetail{
			CreatedAt: time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC),
			StartedAt: time.Date(2026, 3, 5, 14, 0, 0, 0, time.UTC),
			RestartPolicy: "always",
			Command: "/docker-entrypoint.sh nginx -g 'daemon off;'",
			Mounts: []collector.Mount{
				{Source: "/etc/nginx", Destination: "/etc/nginx", Mode: "ro", Type: "bind"},
			},
			Networks: []collector.NetworkAddress{
				{Name: "bridge", IPv4: "172.17.0.2"},
			},
			PublishedPorts: []collector.PublishedPort{
				{HostIP: "0.0.0.0", HostPort: 80, ContainerPort: 80, Type: "tcp"},
			},
		},
	}
}

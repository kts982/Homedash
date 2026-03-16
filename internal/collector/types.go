package collector

import (
	"fmt"
	"math"
	"strings"
	"time"
)

type SystemData struct {
	Hostname     string
	Uptime       time.Duration
	CPUPercent   float64
	CPUCount     int
	RunningTasks int
	TotalTasks   int
	MemTotal     uint64 // bytes
	MemUsed      uint64 // bytes
	MemPercent   float64
	SwapTotal    uint64 // bytes
	SwapUsed     uint64 // bytes
	SwapPercent  float64
	OpenFiles    uint64
	MaxFiles     uint64
	LoadAvg      [3]float64
	Disks        []DiskInfo
	NetRxRate    float64 // bytes/sec download
	NetTxRate    float64 // bytes/sec upload
	Warnings     []string
	CollectedAt  time.Time
}

type DiskInfo struct {
	Mount   string
	Total   uint64 // bytes
	Used    uint64 // bytes
	Percent float64
}

type Port struct {
	IP          string
	PrivatePort int
	PublicPort  int
	Type        string // "tcp" or "udp"
}

func FormatPorts(ports []Port) string {
	var parts []string
	for _, p := range ports {
		if p.PublicPort > 0 {
			parts = append(parts, fmt.Sprintf("%d->%d/%s", p.PublicPort, p.PrivatePort, p.Type))
		}
	}
	if len(parts) == 0 {
		return "-"
	}
	return strings.Join(parts, ", ")
}

type Container struct {
	ID      string
	Name    string
	Stack   string
	Image   string
	State   string // running, exited, paused, etc.
	Health  string // healthy, unhealthy, starting, none
	CPUPerc float64
	MemUsed uint64 // bytes
	Ports   []Port
	NetRx   uint64 // cumulative bytes received
	NetTx   uint64 // cumulative bytes transmitted
}

type Mount struct {
	Source      string // host path or volume name
	Destination string // container path
	Mode        string // rw, ro
	Type        string // bind, volume, tmpfs
}

type NetworkAddress struct {
	Name string
	IPv4 string
	IPv6 string
}

type PublishedPort struct {
	HostIP        string
	HostPort      int
	ContainerPort int
	Type          string
}

type ContainerDetail struct {
	Mounts         []Mount
	Labels         map[string]string
	RestartPolicy  string
	Command        string
	CreatedAt      time.Time
	StartedAt      time.Time
	Networks       []NetworkAddress
	PublishedPorts []PublishedPort
}

func FormatPublishedPorts(ports []PublishedPort) string {
	var parts []string
	for _, p := range ports {
		if p.HostPort <= 0 || p.ContainerPort <= 0 {
			continue
		}
		host := strings.TrimSpace(p.HostIP)
		switch host {
		case "", "0.0.0.0", "::":
			host = "*"
		default:
			if strings.Contains(host, ":") && !strings.HasPrefix(host, "[") {
				host = "[" + host + "]"
			}
		}
		parts = append(parts, fmt.Sprintf("%s:%d->%d/%s", host, p.HostPort, p.ContainerPort, p.Type))
	}
	if len(parts) == 0 {
		return "-"
	}
	return strings.Join(parts, ", ")
}

type DockerData struct {
	Containers  []Container
	Running     int
	Total       int
	CollectedAt time.Time
}

type WeatherData struct {
	Location    string
	TempC       string
	FeelsLikeC  string
	Condition   string
	Humidity    string
	WindSpeed   string
	WindDir     string
	Icon        string
	CollectedAt time.Time
}

// FormatRate formats bytes/sec into human-readable rate string.
func FormatRate(bps float64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
		TB = GB * 1024
	)
	switch {
	case bps <= 0:
		return "0B/s"
	case bps >= float64(TB):
		return fmt.Sprintf("%.1fT/s", bps/float64(TB))
	case bps >= float64(GB):
		return fmt.Sprintf("%.1fG/s", bps/float64(GB))
	case bps >= float64(MB):
		return fmt.Sprintf("%dM/s", int64(math.Round(bps/float64(MB))))
	case bps >= float64(KB):
		return fmt.Sprintf("%dK/s", int64(math.Round(bps/float64(KB))))
	default:
		return fmt.Sprintf("%dB/s", int64(math.Round(bps)))
	}
}

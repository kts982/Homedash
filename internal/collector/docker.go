package collector

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

const defaultDockerHost = "unix:///var/run/docker.sock"
const dockerAPIVersion = "v1.47"

// maxResponseSize limits Docker API response bodies to prevent memory exhaustion.
const maxResponseSize = 8 * 1024 * 1024 // 8MB

// maxStatsResponseSize limits per-container stats responses.
const maxStatsResponseSize = 1 * 1024 * 1024 // 1MB

// maxLogFrameSize limits individual Docker log stream frames.
const maxLogFrameSize = 1 * 1024 * 1024 // 1MB

var dockerHost = defaultDockerHost

var dockerClient = newDockerClient(10 * time.Second)
var dockerStatsClient = newDockerClient(5 * time.Second)
var dockerActionClient = newDockerClient(30 * time.Second)

func SetDockerHost(host string) {
	host = strings.TrimSpace(host)
	if host == "" {
		host = defaultDockerHost
	}
	dockerHost = host
	dockerClient = newDockerClient(10 * time.Second)
	dockerStatsClient = newDockerClient(5 * time.Second)
	dockerActionClient = newDockerClient(30 * time.Second)
}

func currentDockerHost() string {
	host := strings.TrimSpace(dockerHost)
	if host == "" {
		return defaultDockerHost
	}
	return host
}

func dockerBaseURL() string {
	host := currentDockerHost()
	switch {
	case strings.HasPrefix(host, "unix://") || strings.HasPrefix(host, "/"):
		return "http://localhost"
	case strings.HasPrefix(host, "tcp://"):
		return "http://" + strings.TrimPrefix(host, "tcp://")
	case strings.HasPrefix(host, "http://"), strings.HasPrefix(host, "https://"):
		return host
	default:
		return "http://" + host
	}
}

func dockerURL(path string) string {
	base := strings.TrimRight(dockerBaseURL(), "/")
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return base + path
}

func newDockerClient(timeout time.Duration) *http.Client {
	host := currentDockerHost()
	if strings.HasPrefix(host, "unix://") || strings.HasPrefix(host, "/") {
		socketPath := strings.TrimPrefix(host, "unix://")
		return &http.Client{
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
					var d net.Dialer
					return d.DialContext(ctx, "unix", socketPath)
				},
			},
			Timeout: timeout,
		}
	}
	return &http.Client{Timeout: timeout}
}

type dockerContainer struct {
	ID     string            `json:"Id"`
	Names  []string          `json:"Names"`
	Image  string            `json:"Image"`
	State  string            `json:"State"`
	Status string            `json:"Status"`
	Labels map[string]string `json:"Labels"`
	Ports  []struct {
		IP          string `json:"IP"`
		PrivatePort int    `json:"PrivatePort"`
		PublicPort  int    `json:"PublicPort"`
		Type        string `json:"Type"`
	} `json:"Ports"`
}

type dockerStats struct {
	CPUStats struct {
		CPUUsage struct {
			TotalUsage uint64 `json:"total_usage"`
		} `json:"cpu_usage"`
		SystemCPUUsage uint64 `json:"system_cpu_usage"`
		OnlineCPUs     int    `json:"online_cpus"`
	} `json:"cpu_stats"`
	PrecpuStats struct {
		CPUUsage struct {
			TotalUsage uint64 `json:"total_usage"`
		} `json:"cpu_usage"`
		SystemCPUUsage uint64 `json:"system_cpu_usage"`
	} `json:"precpu_stats"`
	MemoryStats struct {
		Usage uint64 `json:"usage"`
		Stats struct {
			InactiveFile uint64 `json:"inactive_file"`
		} `json:"stats"`
	} `json:"memory_stats"`
	Networks map[string]struct {
		RxBytes uint64 `json:"rx_bytes"`
		TxBytes uint64 `json:"tx_bytes"`
	} `json:"networks"`
}

type containerStats struct {
	CPUPct  float64
	MemUsed uint64
	NetRx   uint64
	NetTx   uint64
}

func CollectDocker() (DockerData, error) {
	var data DockerData
	data.CollectedAt = time.Now()

	// List containers
	resp, err := dockerClient.Get(dockerURL(fmt.Sprintf("/%s/containers/json?all=true", dockerAPIVersion)))
	if err != nil {
		return data, fmt.Errorf("docker list: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return data, fmt.Errorf("docker list: status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return data, fmt.Errorf("docker read: %w", err)
	}

	var containers []dockerContainer
	if err := json.Unmarshal(body, &containers); err != nil {
		return data, fmt.Errorf("docker parse: %w", err)
	}

	data.Total = len(containers)

	// Collect stats in parallel with worker pool
	results := make([]containerStats, len(containers))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 5) // 5 concurrent workers

	for i, c := range containers {
		if c.State != "running" {
			continue
		}
		wg.Add(1)
		go func(idx int, id string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			results[idx] = fetchContainerStats(id)
		}(i, c.ID)
	}
	wg.Wait()

	for i, c := range containers {
		name := ""
		if len(c.Names) > 0 {
			name = strings.TrimPrefix(c.Names[0], "/")
		}

		stack := c.Labels["com.docker.compose.project"]

		health := "-"
		if strings.Contains(c.Status, "unhealthy") {
			health = "unhealthy"
		} else if strings.Contains(c.Status, "healthy") {
			health = "healthy"
		} else if strings.Contains(c.Status, "starting") && strings.Contains(c.Status, "health") {
			health = "starting"
		}

		var ports []Port
		for _, p := range c.Ports {
			ports = append(ports, Port{
				IP:          p.IP,
				PrivatePort: p.PrivatePort,
				PublicPort:  p.PublicPort,
				Type:        p.Type,
			})
		}

		container := Container{
			ID:      c.ID,
			Name:    name,
			Stack:   stack,
			Image:   c.Image,
			State:   c.State,
			Health:  health,
			CPUPerc: results[i].CPUPct,
			MemUsed: results[i].MemUsed,
			Ports:   ports,
			NetRx:   results[i].NetRx,
			NetTx:   results[i].NetTx,
		}

		if c.State == "running" {
			data.Running++
		}

		data.Containers = append(data.Containers, container)
	}

	// Sort: running first, then by stack, then by name
	sort.Slice(data.Containers, func(i, j int) bool {
		a, b := data.Containers[i], data.Containers[j]
		if a.State != b.State {
			if a.State == "running" {
				return true
			}
			if b.State == "running" {
				return false
			}
		}
		if a.Stack != b.Stack {
			return a.Stack < b.Stack
		}
		return a.Name < b.Name
	})

	return data, nil
}

func fetchContainerStats(id string) containerStats {
	url := dockerURL(fmt.Sprintf("/%s/containers/%s/stats?stream=false&one-shot=true", dockerAPIVersion, id))

	resp, err := dockerStatsClient.Get(url)
	if err != nil {
		return containerStats{}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return containerStats{}
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxStatsResponseSize))
	if err != nil {
		return containerStats{}
	}

	var stats dockerStats
	if err := json.Unmarshal(body, &stats); err != nil {
		return containerStats{}
	}

	var result containerStats

	// CPU percentage
	cpuDelta := float64(stats.CPUStats.CPUUsage.TotalUsage - stats.PrecpuStats.CPUUsage.TotalUsage)
	systemDelta := float64(stats.CPUStats.SystemCPUUsage - stats.PrecpuStats.SystemCPUUsage)
	if systemDelta > 0 && stats.CPUStats.OnlineCPUs > 0 {
		result.CPUPct = (cpuDelta / systemDelta) * float64(stats.CPUStats.OnlineCPUs) * 100.0
	}

	// Memory (usage minus cache, guard underflow)
	if stats.MemoryStats.Usage >= stats.MemoryStats.Stats.InactiveFile {
		result.MemUsed = stats.MemoryStats.Usage - stats.MemoryStats.Stats.InactiveFile
	}

	// Network (aggregate all interfaces)
	for _, netStats := range stats.Networks {
		result.NetRx += netStats.RxBytes
		result.NetTx += netStats.TxBytes
	}

	return result
}

func FetchContainerLogs(containerID string, tail int) ([]string, error) {
	url := dockerURL(fmt.Sprintf("/%s/containers/%s/logs?stdout=1&stderr=1&tail=%d&timestamps=1", dockerAPIVersion, containerID, tail))

	resp, err := dockerStatsClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("docker logs: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("docker logs: status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return nil, fmt.Errorf("docker logs read: %w", err)
	}

	return parseDockerLogs(body), nil
}

func parseDockerLogs(data []byte) []string {
	if len(data) == 0 {
		return nil
	}

	// Detect TTY mode: multiplexed streams start with 0x00, 0x01, or 0x02
	if data[0] != 0x00 && data[0] != 0x01 && data[0] != 0x02 {
		// Plain text (TTY mode) — just split lines
		var lines []string
		for _, line := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
			if line != "" {
				lines = append(lines, line)
			}
		}
		return lines
	}

	// Multiplexed stream: 8-byte header per frame
	var lines []string
	pos := 0
	for pos+8 <= len(data) {
		frameSize := binary.BigEndian.Uint32(data[pos+4 : pos+8])
		pos += 8

		end := pos + int(frameSize)
		if end > len(data) {
			end = len(data)
		}

		frame := string(data[pos:end])
		for _, line := range strings.Split(strings.TrimRight(frame, "\n"), "\n") {
			if line != "" {
				lines = append(lines, line)
			}
		}
		pos = end
	}
	return lines
}

// readDockerFramePayload keeps frame alignment even when we cap how much of an
// oversized payload we retain for display.
func readDockerFramePayload(r io.Reader, frameSize uint32) ([]byte, error) {
	captureSize := int(frameSize)
	if captureSize > maxLogFrameSize {
		captureSize = maxLogFrameSize
	}

	payload := make([]byte, captureSize)
	n, err := io.ReadFull(r, payload)
	payload = payload[:n]
	if err != nil {
		return payload, err
	}

	remaining := int64(frameSize) - int64(captureSize)
	if remaining > 0 {
		if _, err := io.CopyN(io.Discard, r, remaining); err != nil {
			return payload, err
		}
	}

	return payload, nil
}

// StreamContainerLogs streams container logs and sends each parsed line to lineCh.
// It blocks until the context is cancelled or the stream ends.
func StreamContainerLogs(ctx context.Context, containerID string, tail int, lineCh chan<- string) error {
	url := dockerURL(fmt.Sprintf("/%s/containers/%s/logs?stdout=1&stderr=1&tail=%d&follow=1&timestamps=1", dockerAPIVersion, containerID, tail))

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("docker stream logs request: %w", err)
	}

	// No timeout — streaming connection stays open until context cancelled
	client := newDockerClient(0)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("docker stream logs: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("docker stream logs: status %d", resp.StatusCode)
	}

	// Detect TTY mode from first 8 bytes
	header := make([]byte, 8)
	_, err = io.ReadFull(resp.Body, header)
	if err != nil {
		if err == io.EOF || err == io.ErrUnexpectedEOF || ctx.Err() != nil {
			return nil
		}
		return fmt.Errorf("docker stream logs read header: %w", err)
	}

	isTTY := header[0] != 0x00 && header[0] != 0x01 && header[0] != 0x02

	if isTTY {
		// TTY mode: plain text stream
		remainder := string(header)
		buf := make([]byte, 4096)
		for {
			n, readErr := resp.Body.Read(buf)
			if n > 0 {
				remainder += string(buf[:n])
				for {
					idx := strings.IndexByte(remainder, '\n')
					if idx < 0 {
						break
					}
					line := remainder[:idx]
					remainder = remainder[idx+1:]
					if line != "" {
						select {
						case lineCh <- line:
						case <-ctx.Done():
							return nil
						}
					}
				}
			}
			if readErr != nil {
				if remainder != "" {
					select {
					case lineCh <- remainder:
					case <-ctx.Done():
					}
				}
				if readErr == io.EOF || ctx.Err() != nil {
					return nil
				}
				return readErr
			}
		}
	}

	// Multiplexed mode: process the first frame
	frameSize := binary.BigEndian.Uint32(header[4:8])
	frameData, err := readDockerFramePayload(resp.Body, frameSize)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		if ctx.Err() != nil {
			return nil
		}
		return fmt.Errorf("docker stream logs read frame: %w", err)
	}
	for _, line := range strings.Split(strings.TrimRight(string(frameData), "\n"), "\n") {
		if line != "" {
			select {
			case lineCh <- line:
			case <-ctx.Done():
				return nil
			}
		}
	}

	// Continue reading subsequent frames
	for {
		_, err := io.ReadFull(resp.Body, header)
		if err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF || ctx.Err() != nil {
				return nil
			}
			return err
		}

		frameSize := binary.BigEndian.Uint32(header[4:8])
		if frameSize == 0 {
			continue
		}
		frameData, err := readDockerFramePayload(resp.Body, frameSize)
		if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}

		for _, line := range strings.Split(strings.TrimRight(string(frameData), "\n"), "\n") {
			if line != "" {
				select {
				case lineCh <- line:
				case <-ctx.Done():
					return nil
				}
			}
		}
	}
}

func InspectContainer(containerID string) (ContainerDetail, error) {
	var detail ContainerDetail

	resp, err := dockerClient.Get(
		dockerURL(fmt.Sprintf("/%s/containers/%s/json", dockerAPIVersion, containerID)))
	if err != nil {
		return detail, fmt.Errorf("docker inspect: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return detail, fmt.Errorf("docker inspect read: %w", err)
	}

	if resp.StatusCode != 200 {
		return detail, fmt.Errorf("docker inspect: status %d: %s", resp.StatusCode, string(body))
	}

	var raw struct {
		Config struct {
			Labels map[string]string `json:"Labels"`
		} `json:"Config"`
		Mounts []struct {
			Type        string `json:"Type"`
			Source      string `json:"Source"`
			Destination string `json:"Destination"`
			Mode        string `json:"Mode"`
		} `json:"Mounts"`
	}

	if err := json.Unmarshal(body, &raw); err != nil {
		return detail, fmt.Errorf("docker inspect parse: %w", err)
	}

	detail.Labels = raw.Config.Labels
	for _, m := range raw.Mounts {
		detail.Mounts = append(detail.Mounts, Mount{
			Source:      m.Source,
			Destination: m.Destination,
			Mode:        m.Mode,
			Type:        m.Type,
		})
	}

	return detail, nil
}

func ContainerAction(containerID, action string) error {
	url := dockerURL(fmt.Sprintf("/%s/containers/%s/%s", dockerAPIVersion, containerID, action))

	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return fmt.Errorf("docker %s request: %w", action, err)
	}

	resp, err := dockerActionClient.Do(req)
	if err != nil {
		return fmt.Errorf("docker %s: %w", action, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 204 || resp.StatusCode == 304 {
		return nil
	}

	body, _ := io.ReadAll(io.LimitReader(resp.Body, maxStatsResponseSize))
	return fmt.Errorf("docker %s failed (%d): %s", action, resp.StatusCode, string(body))
}

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
	"strconv"
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

// maxMergedLogLines keeps stack log views bounded when many containers contribute tail lines.
const maxMergedLogLines = 1000

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

	var (
		lines     []string
		remainder string
	)
	emitLine := func(line string) error {
		lines = append(lines, line)
		return nil
	}

	// Detect TTY mode: multiplexed streams start with 0x00, 0x01, or 0x02
	if data[0] != 0x00 && data[0] != 0x01 && data[0] != 0x02 {
		_ = processDockerLogChunk(&remainder, data, emitLine)
		_ = flushDockerLogRemainder(&remainder, emitLine)
		return lines
	}

	// Multiplexed stream: 8-byte header per frame
	pos := 0
	for pos+8 <= len(data) {
		frameSize := binary.BigEndian.Uint32(data[pos+4 : pos+8])
		pos += 8

		end := pos + int(frameSize)
		if end > len(data) {
			end = len(data)
		}

		_ = processDockerLogChunk(&remainder, data[pos:end], emitLine)
		pos = end
	}
	_ = flushDockerLogRemainder(&remainder, emitLine)
	return lines
}

func processDockerLogChunk(remainder *string, chunk []byte, emit func(string) error) error {
	if len(chunk) == 0 {
		return nil
	}

	*remainder += string(chunk)
	for {
		idx := strings.IndexByte(*remainder, '\n')
		if idx < 0 {
			return nil
		}

		line := (*remainder)[:idx]
		*remainder = (*remainder)[idx+1:]
		if line == "" {
			continue
		}
		if err := emit(line); err != nil {
			return err
		}
	}
}

func flushDockerLogRemainder(remainder *string, emit func(string) error) error {
	if *remainder == "" {
		return nil
	}

	line := *remainder
	*remainder = ""
	if line == "" {
		return nil
	}
	return emit(line)
}

// readDockerFramePayload keeps frame alignment even when we cap how much of an
// oversized payload we retain for display. The returned bool reports whether we
// discarded part of the frame payload.
func readDockerFramePayload(r io.Reader, frameSize uint32) ([]byte, bool, error) {
	captureSize := int(frameSize)
	truncated := captureSize > maxLogFrameSize
	if captureSize > maxLogFrameSize {
		captureSize = maxLogFrameSize
	}

	payload := make([]byte, captureSize)
	n, err := io.ReadFull(r, payload)
	payload = payload[:n]
	if err != nil {
		return payload, truncated, err
	}

	remaining := int64(frameSize) - int64(captureSize)
	if remaining > 0 {
		if _, err := io.CopyN(io.Discard, r, remaining); err != nil {
			return payload, truncated, err
		}
	}

	return payload, truncated, nil
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
	remainder := ""
	emitLine := func(line string) error {
		select {
		case lineCh <- line:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	if isTTY {
		if err := processDockerLogChunk(&remainder, header, emitLine); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		buf := make([]byte, 4096)
		for {
			n, readErr := resp.Body.Read(buf)
			if n > 0 {
				if err := processDockerLogChunk(&remainder, buf[:n], emitLine); err != nil {
					if ctx.Err() != nil {
						return nil
					}
					return err
				}
			}
			if readErr != nil {
				if err := flushDockerLogRemainder(&remainder, emitLine); err != nil && ctx.Err() == nil {
					return err
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
	frameData, truncated, err := readDockerFramePayload(resp.Body, frameSize)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		if ctx.Err() != nil {
			return nil
		}
		return fmt.Errorf("docker stream logs read frame: %w", err)
	}
	if err := processDockerLogChunk(&remainder, frameData, emitLine); err != nil {
		if ctx.Err() != nil {
			return nil
		}
		return err
	}
	if truncated {
		if err := flushDockerLogRemainder(&remainder, emitLine); err != nil && ctx.Err() == nil {
			return err
		}
	}
	if err == io.EOF || err == io.ErrUnexpectedEOF {
		if err := flushDockerLogRemainder(&remainder, emitLine); err != nil && ctx.Err() == nil {
			return err
		}
		return nil
	}

	// Continue reading subsequent frames
	for {
		_, err := io.ReadFull(resp.Body, header)
		if err != nil {
			if flushErr := flushDockerLogRemainder(&remainder, emitLine); flushErr != nil && ctx.Err() == nil {
				return flushErr
			}
			if err == io.EOF || err == io.ErrUnexpectedEOF || ctx.Err() != nil {
				return nil
			}
			return err
		}

		frameSize := binary.BigEndian.Uint32(header[4:8])
		if frameSize == 0 {
			continue
		}
		frameData, truncated, err := readDockerFramePayload(resp.Body, frameSize)
		if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		if err := processDockerLogChunk(&remainder, frameData, emitLine); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		if truncated {
			if err := flushDockerLogRemainder(&remainder, emitLine); err != nil && ctx.Err() == nil {
				return err
			}
		}
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			if err := flushDockerLogRemainder(&remainder, emitLine); err != nil && ctx.Err() == nil {
				return err
			}
			return nil
		}
	}
}

type stackLogTarget struct {
	ID   string
	Name string
}

type stackLogEntry struct {
	line         string
	timestamp    time.Time
	hasTimestamp bool
	order        int
}

func FetchStackLogs(containers []Container, stackName string, tail int) ([]string, error) {
	targets := stackLogTargets(containers, stackName)
	if len(targets) == 0 {
		return nil, nil
	}

	type fetchResult struct {
		target stackLogTarget
		lines  []string
		err    error
	}

	results := make(chan fetchResult, len(targets))
	for _, target := range targets {
		go func(target stackLogTarget) {
			lines, err := FetchContainerLogs(target.ID, tail)
			results <- fetchResult{
				target: target,
				lines:  lines,
				err:    err,
			}
		}(target)
	}

	var (
		entries  []stackLogEntry
		failures []string
		order    int
	)
	for range targets {
		result := <-results
		if result.err != nil {
			failures = append(failures, result.target.Name)
			continue
		}
		for _, line := range result.lines {
			prefixed := prefixStackLogLine(result.target.Name, line)
			ts, ok := parseDockerLogTimestamp(prefixed)
			entries = append(entries, stackLogEntry{
				line:         prefixed,
				timestamp:    ts,
				hasTimestamp: ok,
				order:        order,
			})
			order++
		}
	}

	sort.SliceStable(entries, func(i, j int) bool {
		left := entries[i]
		right := entries[j]
		switch {
		case left.hasTimestamp && right.hasTimestamp:
			if !left.timestamp.Equal(right.timestamp) {
				return left.timestamp.Before(right.timestamp)
			}
		case left.hasTimestamp != right.hasTimestamp:
			return left.hasTimestamp
		}
		return left.order < right.order
	})

	if len(entries) > maxMergedLogLines {
		entries = entries[len(entries)-maxMergedLogLines:]
	}

	lines := make([]string, 0, len(entries))
	for _, entry := range entries {
		lines = append(lines, entry.line)
	}

	if len(lines) == 0 && len(failures) > 0 {
		return nil, fmt.Errorf("docker stack logs: failed for %s", strings.Join(failures, ", "))
	}
	return lines, nil
}

func StreamStackLogs(ctx context.Context, containers []Container, stackName string, tail int, lineCh chan<- string) error {
	targets := stackLogTargets(containers, stackName)
	if len(targets) == 0 {
		return nil
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	errCh := make(chan error, len(targets))
	var wg sync.WaitGroup

	for _, target := range targets {
		wg.Add(1)
		go func(target stackLogTarget) {
			defer wg.Done()

			containerCh := make(chan string, 64)
			streamDone := make(chan error, 1)
			go func() {
				streamDone <- StreamContainerLogs(ctx, target.ID, tail, containerCh)
				close(containerCh)
			}()

			for {
				select {
				case <-ctx.Done():
					return
				case line, ok := <-containerCh:
					if !ok {
						if err := <-streamDone; err != nil && ctx.Err() == nil {
							select {
							case errCh <- fmt.Errorf("%s: %w", target.Name, err):
							default:
							}
							cancel()
						}
						return
					}
					select {
					case lineCh <- prefixStackLogLine(target.Name, line):
					case <-ctx.Done():
						return
					}
				}
			}
		}(target)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-ctx.Done():
		<-done
		if len(errCh) > 0 {
			close(errCh)
			var failures []string
			for err := range errCh {
				failures = append(failures, err.Error())
			}
			if len(failures) > 0 {
				return fmt.Errorf("docker stack stream logs: %s", strings.Join(failures, "; "))
			}
		}
		return nil
	case <-done:
		close(errCh)
		var failures []string
		for err := range errCh {
			failures = append(failures, err.Error())
		}
		if len(failures) > 0 {
			return fmt.Errorf("docker stack stream logs: %s", strings.Join(failures, "; "))
		}
		return nil
	}
}

func stackLogTargets(containers []Container, stackName string) []stackLogTarget {
	var targets []stackLogTarget
	for _, c := range containers {
		if c.Stack != stackName {
			continue
		}
		targets = append(targets, stackLogTarget{
			ID:   c.ID,
			Name: c.Name,
		})
	}
	sort.Slice(targets, func(i, j int) bool {
		if targets[i].Name != targets[j].Name {
			return targets[i].Name < targets[j].Name
		}
		return targets[i].ID < targets[j].ID
	})
	return targets
}

func prefixStackLogLine(containerName, line string) string {
	containerName = strings.TrimSpace(containerName)
	if containerName == "" {
		return line
	}
	if ts, rest, ok := splitDockerTimestampPrefix(line); ok {
		if rest == "" {
			return fmt.Sprintf("%s [%s]", ts, containerName)
		}
		return fmt.Sprintf("%s [%s] %s", ts, containerName, rest)
	}
	return fmt.Sprintf("[%s] %s", containerName, line)
}

func parseDockerLogTimestamp(line string) (time.Time, bool) {
	ts, _, ok := splitDockerTimestampPrefix(line)
	if !ok {
		return time.Time{}, false
	}
	parsed, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		return time.Time{}, false
	}
	return parsed, true
}

func splitDockerTimestampPrefix(line string) (string, string, bool) {
	if len(line) <= len("2006-01-02T15:04:05Z") || line[4] != '-' || line[7] != '-' || line[10] != 'T' {
		return "", "", false
	}
	spaceIdx := strings.IndexByte(line, ' ')
	if spaceIdx <= 19 {
		return "", "", false
	}
	ts := line[:spaceIdx]
	if _, err := time.Parse(time.RFC3339Nano, ts); err != nil {
		return "", "", false
	}
	return ts, line[spaceIdx+1:], true
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
		Created string   `json:"Created"`
		Path    string   `json:"Path"`
		Args    []string `json:"Args"`
		Config  struct {
			Labels map[string]string `json:"Labels"`
		} `json:"Config"`
		State struct {
			StartedAt string `json:"StartedAt"`
		} `json:"State"`
		HostConfig struct {
			RestartPolicy struct {
				Name string `json:"Name"`
			} `json:"RestartPolicy"`
		} `json:"HostConfig"`
		Mounts []struct {
			Type        string `json:"Type"`
			Source      string `json:"Source"`
			Destination string `json:"Destination"`
			Mode        string `json:"Mode"`
		} `json:"Mounts"`
		NetworkSettings struct {
			Ports map[string][]struct {
				HostIP   string `json:"HostIp"`
				HostPort string `json:"HostPort"`
			} `json:"Ports"`
			Networks map[string]struct {
				IPAddress         string `json:"IPAddress"`
				GlobalIPv6Address string `json:"GlobalIPv6Address"`
			} `json:"Networks"`
		} `json:"NetworkSettings"`
	}

	if err := json.Unmarshal(body, &raw); err != nil {
		return detail, fmt.Errorf("docker inspect parse: %w", err)
	}

	detail.Labels = raw.Config.Labels
	detail.RestartPolicy = formatRestartPolicy(raw.HostConfig.RestartPolicy.Name)
	detail.Command = formatContainerCommand(raw.Path, raw.Args)
	detail.CreatedAt = parseDockerTimestamp(raw.Created)
	detail.StartedAt = parseDockerTimestamp(raw.State.StartedAt)
	for _, m := range raw.Mounts {
		detail.Mounts = append(detail.Mounts, Mount{
			Source:      m.Source,
			Destination: m.Destination,
			Mode:        m.Mode,
			Type:        m.Type,
		})
	}
	var networkNames []string
	for name := range raw.NetworkSettings.Networks {
		networkNames = append(networkNames, name)
	}
	sort.Strings(networkNames)
	for _, name := range networkNames {
		network := raw.NetworkSettings.Networks[name]
		detail.Networks = append(detail.Networks, NetworkAddress{
			Name: name,
			IPv4: network.IPAddress,
			IPv6: network.GlobalIPv6Address,
		})
	}
	var publishedKeys []string
	for key := range raw.NetworkSettings.Ports {
		publishedKeys = append(publishedKeys, key)
	}
	sort.Strings(publishedKeys)
	for _, key := range publishedKeys {
		containerPort, proto := parseDockerPortSpec(key)
		if containerPort <= 0 || proto == "" {
			continue
		}
		bindings := raw.NetworkSettings.Ports[key]
		for _, binding := range bindings {
			hostPort, err := strconv.Atoi(strings.TrimSpace(binding.HostPort))
			if err != nil || hostPort <= 0 {
				continue
			}
			detail.PublishedPorts = append(detail.PublishedPorts, PublishedPort{
				HostIP:        strings.TrimSpace(binding.HostIP),
				HostPort:      hostPort,
				ContainerPort: containerPort,
				Type:          proto,
			})
		}
	}
	sort.Slice(detail.PublishedPorts, func(i, j int) bool {
		left := detail.PublishedPorts[i]
		right := detail.PublishedPorts[j]
		switch {
		case left.ContainerPort != right.ContainerPort:
			return left.ContainerPort < right.ContainerPort
		case left.HostPort != right.HostPort:
			return left.HostPort < right.HostPort
		case left.Type != right.Type:
			return left.Type < right.Type
		default:
			return left.HostIP < right.HostIP
		}
	})

	return detail, nil
}

func parseDockerPortSpec(value string) (int, string) {
	containerPortStr, proto, ok := strings.Cut(strings.TrimSpace(value), "/")
	if !ok {
		return 0, ""
	}
	containerPort, err := strconv.Atoi(containerPortStr)
	if err != nil || containerPort <= 0 {
		return 0, ""
	}
	proto = strings.TrimSpace(proto)
	if proto == "" {
		return 0, ""
	}
	return containerPort, proto
}

func formatRestartPolicy(policy string) string {
	policy = strings.TrimSpace(policy)
	if policy == "" {
		return "-"
	}
	return policy
}

func formatContainerCommand(path string, args []string) string {
	var parts []string
	if trimmed := strings.TrimSpace(path); trimmed != "" {
		parts = append(parts, shellQuote(trimmed))
	}
	for _, arg := range args {
		if trimmed := strings.TrimSpace(arg); trimmed != "" {
			parts = append(parts, shellQuote(trimmed))
		}
	}
	if len(parts) == 0 {
		return "-"
	}
	return strings.Join(parts, " ")
}

func shellQuote(s string) string {
	if s == "" {
		return `""`
	}
	if strings.ContainsAny(s, " \t\n\"'") {
		return strconv.Quote(s)
	}
	return s
}

func parseDockerTimestamp(value string) time.Time {
	value = strings.TrimSpace(value)
	if value == "" || strings.HasPrefix(value, "0001-01-01T00:00:00") {
		return time.Time{}
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}
	}
	return parsed
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

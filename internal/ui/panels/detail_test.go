package panels

import (
	"strings"
	"testing"
	"time"

	"github.com/kts982/homedash/internal/collector"
)

func TestRenderDetailAcceptsShortContainerID(t *testing.T) {
	c := &collector.Container{
		ID:    "abc",
		Name:  "svc",
		Image: "svc:latest",
		State: "running",
	}

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("RenderDetail() panicked for short container ID: %v", r)
		}
	}()

	view := RenderDetail(c, nil, "", []string{"log line"}, nil, "", "", 0, 80, 20, false, LogSearch{})
	if !strings.Contains(view, "abc") {
		t.Fatalf("RenderDetail() output does not contain short container ID: %q", view)
	}
}

func TestRenderDetailShowsPolishedMetadata(t *testing.T) {
	c := &collector.Container{
		ID:    "abc123def456",
		Name:  "db",
		Image: "postgres:16",
		State: "running",
		Stack: "homedash",
		Ports: []collector.Port{{PublicPort: 5432, PrivatePort: 5432, Type: "tcp"}},
	}
	meta := &collector.ContainerDetail{
		RestartPolicy: "unless-stopped",
		Command:       `/docker-entrypoint.sh postgres -c shared_buffers=256MB`,
		CreatedAt:     time.Date(2026, 3, 1, 8, 0, 0, 0, time.UTC),
		StartedAt:     time.Date(2026, 3, 6, 12, 34, 0, 0, time.UTC),
		Networks: []collector.NetworkAddress{
			{Name: "app", IPv4: "172.20.0.5"},
			{Name: "edge", IPv4: "172.21.0.9", IPv6: "fd00::9"},
		},
		PublishedPorts: []collector.PublishedPort{
			{HostIP: "0.0.0.0", HostPort: 8080, ContainerPort: 80, Type: "tcp"},
			{HostIP: "127.0.0.1", HostPort: 5432, ContainerPort: 5432, Type: "tcp"},
		},
		Mounts: []collector.Mount{
			{Source: "/host/config", Destination: "/var/lib/postgresql/data", Type: "bind", Mode: "rw"},
		},
		Labels: map[string]string{
			"com.docker.compose.project": "homedash",
			"com.docker.compose.service": "db",
			"com.docker.compose.version": "2.24",
			"homepage.group":             "apps",
			"traefik.enable":             "true",
		},
	}

	view := RenderDetail(c, meta, "homedash", []string{"log line"}, nil, "", "", 0, 100, 24, false, LogSearch{})
	plain := stripANSI(view)

	for _, want := range []string{
		"Policy   unless-stopped",
		"Time     start 2026-03-06 12:34Z  create 2026-03-01 08:00Z",
		"Cmd      /docker-entrypoint.sh postgres -c shared_buffers=256MB",
		"Network  app 172.20.0.5  edge 172.21.0.9,fd00::9",
		"Publish  *:8080->80/tcp, 127.0.0.1:5432->5432/tcp",
		"URLs     http://localhost:8080  http://homedash:8080",
		"Mounts   bind:rw /host/config → /var/lib/postgresql/data",
		"Compose  homedash/db  v2.24",
		"Labels   homepage.group=apps, traefik.enable=true",
	} {
		if !strings.Contains(plain, want) {
			t.Fatalf("RenderDetail() = %q, want substring %q", plain, want)
		}
	}
}

func TestRenderDetailSummarizesPublishedMetadataInNarrowWidth(t *testing.T) {
	c := &collector.Container{
		ID:    "abc123def456",
		Name:  "web",
		Image: "nginx:latest",
		State: "running",
		Stack: "homedash",
	}
	meta := &collector.ContainerDetail{
		PublishedPorts: []collector.PublishedPort{
			{HostIP: "0.0.0.0", HostPort: 8080, ContainerPort: 80, Type: "tcp"},
			{HostIP: "::", HostPort: 8443, ContainerPort: 443, Type: "tcp"},
			{HostIP: "127.0.0.1", HostPort: 5432, ContainerPort: 5432, Type: "tcp"},
		},
	}

	view := RenderDetail(c, meta, "homedash", []string{"log line"}, nil, "", "", 0, 44, 20, false, LogSearch{})
	plain := stripANSI(view)

	for _, want := range []string{
		"Publish  *:8080->80/tcp +2 more",
		"URLs     http://localhost:8080 +3 more",
	} {
		if !strings.Contains(plain, want) {
			t.Fatalf("RenderDetail() = %q, want substring %q", plain, want)
		}
	}
}

func TestRenderDetailShowsLiveWaitingState(t *testing.T) {
	c := &collector.Container{
		ID:    "abc123",
		Name:  "svc",
		Image: "svc:latest",
		State: "running",
	}

	view := RenderDetail(c, nil, "", nil, nil, "", "", 0, 90, 20, true, LogSearch{})
	plain := stripANSI(view)

	for _, want := range []string{
		"LOGS (live)",
		"Waiting for live log output...",
		"Follow mode is active.",
		"ctrl+u/d page",
		"/ search",
	} {
		if !strings.Contains(plain, want) {
			t.Fatalf("RenderDetail() = %q, want substring %q", plain, want)
		}
	}
}

func TestRenderDetailShowsLiveTitleWhenFollowingLoadedLogs(t *testing.T) {
	c := &collector.Container{
		ID:    "abc123",
		Name:  "svc",
		Image: "svc:latest",
		State: "running",
	}

	view := RenderDetail(c, nil, "", []string{"line one", "line two"}, nil, "", "", 0, 90, 20, true, LogSearch{})
	plain := stripANSI(view)

	for _, want := range []string{
		"LOGS (live)",
		"following",
		"2 lines",
		"1-2/2",
	} {
		if !strings.Contains(plain, want) {
			t.Fatalf("RenderDetail() = %q, want substring %q", plain, want)
		}
	}
}

func TestRenderDetailShowsLogErrorState(t *testing.T) {
	c := &collector.Container{
		ID:    "abc123",
		Name:  "svc",
		Image: "svc:latest",
		State: "running",
	}

	view := RenderDetail(c, nil, "", nil, assertErr("socket closed"), "", "", 0, 90, 20, false, LogSearch{})
	plain := stripANSI(view)

	for _, want := range []string{
		"LOGS (error)",
		"Log refresh failed",
		"socket closed",
	} {
		if !strings.Contains(plain, want) {
			t.Fatalf("RenderDetail() = %q, want substring %q", plain, want)
		}
	}
}

func TestRenderStackDetailShowsSummaryAndLogs(t *testing.T) {
	stack := &StackDetail{
		Name:           "media",
		ContainerCount: 3,
		RunningCount:   2,
		UnhealthyCount: 1,
		StoppedCount:   1,
		CPUPerc:        4.2,
		MemUsed:        512 * 1024 * 1024,
		Containers: []StackDetailContainer{
			{Name: "db", State: "running", Image: "postgres:16", Ports: "5432->5432/tcp", CPUPerc: 1.4, MemUsed: 512 * 1024 * 1024, NetRx: 5 * 1024 * 1024, NetTx: 2 * 1024 * 1024},
			{Name: "web", State: "running", Health: "unhealthy", Image: "nginx:latest", Ports: "8080->80/tcp", CPUPerc: 3.5, MemUsed: 256 * 1024 * 1024, NetRx: 20 * 1024 * 1024, NetTx: 4 * 1024 * 1024},
			{Name: "worker", State: "exited", Image: "worker:latest", CPUPerc: 0, MemUsed: 64 * 1024 * 1024, NetRx: 1 * 1024 * 1024, NetTx: 1 * 1024 * 1024},
		},
	}

	view := RenderStackDetail(stack, []string{"2026-03-06T12:00:00Z [web] ready"}, nil, "", "", 0, 120, 24, false, LogSearch{})
	plain := stripANSI(view)

	for _, want := range []string{
		"media  stack",
		"Status   2/3 up  1 unhealthy  1 stopped",
		"Hotspot  CPU web 3.5%  Mem db 512M  Net web 20M rx / 4M tx",
		"Members  db running, web running unhealthy, worker exited",
		"Images   postgres:16, nginx:latest, worker:latest",
		"Ports    db 5432->5432/tcp, web 8080->80/tcp",
		"LOGS",
		"[web] ready",
		"f follow",
		"s stop",
		"R restart",
	} {
		if !strings.Contains(plain, want) {
			t.Fatalf("RenderStackDetail() = %q, want substring %q", plain, want)
		}
	}
}

func TestStackDetailHotspotsPreferRunningTraffic(t *testing.T) {
	line := stackDetailHotspotsLine([]StackDetailContainer{
		{Name: "archived", State: "exited", NetRx: 0, NetTx: 0},
		{Name: "api", State: "running", NetRx: 20 * 1024 * 1024, NetTx: 4 * 1024 * 1024},
		{Name: "worker", State: "running", NetRx: 0, NetTx: 0},
	}, 120)

	if !strings.Contains(line, "Net api 20M rx / 4M tx") {
		t.Fatalf("stackDetailHotspotsLine() = %q, want running traffic hotspot", line)
	}
}

func TestRenderStackDetailShowsWaitingState(t *testing.T) {
	stack := &StackDetail{
		Name:           "media",
		ContainerCount: 2,
		RunningCount:   1,
		StoppedCount:   1,
	}

	view := RenderStackDetail(stack, nil, nil, "", "", 0, 90, 20, true, LogSearch{})
	plain := stripANSI(view)

	for _, want := range []string{
		"Waiting for stack log output...",
		"Follow mode is active across running containers.",
		"S start",
		"s stop",
	} {
		if !strings.Contains(plain, want) {
			t.Fatalf("RenderStackDetail() = %q, want substring %q", plain, want)
		}
	}
}

type assertErr string

func (e assertErr) Error() string { return string(e) }

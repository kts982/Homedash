package panels

import (
	"strings"
	"testing"
	"time"

	"github.com/kostas/homedash/internal/collector"
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

	view := RenderDetail(c, nil, []string{"log line"}, nil, "", "", 0, 80, 20, false)
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
		Mounts: []collector.Mount{
			{Source: "/host/config", Destination: "/var/lib/postgresql/data", Type: "bind", Mode: "rw"},
		},
		Labels: map[string]string{
			"com.docker.compose.project": "homedash",
			"com.docker.compose.service": "db",
			"com.docker.compose.version": "2.24",
		},
	}

	view := RenderDetail(c, meta, []string{"log line"}, nil, "", "", 0, 100, 24, false)
	plain := stripANSI(view)

	for _, want := range []string{
		"Policy   unless-stopped",
		"Time     start 2026-03-06 12:34Z  create 2026-03-01 08:00Z",
		"Cmd      /docker-entrypoint.sh postgres -c shared_buffers=256MB",
		"Addr     app 172.20.0.5  edge 172.21.0.9,fd00::9",
		"Compose  homedash/db  v2.24",
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

	view := RenderDetail(c, nil, nil, nil, "", "", 0, 90, 20, true)
	plain := stripANSI(view)

	for _, want := range []string{
		"LOGS (live)",
		"Waiting for live log output...",
		"Follow mode is active.",
		"ctrl+u/d page",
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

	view := RenderDetail(c, nil, nil, assertErr("socket closed"), "", "", 0, 90, 20, false)
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

type assertErr string

func (e assertErr) Error() string { return string(e) }

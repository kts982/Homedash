package collector

import (
	"context"
	"encoding/binary"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
)

func dockerFrame(stream byte, payload string) []byte {
	frame := make([]byte, 8+len(payload))
	frame[0] = stream
	binary.BigEndian.PutUint32(frame[4:8], uint32(len(payload)))
	copy(frame[8:], payload)
	return frame
}

func TestHealthDetection(t *testing.T) {
	tests := []struct {
		status string
		want   string
	}{
		{"Up 5 minutes (healthy)", "healthy"},
		{"Up 5 minutes (unhealthy)", "unhealthy"},
		{"Up 5 minutes (health: starting)", "starting"},
		{"Up 5 minutes", "-"},
		{"Exited (0) 2 hours ago", "-"},
	}

	for _, tc := range tests {
		t.Run(tc.status, func(t *testing.T) {
			health := "-"
			if strings.Contains(tc.status, "unhealthy") {
				health = "unhealthy"
			} else if strings.Contains(tc.status, "healthy") {
				health = "healthy"
			} else if strings.Contains(tc.status, "starting") && strings.Contains(tc.status, "health") {
				health = "starting"
			}
			if health != tc.want {
				t.Fatalf("health(%q) = %q, want %q", tc.status, health, tc.want)
			}
		})
	}
}

func TestParseDockerLogs(t *testing.T) {
	t.Run("empty input", func(t *testing.T) {
		got := parseDockerLogs(nil)
		if got != nil {
			t.Fatalf("parseDockerLogs(nil) = %#v, want nil", got)
		}
	})

	t.Run("tty mode plain text", func(t *testing.T) {
		input := []byte("line one\nline two\n")
		want := []string{"line one", "line two"}
		got := parseDockerLogs(input)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("parseDockerLogs() = %#v, want %#v", got, want)
		}
	})

	t.Run("multiplexed mode multi line", func(t *testing.T) {
		input := append(dockerFrame(0x01, "stdout one\nstdout two\n"), dockerFrame(0x02, "stderr one\n")...)
		want := []string{"stdout one", "stdout two", "stderr one"}
		got := parseDockerLogs(input)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("parseDockerLogs() = %#v, want %#v", got, want)
		}
	})

	t.Run("multiplexed mode joins lines across frames", func(t *testing.T) {
		input := append(dockerFrame(0x01, "stdout "), dockerFrame(0x01, "one\nstderr")...)
		input = append(input, dockerFrame(0x02, " one\n")...)
		want := []string{"stdout one", "stderr one"}
		got := parseDockerLogs(input)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("parseDockerLogs() = %#v, want %#v", got, want)
		}
	})

	t.Run("truncated frame payload", func(t *testing.T) {
		frame := dockerFrame(0x01, "abcdef\n")
		input := frame[:8+3] // header says 7 bytes, only 3 remain
		want := []string{"abc"}
		got := parseDockerLogs(input)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("parseDockerLogs() = %#v, want %#v", got, want)
		}
	})

	t.Run("truncated header ignored", func(t *testing.T) {
		input := []byte{0x01, 0x00, 0x00}
		got := parseDockerLogs(input)
		if got != nil {
			t.Fatalf("parseDockerLogs() = %#v, want nil", got)
		}
	})
}

func TestStreamContainerLogsKeepsFrameAlignmentAfterOversizedFrame(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if r.URL.Path != "/v1.47/containers/abc123/logs" {
			http.NotFound(w, r)
			return
		}

		oversized := strings.Repeat("x", maxLogFrameSize+32)
		payload := append(dockerFrame(0x01, oversized), dockerFrame(0x01, "after oversize\n")...)
		_, _ = w.Write(payload)
	}))
	defer server.Close()

	saveDockerState(t)
	SetDockerHost(server.URL)

	lines := make(chan string, 4)
	err := StreamContainerLogs(context.Background(), "abc123", 50, lines)
	close(lines)
	if err != nil {
		t.Fatalf("StreamContainerLogs() returned error: %v", err)
	}

	var got []string
	for line := range lines {
		got = append(got, line)
	}

	if len(got) < 2 {
		t.Fatalf("StreamContainerLogs() lines = %#v, want oversized frame output plus trailing line", got)
	}
	if got[len(got)-1] != "after oversize" {
		t.Fatalf("last streamed line = %q, want %q", got[len(got)-1], "after oversize")
	}
}

func TestStreamContainerLogsJoinsLinesAcrossFrames(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if r.URL.Path != "/v1.47/containers/abc123/logs" {
			http.NotFound(w, r)
			return
		}

		payload := append(dockerFrame(0x01, "split"), dockerFrame(0x01, " line\nsecond")...)
		payload = append(payload, dockerFrame(0x02, " line\n")...)
		_, _ = w.Write(payload)
	}))
	defer server.Close()

	saveDockerState(t)
	SetDockerHost(server.URL)

	lines := make(chan string, 4)
	err := StreamContainerLogs(context.Background(), "abc123", 50, lines)
	close(lines)
	if err != nil {
		t.Fatalf("StreamContainerLogs() returned error: %v", err)
	}

	var got []string
	for line := range lines {
		got = append(got, line)
	}

	want := []string{"split line", "second line"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("StreamContainerLogs() lines = %#v, want %#v", got, want)
	}
}

func saveDockerState(t *testing.T) {
	t.Helper()
	oldHost := dockerHost
	oldClient := dockerClient
	oldStatsClient := dockerStatsClient
	oldActionClient := dockerActionClient
	t.Cleanup(func() {
		dockerHost = oldHost
		dockerClient = oldClient
		dockerStatsClient = oldStatsClient
		dockerActionClient = oldActionClient
	})
}

func TestDockerBaseURL(t *testing.T) {
	saveDockerState(t)

	tests := []struct {
		name string
		host string
		want string
	}{
		{name: "empty host uses default unix socket", host: "", want: "http://localhost"},
		{name: "unix scheme", host: "unix:///var/run/docker.sock", want: "http://localhost"},
		{name: "absolute socket path", host: "/var/run/docker.sock", want: "http://localhost"},
		{name: "tcp scheme", host: "tcp://127.0.0.1:2375", want: "http://127.0.0.1:2375"},
		{name: "http scheme", host: "http://docker.local:2375", want: "http://docker.local:2375"},
		{name: "https scheme", host: "https://docker.local:2376", want: "https://docker.local:2376"},
		{name: "bare host", host: "127.0.0.1:2375", want: "http://127.0.0.1:2375"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dockerHost = tc.host
			if got := dockerBaseURL(); got != tc.want {
				t.Fatalf("dockerBaseURL() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestDockerURL(t *testing.T) {
	saveDockerState(t)

	dockerHost = "http://127.0.0.1:2375/"

	tests := []struct {
		path string
		want string
	}{
		{path: "/v1.47/info", want: "http://127.0.0.1:2375/v1.47/info"},
		{path: "v1.47/info", want: "http://127.0.0.1:2375/v1.47/info"},
	}

	for _, tc := range tests {
		if got := dockerURL(tc.path); got != tc.want {
			t.Fatalf("dockerURL(%q) = %q, want %q", tc.path, got, tc.want)
		}
	}
}

func TestInspectContainer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if r.URL.Path != "/v1.47/containers/abc123/json" {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"Created": "2026-03-01T08:00:00.000000000Z",
			"Path": "/docker-entrypoint.sh",
			"Args": ["postgres", "-c", "shared_buffers=256MB"],
			"Config": {
				"Labels": {
					"com.docker.compose.project": "homedash",
					"com.example.env": "prod"
				}
			},
			"State": {
				"StartedAt": "2026-03-06T12:34:56.000000000Z"
			},
			"HostConfig": {
				"RestartPolicy": {
					"Name": "unless-stopped"
				}
			},
			"Mounts": [
				{
					"Type": "bind",
					"Source": "/host/config",
					"Destination": "/app/config",
					"Mode": "ro"
				},
				{
					"Type": "volume",
					"Source": "app-data",
					"Destination": "/data",
					"Mode": "rw"
				}
			],
			"NetworkSettings": {
				"Networks": {
					"app": {
						"IPAddress": "172.20.0.5",
						"GlobalIPv6Address": ""
					},
					"edge": {
						"IPAddress": "172.21.0.9",
						"GlobalIPv6Address": "fd00::9"
					}
				}
			}
		}`))
	}))
	defer server.Close()

	saveDockerState(t)

	SetDockerHost(server.URL)
	dockerClient = server.Client()

	detail, err := InspectContainer("abc123")
	if err != nil {
		t.Fatalf("InspectContainer() returned error: %v", err)
	}

	if detail.Labels["com.docker.compose.project"] != "homedash" {
		t.Fatalf("compose label = %q, want %q", detail.Labels["com.docker.compose.project"], "homedash")
	}
	if detail.Labels["com.example.env"] != "prod" {
		t.Fatalf("env label = %q, want %q", detail.Labels["com.example.env"], "prod")
	}

	wantMounts := []Mount{
		{
			Source:      "/host/config",
			Destination: "/app/config",
			Mode:        "ro",
			Type:        "bind",
		},
		{
			Source:      "app-data",
			Destination: "/data",
			Mode:        "rw",
			Type:        "volume",
		},
	}
	if !reflect.DeepEqual(detail.Mounts, wantMounts) {
		t.Fatalf("mounts = %#v, want %#v", detail.Mounts, wantMounts)
	}
	if detail.RestartPolicy != "unless-stopped" {
		t.Fatalf("restart policy = %q, want %q", detail.RestartPolicy, "unless-stopped")
	}
	if detail.Command != `/docker-entrypoint.sh postgres -c shared_buffers=256MB` {
		t.Fatalf("command = %q, want %q", detail.Command, `/docker-entrypoint.sh postgres -c shared_buffers=256MB`)
	}
	if got := detail.CreatedAt.UTC().Format(time.RFC3339); got != "2026-03-01T08:00:00Z" {
		t.Fatalf("CreatedAt = %q, want %q", got, "2026-03-01T08:00:00Z")
	}
	if got := detail.StartedAt.UTC().Format(time.RFC3339); got != "2026-03-06T12:34:56Z" {
		t.Fatalf("StartedAt = %q, want %q", got, "2026-03-06T12:34:56Z")
	}
	wantNetworks := []NetworkAddress{
		{Name: "app", IPv4: "172.20.0.5"},
		{Name: "edge", IPv4: "172.21.0.9", IPv6: "fd00::9"},
	}
	if !reflect.DeepEqual(detail.Networks, wantNetworks) {
		t.Fatalf("networks = %#v, want %#v", detail.Networks, wantNetworks)
	}
}

func TestContainerAction(t *testing.T) {
	tests := []struct {
		name        string
		status      int
		body        string
		wantErr     bool
		errContains string
	}{
		{name: "204 success", status: http.StatusNoContent},
		{name: "304 already in requested state", status: http.StatusNotModified},
		{
			name:        "error status",
			status:      http.StatusInternalServerError,
			body:        "boom",
			wantErr:     true,
			errContains: "docker start failed (500): boom",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost {
					http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
					return
				}
				if r.URL.Path != "/v1.47/containers/abc123/start" {
					http.NotFound(w, r)
					return
				}
				w.WriteHeader(tc.status)
				if tc.body != "" {
					_, _ = w.Write([]byte(tc.body))
				}
			}))
			defer server.Close()

			saveDockerState(t)

			SetDockerHost(server.URL)

			err := ContainerAction("abc123", "start")
			if tc.wantErr {
				if err == nil {
					t.Fatal("ContainerAction() expected error, got nil")
				}
				if !strings.Contains(err.Error(), tc.errContains) {
					t.Fatalf("ContainerAction() error = %q, want substring %q", err.Error(), tc.errContains)
				}
				return
			}

			if err != nil {
				t.Fatalf("ContainerAction() returned error: %v", err)
			}
		})
	}
}

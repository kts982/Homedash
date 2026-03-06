package collector

import "testing"

func TestFormatPorts(t *testing.T) {
	tests := []struct {
		name  string
		ports []Port
		want  string
	}{
		{
			name:  "empty ports list",
			ports: nil,
			want:  "-",
		},
		{
			name: "single public port",
			ports: []Port{
				{PrivatePort: 80, PublicPort: 8080, Type: "tcp"},
			},
			want: "8080->80/tcp",
		},
		{
			name: "multiple public ports",
			ports: []Port{
				{PrivatePort: 80, PublicPort: 8080, Type: "tcp"},
				{PrivatePort: 443, PublicPort: 8443, Type: "tcp"},
				{PrivatePort: 53, PublicPort: 5353, Type: "udp"},
			},
			want: "8080->80/tcp, 8443->443/tcp, 5353->53/udp",
		},
		{
			name: "only private ports are filtered",
			ports: []Port{
				{PrivatePort: 80, PublicPort: 0, Type: "tcp"},
				{PrivatePort: 53, PublicPort: 0, Type: "udp"},
			},
			want: "-",
		},
		{
			name: "mixed public and private ports",
			ports: []Port{
				{PrivatePort: 80, PublicPort: 0, Type: "tcp"},
				{PrivatePort: 443, PublicPort: 8443, Type: "tcp"},
			},
			want: "8443->443/tcp",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := FormatPorts(tc.ports)
			if got != tc.want {
				t.Fatalf("FormatPorts() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestFormatPublishedPorts(t *testing.T) {
	tests := []struct {
		name  string
		ports []PublishedPort
		want  string
	}{
		{
			name:  "empty ports list",
			ports: nil,
			want:  "-",
		},
		{
			name: "wildcard and localhost bindings",
			ports: []PublishedPort{
				{HostIP: "0.0.0.0", HostPort: 8080, ContainerPort: 80, Type: "tcp"},
				{HostIP: "127.0.0.1", HostPort: 5432, ContainerPort: 5432, Type: "tcp"},
			},
			want: "*:8080->80/tcp, 127.0.0.1:5432->5432/tcp",
		},
		{
			name: "ipv6 binding is bracketed",
			ports: []PublishedPort{
				{HostIP: "fd00::9", HostPort: 8443, ContainerPort: 443, Type: "tcp"},
			},
			want: "[fd00::9]:8443->443/tcp",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := FormatPublishedPorts(tc.ports)
			if got != tc.want {
				t.Fatalf("FormatPublishedPorts() = %q, want %q", got, tc.want)
			}
		})
	}
}

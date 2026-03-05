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

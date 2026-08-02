package registry

import "testing"

func TestParseReference(t *testing.T) {
	tests := []struct {
		name       string
		image      string
		wantHost   string
		wantRepo   string
		wantTag    string
		wantDigest string
		wantErr    bool
	}{
		// Docker Hub defaulting
		{"bare name", "nginx", defaultRegistry, "library/nginx", "latest", "", false},
		{"bare name with tag", "nginx:alpine", defaultRegistry, "library/nginx", "alpine", "", false},
		{"hub namespace", "adguard/adguardhome:latest", defaultRegistry, "adguard/adguardhome", "latest", "", false},
		{"hub namespace no tag", "crazymax/diun", defaultRegistry, "crazymax/diun", "latest", "", false},

		// Explicit registries — the first segment is a host only if it has a
		// dot or colon, which is what separates these from the cases above.
		{"ghcr", "ghcr.io/gethomepage/homepage:latest", "ghcr.io", "gethomepage/homepage", "latest", "", false},
		{"deep path", "git.tsioumpris.de/kostas/mcp-sap-docs:abap", "git.tsioumpris.de", "kostas/mcp-sap-docs", "abap", "", false},
		{"forgejo code host", "code.forgejo.org/forgejo/runner:12", "code.forgejo.org", "forgejo/runner", "12", "", false},
		{"localhost is a host", "localhost/myapp:dev", "localhost", "myapp", "dev", "", false},
		{"host with port", "localhost:5000/myapp:dev", "localhost:5000", "myapp", "dev", "", false},
		{"host with port, no tag", "registry.local:5000/team/app", "registry.local:5000", "team/app", "latest", "", false},

		// Locally-built images with no registry component.
		{"local build", "caddy-hetzner:local", defaultRegistry, "library/caddy-hetzner", "local", "", false},

		// Digest pins
		{"digest pin", "nginx@sha256:abc123", defaultRegistry, "library/nginx", "", "sha256:abc123", false},
		{"tag and digest", "nginx:alpine@sha256:abc123", defaultRegistry, "library/nginx", "alpine", "sha256:abc123", false},

		// Failures
		{"empty", "", "", "", "", "", true},
		{"whitespace only", "   ", "", "", "", "", true},
		{"empty tag", "nginx:", "", "", "", "", true},
		{"empty digest", "nginx@", "", "", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ref, err := ParseReference(tt.image)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseReference(%q) error = %v, wantErr %v", tt.image, err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if ref.Registry != tt.wantHost {
				t.Errorf("Registry = %q, want %q", ref.Registry, tt.wantHost)
			}
			if ref.Repository != tt.wantRepo {
				t.Errorf("Repository = %q, want %q", ref.Repository, tt.wantRepo)
			}
			if ref.Tag != tt.wantTag {
				t.Errorf("Tag = %q, want %q", ref.Tag, tt.wantTag)
			}
			if ref.Digest != tt.wantDigest {
				t.Errorf("Digest = %q, want %q", ref.Digest, tt.wantDigest)
			}
		})
	}
}

func TestDigestOf(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"nginx@sha256:abc123", "sha256:abc123"},
		{"ghcr.io/team/app@sha256:def456", "sha256:def456"},
		{"caddy-hetzner@sha256:c5e54aa2", "sha256:c5e54aa2"},
		{"nginx:alpine", ""},
		{"", ""},
	}
	for _, tt := range tests {
		if got := DigestOf(tt.in); got != tt.want {
			t.Errorf("DigestOf(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestParseChallenge(t *testing.T) {
	tests := []struct {
		name        string
		header      string
		wantRealm   string
		wantService string
		wantErr     bool
	}{
		{
			"forgejo form",
			`Bearer realm="https://git.tsioumpris.de/v2/token",service="container_registry",scope="*"`,
			"https://git.tsioumpris.de/v2/token", "container_registry", false,
		},
		{
			"docker hub form",
			`Bearer realm="https://auth.docker.io/token",service="registry.docker.io"`,
			"https://auth.docker.io/token", "registry.docker.io", false,
		},
		{
			"spaces after commas",
			`Bearer realm="https://ghcr.io/token", service="ghcr.io", scope="repository:x/y:pull"`,
			"https://ghcr.io/token", "ghcr.io", false,
		},
		{
			"unquoted values",
			`Bearer realm=https://example.com/token,service=example`,
			"https://example.com/token", "example", false,
		},
		{
			"lowercase scheme",
			`bearer realm="https://example.com/token"`,
			"https://example.com/token", "", false,
		},
		{
			"comma inside quoted scope must not split",
			`Bearer realm="https://example.com/token",scope="repository:a:pull,push",service="svc"`,
			"https://example.com/token", "svc", false,
		},
		{"empty header", "", "", "", true},
		{"basic auth is not supported", `Basic realm="example"`, "", "", true},
		{"no realm", `Bearer service="registry.docker.io"`, "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ch, err := parseChallenge(tt.header)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseChallenge() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if ch.Realm != tt.wantRealm {
				t.Errorf("Realm = %q, want %q", ch.Realm, tt.wantRealm)
			}
			if ch.Service != tt.wantService {
				t.Errorf("Service = %q, want %q", ch.Service, tt.wantService)
			}
		})
	}
}

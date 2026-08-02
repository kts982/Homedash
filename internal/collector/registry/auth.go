package registry

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Credentials is a resolved username/password pair for one registry.
type Credentials struct {
	Username string
	Password string

	// HelperName is set when the credential is held by an external helper
	// binary rather than in the config file. HomeDash does not shell out, so
	// such a credential cannot be used — the name is carried so the failure
	// can say *why* instead of silently reporting "no update".
	HelperName string
}

// Usable reports whether these credentials can actually be sent.
func (c Credentials) Usable() bool {
	return c.Username != "" && c.HelperName == ""
}

// dockerConfig is the subset of ~/.docker/config.json that matters here.
type dockerConfig struct {
	Auths map[string]struct {
		Auth     string `json:"auth"`
		Username string `json:"username"`
		Password string `json:"password"`
	} `json:"auths"`
	CredsStore  string            `json:"credsStore"`
	CredHelpers map[string]string `json:"credHelpers"`
}

// CredentialStore resolves registry credentials.
type CredentialStore interface {
	Lookup(registryHost string) Credentials
}

// DockerConfigStore reads credentials from a Docker CLI config file.
type DockerConfigStore struct {
	cfg dockerConfig
}

// LoadDockerConfig reads the Docker CLI config. A missing or unreadable file
// is not an error — it simply means every lookup comes back empty, and checks
// proceed anonymously.
func LoadDockerConfig(path string) *DockerConfigStore {
	store := &DockerConfigStore{}
	raw, err := os.ReadFile(path) //nolint:gosec // path is the user's own docker config
	if err != nil {
		return store
	}
	_ = json.Unmarshal(raw, &store.cfg)
	return store
}

// DefaultDockerConfigPath honours DOCKER_CONFIG, matching the Docker CLI, then
// falls back to ~/.docker/config.json.
func DefaultDockerConfigPath() string {
	if dir := strings.TrimSpace(os.Getenv("DOCKER_CONFIG")); dir != "" {
		return filepath.Join(dir, "config.json")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".docker", "config.json")
}

// Lookup returns credentials for a registry host, or the zero value if none
// are configured.
func (s *DockerConfigStore) Lookup(registryHost string) Credentials {
	if s == nil {
		return Credentials{}
	}

	// A per-registry helper wins over the global one.
	if helper, ok := s.cfg.CredHelpers[registryHost]; ok && helper != "" {
		return Credentials{HelperName: helper}
	}

	for _, key := range authKeyCandidates(registryHost) {
		entry, ok := s.cfg.Auths[key]
		if !ok {
			continue
		}
		if entry.Username != "" && entry.Password != "" {
			return Credentials{Username: entry.Username, Password: entry.Password}
		}
		if entry.Auth != "" {
			decoded, err := base64.StdEncoding.DecodeString(entry.Auth)
			if err != nil {
				continue
			}
			user, pass, found := strings.Cut(string(decoded), ":")
			if found && user != "" {
				return Credentials{Username: user, Password: pass}
			}
		}
		// The entry exists but holds no inline credential, which is what a
		// credsStore setup looks like.
		if s.cfg.CredsStore != "" {
			return Credentials{HelperName: s.cfg.CredsStore}
		}
	}

	if s.cfg.CredsStore != "" {
		return Credentials{HelperName: s.cfg.CredsStore}
	}
	return Credentials{}
}

// authKeyCandidates lists the forms a registry may be keyed under in
// config.json. Docker Hub is the awkward one: `docker login` writes the
// legacy v1 URL, not the registry host actually queried.
func authKeyCandidates(registryHost string) []string {
	candidates := []string{registryHost}
	if registryHost == defaultRegistry {
		candidates = append(candidates,
			"https://index.docker.io/v1/",
			"index.docker.io",
			"docker.io",
		)
	}
	return candidates
}

// challenge is a parsed WWW-Authenticate Bearer challenge.
type challenge struct {
	Realm   string
	Service string
	Scope   string
}

// parseChallenge reads a `WWW-Authenticate: Bearer realm="…",service="…"`
// header. Registries vary in spacing, quoting and which parameters they send,
// so this tolerates all of it and only insists on a realm.
func parseChallenge(header string) (challenge, error) {
	header = strings.TrimSpace(header)
	if header == "" {
		return challenge{}, fmt.Errorf("empty WWW-Authenticate header")
	}

	scheme, params, found := strings.Cut(header, " ")
	if !found || !strings.EqualFold(scheme, "Bearer") {
		return challenge{}, fmt.Errorf("unsupported auth scheme in %q", header)
	}

	var c challenge
	for _, part := range splitChallengeParams(params) {
		key, value, ok := strings.Cut(part, "=")
		if !ok {
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		value = strings.Trim(strings.TrimSpace(value), `"`)
		switch key {
		case "realm":
			c.Realm = value
		case "service":
			c.Service = value
		case "scope":
			c.Scope = value
		}
	}

	if c.Realm == "" {
		return challenge{}, fmt.Errorf("no realm in challenge %q", header)
	}
	return c, nil
}

// splitChallengeParams splits on commas that are not inside quotes — scope
// values legitimately contain commas.
func splitChallengeParams(params string) []string {
	var (
		out     []string
		current strings.Builder
		inQuote bool
	)
	for _, r := range params {
		switch {
		case r == '"':
			inQuote = !inQuote
			current.WriteRune(r)
		case r == ',' && !inQuote:
			out = append(out, current.String())
			current.Reset()
		default:
			current.WriteRune(r)
		}
	}
	if current.Len() > 0 {
		out = append(out, current.String())
	}
	return out
}

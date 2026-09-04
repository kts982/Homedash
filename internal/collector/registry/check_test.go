package registry

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	localDigest  = "sha256:1111111111111111111111111111111111111111111111111111111111111111"
	remoteDigest = "sha256:2222222222222222222222222222222222222222222222222222222222222222"
)

// fakeRegistry models a Registry v2 endpoint including the auth challenge.
type fakeRegistry struct {
	// requireAuth makes the first manifest HEAD return 401 with a challenge.
	requireAuth bool
	// requireBasic makes the token endpoint reject anonymous requests.
	requireBasic bool
	// wantUser/wantPass are the credentials the token endpoint accepts.
	wantUser, wantPass string
	// digest is returned in Docker-Content-Digest on success.
	digest string
	// omitDigestHeader returns 200 with no digest header.
	omitDigestHeader bool
	// manifestStatus overrides the authenticated manifest response status.
	manifestStatus int
	// tokenStatus overrides the token endpoint status.
	tokenStatus int
	// malformedChallenge sends a WWW-Authenticate header that cannot parse.
	malformedChallenge bool
	// malformedToken sends non-JSON from the token endpoint.
	malformedToken bool
	// emptyToken sends valid JSON with no token in it.
	emptyToken bool

	// observed state, for assertions
	gotBasicAuth  bool
	gotScope      string
	manifestCalls int
}

func (f *fakeRegistry) handler(t *testing.T) http.Handler {
	t.Helper()
	mux := http.NewServeMux()

	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		f.gotScope = r.URL.Query().Get("scope")
		user, pass, ok := r.BasicAuth()
		f.gotBasicAuth = ok

		if f.tokenStatus != 0 {
			w.WriteHeader(f.tokenStatus)
			return
		}
		if f.requireBasic {
			if !ok || user != f.wantUser || pass != f.wantPass {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
		}
		if f.malformedToken {
			_, _ = w.Write([]byte("this is not json"))
			return
		}
		if f.emptyToken {
			_ = json.NewEncoder(w).Encode(map[string]string{"unrelated": "field"})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"token": "test-token"})
	})

	mux.HandleFunc("/v2/", func(w http.ResponseWriter, r *http.Request) {
		f.manifestCalls++
		authorized := r.Header.Get("Authorization") != ""

		if f.requireAuth && !authorized {
			if f.malformedChallenge {
				w.Header().Set("WWW-Authenticate", "Bearer this-has-no-realm")
			} else {
				w.Header().Set("WWW-Authenticate",
					fmt.Sprintf(`Bearer realm="http://%s/token",service="test"`, r.Host))
			}
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if f.manifestStatus != 0 {
			w.WriteHeader(f.manifestStatus)
			return
		}
		if !f.omitDigestHeader {
			w.Header().Set("Docker-Content-Digest", f.digest)
		}
		w.WriteHeader(http.StatusOK)
	})

	return mux
}

// newChecker wires a checker to a fake registry.
func newChecker(t *testing.T, f *fakeRegistry, creds CredentialStore) (*HTTPChecker, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(f.handler(t))
	t.Cleanup(srv.Close)

	return &HTTPChecker{
		Client:      srv.Client(),
		Credentials: creds,
		BaseURL:     srv.URL,
		Concurrency: 2,
		Now:         func() time.Time { return time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC) },
	}, srv
}

func checkOne(t *testing.T, c *HTTPChecker, img Image) Status {
	t.Helper()
	results := c.Check(context.Background(), []Image{img})
	if len(results) != 1 {
		t.Fatalf("Check returned %d results, want 1", len(results))
	}
	return results[0]
}

func TestCheckDetectsUpToDate(t *testing.T) {
	f := &fakeRegistry{digest: localDigest}
	c, _ := newChecker(t, f, nil)

	got := checkOne(t, c, Image{Ref: "nginx:alpine", LocalDigest: localDigest})
	if got.State != StateCurrent {
		t.Fatalf("State = %v (%s), want current", got.State, got.Reason)
	}
	if got.RemoteDigest != localDigest {
		t.Errorf("RemoteDigest = %q, want %q", got.RemoteDigest, localDigest)
	}
}

func TestCheckDetectsUpdateAvailable(t *testing.T) {
	f := &fakeRegistry{digest: remoteDigest}
	c, _ := newChecker(t, f, nil)

	got := checkOne(t, c, Image{Ref: "nginx:alpine", LocalDigest: localDigest})
	if got.State != StateAvailable {
		t.Fatalf("State = %v (%s), want available", got.State, got.Reason)
	}
	if got.RemoteDigest != remoteDigest {
		t.Errorf("RemoteDigest = %q, want %q", got.RemoteDigest, remoteDigest)
	}
}

// The 401 → challenge → token → retry path is the normal one, even for
// public images on Docker Hub.
func TestCheckFollowsAuthChallengeAnonymously(t *testing.T) {
	f := &fakeRegistry{requireAuth: true, digest: remoteDigest}
	c, _ := newChecker(t, f, nil)

	got := checkOne(t, c, Image{Ref: "nginx:alpine", LocalDigest: localDigest})
	if got.State != StateAvailable {
		t.Fatalf("State = %v (%s), want available", got.State, got.Reason)
	}
	if f.manifestCalls != 2 {
		t.Errorf("manifest was requested %d times, want 2 (challenge then retry)", f.manifestCalls)
	}
	if want := "repository:library/nginx:pull"; f.gotScope != want {
		t.Errorf("token scope = %q, want %q", f.gotScope, want)
	}
}

// A private registry: the token request must carry the stored credentials.
func TestCheckSendsStoredCredentials(t *testing.T) {
	f := &fakeRegistry{
		requireAuth:  true,
		requireBasic: true,
		wantUser:     "Kostas",
		wantPass:     "s3cret",
		digest:       localDigest,
	}
	creds := staticStore{"": Credentials{Username: "Kostas", Password: "s3cret"}}
	c, _ := newChecker(t, f, creds)

	got := checkOne(t, c, Image{Ref: "git.example.de/kostas/app:abap", LocalDigest: localDigest})
	if got.State != StateCurrent {
		t.Fatalf("State = %v (%s), want current", got.State, got.Reason)
	}
	if !f.gotBasicAuth {
		t.Error("token request did not carry basic auth")
	}
}

// Docker Hub returns 401 for repositories that do not exist, so a locally
// built image is indistinguishable from a private one. Both are unwatchable,
// not errors — this must not look like a problem in the UI.
func TestCheckReportsUnwatchableForInaccessibleRepo(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound} {
		t.Run(fmt.Sprintf("HTTP %d", status), func(t *testing.T) {
			f := &fakeRegistry{manifestStatus: status}
			c, _ := newChecker(t, f, nil)

			got := checkOne(t, c, Image{Ref: "caddy-hetzner:local", LocalDigest: localDigest})
			if got.State != StateUnwatchable {
				t.Fatalf("State = %v (%s), want unwatchable", got.State, got.Reason)
			}
			if got.Reason == "" {
				t.Error("Reason is empty; unwatchable results must explain themselves")
			}
		})
	}
}

// An image with no local digest cannot be compared against anything.
func TestCheckReportsUnwatchableWithoutLocalDigest(t *testing.T) {
	f := &fakeRegistry{digest: remoteDigest}
	c, _ := newChecker(t, f, nil)

	got := checkOne(t, c, Image{Ref: "nginx:alpine", LocalDigest: ""})
	if got.State != StateUnwatchable {
		t.Fatalf("State = %v, want unwatchable", got.State)
	}
	if f.manifestCalls != 0 {
		t.Error("a registry request was made despite there being nothing to compare")
	}
}

// A digest-pinned image can never move under that reference.
func TestCheckReportsUnwatchableForDigestPin(t *testing.T) {
	f := &fakeRegistry{digest: remoteDigest}
	c, _ := newChecker(t, f, nil)

	got := checkOne(t, c, Image{Ref: "nginx@" + localDigest, LocalDigest: localDigest})
	if got.State != StateUnwatchable {
		t.Fatalf("State = %v, want unwatchable", got.State)
	}
	if !strings.Contains(got.Reason, "digest") {
		t.Errorf("Reason = %q, want it to mention the digest pin", got.Reason)
	}
}

// Credentials behind a helper binary cannot be used, and must be reported
// rather than silently degrading to "no update available".
func TestCheckReportsUnwatchableForCredentialHelper(t *testing.T) {
	f := &fakeRegistry{requireAuth: true, requireBasic: true, wantUser: "x", digest: localDigest}
	creds := staticStore{"": Credentials{HelperName: "desktop"}}
	c, _ := newChecker(t, f, creds)

	got := checkOne(t, c, Image{Ref: "git.example.de/kostas/app:abap", LocalDigest: localDigest})
	if got.State != StateUnwatchable {
		t.Fatalf("State = %v (%s), want unwatchable", got.State, got.Reason)
	}
	if !strings.Contains(got.Reason, "desktop") {
		t.Errorf("Reason = %q, want it to name the credential helper", got.Reason)
	}
}

// Transport-level and server-side failures are errors, not unwatchable —
// they may succeed on a later attempt.
func TestCheckReportsErrorForServerFailure(t *testing.T) {
	cases := []struct {
		name string
		f    *fakeRegistry
	}{
		{"500 from manifest", &fakeRegistry{manifestStatus: http.StatusInternalServerError}},
		{"missing digest header", &fakeRegistry{omitDigestHeader: true}},
		{"malformed challenge", &fakeRegistry{requireAuth: true, malformedChallenge: true}},
		{"malformed token response", &fakeRegistry{requireAuth: true, malformedToken: true}},
		{"empty token response", &fakeRegistry{requireAuth: true, emptyToken: true}},
		{"500 from token endpoint", &fakeRegistry{requireAuth: true, tokenStatus: http.StatusInternalServerError}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, _ := newChecker(t, tc.f, nil)
			got := checkOne(t, c, Image{Ref: "nginx:alpine", LocalDigest: localDigest})
			if got.State != StateError {
				t.Fatalf("State = %v (%s), want error", got.State, got.Reason)
			}
			if got.Reason == "" {
				t.Error("Reason is empty; errors must explain themselves")
			}
		})
	}
}

func TestCheckReportsErrorWhenRegistryUnreachable(t *testing.T) {
	c := &HTTPChecker{
		Client:  &http.Client{Timeout: time.Second},
		BaseURL: "http://127.0.0.1:1", // nothing listens here
		Now:     time.Now,
	}
	got := checkOne(t, c, Image{Ref: "nginx:alpine", LocalDigest: localDigest})
	if got.State != StateError {
		t.Fatalf("State = %v (%s), want error", got.State, got.Reason)
	}
}

// Results must line up with the inputs even though checks run concurrently.
func TestCheckPreservesInputOrder(t *testing.T) {
	f := &fakeRegistry{digest: remoteDigest}
	c, _ := newChecker(t, f, nil)

	images := []Image{
		{Ref: "a:1", LocalDigest: localDigest},  // available
		{Ref: "b:1", LocalDigest: remoteDigest}, // current
		{Ref: "c:1", LocalDigest: ""},           // unwatchable
		{Ref: "d:1", LocalDigest: localDigest},  // available
	}
	got := c.Check(context.Background(), images)

	if len(got) != len(images) {
		t.Fatalf("got %d results, want %d", len(got), len(images))
	}
	want := []State{StateAvailable, StateCurrent, StateUnwatchable, StateAvailable}
	for i := range want {
		if got[i].Ref != images[i].Ref {
			t.Errorf("result %d is for %q, want %q", i, got[i].Ref, images[i].Ref)
		}
		if got[i].State != want[i] {
			t.Errorf("result %d (%s) state = %v, want %v", i, got[i].Ref, got[i].State, want[i])
		}
	}
}

func TestCheckEmptyInput(t *testing.T) {
	c := &HTTPChecker{}
	if got := c.Check(context.Background(), nil); len(got) != 0 {
		t.Fatalf("got %d results for empty input, want 0", len(got))
	}
}

func TestCheckHonoursCancelledContext(t *testing.T) {
	f := &fakeRegistry{digest: remoteDigest}
	c, _ := newChecker(t, f, nil)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	got := c.Check(ctx, []Image{{Ref: "nginx:alpine", LocalDigest: localDigest}})
	if len(got) != 1 {
		t.Fatalf("got %d results, want 1", len(got))
	}
	if got[0].State != StateError {
		t.Errorf("State = %v, want error for a cancelled check", got[0].State)
	}
}

// staticStore is a CredentialStore returning the same credentials for any
// host (keyed by "" for convenience in tests).
type staticStore map[string]Credentials

func (s staticStore) Lookup(host string) Credentials {
	if c, ok := s[host]; ok {
		return c
	}
	return s[""]
}

// ── Credential loading ─────────────────────────────────────────────

func writeDockerConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestDockerConfigStoreLookup(t *testing.T) {
	encoded := base64.StdEncoding.EncodeToString([]byte("Kostas:t0ken"))
	path := writeDockerConfig(t, fmt.Sprintf(`{
	  "auths": {
	    "git.tsioumpris.de": {"auth": %q},
	    "explicit.example": {"username": "alice", "password": "pw"}
	  }
	}`, encoded))

	store := LoadDockerConfig(path)

	t.Run("base64 auth field", func(t *testing.T) {
		got := store.Lookup("git.tsioumpris.de")
		if got.Username != "Kostas" || got.Password != "t0ken" {
			t.Errorf("got %+v, want Kostas/t0ken", got)
		}
		if !got.Usable() {
			t.Error("credentials should be usable")
		}
	})

	t.Run("explicit username and password", func(t *testing.T) {
		got := store.Lookup("explicit.example")
		if got.Username != "alice" || got.Password != "pw" {
			t.Errorf("got %+v, want alice/pw", got)
		}
	})

	t.Run("unknown host", func(t *testing.T) {
		got := store.Lookup("unknown.example")
		if got.Usable() || got.HelperName != "" {
			t.Errorf("got %+v, want zero value", got)
		}
	})
}

// `docker login` writes the legacy v1 URL for Docker Hub, not the host the
// manifest request actually goes to.
func TestDockerConfigStoreResolvesDockerHubLegacyKey(t *testing.T) {
	encoded := base64.StdEncoding.EncodeToString([]byte("hubuser:hubpass"))
	path := writeDockerConfig(t, fmt.Sprintf(`{
	  "auths": {"https://index.docker.io/v1/": {"auth": %q}}
	}`, encoded))

	got := LoadDockerConfig(path).Lookup(defaultRegistry)
	if got.Username != "hubuser" {
		t.Errorf("Username = %q, want hubuser (legacy Docker Hub key should resolve)", got.Username)
	}
}

func TestDockerConfigStoreReportsCredentialHelpers(t *testing.T) {
	t.Run("global credsStore applies to logged-in hosts", func(t *testing.T) {
		// `docker login` writes an empty auths entry when a helper holds the
		// secret; that entry is what marks the host as helper-backed.
		path := writeDockerConfig(t, `{"credsStore": "desktop", "auths": {"ghcr.io": {}}}`)
		got := LoadDockerConfig(path).Lookup("ghcr.io")
		if got.HelperName != "desktop" {
			t.Errorf("HelperName = %q, want desktop", got.HelperName)
		}
		if got.Usable() {
			t.Error("helper-backed credentials must not be reported as usable")
		}
	})

	t.Run("global credsStore does not cover hosts never logged in to", func(t *testing.T) {
		path := writeDockerConfig(t, `{"credsStore": "desktop", "auths": {}}`)
		got := LoadDockerConfig(path).Lookup("any.example")
		if got.Usable() || got.HelperName != "" {
			t.Errorf("got %+v, want zero value (anonymous access)", got)
		}
	})

	t.Run("per-registry credHelpers wins", func(t *testing.T) {
		path := writeDockerConfig(t, `{
		  "credsStore": "desktop",
		  "credHelpers": {"gcr.io": "gcloud"},
		  "auths": {}
		}`)
		got := LoadDockerConfig(path).Lookup("gcr.io")
		if got.HelperName != "gcloud" {
			t.Errorf("HelperName = %q, want gcloud", got.HelperName)
		}
	})
}

// A missing or corrupt config must degrade to anonymous, not fail.
func TestDockerConfigStoreToleratesMissingAndMalformedFiles(t *testing.T) {
	t.Run("missing file", func(t *testing.T) {
		got := LoadDockerConfig(filepath.Join(t.TempDir(), "nope.json")).Lookup("any.example")
		if got.Usable() || got.HelperName != "" {
			t.Errorf("got %+v, want zero value", got)
		}
	})

	t.Run("malformed json", func(t *testing.T) {
		path := writeDockerConfig(t, `{ this is not valid json `)
		got := LoadDockerConfig(path).Lookup("any.example")
		if got.Usable() {
			t.Errorf("got %+v, want zero value", got)
		}
	})

	t.Run("nil store", func(t *testing.T) {
		var store *DockerConfigStore
		if got := store.Lookup("any.example"); got.Usable() {
			t.Errorf("got %+v, want zero value", got)
		}
	})
}

func TestDefaultDockerConfigPathHonoursEnv(t *testing.T) {
	t.Setenv("DOCKER_CONFIG", "/custom/docker")
	if got, want := DefaultDockerConfigPath(), filepath.Join("/custom/docker", "config.json"); got != want {
		t.Errorf("DefaultDockerConfigPath() = %q, want %q", got, want)
	}
}

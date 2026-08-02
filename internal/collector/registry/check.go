package registry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// acceptManifests must list both the OCI index and the Docker manifest list.
// Sending only one silently misses images published in the other format.
const acceptManifests = "application/vnd.oci.image.index.v1+json," +
	"application/vnd.docker.distribution.manifest.list.v2+json," +
	"application/vnd.oci.image.manifest.v1+json," +
	"application/vnd.docker.distribution.manifest.v2+json"

// maxTokenResponseSize bounds the token endpoint response, which should be a
// small JSON object. Matches the defensive caps used elsewhere in collector.
const maxTokenResponseSize = 1 << 20 // 1 MB

// defaultConcurrency matches the stats worker pool in docker.go.
const defaultConcurrency = 5

// State classifies the outcome of a check.
type State int

const (
	// StateCurrent means the registry digest matches the local one.
	StateCurrent State = iota
	// StateAvailable means the registry has a different digest for this tag.
	StateAvailable
	// StateUnwatchable means the image cannot be checked at all: built
	// locally, pinned to a digest, or in a registry we have no access to.
	// This is a normal condition, not a failure.
	StateUnwatchable
	// StateError means the check itself failed — network, 5xx, malformed
	// response. Distinct from Unwatchable because it may succeed on retry.
	StateError
)

func (s State) String() string {
	switch s {
	case StateCurrent:
		return "current"
	case StateAvailable:
		return "available"
	case StateUnwatchable:
		return "unwatchable"
	default:
		return "error"
	}
}

// Image is one image to check.
type Image struct {
	// Ref is the image reference as Docker reports it.
	Ref string
	// LocalDigest is the digest portion of the image's RepoDigests entry.
	// Empty means Docker has no registry digest for it.
	LocalDigest string
}

// Status is the result of checking one image.
type Status struct {
	Ref          string
	State        State
	LocalDigest  string
	RemoteDigest string
	// Reason explains Unwatchable and Error states in human terms.
	Reason    string
	CheckedAt time.Time
}

// Checker checks images for updates.
type Checker interface {
	Check(ctx context.Context, images []Image) []Status
}

// HTTPChecker compares local and registry digests over the Registry v2 API.
type HTTPChecker struct {
	// Client issues the requests. Defaults to a 15s-timeout client.
	Client *http.Client
	// Credentials resolves per-registry logins. May be nil (anonymous only).
	Credentials CredentialStore
	// Concurrency caps in-flight checks. Defaults to defaultConcurrency.
	Concurrency int
	// BaseURL overrides the scheme+host for every request. Test hook only;
	// empty means derive https://<registry> from each reference.
	BaseURL string
	// Now supplies timestamps. Defaults to time.Now.
	Now func() time.Time
}

// NewHTTPChecker builds a checker using the user's Docker credentials.
func NewHTTPChecker() *HTTPChecker {
	return &HTTPChecker{
		Client:      &http.Client{Timeout: 15 * time.Second},
		Credentials: LoadDockerConfig(DefaultDockerConfigPath()),
		Concurrency: defaultConcurrency,
	}
}

func (c *HTTPChecker) client() *http.Client {
	if c.Client != nil {
		return c.Client
	}
	return &http.Client{Timeout: 15 * time.Second}
}

func (c *HTTPChecker) now() time.Time {
	if c.Now != nil {
		return c.Now()
	}
	return time.Now()
}

func (c *HTTPChecker) concurrency() int {
	if c.Concurrency > 0 {
		return c.Concurrency
	}
	return defaultConcurrency
}

// Check resolves every image concurrently, preserving input order. It never
// returns an error: a per-image failure is reported as StateError so one bad
// registry cannot hide the results for every other image.
func (c *HTTPChecker) Check(ctx context.Context, images []Image) []Status {
	results := make([]Status, len(images))
	if len(images) == 0 {
		return results
	}

	sem := make(chan struct{}, c.concurrency())
	var wg sync.WaitGroup

	for i, img := range images {
		wg.Add(1)
		go func(i int, img Image) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				results[i] = Status{
					Ref:       img.Ref,
					State:     StateError,
					Reason:    "check cancelled",
					CheckedAt: c.now(),
				}
				return
			}
			results[i] = c.checkOne(ctx, img)
		}(i, img)
	}

	wg.Wait()
	return results
}

// checkOne runs the full flow for a single image.
func (c *HTTPChecker) checkOne(ctx context.Context, img Image) Status {
	status := Status{
		Ref:         img.Ref,
		LocalDigest: img.LocalDigest,
		CheckedAt:   c.now(),
	}

	ref, err := ParseReference(img.Ref)
	if err != nil {
		status.State = StateUnwatchable
		status.Reason = err.Error()
		return status
	}

	// A digest-pinned image can never change under that reference.
	if ref.Digest != "" {
		status.State = StateUnwatchable
		status.Reason = "pinned to a digest; nothing to track"
		return status
	}

	// Without a local digest there is nothing to compare against. This is
	// what an image loaded from a tarball looks like. Note it is NOT a
	// reliable signal for locally-built images: with the containerd image
	// store those do get a RepoDigests entry.
	if img.LocalDigest == "" {
		status.State = StateUnwatchable
		status.Reason = "no registry digest recorded locally"
		return status
	}

	remote, err := c.remoteDigest(ctx, ref)
	if err != nil {
		var unwatchable *unwatchableError
		if errors.As(err, &unwatchable) {
			status.State = StateUnwatchable
			status.Reason = unwatchable.reason
			return status
		}
		status.State = StateError
		status.Reason = err.Error()
		return status
	}

	status.RemoteDigest = remote
	if remote == img.LocalDigest {
		status.State = StateCurrent
	} else {
		status.State = StateAvailable
	}
	return status
}

// unwatchableError marks a failure that means "cannot be tracked" rather than
// "try again later".
type unwatchableError struct {
	reason string
}

func (e *unwatchableError) Error() string { return e.reason }

// remoteDigest performs the manifest HEAD, handling the auth challenge.
func (c *HTTPChecker) remoteDigest(ctx context.Context, ref Reference) (string, error) {
	endpoint := fmt.Sprintf("%s/v2/%s/manifests/%s", c.baseURL(ref), ref.Repository, url.PathEscape(ref.Tag))

	resp, err := c.headManifest(ctx, endpoint, "")
	if err != nil {
		return "", err
	}

	// A 401 carries the challenge telling us where to get a token. This is
	// the normal path even for public Docker Hub images.
	if resp.StatusCode == http.StatusUnauthorized {
		authHeader := strings.TrimSpace(resp.Header.Get("WWW-Authenticate"))
		closeBody(resp)

		// A 401 with no challenge at all means the registry is refusing
		// without offering any way in — we cannot track this image. That is
		// different from a challenge we fail to parse, which is a misbehaving
		// registry and may work on a later attempt.
		if authHeader == "" {
			return "", &unwatchableError{
				reason: fmt.Sprintf("%s requires authentication but offered no method (HTTP 401)", ref.Registry),
			}
		}

		ch, err := parseChallenge(authHeader)
		if err != nil {
			return "", fmt.Errorf("registry %s: %w", ref.Registry, err)
		}

		token, err := c.fetchToken(ctx, ref, ch)
		if err != nil {
			return "", err
		}

		resp, err = c.headManifest(ctx, endpoint, token)
		if err != nil {
			return "", err
		}
	}
	defer closeBody(resp)

	switch resp.StatusCode {
	case http.StatusOK:
		digest := resp.Header.Get("Docker-Content-Digest")
		if digest == "" {
			return "", fmt.Errorf("registry %s returned no Docker-Content-Digest header", ref.Registry)
		}
		return digest, nil

	case http.StatusNotFound, http.StatusUnauthorized, http.StatusForbidden:
		// Docker Hub returns 401 for repositories that do not exist, so a
		// missing repo and a private one are indistinguishable here. Both
		// mean the same thing to us: we cannot track this image.
		return "", &unwatchableError{
			reason: fmt.Sprintf("not found in %s, or no access (HTTP %d)", ref.Registry, resp.StatusCode),
		}

	default:
		return "", fmt.Errorf("registry %s returned HTTP %d", ref.Registry, resp.StatusCode)
	}
}

func (c *HTTPChecker) baseURL(ref Reference) string {
	if c.BaseURL != "" {
		return strings.TrimSuffix(c.BaseURL, "/")
	}
	return "https://" + ref.Registry
}

func (c *HTTPChecker) headManifest(ctx context.Context, endpoint, token string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("build manifest request: %w", err)
	}
	req.Header.Set("Accept", acceptManifests)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := c.client().Do(req)
	if err != nil {
		return nil, fmt.Errorf("query manifest: %w", err)
	}
	return resp, nil
}

// tokenResponse covers both spellings registries use for the field.
type tokenResponse struct {
	Token       string `json:"token"`
	AccessToken string `json:"access_token"`
}

func (t tokenResponse) value() string {
	if t.Token != "" {
		return t.Token
	}
	return t.AccessToken
}

// fetchToken exchanges the challenge for a bearer token, sending stored
// credentials as HTTP Basic when they exist.
func (c *HTTPChecker) fetchToken(ctx context.Context, ref Reference, ch challenge) (string, error) {
	tokenURL, err := url.Parse(ch.Realm)
	if err != nil {
		return "", fmt.Errorf("invalid token realm %q: %w", ch.Realm, err)
	}

	query := tokenURL.Query()
	if ch.Service != "" {
		query.Set("service", ch.Service)
	}
	query.Set("scope", fmt.Sprintf("repository:%s:pull", ref.Repository))
	tokenURL.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, tokenURL.String(), nil)
	if err != nil {
		return "", fmt.Errorf("build token request: %w", err)
	}

	if c.Credentials != nil {
		creds := c.Credentials.Lookup(ref.Registry)
		switch {
		case creds.Usable():
			req.SetBasicAuth(creds.Username, creds.Password)
		case creds.HelperName != "":
			// The credential lives in an external helper binary. HomeDash
			// does not spawn subprocesses, so this cannot be used. Say so
			// rather than reporting a misleading "no update available".
			return "", &unwatchableError{
				reason: fmt.Sprintf("credentials for %s are held by credential helper %q, which HomeDash cannot invoke",
					ref.Registry, creds.HelperName),
			}
		}
	}

	resp, err := c.client().Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch token: %w", err)
	}
	defer closeBody(resp)

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return "", &unwatchableError{
			reason: fmt.Sprintf("not authorised for %s (HTTP %d); log in with `docker login %s`",
				ref.Registry, resp.StatusCode, ref.Registry),
		}
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token endpoint returned HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxTokenResponseSize))
	if err != nil {
		return "", fmt.Errorf("read token response: %w", err)
	}

	var parsed tokenResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("parse token response: %w", err)
	}
	if parsed.value() == "" {
		return "", fmt.Errorf("token endpoint returned no token")
	}
	return parsed.value(), nil
}

func closeBody(resp *http.Response) {
	if resp != nil && resp.Body != nil {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxTokenResponseSize))
		_ = resp.Body.Close()
	}
}

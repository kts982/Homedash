package collector

import "testing"

func TestParseImageDigestsKeysByCanonicalReference(t *testing.T) {
	digests := parseImageDigests([]dockerImage{
		{RepoTags: []string{"postgres:latest"}, RepoDigests: []string{"postgres@sha256:aaa"}},
		{RepoTags: []string{"nginx:alpine"}, RepoDigests: []string{"nginx@sha256:bbb"}},
		{RepoTags: []string{"<none>:<none>"}, RepoDigests: []string{"nginx@sha256:ccc"}},
		{RepoTags: []string{"loaded:tar"}},
	})

	// The container side may spell the same image several ways.
	for _, image := range []string{"postgres", "postgres:latest", "docker.io/library/postgres", "index.docker.io/postgres:latest"} {
		if got := digests[canonicalRef(image)]; got != "sha256:aaa" {
			t.Errorf("digest for %q = %q, want sha256:aaa", image, got)
		}
	}
	if got := digests[canonicalRef("docker.io/library/nginx:alpine")]; got != "sha256:bbb" {
		t.Errorf("nginx digest = %q, want sha256:bbb", got)
	}
	if got, ok := digests[canonicalRef("loaded:tar")]; !ok || got != "" {
		t.Errorf("image without RepoDigests: got %q (present=%v), want present and empty", got, ok)
	}
	if len(digests) != 3 {
		t.Errorf("len(digests) = %d, want 3 (<none> tags are skipped)", len(digests))
	}
}

func TestDigestForTagMatchesRepository(t *testing.T) {
	repoDigests := []string{
		"ghcr.io/me/app@sha256:from-ghcr",
		"git.example.de/me/app@sha256:from-forgejo",
	}
	if got := digestForTag("git.example.de/me/app:latest", repoDigests); got != "sha256:from-forgejo" {
		t.Errorf("forgejo tag matched %q, want the forgejo digest", got)
	}
	if got := digestForTag("ghcr.io/me/app:latest", repoDigests); got != "sha256:from-ghcr" {
		t.Errorf("ghcr tag matched %q, want the ghcr digest", got)
	}
	// No repository match falls back to the first entry rather than nothing.
	if got := digestForTag("other.example/app:1", repoDigests); got != "sha256:from-ghcr" {
		t.Errorf("unmatched tag = %q, want first entry", got)
	}
}

func TestBuildUpdateTargetsDedupesAndNormalises(t *testing.T) {
	digests := map[string]string{canonicalRef("nginx:alpine"): "sha256:bbb"}
	targets := buildUpdateTargets([]Container{
		{Image: "nginx:alpine"},
		{Image: "docker.io/library/nginx:alpine"},
		{Image: "nginx:alpine"},
		{Image: ""},
	}, digests)

	if len(targets) != 2 {
		t.Fatalf("len(targets) = %d, want 2 (dedupe by raw ref, skip empty)", len(targets))
	}
	for _, target := range targets {
		if target.LocalDigest != "sha256:bbb" {
			t.Errorf("target %q LocalDigest = %q, want sha256:bbb", target.Ref, target.LocalDigest)
		}
	}
}

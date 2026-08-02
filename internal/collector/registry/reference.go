// Package registry checks whether the image a container runs has been
// superseded in its registry.
//
// It compares digests, not tags: for the tag already in use, it asks the
// registry what manifest that tag currently points at and compares it with the
// digest recorded locally. That covers moving tags like :latest, :15 and
// :abap, which is what self-hosted deployments actually pin to. It does not
// try to discover that a *newer tag* exists — tag naming is unstandardised,
// so that needs per-image configuration to avoid false positives.
package registry

import (
	"fmt"
	"strings"
)

// defaultRegistry is where a bare image name resolves, matching Docker's own
// reference resolution.
const defaultRegistry = "registry-1.docker.io"

// defaultNamespace prefixes single-segment Docker Hub names: "nginx" is
// really "library/nginx".
const defaultNamespace = "library"

// Reference is a parsed image reference, split into the pieces the registry
// API needs.
type Reference struct {
	// Registry is the host to query, e.g. "registry-1.docker.io".
	Registry string
	// Repository is the path within that host, e.g. "library/nginx".
	Repository string
	// Tag is the tag being tracked, e.g. "alpine".
	Tag string
	// Digest is set when the reference pinned a digest instead of a tag.
	// Such an image can never be updated in place, so it is unwatchable.
	Digest string
}

// String rebuilds a canonical reference, mainly for error messages.
func (r Reference) String() string {
	if r.Digest != "" {
		return fmt.Sprintf("%s/%s@%s", r.Registry, r.Repository, r.Digest)
	}
	return fmt.Sprintf("%s/%s:%s", r.Registry, r.Repository, r.Tag)
}

// ParseReference splits a Docker image reference into registry, repository and
// tag, applying Docker's defaulting rules.
//
// The tricky part is deciding whether the first path segment is a registry
// host or a namespace: "kostas/app" means Docker Hub user kostas, while
// "git.example.de/kostas/app" means a private registry. Docker's rule is that
// the first segment is a host only if it contains a dot or colon, or is
// exactly "localhost".
func ParseReference(image string) (Reference, error) {
	image = strings.TrimSpace(image)
	if image == "" {
		return Reference{}, fmt.Errorf("empty image reference")
	}

	remainder := image
	var ref Reference

	// A digest pin takes precedence over any tag.
	if at := strings.LastIndex(remainder, "@"); at >= 0 {
		ref.Digest = remainder[at+1:]
		remainder = remainder[:at]
		if ref.Digest == "" {
			return Reference{}, fmt.Errorf("image %q has an empty digest", image)
		}
	}

	// Split off the registry host, if the first segment looks like one.
	name := remainder
	if slash := strings.Index(remainder, "/"); slash >= 0 {
		candidate := remainder[:slash]
		if isRegistryHost(candidate) {
			ref.Registry = candidate
			name = remainder[slash+1:]
		}
	}
	if ref.Registry == "" {
		ref.Registry = defaultRegistry
	}

	// A colon after the last slash is a tag; before it, it is a host port.
	if colon := strings.LastIndex(name, ":"); colon >= 0 && !strings.Contains(name[colon:], "/") {
		ref.Tag = name[colon+1:]
		name = name[:colon]
		if ref.Tag == "" {
			return Reference{}, fmt.Errorf("image %q has an empty tag", image)
		}
	}

	if name == "" {
		return Reference{}, fmt.Errorf("image %q has no repository", image)
	}

	// Single-segment Docker Hub names live under library/.
	if ref.Registry == defaultRegistry && !strings.Contains(name, "/") {
		name = defaultNamespace + "/" + name
	}
	ref.Repository = name

	if ref.Tag == "" && ref.Digest == "" {
		ref.Tag = "latest"
	}
	return ref, nil
}

// isRegistryHost reports whether segment is a registry host rather than a
// Docker Hub namespace. This is Docker's own heuristic.
func isRegistryHost(segment string) bool {
	return segment == "localhost" ||
		strings.Contains(segment, ".") ||
		strings.Contains(segment, ":")
}

// DigestOf extracts the digest from a RepoDigests entry such as
// "nginx@sha256:abc…". Returns "" if the entry carries no digest.
func DigestOf(repoDigest string) string {
	if at := strings.LastIndex(repoDigest, "@"); at >= 0 {
		return repoDigest[at+1:]
	}
	return ""
}

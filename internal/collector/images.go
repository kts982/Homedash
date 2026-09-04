package collector

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/kts982/homedash/internal/collector/registry"
)

// dockerImage is the subset of GET /images/json that matters for update
// checks. RepoDigests is what the registry check compares against; RepoTags
// is how a container's Image field is matched back to an image record.
type dockerImage struct {
	RepoTags    []string `json:"RepoTags"`
	RepoDigests []string `json:"RepoDigests"`
}

// UpdateTargets builds the list of images to check from the running
// containers, de-duplicated by reference.
//
// A single GET /images/json supplies every local digest, so this costs one
// request regardless of container count. Containers sharing an image (as two
// stacks running nginx:alpine would) produce a single target.
func UpdateTargets(containers []Container) ([]registry.Image, error) {
	digests, err := imageDigests()
	if err != nil {
		return nil, err
	}

	return buildUpdateTargets(containers, digests), nil
}

func buildUpdateTargets(containers []Container, digests map[string]string) []registry.Image {
	seen := make(map[string]bool, len(containers))
	targets := make([]registry.Image, 0, len(containers))
	for _, c := range containers {
		ref := strings.TrimSpace(c.Image)
		if ref == "" || seen[ref] {
			continue
		}
		seen[ref] = true
		targets = append(targets, registry.Image{
			Ref:         ref,
			LocalDigest: digests[canonicalRef(ref)],
		})
	}
	return targets
}

// canonicalRef normalises an image reference so that the string a container
// was started with and the short form Docker records in RepoTags key the same
// digest: compose's `image: postgres` becomes RepoTags "postgres:latest", and
// "docker.io/library/nginx:alpine" becomes "nginx:alpine". Unparseable input
// is returned trimmed so it still keys consistently on both sides.
func canonicalRef(image string) string {
	ref, err := registry.ParseReference(image)
	if err != nil {
		return strings.TrimSpace(image)
	}
	return ref.String()
}

// imageDigests maps an image reference to its local registry digest.
//
// The digest is deliberately keyed by RepoTags rather than by image ID: a
// container's Image field is a tag, and the same image ID can carry several
// tags. An image with no RepoDigests entry maps to "", which the checker
// treats as unwatchable.
func imageDigests() (map[string]string, error) {
	resp, err := dockerClient.Get(dockerURL(fmt.Sprintf("/%s/images/json", dockerAPIVersion)))
	if err != nil {
		return nil, fmt.Errorf("docker images: %w", err)
	}
	defer closeQuietly(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("docker images: status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return nil, fmt.Errorf("docker images read: %w", err)
	}

	var images []dockerImage
	if err := json.Unmarshal(body, &images); err != nil {
		return nil, fmt.Errorf("docker images parse: %w", err)
	}
	return parseImageDigests(images), nil
}

// parseImageDigests maps each canonical tag to the registry digest recorded
// for it. An image pushed to two registries carries two RepoDigests entries
// whose digests can legitimately differ, so the entry is matched to the tag
// by repository rather than taking the first one.
func parseImageDigests(images []dockerImage) map[string]string {
	digests := make(map[string]string, len(images))
	for _, img := range images {
		for _, tag := range img.RepoTags {
			if tag == "" || tag == "<none>:<none>" {
				continue
			}
			digests[canonicalRef(tag)] = digestForTag(tag, img.RepoDigests)
		}
	}
	return digests
}

func digestForTag(tag string, repoDigests []string) string {
	if len(repoDigests) == 0 {
		return ""
	}
	if tagRef, err := registry.ParseReference(tag); err == nil {
		for _, entry := range repoDigests {
			ref, err := registry.ParseReference(entry)
			if err == nil && ref.Registry == tagRef.Registry && ref.Repository == tagRef.Repository && ref.Digest != "" {
				return ref.Digest
			}
		}
	}
	return registry.DigestOf(repoDigests[0])
}

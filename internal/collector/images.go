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
			LocalDigest: digests[ref],
		})
	}
	return targets, nil
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

	digests := make(map[string]string, len(images))
	for _, img := range images {
		digest := ""
		if len(img.RepoDigests) > 0 {
			digest = registry.DigestOf(img.RepoDigests[0])
		}
		for _, tag := range img.RepoTags {
			if tag != "" && tag != "<none>:<none>" {
				digests[tag] = digest
			}
		}
	}
	return digests, nil
}

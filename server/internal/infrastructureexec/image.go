package infrastructureexec

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/distribution/reference"
)

var digestPattern = regexp.MustCompile(`sha256:[a-f0-9]{64}`)
var pullDigestPattern = regexp.MustCompile(`(?m)^Digest: (sha256:[a-f0-9]{64})\r?$`)

type inspectedManifest struct {
	Descriptor       struct{ Digest string }
	SchemaV2Manifest struct{ Config struct{ Digest string } }
	OCIManifest      struct{ Config struct{ Digest string } }
}

type ResolvedImage struct {
	Source  string   `json:"source"`
	Pinned  string   `json:"pinned"`
	Digest  string   `json:"digest"`
	Digests []string `json:"verification_digests"`
}

func ParseImage(source string) (reference.Named, error) {
	if len(source) > 1024 || source != strings.TrimSpace(source) {
		return nil, errors.New("source image must be a complete registry image reference")
	}
	ref, err := reference.ParseNormalizedNamed(source)
	if err != nil || strings.Contains(source, "://") {
		return nil, errors.New("source image must be a valid registry image reference")
	}
	first, _, hasSlash := strings.Cut(source, "/")
	if !hasSlash || (!strings.ContainsAny(first, ".:") && first != "localhost") {
		return nil, errors.New("source image must include the registry hostname")
	}
	return ref, nil
}

func manifestDigests(data []byte, root string) ([]string, error) {
	var values []inspectedManifest
	if strings.HasPrefix(strings.TrimSpace(string(data)), "[") {
		if err := json.Unmarshal(data, &values); err != nil {
			return nil, errors.New("Docker returned an invalid image manifest list")
		}
	} else {
		var value inspectedManifest
		if err := json.Unmarshal(data, &value); err != nil {
			return nil, errors.New("Docker returned an invalid image manifest")
		}
		values = append(values, value)
	}
	accepted := map[string]bool{root: true}
	valid := false
	for _, value := range values {
		for _, candidate := range []string{value.Descriptor.Digest, value.SchemaV2Manifest.Config.Digest, value.OCIManifest.Config.Digest} {
			if digestPattern.FindString(candidate) == candidate && len(candidate) == 71 {
				accepted[candidate] = true
				valid = true
			}
		}
	}
	if !valid {
		return nil, errors.New("Docker manifest did not identify the image content")
	}
	result := make([]string, 0, len(accepted))
	for value := range accepted {
		result = append(result, value)
	}
	sort.Strings(result)
	return result, nil
}

func inspectLocalPinnedImage(ctx context.Context, runner Runner, pinned, digest string) ([]string, error) {
	// Docker 17 does not provide `manifest inspect`. Pull the immutable
	// reference before associating its repository digest with the local image ID.
	if _, err := runner.Run(ctx, "docker", "pull", pinned); err != nil {
		return nil, fmt.Errorf("pull pinned image for inspection: %w", err)
	}
	data, err := runner.Run(ctx, "docker", "image", "inspect", "--format", `{"Id":{{json .Id}},"RepoDigests":{{json .RepoDigests}}}`, pinned)
	if err != nil {
		return nil, fmt.Errorf("inspect pinned local image: %w", err)
	}
	var image struct {
		ID          string `json:"Id"`
		RepoDigests []string
	}
	if json.Unmarshal(data, &image) != nil || len(image.ID) != 71 || digestPattern.FindString(image.ID) != image.ID {
		return nil, errors.New("Docker did not identify the pinned local image content")
	}
	matched := false
	for _, candidate := range image.RepoDigests {
		ref, err := reference.ParseNormalizedNamed(candidate)
		if err == nil && ref.String() == pinned {
			matched = true
			break
		}
	}
	if !matched {
		return nil, errors.New("Docker local image does not match the pinned repository digest")
	}
	digests := []string{digest}
	if image.ID != digest {
		digests = append(digests, image.ID)
	}
	sort.Strings(digests)
	return digests, nil
}

func ResolveImage(ctx context.Context, runner Runner, source string) (ResolvedImage, error) {
	ref, err := ParseImage(source)
	if err != nil {
		return ResolvedImage{}, err
	}
	var digest string
	if canonical, ok := ref.(reference.Canonical); ok {
		digest = canonical.Digest().String()
	} else {
		output, err := runner.Run(ctx, "docker", "pull", ref.String())
		if err != nil {
			return ResolvedImage{}, fmt.Errorf("pull source image: %w", err)
		}
		match := pullDigestPattern.FindSubmatch(output)
		if len(match) != 2 {
			return ResolvedImage{}, errors.New("Docker pull did not return an immutable image digest")
		}
		digest = string(match[1])
	}
	if digestPattern.FindString(digest) != digest || len(digest) != 71 {
		return ResolvedImage{}, errors.New("only sha256 image manifests are supported")
	}
	pinned := reference.TrimNamed(ref).Name() + "@" + digest
	manifest, err := runner.Run(ctx, "docker", "manifest", "inspect", "--verbose", pinned)
	if err != nil {
		message := err.Error()
		if !strings.Contains(message, "unknown flag: --verbose") && !strings.Contains(message, "'manifest' is not a docker command") && !strings.Contains(message, `unknown command "manifest"`) {
			return ResolvedImage{}, fmt.Errorf("inspect pinned image manifest: %w", err)
		}
		digests, err := inspectLocalPinnedImage(ctx, runner, pinned, digest)
		if err != nil {
			return ResolvedImage{}, err
		}
		return ResolvedImage{Source: source, Pinned: pinned, Digest: digest, Digests: digests}, nil
	}
	digests, err := manifestDigests(manifest, digest)
	return ResolvedImage{Source: source, Pinned: pinned, Digest: digest, Digests: digests}, err
}

package core

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/chenhongyang/novel-studio/internal/domain"
	"github.com/chenhongyang/novel-studio/internal/protocol"
)

var sha256HexPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

func designArtifactRef(artifactType, digest string) string {
	return artifactType + "@sha256:" + digest
}

func designBundleRef(digest string) string {
	return "design_bundle@sha256:" + digest
}

func canonicalDesignArtifact(raw []byte) (domain.CoreDesignArtifact, []byte, string, error) {
	var artifact domain.CoreDesignArtifact
	if err := protocol.DecodeJSON(raw, &artifact); err != nil {
		return artifact, nil, "", err
	}
	artifact.ArtifactType = strings.TrimSpace(artifact.ArtifactType)
	if artifact.SchemaVersion != 1 || artifact.ArtifactType == "" || artifact.Payload == nil {
		return artifact, nil, "", fmt.Errorf("invalid design artifact envelope")
	}
	sort.Strings(artifact.Inputs)
	sort.Strings(artifact.Sources)
	if hasDuplicateStrings(artifact.Inputs) || hasDuplicateStrings(artifact.Sources) {
		return artifact, nil, "", fmt.Errorf("design refs must be unique")
	}
	canonical, err := json.Marshal(artifact)
	if err != nil {
		return artifact, nil, "", err
	}
	digest := sha256Bytes(canonical)
	return artifact, canonical, designArtifactRef(artifact.ArtifactType, digest), nil
}

func canonicalDesignBundle(raw []byte) (domain.CoreDesignBundle, []byte, string, error) {
	var bundle domain.CoreDesignBundle
	if err := protocol.DecodeJSON(raw, &bundle); err != nil {
		return bundle, nil, "", err
	}
	if bundle.SchemaVersion != 1 || len(bundle.Selections) == 0 {
		return bundle, nil, "", fmt.Errorf("invalid design bundle")
	}
	canonical, err := json.Marshal(bundle)
	if err != nil {
		return bundle, nil, "", err
	}
	digest := sha256Bytes(canonical)
	return bundle, canonical, designBundleRef(digest), nil
}

func parseDesignArtifactRef(ref string) (artifactType string, digest string, err error) {
	artifactType, digest, ok := strings.Cut(ref, "@sha256:")
	if !ok || artifactType == "" || !sha256HexPattern.MatchString(digest) {
		return "", "", fmt.Errorf("invalid design artifact ref %q", ref)
	}
	return artifactType, digest, nil
}

func parseDesignBundleRef(ref string) (digest string, err error) {
	const prefix = "design_bundle@sha256:"
	if !strings.HasPrefix(ref, prefix) {
		return "", fmt.Errorf("invalid design bundle ref %q", ref)
	}
	digest = strings.TrimPrefix(ref, prefix)
	if !sha256HexPattern.MatchString(digest) {
		return "", fmt.Errorf("invalid design bundle ref %q", ref)
	}
	return digest, nil
}

func hasDuplicateStrings(items []string) bool {
	seen := make(map[string]bool, len(items))
	for _, item := range items {
		if seen[item] {
			return true
		}
		seen[item] = true
	}
	return false
}

var foundationDesignSlots = map[string]string{
	"foundation":       "foundation.json",
	"characters":       "characters.json",
	"world":            "world.json",
	"book_plan":        "book_plan.json",
	"ending_contract":  "ending_contract.json",
	"style_profile":    "style_profile.json",
	"platform_profile": "platform_profile.json",
}

func (p *Project) foundationArtifactsFromDesignBundle(
	bundle domain.CoreDesignBundle,
) (map[string][]byte, error) {
	out := make(map[string][]byte, len(foundationDesignSlots))
	for slot, fileName := range foundationDesignSlots {
		ref, ok := bundle.Selections[slot]
		if !ok {
			return nil, fmt.Errorf("design bundle is missing %s", slot)
		}
		artifact, err := p.loadDesignArtifactRef(ref)
		if err != nil {
			return nil, err
		}
		if artifact.ArtifactType != slot {
			return nil, fmt.Errorf("%s selects artifact type %s", slot, artifact.ArtifactType)
		}
		raw, err := json.Marshal(artifact.Payload)
		if err != nil {
			return nil, err
		}
		out[fileName] = raw
	}
	return out, nil
}

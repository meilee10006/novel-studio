package core

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/chenhongyang/novel-studio/internal/protocol"
)

const maxInboxManifestBytes = 64 << 10

type stableObservation struct {
	ObservedDigest string
	ObservedAt     string
	SnapshotDigest string
}

func readInboxCandidate(workspaceRoot, baseRel string, fileNames []string, maxTotal int) (map[string][]byte, string, error) {
	if maxTotal <= 0 {
		return nil, "", fmt.Errorf("max total size must be positive")
	}
	for _, name := range fileNames {
		if name == "" || name == "." || name == ".." || name == "manifest.json" || filepath.Base(name) != name {
			return nil, "", fmt.Errorf("submission file list must contain basenames")
		}
	}
	inbox := filepath.Join(workspaceRoot, baseRel)
	if problem := validateInboxEntries(inbox, fileNames); problem != "" {
		return nil, "", fmt.Errorf("%s", problem)
	}

	files := make(map[string][]byte, len(fileNames)+1)
	manifestRel := filepath.Join(baseRel, "manifest.json")
	manifestRaw, err := protocol.ReadUTF8(workspaceRoot, manifestRel, maxInboxManifestBytes)
	if err != nil {
		return nil, "", err
	}
	files["manifest.json"] = manifestRaw
	total := len(manifestRaw)
	for _, name := range fileNames {
		rel := filepath.Join(baseRel, name)
		data, err := protocol.ReadUTF8(workspaceRoot, rel, protocol.DefaultMaxTextSize)
		if err != nil {
			return nil, "", err
		}
		total += len(data)
		if total > maxTotal {
			return nil, "", fmt.Errorf("submission exceeds max total size")
		}
		files[name] = data
	}
	scanDigest, err := digestArtifactManifest(digestArtifacts(files))
	if err != nil {
		return nil, "", err
	}
	return files, scanDigest, nil
}

func advanceStableObservation(current stableObservation, scanDigest string, now time.Time, quiet time.Duration) (stableObservation, string) {
	if current.SnapshotDigest != "" {
		if scanDigest == current.SnapshotDigest {
			return current, "LOCKED_SAME"
		}
		return current, "LOCKED_CONFLICT"
	}
	if current.ObservedDigest != scanDigest {
		current.ObservedDigest = scanDigest
		current.ObservedAt = now.UTC().Format(time.RFC3339Nano)
		return current, "PENDING"
	}
	observedAt, err := time.Parse(time.RFC3339Nano, current.ObservedAt)
	if err == nil && now.UTC().Sub(observedAt) < quiet {
		return current, "PENDING"
	}
	return current, "READY_TO_SNAPSHOT"
}

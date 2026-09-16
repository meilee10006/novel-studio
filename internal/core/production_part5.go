package core

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/chenhongyang/novel-studio/internal/domain"
	"github.com/chenhongyang/novel-studio/internal/protocol"
	"path/filepath"
	"sort"
	"time"
)

func (p *Project) acceptFoundation(project *domain.CoreProjectState, state *domain.CoreProductionState, task *domain.CoreTask, attempt *domain.CoreAttempt, sub FoundationSubmission, artifacts map[string][]byte, mappings []IDMapping, longform domain.CoreLongformState, planning domain.CorePlanningState) (FoundationSettlement, error) {
	artifactDigests := digestArtifacts(artifacts)
	submissionDigest, err := digestArtifactManifest(artifactDigests)
	if err != nil {
		return FoundationSettlement{}, err
	}
	newRevision := state.Revision + 1
	canonState := &domain.CoreCanonState{SchemaVersion: coreSchemaVersion, Revision: newRevision, ProjectID: project.ProjectID, LastTaskID: task.TaskID, LastAttemptID: attempt.AttemptID, Longform: longform, Planning: planning}
	stateDigest, err := digestJSON(canonState)
	if err != nil {
		return FoundationSettlement{}, err
	}
	newRoot, err := computeCanonRoot(newRevision, state.CanonRoot, stateDigest, artifactDigests)
	if err != nil {
		return FoundationSettlement{}, err
	}
	head := &domain.CoreCanonHead{SchemaVersion: coreSchemaVersion, Revision: newRevision, ParentRoot: state.CanonRoot, Root: newRoot, StateDigest: stateDigest, ArtifactDigests: artifactDigests}
	if err := p.store.SaveCoreCanon(canonState, head, artifacts); err != nil {
		return FoundationSettlement{}, err
	}
	validationDigest, _ := digestJSON(map[string]any{"result": "ACCEPTED", "violations": []string{}})
	receipt := &domain.CoreReceipt{
		SchemaVersion: coreSchemaVersion, ProjectID: project.ProjectID,
		TaskID: task.TaskID, AttemptID: attempt.AttemptID, PreviousRoot: state.CanonRoot,
		TaskDigest: attempt.TaskDigest, SubmissionDigest: submissionDigest, ArtifactDigests: artifactDigests,
		ValidationDigest: validationDigest, Result: "ACCEPTED", NewRoot: newRoot,
		IDMappings: mappings, CommittedAt: time.Now().UTC().Format(time.RFC3339Nano),
	}
	rel, err := p.store.SaveCoreReceipt(receipt)
	if err != nil {
		return FoundationSettlement{}, err
	}
	state.Revision, state.CanonRoot = newRevision, newRoot
	state.NextEntitySeq += len(mappings)
	chapterTask := newTask(state, "chapter", "chapter:1", newRoot)
	addRollingPlanningObligation(chapterTask, planning, 1)
	chapterAttempt, err := newAttempt(state, chapterTask, "initial", chapterArtifactNames, project.ProtocolVersion)
	if err != nil {
		return FoundationSettlement{}, err
	}
	state.ActiveTask, state.ActiveAttempt = chapterTask, chapterAttempt
	if err := p.store.SaveCoreProductionState(state); err != nil {
		return FoundationSettlement{}, err
	}
	if err := writeWorkspaceJSON(project.WorkspaceRoot, filepath.Join("exchange", "result", attempt.AttemptID+".json"), map[string]any{
		"schema_version": protocol.MachineSchemaVersion, "task_id": task.TaskID,
		"attempt_id": attempt.AttemptID, "result": "ACCEPTED", "new_canon_root": newRoot, "id_mappings": mappings,
	}); err != nil {
		return FoundationSettlement{}, err
	}
	if err := p.writeActiveAttempt(project, state); err != nil {
		return FoundationSettlement{}, err
	}
	return FoundationSettlement{Result: "ACCEPTED", NewCanonRoot: newRoot, IDMappings: mappings, ReceiptPath: filepath.Join(p.root, rel)}, nil
}
func digestArtifacts(artifacts map[string][]byte) map[string]string {
	out := make(map[string]string, len(artifacts))
	for name, data := range artifacts {
		out[name] = sha256Bytes(data)
	}
	return out
}
func digestArtifactManifest(digests map[string]string) (string, error) {
	type item struct {
		Path   string `json:"path"`
		Digest string `json:"digest"`
	}
	items := make([]item, 0, len(digests))
	for path, digest := range digests {
		items = append(items, item{path, digest})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Path < items[j].Path })
	return digestJSON(items)
}
func computeCanonRoot(revision int, parentRoot, stateDigest string, artifactDigests map[string]string) (string, error) {
	return digestJSON(struct {
		SchemaVersion   int               `json:"schema_version"`
		Revision        int               `json:"revision"`
		ParentRoot      string            `json:"parent_root"`
		StateDigest     string            `json:"state_digest"`
		ArtifactDigests map[string]string `json:"artifact_digests"`
	}{coreSchemaVersion, revision, parentRoot, stateDigest, artifactDigests})
}
func digestJSON(v any) (string, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	canonical, err := canonicalJSON(data)
	if err != nil {
		return "", err
	}
	return sha256Bytes(canonical), nil
}
func sha256Bytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
func (p *Project) RecomputeCanonRoot() (string, error) {
	head, err := p.store.LoadCoreCanonHead()
	if err != nil {
		return "", err
	}
	if head == nil {
		return "", fmt.Errorf("canon head does not exist")
	}
	stateRaw, err := p.store.ReadCoreCanonStateBytes()
	if err != nil {
		return "", err
	}
	stateCanonical, err := canonicalJSON(stateRaw)
	if err != nil {
		return "", err
	}
	artifactDigests := make(map[string]string, len(head.ArtifactDigests))
	for name := range head.ArtifactDigests {
		raw, err := p.store.ReadCoreCanonArtifact(name)
		if err != nil {
			return "", err
		}
		digest, err := digestCanonArtifact(name, raw)
		if err != nil {
			return "", err
		}
		artifactDigests[name] = digest
	}
	return computeCanonRoot(head.Revision, head.ParentRoot, sha256Bytes(stateCanonical), artifactDigests)
}
func canonicalJSON(raw []byte) ([]byte, error) {
	var value any
	if err := protocol.DecodeJSON(raw, &value); err != nil {
		return nil, err
	}
	return json.Marshal(value)
}

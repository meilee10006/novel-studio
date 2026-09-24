package core

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"time"

	"github.com/chenhongyang/novel-studio/internal/domain"
	"github.com/chenhongyang/novel-studio/internal/protocol"
)

func (p *Project) prepareFoundationCommit(
	project *domain.CoreProjectState,
	state *domain.CoreProductionState,
	task *domain.CoreTask,
	attempt *domain.CoreAttempt,
	prepared preparedFoundation,
	foundationDesignRoot string,
) (*domain.CoreCommitJournal, error) {
	if project == nil || state == nil || task == nil || attempt == nil {
		return nil, fmt.Errorf("foundation commit inputs are required")
	}
	artifactDigests := digestArtifacts(prepared.Canonical)
	submissionDigest, err := digestArtifactManifest(artifactDigests)
	if err != nil {
		return nil, err
	}
	newRevision := state.Revision + 1
	canonState := domain.CoreCanonState{
		SchemaVersion: coreSchemaVersion,
		Revision:      newRevision,
		ProjectID:     project.ProjectID,
		LastTaskID:    task.TaskID,
		LastAttemptID: attempt.AttemptID,
		Longform:      prepared.Longform,
		Planning:      prepared.Planning,
	}
	stateDigest, err := digestJSON(canonState)
	if err != nil {
		return nil, err
	}
	newRoot, err := computeCanonRoot(newRevision, state.CanonRoot, stateDigest, artifactDigests)
	if err != nil {
		return nil, err
	}
	canonHead := domain.CoreCanonHead{
		SchemaVersion:   coreSchemaVersion,
		Revision:        newRevision,
		ParentRoot:      state.CanonRoot,
		Root:            newRoot,
		StateDigest:     stateDigest,
		ArtifactDigests: artifactDigests,
	}
	validationDigest, err := digestJSON(map[string]any{
		"result":     "ACCEPTED",
		"violations": []string{},
	})
	if err != nil {
		return nil, err
	}
	receipt := domain.CoreReceipt{
		SchemaVersion:        coreSchemaVersion,
		ProjectID:            project.ProjectID,
		FoundationDesignRoot: foundationDesignRoot,
		TaskID:               task.TaskID,
		AttemptID:            attempt.AttemptID,
		PreviousRoot:         state.CanonRoot,
		TaskDigest:           attempt.TaskDigest,
		SubmissionDigest:     submissionDigest,
		ArtifactDigests:      artifactDigests,
		ValidationDigest:     validationDigest,
		Result:               "ACCEPTED",
		NewRoot:              newRoot,
		IDMappings:           prepared.Mappings,
		CommittedAt:          time.Now().UTC().Format(time.RFC3339Nano),
	}

	next := *state
	next.Revision = newRevision
	next.CanonRoot = newRoot
	next.NextEntitySeq += len(prepared.Mappings)
	chapterTask := newTask(&next, "chapter", "chapter:1", newRoot)
	addRollingPlanningObligation(chapterTask, prepared.Planning, 1)
	chapterAttempt, err := newAttempt(
		&next,
		chapterTask,
		"initial",
		chapterArtifactNames,
		project.ProtocolVersion,
	)
	if err != nil {
		return nil, err
	}
	next.ActiveTask = chapterTask
	next.ActiveAttempt = chapterAttempt

	artifactNames := append([]string(nil), foundationArtifactNames...)
	sort.Strings(artifactNames)
	journal := &domain.CoreCommitJournal{
		SchemaVersion: coreSchemaVersion,
		State:         "prepared",
		Kind:          "foundation",
		TaskID:        task.TaskID,
		AttemptID:     attempt.AttemptID,
		PreviousRoot:  state.CanonRoot,
		NewRoot:       newRoot,
		ArtifactNames: artifactNames,
		CanonState:    canonState,
		CanonHead:     canonHead,
		Receipt:       receipt,
		NextState:     next,
	}
	if err := p.store.SaveCorePreparedArtifacts(attempt.AttemptID, prepared.Canonical); err != nil {
		return nil, err
	}
	if err := p.store.SaveCoreCommitJournal(journal); err != nil {
		return nil, err
	}
	return journal, nil
}

func (p *Project) applyFoundationCommit(
	project *domain.CoreProjectState,
	journal *domain.CoreCommitJournal,
) (FoundationSettlement, error) {
	if project == nil || journal == nil {
		return FoundationSettlement{}, fmt.Errorf("foundation commit journal is required")
	}
	if journal.Kind != "foundation" {
		return FoundationSettlement{}, fmt.Errorf("commit journal kind %q is not foundation", journal.Kind)
	}
	artifacts := make(map[string][]byte, len(journal.ArtifactNames))
	for _, name := range journal.ArtifactNames {
		data, err := p.store.ReadCorePreparedArtifact(journal.AttemptID, name)
		if err != nil {
			return FoundationSettlement{}, err
		}
		artifacts[name] = data
	}

	journal.State = "applying"
	if err := p.store.SaveCoreCommitJournal(journal); err != nil {
		return FoundationSettlement{}, err
	}
	if err := p.store.SaveCoreCanon(&journal.CanonState, &journal.CanonHead, artifacts); err != nil {
		return FoundationSettlement{}, err
	}
	if err := p.maybeCommitFault("canon"); err != nil {
		return FoundationSettlement{}, err
	}

	rel, err := p.store.SaveCoreReceipt(&journal.Receipt)
	if err != nil {
		return FoundationSettlement{}, err
	}
	if err := p.maybeCommitFault("receipt"); err != nil {
		return FoundationSettlement{}, err
	}
	if err := p.store.SaveCoreProductionState(&journal.NextState); err != nil {
		return FoundationSettlement{}, err
	}
	if err := p.maybeCommitFault("production"); err != nil {
		return FoundationSettlement{}, err
	}

	if journal.Receipt.FoundationDesignRoot == "" {
		record, err := p.store.LoadCoreSubmissionRecord(journal.AttemptID)
		if err != nil {
			return FoundationSettlement{}, err
		}
		if record != nil {
			record.State = "SETTLED"
			record.Result = "ACCEPTED"
			record.NewCanonRoot = journal.NewRoot
			record.ReceiptPath = rel
			record.Problem = ""
			if err := p.store.SaveCoreSubmissionRecord(record); err != nil {
				return FoundationSettlement{}, err
			}
		}
		if err := writeWorkspaceJSON(
			project.WorkspaceRoot,
			filepath.Join("exchange", "result", journal.AttemptID+".json"),
			map[string]any{
				"schema_version": protocol.MachineSchemaVersion,
				"task_id":        journal.TaskID,
				"attempt_id":     journal.AttemptID,
				"result":         "ACCEPTED",
				"new_canon_root": journal.NewRoot,
				"id_mappings":    journal.Receipt.IDMappings,
			},
		); err != nil {
			return FoundationSettlement{}, err
		}
	}

	if err := p.writeActiveAttempt(project, &journal.NextState); err != nil {
		return FoundationSettlement{}, err
	}
	if err := p.maybeCommitFault("ready"); err != nil {
		return FoundationSettlement{}, err
	}
	journal.State = "committed"
	if err := p.store.SaveCoreCommitJournal(journal); err != nil {
		return FoundationSettlement{}, err
	}
	return FoundationSettlement{
		Result:       "ACCEPTED",
		NewCanonRoot: journal.NewRoot,
		IDMappings:   journal.Receipt.IDMappings,
		ReceiptPath:  filepath.Join(p.root, rel),
	}, nil
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

package core

import (
	"fmt"
	"path/filepath"
	"sort"
	"time"

	"github.com/chenhongyang/novel-studio/internal/domain"
	"github.com/chenhongyang/novel-studio/internal/protocol"
)

func (p *Project) maybeCommitFault(stage string) error {
	if p.commitFault == nil {
		return nil
	}
	return p.commitFault(stage)
}

func (p *Project) recoverPendingCommit() error {
	journal, err := p.store.LoadPendingCoreCommitJournal()
	if err != nil || journal == nil {
		return err
	}
	project, err := p.store.LoadCoreProjectState()
	if err != nil || project == nil {
		if err == nil {
			err = fmt.Errorf("project is not initialized")
		}
		return err
	}
	_, err = p.applyChapterCommit(project, journal)
	return err
}

func (p *Project) prepareChapterCommit(project *domain.CoreProjectState, state *domain.CoreProductionState, task *domain.CoreTask, attempt *domain.CoreAttempt, record *domain.CoreSubmissionRecord, received, canonical map[string][]byte, mappings []IDMapping, chapter int, longform domain.CoreLongformState) (*domain.CoreCommitJournal, error) {
	head, err := p.store.LoadCoreCanonHead()
	if err != nil || head == nil {
		if err == nil {
			err = fmt.Errorf("canon head does not exist")
		}
		return nil, err
	}
	logicalArtifacts := make(map[string][]byte, len(canonical))
	artifactDigests := make(map[string]string, len(head.ArtifactDigests)+len(canonical))
	for name, digest := range head.ArtifactDigests {
		artifactDigests[name] = digest
	}
	prefix := fmt.Sprintf("chapters/%06d", chapter)
	artifactNames := make([]string, 0, len(canonical))
	for name, data := range canonical {
		logical := filepath.ToSlash(filepath.Join(prefix, name))
		logicalArtifacts[logical] = data
		digest, err := digestCanonArtifact(logical, data)
		if err != nil {
			return nil, err
		}
		artifactDigests[logical] = digest
		artifactNames = append(artifactNames, logical)
	}
	sort.Strings(artifactNames)

	newRevision := state.Revision + 1
	canonState := domain.CoreCanonState{
		SchemaVersion: coreSchemaVersion, Revision: newRevision,
		ProjectID: project.ProjectID, LastTaskID: task.TaskID,
		LastAttemptID: attempt.AttemptID, LatestChapter: chapter, Longform: longform,
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
		SchemaVersion: coreSchemaVersion, Revision: newRevision,
		ParentRoot: state.CanonRoot, Root: newRoot,
		StateDigest: stateDigest, ArtifactDigests: artifactDigests,
	}
	validationDigest, err := digestJSON(map[string]any{"result": "ACCEPTED", "violations": []string{}})
	if err != nil {
		return nil, err
	}
	receipt := domain.CoreReceipt{
		SchemaVersion: coreSchemaVersion, ProjectID: project.ProjectID,
		TaskID: task.TaskID, AttemptID: attempt.AttemptID,
		PreviousRoot: state.CanonRoot, TaskDigest: attempt.TaskDigest,
		SubmissionDigest: record.SnapshotDigest, ArtifactDigests: digestArtifacts(received),
		ValidationDigest: validationDigest, Result: "ACCEPTED", NewRoot: newRoot,
		IDMappings: mappings, CommittedAt: time.Now().UTC().Format(time.RFC3339Nano),
	}

	next := *state
	next.Revision = newRevision
	next.CanonRoot = newRoot
	next.NextEntitySeq += len(mappings)
	nextTask := newTask(&next, "chapter", fmt.Sprintf("chapter:%d", chapter+1), newRoot)
	nextAttempt, err := newAttempt(&next, nextTask, "initial", chapterArtifactNames, project.ProtocolVersion)
	if err != nil {
		return nil, err
	}
	next.ActiveTask, next.ActiveAttempt = nextTask, nextAttempt

	journal := &domain.CoreCommitJournal{
		SchemaVersion: coreSchemaVersion, State: "prepared",
		TaskID: task.TaskID, AttemptID: attempt.AttemptID, Chapter: chapter,
		PreviousRoot: state.CanonRoot, NewRoot: newRoot,
		ArtifactNames: artifactNames, CanonState: canonState, CanonHead: canonHead,
		Receipt: receipt, NextState: next,
	}
	if err := p.store.SaveCorePreparedArtifacts(attempt.AttemptID, logicalArtifacts); err != nil {
		return nil, err
	}
	if err := p.store.SaveCoreCommitJournal(journal); err != nil {
		return nil, err
	}
	return journal, nil
}

func (p *Project) applyChapterCommit(project *domain.CoreProjectState, journal *domain.CoreCommitJournal) (ChapterSettlement, error) {
	if journal == nil {
		return ChapterSettlement{}, fmt.Errorf("commit journal is required")
	}
	artifacts := make(map[string][]byte, len(journal.ArtifactNames))
	for _, name := range journal.ArtifactNames {
		data, err := p.store.ReadCorePreparedArtifact(journal.AttemptID, name)
		if err != nil {
			return ChapterSettlement{}, err
		}
		artifacts[name] = data
	}
	journal.State = "applying"
	if err := p.store.SaveCoreCommitJournal(journal); err != nil {
		return ChapterSettlement{}, err
	}
	if err := p.store.SaveCoreCanon(&journal.CanonState, &journal.CanonHead, artifacts); err != nil {
		return ChapterSettlement{}, err
	}
	if err := p.maybeCommitFault("canon"); err != nil {
		return ChapterSettlement{}, err
	}

	rel, err := p.store.SaveCoreReceipt(&journal.Receipt)
	if err != nil {
		return ChapterSettlement{}, err
	}
	if err := p.maybeCommitFault("receipt"); err != nil {
		return ChapterSettlement{}, err
	}
	chapterLogical := filepath.ToSlash(filepath.Join("chapters", fmt.Sprintf("%06d", journal.Chapter), "chapter.md"))
	if _, err := p.store.Checkpoints.Append(domain.ChapterScope(journal.Chapter), "commit", filepath.Join("meta", "core", "canon", "artifacts", filepath.FromSlash(chapterLogical)), "sha256:"+journal.CanonHead.ArtifactDigests[chapterLogical]); err != nil {
		return ChapterSettlement{}, err
	}
	if err := p.maybeCommitFault("checkpoint"); err != nil {
		return ChapterSettlement{}, err
	}

	body, err := p.store.ReadCorePreparedArtifact(journal.AttemptID, chapterLogical)
	if err != nil {
		return ChapterSettlement{}, err
	}
	if err := protocol.WriteUTF8Atomic(project.WorkspaceRoot, filepath.Join("published", "chapters", fmt.Sprintf("%06d.md", journal.Chapter)), body, 0o644); err != nil {
		return ChapterSettlement{}, err
	}
	if err := p.maybeCommitFault("published"); err != nil {
		return ChapterSettlement{}, err
	}
	if err := p.store.SaveCoreProductionState(&journal.NextState); err != nil {
		return ChapterSettlement{}, err
	}
	if err := p.maybeCommitFault("production"); err != nil {
		return ChapterSettlement{}, err
	}

	record, err := p.store.LoadCoreSubmissionRecord(journal.AttemptID)
	if err != nil {
		return ChapterSettlement{}, err
	}
	if record == nil {
		return ChapterSettlement{}, fmt.Errorf("submission record missing during recovery")
	}
	record.State = "SETTLED"
	record.Result = "ACCEPTED"
	record.NewCanonRoot = journal.NewRoot
	record.ReceiptPath = rel
	record.Problem = ""
	if err := p.store.SaveCoreSubmissionRecord(record); err != nil {
		return ChapterSettlement{}, err
	}
	if err := writeWorkspaceJSON(project.WorkspaceRoot, filepath.Join("exchange", "result", journal.AttemptID+".json"), map[string]any{
		"schema_version": protocol.MachineSchemaVersion,
		"task_id":        journal.TaskID, "attempt_id": journal.AttemptID,
		"result": "ACCEPTED", "new_canon_root": journal.NewRoot,
		"id_mappings": journal.Receipt.IDMappings,
	}); err != nil {
		return ChapterSettlement{}, err
	}
	if err := p.maybeCommitFault("result"); err != nil {
		return ChapterSettlement{}, err
	}

	if err := p.writeActiveAttempt(project, &journal.NextState); err != nil {
		return ChapterSettlement{}, err
	}
	if err := p.maybeCommitFault("ready"); err != nil {
		return ChapterSettlement{}, err
	}
	journal.State = "committed"
	if err := p.store.SaveCoreCommitJournal(journal); err != nil {
		return ChapterSettlement{}, err
	}
	return ChapterSettlement{
		Result: "ACCEPTED", NewCanonRoot: journal.NewRoot,
		IDMappings:  journal.Receipt.IDMappings,
		ReceiptPath: filepath.Join(p.root, rel),
	}, nil
}

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

func (p *Project) prepareChapterCommit(project *domain.CoreProjectState, state *domain.CoreProductionState, task *domain.CoreTask, attempt *domain.CoreAttempt, record *domain.CoreSubmissionRecord, received, canonical map[string][]byte, mappings []IDMapping, chapter int, longform domain.CoreLongformState, planning domain.CorePlanningState, planningStatus string, planningRepair bool) (*domain.CoreCommitJournal, error) {
	currentHead, err := p.store.LoadCoreCanonHead()
	if err != nil || currentHead == nil {
		if err == nil {
			err = fmt.Errorf("canon head does not exist")
		}
		return nil, err
	}
	parentRoot := state.CanonRoot
	baseHead := currentHead
	if task.Kind == "revision" {
		_, historicalHead, err := p.loadCanonSnapshotAtRoot(task.BaseCanonRoot)
		if err != nil {
			return nil, err
		}
		parentRoot = task.BaseCanonRoot
		baseHead = &historicalHead
	}

	logicalArtifacts := make(map[string][]byte, len(canonical))
	artifactDigests := make(map[string]string, len(baseHead.ArtifactDigests)+len(canonical))
	for name, digest := range baseHead.ArtifactDigests {
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
		LastAttemptID: attempt.AttemptID, LatestChapter: chapter, Longform: longform, Planning: planning,
	}
	stateDigest, err := digestJSON(canonState)
	if err != nil {
		return nil, err
	}
	newRoot, err := computeCanonRoot(newRevision, parentRoot, stateDigest, artifactDigests)
	if err != nil {
		return nil, err
	}
	canonHead := domain.CoreCanonHead{
		SchemaVersion: coreSchemaVersion, Revision: newRevision,
		ParentRoot: parentRoot, Root: newRoot,
		StateDigest: stateDigest, ArtifactDigests: artifactDigests,
	}
	validationDigest, err := digestJSON(map[string]any{"result": "ACCEPTED", "violations": []string{}, "planning_status": planningStatus})
	if err != nil {
		return nil, err
	}
	receipt := domain.CoreReceipt{
		SchemaVersion: coreSchemaVersion, ProjectID: project.ProjectID,
		TaskID: task.TaskID, AttemptID: attempt.AttemptID,
		PreviousRoot: parentRoot, TaskDigest: attempt.TaskDigest,
		SubmissionDigest: record.SnapshotDigest, ArtifactDigests: digestArtifacts(received),
		ValidationDigest: validationDigest, Result: "ACCEPTED", PlanningStatus: planningStatus, NewRoot: newRoot,
		IDMappings: mappings, CommittedAt: time.Now().UTC().Format(time.RFC3339Nano),
	}

	next := *state
	next.Revision = newRevision
	next.CanonRoot = newRoot
	next.NextEntitySeq += len(mappings)
	if task.Kind == "revision" && state.RevisionReplay != nil && chapter == state.RevisionReplay.StartChapter {
		next.SupersededChapters = mergeSuperseded(state.SupersededChapters, state.RevisionReplay.Superseded)
	}

	required := append([]string(nil), chapterArtifactNames...)
	var nextTask *domain.CoreTask
	nextReason := "initial"
	switch task.Kind {
	case "chapter":
		nextTask = newTask(&next, "chapter", fmt.Sprintf("chapter:%d", chapter+1), newRoot)
	case "revision":
		if state.RevisionReplay == nil {
			return nil, fmt.Errorf("revision task has no active replay state")
		}
		switch {
		case chapter < state.RevisionReplay.OriginalHeadChapter:
			nextTask = newTaskPreservingPending(&next, "revision", fmt.Sprintf("chapter:%d", chapter+1), newRoot)
			nextReason = "rebase"
		case chapter == state.RevisionReplay.OriginalHeadChapter:
			next.RevisionReplay = nil
			nextTask = newTask(&next, "chapter", fmt.Sprintf("chapter:%d", chapter+1), newRoot)
		default:
			return nil, fmt.Errorf("revision chapter %d is past original head %d", chapter, state.RevisionReplay.OriginalHeadChapter)
		}
	default:
		return nil, fmt.Errorf("unsupported commit task kind %q", task.Kind)
	}
	addRollingPlanningObligation(nextTask, planning, chapter+1)
	if planningRepair {
		required = append(required, "planning_patch.json")
		nextTask.Constraints = append(nextTask.Constraints, domain.CoreTaskConstraint{Kind: "planning_repair_required", Instruction: "Provide a valid next Arc plan before this chapter can be accepted."})
	}
	nextAttempt, err := newAttempt(&next, nextTask, nextReason, required, project.ProtocolVersion)
	if err != nil {
		return nil, err
	}
	next.ActiveTask, next.ActiveAttempt = nextTask, nextAttempt

	journal := &domain.CoreCommitJournal{
		SchemaVersion: coreSchemaVersion, State: "prepared",
		TaskID: task.TaskID, AttemptID: attempt.AttemptID, Chapter: chapter,
		PreviousRoot: parentRoot, NewRoot: newRoot,
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
		"planning_status": journal.Receipt.PlanningStatus, "id_mappings": journal.Receipt.IDMappings,
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
		Result: "ACCEPTED", NewCanonRoot: journal.NewRoot, PlanningStatus: journal.Receipt.PlanningStatus,
		IDMappings:  journal.Receipt.IDMappings,
		ReceiptPath: filepath.Join(p.root, rel),
	}, nil
}

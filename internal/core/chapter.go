package core

import (
	"fmt"
	"github.com/chenhongyang/novel-studio/internal/domain"
	"github.com/chenhongyang/novel-studio/internal/protocol"
)

type ChapterSettlement struct {
	Result          string            `json:"result"`
	NewCanonRoot    string            `json:"new_canon_root,omitempty"`
	IDMappings      []IDMapping       `json:"id_mappings,omitempty"`
	Violations      []string          `json:"violations,omitempty"`
	RewriteFeedback []RewriteFeedback `json:"rewrite_feedback,omitempty"`
	ReceiptPath     string            `json:"receipt_path,omitempty"`
	BlockID         string            `json:"block_id,omitempty"`
	PlanningStatus  string            `json:"planning_status,omitempty"`
}

func (p *Project) SettleActiveSnapshot() (ChapterSettlement, error) {
	release, err := p.acquireProjectMutationLock()
	if err != nil {
		return ChapterSettlement{}, err
	}
	defer release()
	return p.settleActiveSnapshotLocked()
}
func (p *Project) settleActiveSnapshotLocked() (ChapterSettlement, error) {
	project, state, err := p.activeProduction()
	if err != nil {
		return ChapterSettlement{}, err
	}
	task, attempt := state.ActiveTask, state.ActiveAttempt
	if state.ActiveBlock != nil && state.ActiveBlock.AttemptID == attempt.AttemptID {
		return ChapterSettlement{Result: "BLOCKED", BlockID: state.ActiveBlock.BlockID}, nil
	}
	if task.Kind != "chapter" && task.Kind != "revision" {
		return ChapterSettlement{}, fmt.Errorf("active task is %q, not chapter or revision", task.Kind)
	}
	journal, err := p.store.LoadCoreCommitJournal(attempt.AttemptID)
	if err != nil {
		return ChapterSettlement{}, err
	}
	if journal != nil && journal.State != "committed" {
		return p.applyChapterCommit(project, journal)
	}
	record, err := p.store.LoadCoreSubmissionRecord(attempt.AttemptID)
	if err != nil {
		return ChapterSettlement{}, err
	}
	if record == nil || record.State != "READY_TO_VALIDATE" {
		return ChapterSettlement{}, fmt.Errorf("active submission is not ready to validate")
	}
	files, manifest, err := p.readSnapshot(attempt)
	if err != nil {
		return ChapterSettlement{}, err
	}
	if err := validateSubmissionIdentity(project, task, attempt, manifest); err != nil {
		return ChapterSettlement{}, err
	}
	all := make(map[string][]byte, len(files)+1)
	for name, data := range files {
		all[name] = data
	}
	manifestRaw, err := p.store.ReadCoreSnapshotFile(attempt.AttemptID, "manifest.json")
	if err != nil {
		return ChapterSettlement{}, err
	}
	all["manifest.json"] = manifestRaw
	snapshotDigest, err := digestArtifactManifest(digestArtifacts(all))
	if err != nil {
		return ChapterSettlement{}, err
	}
	if snapshotDigest != record.SnapshotDigest {
		return ChapterSettlement{}, fmt.Errorf("local snapshot digest mismatch")
	}
	chapterFiles := make(map[string][]byte, len(chapterArtifactNames))
	for _, name := range chapterArtifactNames {
		chapterFiles[name] = files[name]
	}
	canonical, mappings, violations, chapter, err := validateAndCanonicalizeChapter(chapterFiles, state, task)
	if err != nil {
		return ChapterSettlement{}, err
	}
	if len(violations) > 0 {
		return p.rejectChapter(project, state, task, attempt, record, files, violations)
	}
	if requiresArtifact(attempt, "chapter_review.json") {
		reviewCanonical, reviewViolations, err := validateChapterReview(files["chapter_review.json"], files["chapter.md"])
		if err != nil {
			return ChapterSettlement{}, err
		}
		if len(reviewViolations) > 0 {
			return p.rejectChapter(project, state, task, attempt, record, files, reviewViolations)
		}
		canonical["chapter_review.json"] = reviewCanonical
	}
	canonRaw, err := p.store.ReadCoreCanonStateBytes()
	if err != nil {
		return ChapterSettlement{}, err
	}
	var canonState domain.CoreCanonState
	if err := protocol.DecodeJSON(canonRaw, &canonState); err != nil {
		return ChapterSettlement{}, err
	}
	validationCanon := canonState
	if task.Kind == "revision" {
		baseCanon, _, err := p.loadCanonSnapshotAtRoot(task.BaseCanonRoot)
		if err != nil {
			return ChapterSettlement{}, err
		}
		validationCanon = baseCanon
	}
	contractViolations, err := p.validateExtendedChapterContractSemantics(canonical, validationCanon, task)
	if err != nil {
		return ChapterSettlement{}, err
	}
	if len(contractViolations) > 0 {
		return p.rejectChapter(project, state, task, attempt, record, files, contractViolations)
	}
	referenceViolations, err := p.validateCanonicalEntityReferences(canonical, validationCanon.Longform, mappings)
	if err != nil {
		return ChapterSettlement{}, err
	}
	if len(referenceViolations) > 0 {
		return p.rejectChapter(project, state, task, attempt, record, files, referenceViolations)
	}
	nextLongform, longformViolations, err := validateLongformState(validationCanon.Longform, canonical, chapter)
	if err != nil {
		return ChapterSettlement{}, err
	}
	if len(longformViolations) > 0 {
		return p.rejectChapter(project, state, task, attempt, record, files, longformViolations)
	}
	planningStatus, nextPlanning, planningCanonical, planningRepair, planningViolations, err := validateChapterPlanning(files, validationCanon.Planning, chapter, requiresArtifact(attempt, "planning_patch.json"))
	if err != nil {
		return ChapterSettlement{}, err
	}
	if len(planningViolations) > 0 && requiresArtifact(attempt, "planning_patch.json") {
		return p.rejectChapter(project, state, task, attempt, record, files, planningViolations, planningStatus)
	}
	if planningStatus == "accepted" {
		canonical["planning_patch.json"] = planningCanonical
	}
	block, blockViolations, err := detectAuthorDecision(files, task, attempt)
	if err != nil {
		return ChapterSettlement{}, err
	}
	if len(blockViolations) > 0 {
		return p.rejectChapter(project, state, task, attempt, record, files, blockViolations)
	}
	if block != nil {
		return p.blockChapter(project, state, task, attempt, record, files, block)
	}
	return p.acceptChapter(project, state, task, attempt, record, files, canonical, mappings, chapter, nextLongform, nextPlanning, planningStatus, planningRepair)
}
func (p *Project) readSnapshot(attempt *domain.CoreAttempt) (map[string][]byte, protocol.SubmissionManifest, error) {
	manifestRaw, err := p.store.ReadCoreSnapshotFile(attempt.AttemptID, "manifest.json")
	if err != nil {
		return nil, protocol.SubmissionManifest{}, err
	}
	var manifest protocol.SubmissionManifest
	if err := protocol.DecodeJSON(manifestRaw, &manifest); err != nil {
		return nil, protocol.SubmissionManifest{}, err
	}
	files := make(map[string][]byte, len(manifest.Files))
	for _, name := range manifest.Files {
		data, err := p.store.ReadCoreSnapshotFile(attempt.AttemptID, name)
		if err != nil {
			return nil, protocol.SubmissionManifest{}, err
		}
		files[name] = data
	}
	return files, manifest, nil
}

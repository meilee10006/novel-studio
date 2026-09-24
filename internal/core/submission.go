package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/chenhongyang/novel-studio/internal/domain"
	"github.com/chenhongyang/novel-studio/internal/protocol"
)

const maxSubmissionBytes = 32 << 20

type SubmissionStatus = domain.CoreSubmissionRecord

func (p *Project) ScanActiveSubmission() (SubmissionStatus, error) {
	release, err := p.acquireProjectMutationLock()
	if err != nil {
		return SubmissionStatus{}, err
	}
	defer release()
	return p.scanActiveSubmissionLocked()
}

func (p *Project) scanActiveSubmissionLocked() (SubmissionStatus, error) {
	project, production, err := p.activeProduction()
	if err != nil {
		return SubmissionStatus{}, err
	}
	task, attempt := production.ActiveTask, production.ActiveAttempt
	if attemptInputSource(attempt) != "drive" {
		return SubmissionStatus{}, fmt.Errorf("active attempt does not accept Drive submission")
	}
	record, err := p.store.LoadCoreSubmissionRecord(attempt.AttemptID)
	if err != nil {
		return SubmissionStatus{}, err
	}
	if record == nil {
		record = &domain.CoreSubmissionRecord{
			SchemaVersion: coreSchemaVersion, TaskID: task.TaskID,
			AttemptID: attempt.AttemptID, State: "PENDING",
		}
	}
	manifestRel := filepath.Join("exchange", "inbox", task.TaskID, attempt.AttemptID, "manifest.json")
	var manifest protocol.SubmissionManifest
	if err := protocol.ReadJSON(project.WorkspaceRoot, manifestRel, 64<<10, &manifest); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return p.saveSubmissionRecord(record)
		}
		return p.invalidateSubmission(record, err.Error())
	}
	if err := validateSubmissionIdentity(project, task, attempt, manifest); err != nil {
		return p.invalidateSubmission(record, err.Error())
	}

	baseRel := filepath.Join("exchange", "inbox", task.TaskID, attempt.AttemptID)
	files, scanDigest, err := readInboxCandidate(project.WorkspaceRoot, baseRel, manifest.Files, maxSubmissionBytes)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return p.saveSubmissionRecord(record)
		}
		return p.invalidateSubmission(record, err.Error())
	}
	observation, decision := advanceStableObservation(stableObservation{
		ObservedDigest: record.ObservedDigest,
		ObservedAt:     record.ObservedAt,
		SnapshotDigest: record.SnapshotDigest,
	}, scanDigest, time.Now().UTC(), p.submissionQuietPeriod)
	record.ObservedDigest = observation.ObservedDigest
	record.ObservedAt = observation.ObservedAt
	record.SnapshotDigest = observation.SnapshotDigest
	switch decision {
	case "LOCKED_SAME":
		return *record, nil
	case "LOCKED_CONFLICT":
		record.Conflict = true
		record.Problem = "Drive content changed after local snapshot was locked"
		if record.State != "SETTLED" {
			record.State = "INVALID"
		}
		return p.saveSubmissionRecord(record)
	case "PENDING":
		record.State = "PENDING"
		record.Problem = ""
		return p.saveSubmissionRecord(record)
	case "READY_TO_SNAPSHOT":
		if err := p.store.SaveCoreSnapshot(attempt.AttemptID, files); err != nil {
			return SubmissionStatus{}, err
		}
		record.SnapshotDigest = scanDigest
		record.State = "READY_TO_VALIDATE"
		record.Problem = ""
		return p.saveSubmissionRecord(record)
	default:
		return SubmissionStatus{}, fmt.Errorf("unknown stable observation decision %q", decision)
	}
}

func (p *Project) activeProduction() (*domain.CoreProjectState, *domain.CoreProductionState, error) {
	project, err := p.store.LoadCoreProjectState()
	if err != nil || project == nil {
		if err == nil {
			err = fmt.Errorf("project is not initialized")
		}
		return nil, nil, err
	}
	production, err := p.store.LoadCoreProductionState()
	if err != nil || production == nil || production.ActiveTask == nil || production.ActiveAttempt == nil {
		if err == nil {
			err = fmt.Errorf("no active production attempt")
		}
		return nil, nil, err
	}
	return project, production, nil
}
func (p *Project) saveSubmissionRecord(record *domain.CoreSubmissionRecord) (SubmissionStatus, error) {
	if err := p.store.SaveCoreSubmissionRecord(record); err != nil {
		return SubmissionStatus{}, err
	}
	return *record, nil
}

func (p *Project) invalidateSubmission(record *domain.CoreSubmissionRecord, problem string) (SubmissionStatus, error) {
	record.State = "INVALID"
	record.Problem = problem
	return p.saveSubmissionRecord(record)
}

func validateInboxEntries(inbox string, required []string) string {
	entries, err := os.ReadDir(inbox)
	if os.IsNotExist(err) {
		return ""
	}
	if err != nil {
		return err.Error()
	}
	allowed := map[string]bool{"manifest.json": true}
	for _, name := range required {
		allowed[name] = true
	}
	for _, entry := range entries {
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			return "submission contains directory or symlink: " + entry.Name()
		}
		if !allowed[entry.Name()] {
			return "submission contains unknown file: " + entry.Name()
		}
	}
	return ""
}

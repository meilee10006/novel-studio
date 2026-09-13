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
	project, production, err := p.activeProduction()
	if err != nil {
		return SubmissionStatus{}, err
	}
	task, attempt := production.ActiveTask, production.ActiveAttempt
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

	inbox := filepath.Join(project.WorkspaceRoot, "exchange", "inbox", task.TaskID, attempt.AttemptID)
	if problem := validateInboxEntries(inbox, attempt.RequiredArtifacts); problem != "" {
		return p.invalidateSubmission(record, problem)
	}
	files := make(map[string][]byte, len(attempt.RequiredArtifacts)+1)
	manifestRaw, err := protocol.ReadUTF8(project.WorkspaceRoot, manifestRel, 64<<10)
	if err != nil {
		return p.invalidateSubmission(record, err.Error())
	}
	files["manifest.json"] = manifestRaw
	total := len(manifestRaw)
	for _, name := range attempt.RequiredArtifacts {
		rel := filepath.Join("exchange", "inbox", task.TaskID, attempt.AttemptID, name)
		data, err := protocol.ReadUTF8(project.WorkspaceRoot, rel, protocol.DefaultMaxTextSize)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return p.saveSubmissionRecord(record)
			}
			return p.invalidateSubmission(record, err.Error())
		}
		total += len(data)
		if total > maxSubmissionBytes {
			return p.invalidateSubmission(record, "submission exceeds max total size")
		}
		files[name] = data
	}

	scanDigest, err := digestArtifactManifest(digestArtifacts(files))
	if err != nil {
		return SubmissionStatus{}, err
	}
	if record.SnapshotDigest != "" {
		if scanDigest == record.SnapshotDigest {
			return *record, nil
		}
		record.Conflict = true
		record.Problem = "Drive content changed after local snapshot was locked"
		if record.State != "SETTLED" {
			record.State = "INVALID"
		}
		return p.saveSubmissionRecord(record)
	}
	now := time.Now().UTC()
	if record.ObservedDigest != scanDigest {
		record.ObservedDigest = scanDigest
		record.ObservedAt = now.Format(time.RFC3339Nano)
		record.State = "PENDING"
		record.Problem = ""
		return p.saveSubmissionRecord(record)
	}
	if observedAt, err := time.Parse(time.RFC3339Nano, record.ObservedAt); err == nil && now.Sub(observedAt) < p.submissionQuietPeriod {
		return *record, nil
	}
	if err := p.store.SaveCoreSnapshot(attempt.AttemptID, files); err != nil {
		return SubmissionStatus{}, err
	}
	record.SnapshotDigest = scanDigest
	record.State = "READY_TO_VALIDATE"
	record.Problem = ""
	return p.saveSubmissionRecord(record)
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

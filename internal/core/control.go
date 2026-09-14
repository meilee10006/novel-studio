package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/chenhongyang/novel-studio/internal/domain"
	"github.com/chenhongyang/novel-studio/internal/protocol"
)

var controlMessageIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

type ControlStatus = domain.CoreControlRecord

type ControlResult struct {
	MessageID string `json:"message_id"`
	Result    string `json:"result"`
	Problem   string `json:"problem,omitempty"`
	TaskID    string `json:"task_id,omitempty"`
	AttemptID string `json:"attempt_id,omitempty"`
}

func (p *Project) ScanControlMessage(messageID string) (ControlStatus, error) {
	release, err := p.acquireProjectWriteLock()
	if err != nil {
		return ControlStatus{}, err
	}
	defer release()
	if !controlMessageIDPattern.MatchString(messageID) {
		return ControlStatus{}, fmt.Errorf("invalid control message id %q", messageID)
	}
	project, err := p.store.LoadCoreProjectState()
	if err != nil || project == nil {
		if err == nil {
			err = fmt.Errorf("project is not initialized")
		}
		return ControlStatus{}, err
	}
	record, err := p.store.LoadCoreControlRecord(messageID)
	if err != nil {
		return ControlStatus{}, err
	}
	if record == nil {
		record = &domain.CoreControlRecord{SchemaVersion: coreSchemaVersion, MessageID: messageID, State: "PENDING"}
	}
	if record.State == "SETTLED" {
		return *record, nil
	}
	base := filepath.Join("exchange", "control", "inbox", messageID)
	var manifest protocol.ControlManifest
	if err := protocol.ReadJSON(project.WorkspaceRoot, filepath.Join(base, "manifest.json"), 64<<10, &manifest); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return p.saveControlRecord(record)
		}
		return p.invalidateControl(record, err.Error())
	}
	if problem := validateControlManifest(project, messageID, manifest); problem != "" {
		return p.invalidateControl(record, problem)
	}
	controlDir := filepath.Join(project.WorkspaceRoot, base)
	entries, err := os.ReadDir(controlDir)
	if err != nil {
		return ControlStatus{}, err
	}
	for _, entry := range entries {
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			return p.invalidateControl(record, "control message contains directory or symlink")
		}
		if entry.Name() != "control.json" && entry.Name() != "manifest.json" {
			return p.invalidateControl(record, "control message contains unknown file: "+entry.Name())
		}
	}
	files := map[string][]byte{}
	for _, name := range []string{"control.json", "manifest.json"} {
		data, err := protocol.ReadUTF8(project.WorkspaceRoot, filepath.Join(base, name), 256<<10)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return p.saveControlRecord(record)
			}
			return p.invalidateControl(record, err.Error())
		}
		files[name] = data
	}
	digest, err := digestArtifactManifest(digestArtifacts(files))
	if err != nil {
		return ControlStatus{}, err
	}
	if record.SnapshotDigest != "" {
		if record.SnapshotDigest == digest {
			return *record, nil
		}
		record.Conflict = true
		record.State = "INVALID"
		record.Problem = "control message changed after local snapshot was locked"
		return p.saveControlRecord(record)
	}
	now := time.Now().UTC()
	if record.ObservedDigest != digest {
		record.ObservedDigest = digest
		record.ObservedAt = now.Format(time.RFC3339Nano)
		record.State = "PENDING"
		record.Problem = ""
		return p.saveControlRecord(record)
	}
	if observedAt, err := time.Parse(time.RFC3339Nano, record.ObservedAt); err == nil && now.Sub(observedAt) < p.submissionQuietPeriod {
		return *record, nil
	}
	if err := p.store.SaveCoreControlSnapshot(messageID, files); err != nil {
		return ControlStatus{}, err
	}
	record.SnapshotDigest = digest
	record.State = "READY_TO_VALIDATE"
	record.Problem = ""
	return p.saveControlRecord(record)
}

func validateControlManifest(project *domain.CoreProjectState, messageID string, manifest protocol.ControlManifest) string {
	if manifest.SchemaVersion != protocol.MachineSchemaVersion {
		return "unsupported control schema version"
	}
	if manifest.ProjectID != project.ProjectID || manifest.MessageID != messageID {
		return "control manifest identity mismatch"
	}
	if manifest.ProtocolVersion != project.ProtocolVersion {
		return "control protocol version mismatch"
	}
	if len(manifest.Files) != 1 || manifest.Files[0] != "control.json" {
		return "control manifest file list mismatch"
	}
	return ""
}
func (p *Project) saveControlRecord(record *domain.CoreControlRecord) (ControlStatus, error) {
	if err := p.store.SaveCoreControlRecord(record); err != nil {
		return ControlStatus{}, err
	}
	return *record, nil
}

func (p *Project) invalidateControl(record *domain.CoreControlRecord, problem string) (ControlStatus, error) {
	record.State = "INVALID"
	record.Problem = problem
	return p.saveControlRecord(record)
}

func (p *Project) ProcessControlMessage(messageID string) (ControlResult, error) {
	release, err := p.acquireProjectWriteLock()
	if err != nil {
		return ControlResult{}, err
	}
	defer release()
	project, state, err := p.activeProduction()
	if err != nil {
		return ControlResult{}, err
	}
	record, err := p.store.LoadCoreControlRecord(messageID)
	if err != nil {
		return ControlResult{}, err
	}
	if record == nil {
		return ControlResult{}, fmt.Errorf("control message has not been scanned")
	}
	if record.State == "SETTLED" {
		result := ControlResult{MessageID: messageID, Result: record.Result, Problem: record.Problem, TaskID: record.TaskID, AttemptID: record.AttemptID}
		_ = p.writeControlResult(project, result)
		return result, nil
	}
	if record.State == "APPLYING" {
		return p.applyPreparedControl(project, record)
	}
	if record.State != "READY_TO_VALIDATE" {
		return ControlResult{}, fmt.Errorf("control message is not ready to validate")
	}
	controlRaw, err := p.store.ReadCoreControlSnapshotFile(messageID, "control.json")
	if err != nil {
		return ControlResult{}, err
	}
	manifestRaw, err := p.store.ReadCoreControlSnapshotFile(messageID, "manifest.json")
	if err != nil {
		return ControlResult{}, err
	}
	var message domain.CoreControlMessage
	if err := protocol.DecodeJSON(controlRaw, &message); err != nil {
		return p.settleInvalidControl(project, record, err.Error())
	}
	var manifest protocol.ControlManifest
	if err := protocol.DecodeJSON(manifestRaw, &manifest); err != nil {
		return p.settleInvalidControl(project, record, err.Error())
	}
	if message.SchemaVersion != protocol.MachineSchemaVersion || message.ProjectID != project.ProjectID || message.MessageID != messageID {
		return p.settleInvalidControl(project, record, "control message identity mismatch")
	}
	if manifest.BaseCanonRoot != message.BaseCanonRoot {
		return p.settleInvalidControl(project, record, "control base canon root mismatch")
	}
	if message.BaseCanonRoot != state.CanonRoot {
		return p.settleInvalidControl(project, record, "control message canon root is stale")
	}
	switch message.Kind {
	case "block_resolution":
		return p.processBlockResolution(project, state, record, message)
	case "author_directive":
		return p.processAuthorDirective(project, state, record, message)
	default:
		return p.settleInvalidControl(project, record, "unsupported control kind: "+message.Kind)
	}
}

func (p *Project) settleInvalidControl(project *domain.CoreProjectState, record *domain.CoreControlRecord, problem string) (ControlResult, error) {
	result := ControlResult{MessageID: record.MessageID, Result: "INVALID", Problem: problem}
	return p.settleControl(project, record, result)
}
func (p *Project) processBlockResolution(project *domain.CoreProjectState, state *domain.CoreProductionState, record *domain.CoreControlRecord, message domain.CoreControlMessage) (ControlResult, error) {
	if state.ActiveBlock == nil || message.BlockID == "" || message.BlockID != state.ActiveBlock.BlockID {
		return p.settleInvalidControl(project, record, "block resolution does not match current block")
	}
	choice := strings.TrimSpace(message.Choice)
	if choice == "" {
		return p.settleInvalidControl(project, record, "block resolution choice is required")
	}
	next := cloneProductionState(state)
	next.ActiveTask.Constraints = append(next.ActiveTask.Constraints, domain.CoreTaskConstraint{
		MessageID: message.MessageID, Kind: "block_resolution", BlockID: message.BlockID, Choice: choice,
	})
	next.ActiveBlock = nil
	attempt, err := newAttempt(next, next.ActiveTask, "rewrite", next.ActiveAttempt.RequiredArtifacts, project.ProtocolVersion)
	if err != nil {
		return ControlResult{}, err
	}
	next.ActiveAttempt = attempt
	return p.prepareAndApplyControl(project, record, next, ControlResult{
		MessageID: message.MessageID, Result: "ACCEPTED", TaskID: next.ActiveTask.TaskID, AttemptID: attempt.AttemptID,
	})
}
func (p *Project) processAuthorDirective(project *domain.CoreProjectState, state *domain.CoreProductionState, record *domain.CoreControlRecord, message domain.CoreControlMessage) (ControlResult, error) {
	instruction := strings.TrimSpace(message.Instruction)
	if instruction == "" {
		return p.settleInvalidControl(project, record, "author directive instruction is required")
	}
	constraint := domain.CoreTaskConstraint{
		MessageID: message.MessageID, Kind: "author_directive", Scope: message.DirectiveScope, Instruction: instruction,
	}
	next := cloneProductionState(state)
	switch message.DirectiveScope {
	case "future_plan":
		next.PendingControls = append(next.PendingControls, constraint)
		return p.prepareAndApplyControl(project, record, next, ControlResult{
			MessageID: message.MessageID, Result: "ACCEPTED", TaskID: next.ActiveTask.TaskID, AttemptID: next.ActiveAttempt.AttemptID,
		})
	case "historical_revision":
		return p.prepareHistoricalRevision(project, record, next, message, constraint)
	default:
		return p.settleInvalidControl(project, record, "unsupported directive scope: "+message.DirectiveScope)
	}
}
func (p *Project) prepareHistoricalRevision(project *domain.CoreProjectState, record *domain.CoreControlRecord, next *domain.CoreProductionState, message domain.CoreControlMessage, constraint domain.CoreTaskConstraint) (ControlResult, error) {
	if message.Chapter <= 0 {
		return p.settleInvalidControl(project, record, "historical revision chapter must be positive")
	}
	if next.RevisionReplay != nil || next.ActiveTask != nil && next.ActiveTask.Kind == "revision" {
		return p.settleInvalidControl(project, record, "historical revision is already active")
	}
	raw, err := p.store.ReadCoreCanonStateBytes()
	if err != nil {
		return ControlResult{}, err
	}
	var canon domain.CoreCanonState
	if err := protocol.DecodeJSON(raw, &canon); err != nil {
		return ControlResult{}, err
	}
	if message.Chapter > canon.LatestChapter {
		return p.settleInvalidControl(project, record, "historical revision chapter is not accepted canon")
	}
	replay, baseRoot, err := p.buildRevisionReplay(next.CanonRoot, message.Chapter, canon.LatestChapter)
	if err != nil {
		return ControlResult{}, err
	}
	next.RevisionReplay = replay
	task := newTaskPreservingPending(next, "revision", fmt.Sprintf("chapter:%d", message.Chapter), baseRoot)
	task.Constraints = append(task.Constraints, constraint)
	attempt, err := newAttempt(next, task, "initial", chapterArtifactNames, project.ProtocolVersion)
	if err != nil {
		return ControlResult{}, err
	}
	next.ActiveTask, next.ActiveAttempt, next.ActiveBlock = task, attempt, nil
	return p.prepareAndApplyControl(project, record, next, ControlResult{
		MessageID: message.MessageID, Result: "ACCEPTED", TaskID: task.TaskID, AttemptID: attempt.AttemptID,
	})
}
func (p *Project) prepareAndApplyControl(project *domain.CoreProjectState, record *domain.CoreControlRecord, next *domain.CoreProductionState, result ControlResult) (ControlResult, error) {
	record.State = "APPLYING"
	record.Result = result.Result
	record.Problem = result.Problem
	record.TaskID = result.TaskID
	record.AttemptID = result.AttemptID
	record.NextProduction = next
	if err := p.store.SaveCoreControlRecord(record); err != nil {
		return ControlResult{}, err
	}
	return p.applyPreparedControl(project, record)
}

func (p *Project) applyPreparedControl(project *domain.CoreProjectState, record *domain.CoreControlRecord) (ControlResult, error) {
	if record.NextProduction == nil {
		return ControlResult{}, fmt.Errorf("prepared control is missing target production state")
	}
	if err := p.store.SaveCoreProductionState(record.NextProduction); err != nil {
		return ControlResult{}, err
	}
	if err := p.failControlAt("production"); err != nil {
		return ControlResult{}, err
	}
	if err := p.writeActiveAttempt(project, record.NextProduction); err != nil {
		return ControlResult{}, err
	}
	if err := p.failControlAt("ready"); err != nil {
		return ControlResult{}, err
	}
	result := ControlResult{
		MessageID: record.MessageID, Result: record.Result, Problem: record.Problem,
		TaskID: record.TaskID, AttemptID: record.AttemptID,
	}
	record.State = "SETTLED"
	record.NextProduction = nil
	if err := p.store.SaveCoreControlRecord(record); err != nil {
		return ControlResult{}, err
	}
	if err := p.failControlAt("settled"); err != nil {
		return ControlResult{}, err
	}
	if err := p.writeControlResult(project, result); err != nil {
		return ControlResult{}, err
	}
	return result, nil
}

func cloneProductionState(state *domain.CoreProductionState) *domain.CoreProductionState {
	if state == nil {
		return nil
	}
	clone := *state
	if state.ActiveTask != nil {
		task := *state.ActiveTask
		task.Constraints = append([]domain.CoreTaskConstraint(nil), state.ActiveTask.Constraints...)
		clone.ActiveTask = &task
	}
	if state.ActiveAttempt != nil {
		attempt := *state.ActiveAttempt
		attempt.RequiredArtifacts = append([]string(nil), state.ActiveAttempt.RequiredArtifacts...)
		clone.ActiveAttempt = &attempt
	}
	if state.ActiveBlock != nil {
		block := *state.ActiveBlock
		block.ConstraintRefs = append([]string(nil), state.ActiveBlock.ConstraintRefs...)
		block.Options = append([]string(nil), state.ActiveBlock.Options...)
		clone.ActiveBlock = &block
	}
	if state.RevisionReplay != nil {
		replay := *state.RevisionReplay
		replay.Superseded = append([]domain.CoreSupersededChapter(nil), state.RevisionReplay.Superseded...)
		clone.RevisionReplay = &replay
	}
	clone.SupersededChapters = append([]domain.CoreSupersededChapter(nil), state.SupersededChapters...)
	clone.PendingControls = append([]domain.CoreTaskConstraint(nil), state.PendingControls...)
	return &clone
}

func (p *Project) settleControl(project *domain.CoreProjectState, record *domain.CoreControlRecord, result ControlResult) (ControlResult, error) {
	record.State = "SETTLED"
	record.Result = result.Result
	record.Problem = result.Problem
	record.TaskID = result.TaskID
	record.AttemptID = result.AttemptID
	record.NextProduction = nil
	if err := p.store.SaveCoreControlRecord(record); err != nil {
		return ControlResult{}, err
	}
	if err := p.writeControlResult(project, result); err != nil {
		return ControlResult{}, err
	}
	return result, nil
}

func (p *Project) writeControlResult(project *domain.CoreProjectState, result ControlResult) error {
	return writeWorkspaceJSON(project.WorkspaceRoot, filepath.Join("exchange", "control", "result", result.MessageID+".json"), map[string]any{
		"schema_version": protocol.MachineSchemaVersion,
		"message_id":     result.MessageID,
		"result":         result.Result,
		"problem":        result.Problem,
		"task_id":        result.TaskID,
		"attempt_id":     result.AttemptID,
	})
}

func (p *Project) failControlAt(stage string) error {
	if p.controlFault == nil {
		return nil
	}
	return p.controlFault(stage)
}

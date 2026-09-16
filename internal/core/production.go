package core

import (
	"fmt"
	"github.com/chenhongyang/novel-studio/internal/domain"
	"github.com/chenhongyang/novel-studio/internal/protocol"
	"sort"
)

var foundationArtifactNames = []string{
	"book_plan.json",
	"characters.json",
	"ending_contract.json",
	"foundation.json",
	"platform_profile.json",
	"style_profile.json",
	"world.json",
}
var chapterArtifactNames = []string{
	"chapter.md", "chapter_contract.json", "events.json",
	"self_review.json", "state_delta.json",
}

type IDMapping = domain.CoreIDMapping
type FoundationSubmission struct {
	Manifest  protocol.SubmissionManifest
	Artifacts map[string][]byte
}
type FoundationSettlement struct {
	Result          string            `json:"result"`
	NewCanonRoot    string            `json:"new_canon_root,omitempty"`
	IDMappings      []IDMapping       `json:"id_mappings,omitempty"`
	Violations      []string          `json:"violations,omitempty"`
	RewriteFeedback []RewriteFeedback `json:"rewrite_feedback,omitempty"`
	ReceiptPath     string            `json:"receipt_path,omitempty"`
}
type readyDocument struct {
	SchemaVersion   int    `json:"schema_version"`
	ProjectID       string `json:"project_id"`
	TaskID          string `json:"task_id"`
	TaskKind        string `json:"task_kind"`
	AttemptID       string `json:"attempt_id"`
	AttemptReason   string `json:"attempt_reason"`
	Target          string `json:"target"`
	BaseCanonRoot   string `json:"base_canon_root"`
	ProtocolVersion string `json:"protocol_version"`
	TaskDigest      string `json:"task_digest"`
	CompletionNonce string `json:"completion_nonce"`
	Status          string `json:"status"`
	BlockID         string `json:"block_id,omitempty"`
}

func (p *Project) Reconcile() error {
	release, err := p.acquireProjectMutationLock()
	if err != nil {
		return err
	}
	defer release()
	return p.reconcileLocked()
}
func (p *Project) reconcileLocked() error {
	if err := p.recoverPendingCommit(); err != nil {
		return err
	}
	projectState, err := p.store.LoadCoreProjectState()
	if err != nil {
		return err
	}
	if projectState == nil {
		return fmt.Errorf("project is not initialized")
	}
	capability, problem := capabilityStatus(projectState)
	if capability != "passed" {
		production, err := p.store.LoadCoreProductionState()
		if err != nil {
			return err
		}
		if err := p.writeWorkspaceStatus(projectState, production); err != nil {
			return err
		}
		if capability == "pending" {
			return nil
		}
		return fmt.Errorf("capability check failed: %s", problem)
	}

	state, err := p.store.LoadCoreProductionState()
	if err != nil {
		return err
	}
	if state == nil {
		state = newCoreProductionState()
	}
	if state.ActiveTask == nil || state.ActiveAttempt == nil {
		if state.CanonRoot != "" {
			return fmt.Errorf("canon exists without an active task")
		}
		task := newTask(state, "foundation", "foundation", "")
		attempt, err := newAttempt(state, task, "initial", foundationArtifactNames, projectState.ProtocolVersion)
		if err != nil {
			return err
		}
		state.ActiveTask = task
		state.ActiveAttempt = attempt
		if err := p.store.SaveCoreProductionState(state); err != nil {
			return err
		}
	}
	return p.writeActiveAttempt(projectState, state)
}
func newCoreProductionState() *domain.CoreProductionState {
	return &domain.CoreProductionState{
		SchemaVersion:  coreSchemaVersion,
		NextTaskSeq:    1,
		NextAttemptSeq: 1,
		NextEntitySeq:  1,
	}
}
func newTask(state *domain.CoreProductionState, kind, target, baseRoot string) *domain.CoreTask {
	task := &domain.CoreTask{
		TaskID: fmt.Sprintf("task-%06d", state.NextTaskSeq), Kind: kind,
		Target: target, BaseCanonRoot: baseRoot,
	}
	if len(state.PendingControls) > 0 {
		task.Constraints = append([]domain.CoreTaskConstraint(nil), state.PendingControls...)
		state.PendingControls = nil
	}
	state.NextTaskSeq++
	return task
}
func newAttempt(state *domain.CoreProductionState, task *domain.CoreTask, reason string, required []string, protocolVersion string) (*domain.CoreAttempt, error) {
	nonce, err := randomNonce()
	if err != nil {
		return nil, err
	}
	files := append([]string(nil), required...)
	sort.Strings(files)
	attempt := &domain.CoreAttempt{
		AttemptID:         fmt.Sprintf("attempt-%06d", state.NextAttemptSeq),
		Reason:            reason,
		ProtocolVersion:   protocolVersion,
		CompletionNonce:   nonce,
		RequiredArtifacts: files,
	}
	state.NextAttemptSeq++
	digest, err := digestJSON(struct {
		Task              *domain.CoreTask `json:"task"`
		ProtocolVersion   string           `json:"protocol_version"`
		RequiredArtifacts []string         `json:"required_artifacts"`
	}{task, protocolVersion, files})
	if err != nil {
		return nil, err
	}
	attempt.TaskDigest = digest
	return attempt, nil
}

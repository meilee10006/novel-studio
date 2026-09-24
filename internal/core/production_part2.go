package core

import (
	"encoding/json"
	"fmt"
	"github.com/chenhongyang/novel-studio/internal/domain"
	"github.com/chenhongyang/novel-studio/internal/protocol"
	"path/filepath"
	"strings"
)

type preparedFoundation struct {
	Canonical map[string][]byte
	Mappings  []IDMapping
	Longform  domain.CoreLongformState
	Planning  domain.CorePlanningState
}

func prepareFoundationArtifacts(
	artifacts map[string][]byte,
	state *domain.CoreProductionState,
) (preparedFoundation, []string, error) {
	canonical, mappings, violations, err := validateAndCanonicalizeFoundation(artifacts, state)
	if err != nil || len(violations) != 0 {
		return preparedFoundation{}, violations, err
	}

	longform, violations, err := foundationLongformState(canonical["world.json"])
	if err != nil || len(violations) != 0 {
		return preparedFoundation{}, violations, err
	}

	planning, violations, err := foundationPlanningState(canonical["book_plan.json"])
	if err != nil || len(violations) != 0 {
		return preparedFoundation{}, violations, err
	}

	return preparedFoundation{
		Canonical: canonical,
		Mappings:  mappings,
		Longform:  longform,
		Planning:  planning,
	}, nil, nil
}

func (p *Project) writeActiveAttempt(projectState *domain.CoreProjectState, state *domain.CoreProductionState) error {
	if state.ActiveTask == nil || state.ActiveAttempt == nil {
		return fmt.Errorf("active task and attempt are required")
	}
	task := state.ActiveTask
	attempt := state.ActiveAttempt
	base := filepath.Join("exchange", "outbox", task.TaskID, attempt.AttemptID)
	taskDoc := map[string]any{
		"schema_version":     protocol.MachineSchemaVersion,
		"project_id":         projectState.ProjectID,
		"task_id":            task.TaskID,
		"task_kind":          task.Kind,
		"attempt_id":         attempt.AttemptID,
		"attempt_reason":     attempt.Reason,
		"target":             task.Target,
		"base_canon_root":    task.BaseCanonRoot,
		"protocol_version":   attempt.ProtocolVersion,
		"task_digest":        attempt.TaskDigest,
		"required_artifacts": attempt.RequiredArtifacts,
	}
	if err := writeWorkspaceJSON(projectState.WorkspaceRoot, filepath.Join(base, "task.json"), taskDoc); err != nil {
		return err
	}
	if err := writeWorkspaceJSON(projectState.WorkspaceRoot, filepath.Join(base, "constraints.json"), map[string]any{
		"required_artifacts":  attempt.RequiredArtifacts,
		"control_constraints": task.Constraints,
	}); err != nil {
		return err
	}
	contextRaw := []byte("{}")
	canonRaw, err := json.Marshal(map[string]any{"base_canon_root": task.BaseCanonRoot})
	if err != nil {
		return err
	}
	var recentProse []byte
	if task.Kind != "foundation" && task.BaseCanonRoot != "" {
		compiled, err := p.compileTaskPackContext(state)
		if err != nil {
			return err
		}
		contextRaw, canonRaw, recentProse = compiled.ContextJSON, compiled.CanonExcerptJSON, compiled.RecentProse
	}
	for name, data := range map[string][]byte{"context.json": contextRaw, "canon_excerpt.json": canonRaw, "recent_prose.md": recentProse} {
		if err := protocol.WriteUTF8Atomic(projectState.WorkspaceRoot, filepath.Join(base, name), data, 0o644); err != nil {
			return err
		}
	}
	status := "ready"
	blockID := ""
	if state.ActiveBlock != nil && state.ActiveBlock.AttemptID == attempt.AttemptID {
		status = "blocked"
		blockID = state.ActiveBlock.BlockID
	}
	if err := writeWorkspaceJSON(projectState.WorkspaceRoot, filepath.Join("exchange", "READY.json"), readyDocument{
		SchemaVersion:   protocol.MachineSchemaVersion,
		ProjectID:       projectState.ProjectID,
		TaskID:          task.TaskID,
		TaskKind:        task.Kind,
		AttemptID:       attempt.AttemptID,
		AttemptReason:   attempt.Reason,
		Target:          task.Target,
		BaseCanonRoot:   task.BaseCanonRoot,
		ProtocolVersion: attempt.ProtocolVersion,
		TaskDigest:      attempt.TaskDigest,
		CompletionNonce: attempt.CompletionNonce,
		Status:          status,
		BlockID:         blockID,
	}); err != nil {
		return err
	}
	return p.writeWorkspaceStatus(projectState, state)
}
func (p *Project) SettleFoundation(sub FoundationSubmission) (FoundationSettlement, error) {
	release, err := p.acquireProjectMutationLock()
	if err != nil {
		return FoundationSettlement{}, err
	}
	defer release()
	return p.settleFoundationLocked(sub)
}
func (p *Project) settleFoundationLocked(sub FoundationSubmission) (FoundationSettlement, error) {
	projectState, err := p.store.LoadCoreProjectState()
	if err != nil || projectState == nil {
		if err == nil {
			err = fmt.Errorf("project is not initialized")
		}
		return FoundationSettlement{}, err
	}
	state, err := p.store.LoadCoreProductionState()
	if err != nil || state == nil || state.ActiveTask == nil || state.ActiveAttempt == nil {
		if err == nil {
			err = fmt.Errorf("no active production attempt")
		}
		return FoundationSettlement{}, err
	}
	task, attempt := state.ActiveTask, state.ActiveAttempt
	if task.Kind != "foundation" {
		return FoundationSettlement{}, fmt.Errorf("active task is %q, not foundation", task.Kind)
	}
	if err := validateSubmissionIdentity(projectState, task, attempt, sub.Manifest); err != nil {
		return FoundationSettlement{}, err
	}
	prepared, violations, err := prepareFoundationArtifacts(sub.Artifacts, state)
	if err != nil {
		return FoundationSettlement{}, err
	}
	if len(violations) > 0 {
		return p.rejectFoundation(projectState, state, task, attempt, violations)
	}
	journal, err := p.prepareFoundationCommit(
		projectState,
		state,
		task,
		attempt,
		prepared,
		"",
	)
	if err != nil {
		return FoundationSettlement{}, err
	}
	return p.applyFoundationCommit(projectState, journal)
}
func validateSubmissionIdentity(project *domain.CoreProjectState, task *domain.CoreTask, attempt *domain.CoreAttempt, manifest protocol.SubmissionManifest) error {
	if attemptInputSource(attempt) != "drive" {
		return fmt.Errorf("active attempt does not accept Drive submission")
	}
	if manifest.SchemaVersion != protocol.MachineSchemaVersion {
		return fmt.Errorf("unsupported submission schema version %d", manifest.SchemaVersion)
	}
	if manifest.ProjectID != project.ProjectID || manifest.TaskID != task.TaskID || manifest.AttemptID != attempt.AttemptID {
		return fmt.Errorf("submission identity does not match active attempt")
	}
	if manifest.BaseCanonRoot != task.BaseCanonRoot || manifest.ProtocolVersion != attempt.ProtocolVersion || manifest.TaskDigest != attempt.TaskDigest || manifest.CompletionNonce != attempt.CompletionNonce {
		return fmt.Errorf("submission binding does not match active attempt")
	}
	required := make(map[string]bool, len(attempt.RequiredArtifacts))
	allowed := make(map[string]bool, len(attempt.RequiredArtifacts)+1)
	for _, name := range attempt.RequiredArtifacts {
		required[name] = true
		allowed[name] = true
	}
	if task.Kind == "chapter" || task.Kind == "revision" {
		allowed["planning_patch.json"] = true
	}
	seen := make(map[string]bool, len(manifest.Files))
	for _, name := range manifest.Files {
		if name == "" || name == "manifest.json" || strings.ContainsAny(name, "/\\") || seen[name] || !allowed[name] {
			return fmt.Errorf("submission file list does not match active attempt")
		}
		seen[name] = true
	}
	for name := range required {
		if !seen[name] {
			return fmt.Errorf("submission file list does not match active attempt")
		}
	}
	return nil
}

func (p *Project) settleDesignFoundationAttemptLocked() (FoundationSettlement, error) {
	project, err := p.store.LoadCoreProjectState()
	if err != nil || project == nil {
		if err == nil {
			err = fmt.Errorf("project is not initialized")
		}
		return FoundationSettlement{}, err
	}
	if project.DesignMode != domain.DesignModeRequired {
		return FoundationSettlement{}, fmt.Errorf("internal design foundation requires required design mode")
	}
	state, err := p.store.LoadCoreProductionState()
	if err != nil || state == nil {
		if err == nil {
			err = fmt.Errorf("production state is missing")
		}
		return FoundationSettlement{}, err
	}
	if state.CanonRoot != "" {
		return FoundationSettlement{}, fmt.Errorf("internal design foundation requires pre-canon state")
	}
	head, err := p.store.LoadCoreDesignHead()
	if err != nil {
		return FoundationSettlement{}, err
	}
	if head == nil || head.Checkpoint != domain.DesignCheckpointFoundationReady || head.DesignRoot == "" {
		return FoundationSettlement{}, fmt.Errorf("internal design foundation requires foundation_ready head")
	}
	task, attempt := state.ActiveTask, state.ActiveAttempt
	if task == nil || attempt == nil || task.Kind != "foundation" {
		return FoundationSettlement{}, fmt.Errorf("internal design foundation attempt is not active")
	}
	if attemptInputSource(attempt) != "design" {
		return FoundationSettlement{}, fmt.Errorf("foundation attempt input source is not design")
	}
	if attempt.InputRef != head.DesignRoot {
		return FoundationSettlement{}, fmt.Errorf("foundation attempt input ref does not match design head")
	}
	files, err := p.store.ReadCoreSnapshot(attempt.AttemptID, foundationArtifactNames)
	if err != nil {
		return FoundationSettlement{}, err
	}
	prepared, violations, err := prepareFoundationArtifacts(files, state)
	if err != nil {
		return FoundationSettlement{}, fmt.Errorf("foundation-ready snapshot integrity: %w", err)
	}
	if len(violations) != 0 {
		return FoundationSettlement{}, fmt.Errorf(
			"foundation-ready snapshot integrity violations: %s",
			strings.Join(violations, "; "),
		)
	}
	journal, err := p.prepareFoundationCommit(
		project,
		state,
		task,
		attempt,
		prepared,
		head.DesignRoot,
	)
	if err != nil {
		return FoundationSettlement{}, err
	}
	return p.applyFoundationCommit(project, journal)
}

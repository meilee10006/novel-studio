package core

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/chenhongyang/novel-studio/internal/domain"
	"github.com/chenhongyang/novel-studio/internal/protocol"
)

type ProtocolRetryResult struct {
	Result       string `json:"result"`
	OldAttemptID string `json:"old_attempt_id"`
	NewAttemptID string `json:"new_attempt_id"`
	Problem      string `json:"problem,omitempty"`
}

func (p *Project) RetryInvalidSubmission() (ProtocolRetryResult, error) {
	project, state, err := p.activeProduction()
	if err != nil {
		return ProtocolRetryResult{}, err
	}
	if state.ActiveBlock != nil {
		return ProtocolRetryResult{}, fmt.Errorf("project is blocked")
	}
	task, attempt := state.ActiveTask, state.ActiveAttempt
	record, err := p.store.LoadCoreSubmissionRecord(attempt.AttemptID)
	if err != nil {
		return ProtocolRetryResult{}, err
	}
	if record == nil || record.State != "INVALID" || record.Result != "" {
		return ProtocolRetryResult{}, fmt.Errorf("active submission is not retryable")
	}
	problem := record.Problem
	next, err := newAttempt(state, task, "retry", attempt.RequiredArtifacts, project.ProtocolVersion)
	if err != nil {
		return ProtocolRetryResult{}, err
	}
	record.State = "SETTLED"
	record.Result = "RETRY"
	record.Problem = problem
	if err := p.store.SaveCoreSubmissionRecord(record); err != nil {
		return ProtocolRetryResult{}, err
	}
	state.ActiveAttempt = next
	state.ActiveBlock = nil
	if err := p.store.SaveCoreProductionState(state); err != nil {
		return ProtocolRetryResult{}, err
	}
	if err := writeWorkspaceJSON(project.WorkspaceRoot, filepath.Join("exchange", "result", attempt.AttemptID+".json"), map[string]any{
		"schema_version": protocol.MachineSchemaVersion,
		"task_id":        task.TaskID,
		"attempt_id":     attempt.AttemptID,
		"result":         "RETRY",
		"problem":        problem,
		"new_attempt_id": next.AttemptID,
	}); err != nil {
		return ProtocolRetryResult{}, err
	}
	if err := p.writeActiveAttempt(project, state); err != nil {
		return ProtocolRetryResult{}, err
	}
	return ProtocolRetryResult{Result: "RETRY", OldAttemptID: attempt.AttemptID, NewAttemptID: next.AttemptID, Problem: problem}, nil
}
func detectAuthorDecision(files map[string][]byte, task *domain.CoreTask, attempt *domain.CoreAttempt) (*domain.CoreBlock, []string, error) {
	var review map[string]any
	if err := protocol.DecodeJSON(files["self_review.json"], &review); err != nil {
		return nil, nil, err
	}
	raw, exists := review["author_decision_required"]
	if !exists || raw == nil {
		return nil, nil, nil
	}
	decision, ok := raw.(map[string]any)
	if !ok {
		return nil, []string{"author_decision_required must be an object"}, nil
	}

	var contract map[string]any
	if err := protocol.DecodeJSON(files["chapter_contract.json"], &contract); err != nil {
		return nil, nil, err
	}
	known := map[string]bool{}
	if items, ok := contract["hard_constraints"].([]any); ok {
		for _, item := range items {
			if m, ok := item.(map[string]any); ok {
				if id, _ := m["id"].(string); strings.TrimSpace(id) != "" {
					known[strings.TrimSpace(id)] = true
				}
			}
		}
	}
	refs, refProblems := stringArray(decision["constraint_refs"], "author_decision_required.constraint_refs")
	options, optionProblems := stringArray(decision["options"], "author_decision_required.options")
	violations := append(refProblems, optionProblems...)
	conflict, _ := decision["conflict"].(string)
	conflict = strings.TrimSpace(conflict)
	if len(refs) == 0 {
		violations = append(violations, "author decision requires at least one hard constraint reference")
	}
	if conflict == "" {
		violations = append(violations, "author decision conflict is required")
	}
	if len(options) < 2 {
		violations = append(violations, "author decision requires at least two options")
	}
	for _, ref := range refs {
		if !known[ref] {
			violations = append(violations, "author decision references unknown hard constraint: "+ref)
		}
	}
	if len(violations) > 0 {
		sort.Strings(violations)
		return nil, violations, nil
	}
	sort.Strings(refs)
	digest, err := digestJSON(map[string]any{
		"task_id": task.TaskID, "attempt_id": attempt.AttemptID,
		"base_canon_root": task.BaseCanonRoot, "constraint_refs": refs,
		"conflict": conflict, "options": options,
	})
	if err != nil {
		return nil, nil, err
	}
	return &domain.CoreBlock{
		BlockID: "block-" + digest[:16], TaskID: task.TaskID, AttemptID: attempt.AttemptID,
		BaseCanonRoot: task.BaseCanonRoot, ConstraintRefs: refs, Conflict: conflict, Options: options,
	}, nil, nil
}
func stringArray(value any, label string) ([]string, []string) {
	items, ok := value.([]any)
	if !ok {
		return nil, []string{label + " must be an array"}
	}
	out := make([]string, 0, len(items))
	seen := map[string]bool{}
	var problems []string
	for _, item := range items {
		s, ok := item.(string)
		s = strings.TrimSpace(s)
		if !ok || s == "" {
			problems = append(problems, label+" entries must be non-empty strings")
			continue
		}
		if seen[s] {
			problems = append(problems, label+" contains duplicate value: "+s)
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out, problems
}

func (p *Project) blockChapter(project *domain.CoreProjectState, state *domain.CoreProductionState, task *domain.CoreTask, attempt *domain.CoreAttempt, record *domain.CoreSubmissionRecord, files map[string][]byte, block *domain.CoreBlock) (ChapterSettlement, error) {
	validationDigest, err := digestJSON(map[string]any{"result": "BLOCKED", "block": block})
	if err != nil {
		return ChapterSettlement{}, err
	}
	receipt := &domain.CoreReceipt{
		SchemaVersion: coreSchemaVersion, ProjectID: project.ProjectID,
		TaskID: task.TaskID, AttemptID: attempt.AttemptID, PreviousRoot: state.CanonRoot,
		TaskDigest: attempt.TaskDigest, SubmissionDigest: record.SnapshotDigest,
		ArtifactDigests: digestArtifacts(files), ValidationDigest: validationDigest,
		Result: "BLOCKED", NewRoot: state.CanonRoot, CommittedAt: time.Now().UTC().Format(time.RFC3339Nano),
	}
	rel, err := p.store.SaveCoreReceipt(receipt)
	if err != nil {
		return ChapterSettlement{}, err
	}
	record.State = "SETTLED"
	record.Result = "BLOCKED"
	record.NewCanonRoot = state.CanonRoot
	record.ReceiptPath = rel
	record.Problem = ""
	if err := p.store.SaveCoreSubmissionRecord(record); err != nil {
		return ChapterSettlement{}, err
	}
	state.ActiveBlock = block
	if err := p.store.SaveCoreProductionState(state); err != nil {
		return ChapterSettlement{}, err
	}
	if err := writeWorkspaceJSON(project.WorkspaceRoot, filepath.Join("exchange", "result", attempt.AttemptID+".json"), map[string]any{
		"schema_version": protocol.MachineSchemaVersion,
		"task_id":        task.TaskID, "attempt_id": attempt.AttemptID,
		"result": "BLOCKED", "block_id": block.BlockID,
		"constraint_refs": block.ConstraintRefs, "conflict": block.Conflict, "options": block.Options,
	}); err != nil {
		return ChapterSettlement{}, err
	}
	if err := p.writeActiveAttempt(project, state); err != nil {
		return ChapterSettlement{}, err
	}
	return ChapterSettlement{
		Result: "BLOCKED", BlockID: block.BlockID,
		ReceiptPath: filepath.Join(p.root, rel),
	}, nil
}

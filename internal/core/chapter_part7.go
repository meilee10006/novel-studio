package core

import (
	"fmt"
	"github.com/chenhongyang/novel-studio/internal/domain"
	"github.com/chenhongyang/novel-studio/internal/protocol"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

func validateCanonicalEventReferences(value any, known map[string]bool, violations *[]string) {
	switch x := value.(type) {
	case map[string]any:
		if raw, exists := x["event_canon_id"]; exists {
			id := cleanString(raw)
			if id != "" && !known[id] {
				*violations = append(*violations, "state delta references unknown canonical story event: "+id)
			}
		}
		for _, child := range x {
			validateCanonicalEventReferences(child, known, violations)
		}
	case []any:
		for _, child := range x {
			validateCanonicalEventReferences(child, known, violations)
		}
	}
}
func relationshipCharacterIDs(id string) (string, string, bool) {
	for _, sep := range []string{"|", ":"} {
		if strings.Count(id, sep) != 1 {
			continue
		}
		parts := strings.SplitN(id, sep, 2)
		left := strings.TrimSpace(parts[0])
		right := strings.TrimSpace(parts[1])
		if left != "" && right != "" {
			return left, right, true
		}
	}
	return "", "", false
}
func validateOptionalStoryEventStringArray(event map[string]any, field string, violations *[]string) {
	raw, exists := event[field]
	if !exists {
		return
	}
	items, ok := raw.([]any)
	if !ok {
		*violations = append(*violations, "story event "+field+" must be an array")
		return
	}
	for _, item := range items {
		if cleanString(item) == "" {
			*violations = append(*violations, "story event "+field+" entries must be non-empty strings")
			return
		}
	}
}
func validateOptionalStoryEventUniqueStringArray(event map[string]any, field string, violations *[]string) {
	raw, exists := event[field]
	if !exists {
		return
	}
	_, problems := stringArray(raw, "story event "+field)
	*violations = append(*violations, problems...)
}
func anySlice(value any) []any {
	items, _ := value.([]any)
	return items
}
func chapterNumberFromTarget(target string) (int, error) {
	const prefix = "chapter:"
	if !strings.HasPrefix(target, prefix) {
		return 0, fmt.Errorf("invalid chapter target %q", target)
	}
	n, err := strconv.Atoi(strings.TrimPrefix(target, prefix))
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("invalid chapter target %q", target)
	}
	return n, nil
}
func expandChapterReviewRewriteScope(feedback []RewriteFeedback, attempt *domain.CoreAttempt, files map[string][]byte) {
	if attempt == nil {
		return
	}
	hasReviewRewrite := false
	for _, item := range feedback {
		if item.Code == "chapter_review.revise" {
			hasReviewRewrite = true
			break
		}
	}
	if !hasReviewRewrite {
		return
	}

	seen := map[string]bool{}
	scope := make([]string, 0, len(attempt.RequiredArtifacts)+2)
	for _, name := range attempt.RequiredArtifacts {
		if name != "" && !seen[name] {
			seen[name] = true
			scope = append(scope, name)
		}
	}
	for _, name := range []string{"planning_patch.json", "arc_rehearsal.json"} {
		if _, submitted := files[name]; submitted && !seen[name] {
			seen[name] = true
			scope = append(scope, name)
		}
	}
	sort.Strings(scope)
	for i := range feedback {
		if feedback[i].Code == "chapter_review.revise" {
			feedback[i].AllowedScope = append([]string(nil), scope...)
		}
	}
}

func (p *Project) rejectChapter(project *domain.CoreProjectState, state *domain.CoreProductionState, task *domain.CoreTask, attempt *domain.CoreAttempt, record *domain.CoreSubmissionRecord, files map[string][]byte, violations []string, planningStatuses ...string) (ChapterSettlement, error) {
	sort.Strings(violations)
	feedback := buildRewriteFeedback(violations, task)
	expandChapterReviewRewriteScope(feedback, attempt, files)
	planningStatus := ""
	if len(planningStatuses) > 0 {
		planningStatus = planningStatuses[0]
	}
	validationDigest, err := digestJSON(map[string]any{"result": "REWRITE", "violations": violations, "rewrite_feedback": feedback, "planning_status": planningStatus})
	if err != nil {
		return ChapterSettlement{}, err
	}
	receipt := &domain.CoreReceipt{
		SchemaVersion: coreSchemaVersion, ProjectID: project.ProjectID,
		TaskID: task.TaskID, AttemptID: attempt.AttemptID,
		PreviousRoot: state.CanonRoot, TaskDigest: attempt.TaskDigest,
		SubmissionDigest: record.SnapshotDigest, ArtifactDigests: digestArtifacts(files),
		ValidationDigest: validationDigest, Result: "REWRITE", PlanningStatus: planningStatus, NewRoot: state.CanonRoot,
		CommittedAt: time.Now().UTC().Format(time.RFC3339Nano),
	}
	rel, err := p.store.SaveCoreReceipt(receipt)
	if err != nil {
		return ChapterSettlement{}, err
	}
	next, err := newAttempt(state, task, "rewrite", attempt.RequiredArtifacts, project.ProtocolVersion)
	if err != nil {
		return ChapterSettlement{}, err
	}
	state.ActiveAttempt = next
	if err := p.store.SaveCoreProductionState(state); err != nil {
		return ChapterSettlement{}, err
	}
	record.State = "SETTLED"
	record.Result = "REWRITE"
	record.NewCanonRoot = state.CanonRoot
	record.ReceiptPath = rel
	record.Problem = ""
	if err := p.store.SaveCoreSubmissionRecord(record); err != nil {
		return ChapterSettlement{}, err
	}
	if err := writeWorkspaceJSON(project.WorkspaceRoot, filepath.Join("exchange", "result", attempt.AttemptID+".json"), map[string]any{
		"schema_version": protocol.MachineSchemaVersion,
		"task_id":        task.TaskID, "attempt_id": attempt.AttemptID,
		"result": "REWRITE", "violations": violations, "rewrite_feedback": feedback,
	}); err != nil {
		return ChapterSettlement{}, err
	}
	if err := p.writeActiveAttempt(project, state); err != nil {
		return ChapterSettlement{}, err
	}
	return ChapterSettlement{
		Result: "REWRITE", Violations: violations, RewriteFeedback: feedback, PlanningStatus: planningStatus,
		ReceiptPath: filepath.Join(p.root, rel),
	}, nil
}
func (p *Project) acceptChapter(project *domain.CoreProjectState, state *domain.CoreProductionState, task *domain.CoreTask, attempt *domain.CoreAttempt, record *domain.CoreSubmissionRecord, received, canonical map[string][]byte, mappings []IDMapping, chapter int, longform domain.CoreLongformState, planning domain.CorePlanningState, planningStatus string, planningRepair bool) (ChapterSettlement, error) {
	journal, err := p.prepareChapterCommit(project, state, task, attempt, record, received, canonical, mappings, chapter, longform, planning, planningStatus, planningRepair)
	if err != nil {
		return ChapterSettlement{}, err
	}
	return p.applyChapterCommit(project, journal)
}
func digestCanonArtifact(name string, data []byte) (string, error) {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".json":
		canonical, err := canonicalJSON(data)
		if err != nil {
			return "", err
		}
		return sha256Bytes(canonical), nil
	case ".md", ".txt":
		return sha256Bytes(data), nil
	default:
		return "", fmt.Errorf("unsupported canon artifact %q", name)
	}
}

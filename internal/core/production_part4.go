package core

import (
	"fmt"
	"github.com/chenhongyang/novel-studio/internal/domain"
	"github.com/chenhongyang/novel-studio/internal/protocol"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func rewriteFoundationRefs(value any, lookup map[string]string, violations *[]string) {
	switch x := value.(type) {
	case map[string]any:
		entityType, _ := x["entity_type"].(string)
		if rawRef, exists := x["local_ref"]; exists {
			if _, canonExists := x["canon_id"]; canonExists {
				*violations = append(*violations, "foundation reference must not predeclare canonical id")
				return
			}
			localRef, ok := rawRef.(string)
			if !ok || strings.TrimSpace(localRef) == "" {
				*violations = append(*violations, "local_ref must be a non-empty string")
			} else {
				key := strings.TrimSpace(entityType) + "\x00" + strings.TrimSpace(localRef)
				if id := lookup[key]; id != "" {
					x["canon_id"] = id
					delete(x, "local_ref")
				} else {
					*violations = append(*violations, "unknown local reference: "+strings.TrimSpace(entityType)+"/"+strings.TrimSpace(localRef))
				}
			}
		}
		for _, child := range x {
			rewriteFoundationRefs(child, lookup, violations)
		}
	case []any:
		for _, child := range x {
			rewriteFoundationRefs(child, lookup, violations)
		}
	}
}
func exactArtifactSet(artifacts map[string][]byte, expected []string) error {
	got := make([]string, 0, len(artifacts))
	for name := range artifacts {
		got = append(got, name)
	}
	want := append([]string(nil), expected...)
	sort.Strings(got)
	sort.Strings(want)
	if strings.Join(got, "\x00") != strings.Join(want, "\x00") {
		return fmt.Errorf("artifact set does not match task contract")
	}
	return nil
}
func safeIDPrefix(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
		} else if b.Len() > 0 && !strings.HasSuffix(b.String(), "-") {
			b.WriteByte('-')
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "entity"
	}
	return out
}
func (p *Project) rejectFoundation(project *domain.CoreProjectState, state *domain.CoreProductionState, task *domain.CoreTask, attempt *domain.CoreAttempt, violations []string) (FoundationSettlement, error) {
	sort.Strings(violations)
	feedback := buildRewriteFeedback(violations, task)
	validationDigest, _ := digestJSON(map[string]any{"result": "REWRITE", "violations": violations, "rewrite_feedback": feedback})
	receipt := &domain.CoreReceipt{
		SchemaVersion: coreSchemaVersion, ProjectID: project.ProjectID,
		TaskID: task.TaskID, AttemptID: attempt.AttemptID,
		PreviousRoot: state.CanonRoot, TaskDigest: attempt.TaskDigest,
		ValidationDigest: validationDigest, Result: "REWRITE", NewRoot: state.CanonRoot,
		CommittedAt: time.Now().UTC().Format(time.RFC3339Nano),
	}
	rel, err := p.store.SaveCoreReceipt(receipt)
	if err != nil {
		return FoundationSettlement{}, err
	}
	next, err := newAttempt(state, task, "rewrite", foundationArtifactNames, project.ProtocolVersion)
	if err != nil {
		return FoundationSettlement{}, err
	}
	state.ActiveAttempt = next
	if err := p.store.SaveCoreProductionState(state); err != nil {
		return FoundationSettlement{}, err
	}
	if err := writeWorkspaceJSON(project.WorkspaceRoot, filepath.Join("exchange", "result", attempt.AttemptID+".json"), map[string]any{
		"schema_version": protocol.MachineSchemaVersion, "task_id": task.TaskID,
		"attempt_id": attempt.AttemptID, "result": "REWRITE", "violations": violations, "rewrite_feedback": feedback,
	}); err != nil {
		return FoundationSettlement{}, err
	}
	if err := p.writeActiveAttempt(project, state); err != nil {
		return FoundationSettlement{}, err
	}
	return FoundationSettlement{Result: "REWRITE", Violations: violations, RewriteFeedback: feedback, ReceiptPath: filepath.Join(p.root, rel)}, nil
}

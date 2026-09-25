package core

import (
	"encoding/json"
	"reflect"
	"testing"
)

var chapterReviewDimensions = []string{
	"story_progress",
	"reader_payoff",
	"pacing",
	"character_consistency",
	"world_consistency",
	"continuity",
	"ending_hook",
}

func TestLegacyActiveChapterAttemptStillAcceptsWithoutChapterReview(t *testing.T) {
	project, _, workspace, ready := acceptedFoundationProject(t)
	got := submitAndSettleChapter(t, project, workspace, ready, longformChapterArtifacts(1, nil))
	if got.Result != "ACCEPTED" {
		t.Fatalf("legacy settlement=%+v", got)
	}
}

func TestQualityChapterReviewPassAllowsExistingChapterValidation(t *testing.T) {
	project, _, workspace, _ := acceptedFoundationProject(t)
	ready := enableChapterReviewForActiveAttempt(t, project)
	artifacts := longformChapterArtifacts(1, nil)
	artifacts["chapter_review.json"] = validChapterReview(t, "pass", nil, nil)

	got := submitAndSettleChapter(t, project, workspace, ready, artifacts)
	if got.Result != "ACCEPTED" {
		t.Fatalf("settlement=%+v", got)
	}
	stored, err := project.store.ReadCoreCanonArtifact("chapters/000001/chapter_review.json")
	if err != nil {
		t.Fatal(err)
	}
	var review map[string]any
	if err := json.Unmarshal(stored, &review); err != nil {
		t.Fatal(err)
	}
	if review["verdict"] != "pass" || review["subject_ref"] != "chapter.md" {
		t.Fatalf("stored review=%+v", review)
	}
}

func TestQualityChapterReviewRejectsStructuralAndVerdictContradictions(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(map[string]any)
		needle string
	}{
		{
			name: "wrong subject",
			mutate: func(review map[string]any) {
				review["subject_ref"] = "other.md"
			},
			needle: "subject_ref",
		},
		{
			name: "missing dimension",
			mutate: func(review map[string]any) {
				delete(review["dimensions"].(map[string]any), "pacing")
			},
			needle: "pacing",
		},
		{
			name: "invalid dimension status",
			mutate: func(review map[string]any) {
				review["dimensions"].(map[string]any)["pacing"].(map[string]any)["status"] = "maybe"
			},
			needle: "status",
		},
		{
			name: "pass with revise dimension",
			mutate: func(review map[string]any) {
				review["dimensions"].(map[string]any)["pacing"].(map[string]any)["status"] = "revise"
			},
			needle: "verdict=pass",
		},
		{
			name: "pass with blocking issue",
			mutate: func(review map[string]any) {
				review["issues"] = []any{map[string]any{
					"code":            "weak-payoff",
					"severity":        "blocking",
					"description":     "payoff is too weak",
					"evidence_anchor": "章节正文",
				}}
			},
			needle: "verdict=pass",
		},
		{
			name: "blocking issue missing anchor from exact draft",
			mutate: func(review map[string]any) {
				review["verdict"] = "revise"
				review["issues"] = []any{map[string]any{
					"code":            "weak-payoff",
					"severity":        "blocking",
					"description":     "payoff is too weak",
					"evidence_anchor": "这段文字不存在",
				}}
			},
			needle: "evidence_anchor",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			project, _, workspace, _ := acceptedFoundationProject(t)
			ready := enableChapterReviewForActiveAttempt(t, project)
			artifacts := longformChapterArtifacts(1, nil)
			review := validChapterReviewMap("pass")
			tc.mutate(review)
			raw, err := json.Marshal(review)
			if err != nil {
				t.Fatal(err)
			}
			artifacts["chapter_review.json"] = raw

			got := submitAndSettleChapter(t, project, workspace, ready, artifacts)
			if got.Result != "REWRITE" || !containsViolation(got.Violations, tc.needle) {
				t.Fatalf("settlement=%+v", got)
			}
		})
	}
}

func TestQualityChapterReviewReviseForcesRewriteWithoutMovingCanon(t *testing.T) {
	project, _, workspace, _ := acceptedFoundationProject(t)
	before, err := project.Status()
	if err != nil {
		t.Fatal(err)
	}
	ready := enableChapterReviewForActiveAttempt(t, project)
	stateBefore, err := project.store.LoadCoreProductionState()
	if err != nil {
		t.Fatal(err)
	}
	requiredBefore := append([]string(nil), stateBefore.ActiveAttempt.RequiredArtifacts...)

	artifacts := longformChapterArtifacts(1, nil)
	artifacts["chapter_review.json"] = validChapterReview(t, "revise", map[string]string{
		"pacing": "revise",
	}, []map[string]any{{
		"code":            "pace-stall",
		"severity":        "blocking",
		"description":     "middle beat stalls",
		"evidence_anchor": "主角看见门口的灯",
		"suggested_fix":   "compress the transition",
	}})

	got := submitAndSettleChapter(t, project, workspace, ready, artifacts)
	if got.Result != "REWRITE" || !containsViolation(got.Violations, "verdict=revise") {
		t.Fatalf("settlement=%+v", got)
	}

	after, err := project.Status()
	if err != nil {
		t.Fatal(err)
	}
	if after.CanonRoot != before.CanonRoot || after.ActiveTarget != "chapter:1" || after.ActiveAttemptID == ready.AttemptID {
		t.Fatalf("rewrite moved authority: before=%+v after=%+v", before, after)
	}

	stateAfter, err := project.store.LoadCoreProductionState()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(requiredBefore, stateAfter.ActiveAttempt.RequiredArtifacts) {
		t.Fatalf("required artifacts changed: before=%v after=%v", requiredBefore, stateAfter.ActiveAttempt.RequiredArtifacts)
	}
	if len(got.RewriteFeedback) == 0 || got.RewriteFeedback[0].Code != "chapter_review.revise" {
		t.Fatalf("feedback=%+v", got.RewriteFeedback)
	}
}

func enableChapterReviewForActiveAttempt(t *testing.T, project *Project) readyView {
	t.Helper()
	projectState, err := project.store.LoadCoreProjectState()
	if err != nil {
		t.Fatal(err)
	}
	state, err := project.store.LoadCoreProductionState()
	if err != nil {
		t.Fatal(err)
	}
	if projectState == nil || state == nil || state.ActiveTask == nil || state.ActiveAttempt == nil {
		t.Fatal("active production is missing")
	}
	required := append([]string(nil), state.ActiveAttempt.RequiredArtifacts...)
	required = append(required, "chapter_review.json")
	attempt, err := newAttempt(state, state.ActiveTask, "quality-review-test", required, projectState.ProtocolVersion)
	if err != nil {
		t.Fatal(err)
	}
	state.ActiveAttempt = attempt
	state.ActiveBlock = nil
	if err := project.store.SaveCoreProductionState(state); err != nil {
		t.Fatal(err)
	}
	if err := project.writeActiveAttempt(projectState, state); err != nil {
		t.Fatal(err)
	}
	return readReady(t, projectState.WorkspaceRoot)
}

func validChapterReview(t *testing.T, verdict string, statuses map[string]string, issues []map[string]any) []byte {
	t.Helper()
	review := validChapterReviewMap(verdict)
	if statuses != nil {
		dims := review["dimensions"].(map[string]any)
		for name, status := range statuses {
			dims[name].(map[string]any)["status"] = status
		}
	}
	if issues != nil {
		values := make([]any, 0, len(issues))
		for _, issue := range issues {
			values = append(values, issue)
		}
		review["issues"] = values
	}
	raw, err := json.Marshal(review)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func validChapterReviewMap(verdict string) map[string]any {
	dims := make(map[string]any, len(chapterReviewDimensions))
	for _, name := range chapterReviewDimensions {
		dims[name] = map[string]any{"status": "pass", "note": "checked " + name}
	}
	return map[string]any{
		"subject_ref": "chapter.md",
		"verdict":     verdict,
		"dimensions":  dims,
		"issues":      []any{},
	}
}

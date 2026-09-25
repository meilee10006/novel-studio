package core

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestNewChapterAttemptsUseQualityArtifactContract(t *testing.T) {
	project, _, workspace, ready := acceptedFoundationProject(t)
	assertQualityAttemptContract(t, project)

	got := submitAndSettleChapter(t, project, workspace, ready, longformChapterArtifacts(1, nil))
	if got.Result != "ACCEPTED" {
		t.Fatalf("chapter settlement=%+v", got)
	}
	assertQualityAttemptContract(t, project)

	revision := startHistoricalRevision(t, project, workspace, got.NewCanonRoot, 1, "重写第一章")
	if revision.Target != "chapter:1" {
		t.Fatalf("revision=%+v", revision)
	}
	assertQualityAttemptContract(t, project)
}

func TestQualityChapterPlanRejectsInvalidStructure(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(map[string]any)
		needle string
	}{
		{"wrong chapter", func(p map[string]any) { p["chapter"] = float64(2) }, "chapter_plan.chapter"},
		{"wrong base", func(p map[string]any) { p["base_canon_root"] = "wrong-root" }, "base_canon_root"},
		{"missing objective", func(p map[string]any) { p["objective"] = "" }, "objective"},
		{"missing payoff", func(p map[string]any) { p["reader_payoff"] = "" }, "reader_payoff"},
		{"missing hook", func(p map[string]any) { p["ending_hook_intent"] = "" }, "ending_hook_intent"},
		{"too few beats", func(p map[string]any) {
			p["beats"] = []any{map[string]any{"id": "beat-1", "intent": "only"}}
		}, "at least two"},
		{"duplicate beat ids", func(p map[string]any) {
			p["beats"] = []any{
				map[string]any{"id": "same", "intent": "one"},
				map[string]any{"id": "same", "intent": "two"},
			}
		}, "unique"},
		{"empty beat intent", func(p map[string]any) {
			p["beats"] = []any{
				map[string]any{"id": "beat-1", "intent": ""},
				map[string]any{"id": "beat-2", "intent": "two"},
			}
		}, "intent"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			project, _, workspace, ready := acceptedFoundationProject(t)
			artifacts := longformChapterArtifacts(1, nil)
			var plan map[string]any
			if err := json.Unmarshal(artifacts["chapter_plan.json"], &plan); err != nil {
				t.Fatal(err)
			}
			tc.mutate(plan)
			raw, err := json.Marshal(plan)
			if err != nil {
				t.Fatal(err)
			}
			artifacts["chapter_plan.json"] = raw
			got := submitAndSettleChapter(t, project, workspace, ready, artifacts)
			if got.Result != "REWRITE" || !containsViolation(got.Violations, tc.needle) {
				t.Fatalf("settlement=%+v", got)
			}
		})
	}
}

func TestQualityChapterReviewRequiresPlanRef(t *testing.T) {
	project, _, workspace, ready := acceptedFoundationProject(t)
	artifacts := longformChapterArtifacts(1, nil)
	var review map[string]any
	if err := json.Unmarshal(artifacts["chapter_review.json"], &review); err != nil {
		t.Fatal(err)
	}
	delete(review, "plan_ref")
	raw, err := json.Marshal(review)
	if err != nil {
		t.Fatal(err)
	}
	artifacts["chapter_review.json"] = raw
	got := submitAndSettleChapter(t, project, workspace, ready, artifacts)
	if got.Result != "REWRITE" || !containsViolation(got.Violations, "plan_ref") {
		t.Fatalf("settlement=%+v", got)
	}
}

func TestAcceptedQualityPlanAndReviewAreCanonArtifacts(t *testing.T) {
	project, _, workspace, ready := acceptedFoundationProject(t)
	got := submitAndSettleChapter(t, project, workspace, ready, longformChapterArtifacts(1, nil))
	if got.Result != "ACCEPTED" {
		t.Fatalf("settlement=%+v", got)
	}
	for _, name := range []string{
		"chapters/000001/chapter_plan.json",
		"chapters/000001/chapter_review.json",
	} {
		if _, err := project.store.ReadCoreCanonArtifact(name); err != nil {
			t.Fatalf("missing canon quality artifact %s: %v", name, err)
		}
	}
}

func TestQualityPlanRewritePreservesArtifactContract(t *testing.T) {
	project, _, workspace, ready := acceptedFoundationProject(t)
	stateBefore, err := project.store.LoadCoreProductionState()
	if err != nil {
		t.Fatal(err)
	}
	requiredBefore := append([]string(nil), stateBefore.ActiveAttempt.RequiredArtifacts...)

	artifacts := longformChapterArtifacts(1, nil)
	var plan map[string]any
	if err := json.Unmarshal(artifacts["chapter_plan.json"], &plan); err != nil {
		t.Fatal(err)
	}
	plan["objective"] = ""
	artifacts["chapter_plan.json"], _ = json.Marshal(plan)

	got := submitAndSettleChapter(t, project, workspace, ready, artifacts)
	if got.Result != "REWRITE" || len(got.RewriteFeedback) == 0 || got.RewriteFeedback[0].Code != "chapter_plan.invalid" {
		t.Fatalf("settlement=%+v", got)
	}
	stateAfter, err := project.store.LoadCoreProductionState()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(requiredBefore, stateAfter.ActiveAttempt.RequiredArtifacts) {
		t.Fatalf("required artifacts drifted: before=%v after=%v", requiredBefore, stateAfter.ActiveAttempt.RequiredArtifacts)
	}
}

func assertQualityAttemptContract(t *testing.T, project *Project) {
	t.Helper()
	state, err := project.store.LoadCoreProductionState()
	if err != nil {
		t.Fatal(err)
	}
	if state == nil || state.ActiveAttempt == nil {
		t.Fatal("active attempt missing")
	}
	got := map[string]bool{}
	for _, name := range state.ActiveAttempt.RequiredArtifacts {
		got[name] = true
	}
	for _, name := range qualityChapterArtifactNames {
		if !got[name] {
			t.Fatalf("quality attempt missing %s: %v", name, state.ActiveAttempt.RequiredArtifacts)
		}
	}
	if len(got) != len(qualityChapterArtifactNames) {
		t.Fatalf("unexpected quality artifact contract: %v", state.ActiveAttempt.RequiredArtifacts)
	}
}

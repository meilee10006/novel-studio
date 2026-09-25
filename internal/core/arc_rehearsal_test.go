package core

import (
	"encoding/json"
	"testing"
)

func TestQualityPlanningPatchRequiresArcRehearsal(t *testing.T) {
	project, workspace, ready := planningChapterReadyProject(t, 3, 1)
	artifacts := validChapterArtifacts(1)
	artifacts["planning_patch.json"] = []byte(`{"next_arc":{"id":"arc-2","start_chapter":4,"end_chapter":6,"goal":"第二弧目标"}}`)

	got := submitAndSettleChapter(t, project, workspace, ready, artifacts)
	if got.Result != "REWRITE" || !containsViolation(got.Violations, "arc_rehearsal.json") {
		t.Fatalf("settlement=%+v", got)
	}
	if len(got.RewriteFeedback) == 0 || got.RewriteFeedback[0].Code != "arc_rehearsal.invalid" {
		t.Fatalf("feedback=%+v", got.RewriteFeedback)
	}
}

func TestQualityArcRehearsalRequiresPlanningPatch(t *testing.T) {
	project, workspace, ready := planningChapterReadyProject(t, 3, 1)
	artifacts := validChapterArtifacts(1)
	artifacts["arc_rehearsal.json"] = validArcRehearsal(t, []byte(`{"next_arc":{"id":"arc-2","start_chapter":4,"end_chapter":6,"goal":"第二弧目标"}}`))

	got := submitAndSettleChapter(t, project, workspace, ready, artifacts)
	if got.Result != "REWRITE" || !containsViolation(got.Violations, "planning_patch.json") {
		t.Fatalf("settlement=%+v", got)
	}
}

func TestQualityArcRehearsalRejectsInvalidSelection(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(map[string]any)
		needle string
	}{
		{
			name: "wrong subject",
			mutate: func(rehearsal map[string]any) {
				rehearsal["subject_ref"] = "other.json"
			},
			needle: "subject_ref",
		},
		{
			name: "missing selection reason",
			mutate: func(rehearsal map[string]any) {
				rehearsal["selection_reason"] = ""
			},
			needle: "selection_reason",
		},
		{
			name: "missing opportunity",
			mutate: func(rehearsal map[string]any) {
				items := rehearsal["scenarios"].([]any)
				items[0].(map[string]any)["opportunity"] = ""
			},
			needle: "opportunity",
		},
		{
			name: "missing risk",
			mutate: func(rehearsal map[string]any) {
				items := rehearsal["scenarios"].([]any)
				items[0].(map[string]any)["risk"] = ""
			},
			needle: "risk",
		},
		{
			name: "invalid next arc",
			mutate: func(rehearsal map[string]any) {
				items := rehearsal["scenarios"].([]any)
				items[1].(map[string]any)["next_arc"].(map[string]any)["start_chapter"] = float64(0)
			},
			needle: "next_arc",
		},
		{
			name: "fewer than two scenarios",
			mutate: func(rehearsal map[string]any) {
				rehearsal["scenarios"] = rehearsal["scenarios"].([]any)[:1]
			},
			needle: "at least two",
		},
		{
			name: "duplicate scenario id",
			mutate: func(rehearsal map[string]any) {
				items := rehearsal["scenarios"].([]any)
				items[1].(map[string]any)["id"] = items[0].(map[string]any)["id"]
			},
			needle: "unique",
		},
		{
			name: "unknown selected id",
			mutate: func(rehearsal map[string]any) {
				rehearsal["selected_scenario_id"] = "missing"
			},
			needle: "selected_scenario_id",
		},
		{
			name: "selected arc differs from patch",
			mutate: func(rehearsal map[string]any) {
				items := rehearsal["scenarios"].([]any)
				items[0].(map[string]any)["next_arc"].(map[string]any)["goal"] = "不是 patch 的目标"
			},
			needle: "selected scenario",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			project, workspace, ready := planningChapterReadyProject(t, 3, 1)
			patch := []byte(`{"next_arc":{"id":"arc-2","start_chapter":4,"end_chapter":6,"goal":"第二弧目标"}}`)
			artifacts := validChapterArtifacts(1)
			artifacts["planning_patch.json"] = patch

			var rehearsal map[string]any
			if err := json.Unmarshal(validArcRehearsal(t, patch), &rehearsal); err != nil {
				t.Fatal(err)
			}
			tc.mutate(rehearsal)
			raw, err := json.Marshal(rehearsal)
			if err != nil {
				t.Fatal(err)
			}
			artifacts["arc_rehearsal.json"] = raw

			got := submitAndSettleChapter(t, project, workspace, ready, artifacts)
			if got.Result != "REWRITE" || !containsViolation(got.Violations, tc.needle) {
				t.Fatalf("settlement=%+v", got)
			}
		})
	}
}

func TestQualityValidArcRehearsalAllowsPlanningPatchAndCommitsEvidence(t *testing.T) {
	project, workspace, ready := planningChapterReadyProject(t, 3, 1)
	patch := []byte(`{"next_arc":{"id":"arc-2","start_chapter":4,"end_chapter":6,"goal":"第二弧目标"}}`)
	artifacts := validChapterArtifacts(1)
	artifacts["planning_patch.json"] = patch
	artifacts["arc_rehearsal.json"] = validArcRehearsal(t, patch)

	got := submitAndSettleChapter(t, project, workspace, ready, artifacts)
	if got.Result != "ACCEPTED" || got.PlanningStatus != "accepted" {
		t.Fatalf("settlement=%+v", got)
	}
	if _, err := project.store.ReadCoreCanonArtifact("chapters/000001/arc_rehearsal.json"); err != nil {
		t.Fatalf("arc rehearsal missing from Canon: %v", err)
	}
}

func TestQualityRejectedOptionalPlanningDoesNotRejectLegalChapterWhenRehearsalPresent(t *testing.T) {
	project, workspace, ready := planningChapterReadyProject(t, 3, 1)
	patch := []byte(`{"next_arc":{"id":"arc-2","start_chapter":9,"end_chapter":10,"goal":"错误起点"}}`)
	artifacts := validChapterArtifacts(1)
	artifacts["planning_patch.json"] = patch
	artifacts["arc_rehearsal.json"] = validArcRehearsal(t, patch)

	got := submitAndSettleChapter(t, project, workspace, ready, artifacts)
	if got.Result != "ACCEPTED" || got.PlanningStatus != "rejected" {
		t.Fatalf("settlement=%+v", got)
	}
	if _, err := project.store.ReadCoreCanonArtifact("chapters/000001/arc_rehearsal.json"); err == nil {
		t.Fatal("rejected planning rehearsal entered Canon")
	}
}

func TestLegacyPlanningPatchDoesNotRequireArcRehearsal(t *testing.T) {
	project, workspace, _ := planningChapterReadyProject(t, 3, 1)
	ready := enableLegacyChapterForActiveAttempt(t, project)
	artifacts := longformChapterArtifacts(1, nil)
	artifacts["planning_patch.json"] = []byte(`{"next_arc":{"id":"arc-2","start_chapter":4,"end_chapter":6,"goal":"兼容旧 patch-only"}}`)

	got := submitAndSettleChapter(t, project, workspace, ready, artifacts)
	if got.Result != "ACCEPTED" || got.PlanningStatus != "accepted" {
		t.Fatalf("legacy patch-only settlement=%+v", got)
	}
}

func TestPlanningRepairRequiresPatchAndArcRehearsalOnQualityAttempt(t *testing.T) {
	project, workspace, ready := planningChapterReadyProject(t, 1, 1)
	first := submitAndSettleChapter(t, project, workspace, ready, validChapterArtifacts(1))
	if first.Result != "ACCEPTED" {
		t.Fatalf("first settlement=%+v", first)
	}

	state, err := project.store.LoadCoreProductionState()
	if err != nil {
		t.Fatal(err)
	}
	if state == nil || state.ActiveAttempt == nil {
		t.Fatal("next active attempt missing")
	}
	required := map[string]bool{}
	for _, name := range state.ActiveAttempt.RequiredArtifacts {
		required[name] = true
	}
	for _, name := range []string{"planning_patch.json", "arc_rehearsal.json"} {
		if !required[name] {
			t.Fatalf("planning repair missing %s: %v", name, state.ActiveAttempt.RequiredArtifacts)
		}
	}
}

func validArcRehearsal(t *testing.T, patchRaw []byte) []byte {
	t.Helper()
	var patch map[string]any
	if err := json.Unmarshal(patchRaw, &patch); err != nil {
		t.Fatal(err)
	}
	selected, ok := patch["next_arc"].(map[string]any)
	if !ok {
		t.Fatal("patch next_arc missing")
	}
	alternative := map[string]any{
		"id":            "arc-alt",
		"start_chapter": selected["start_chapter"],
		"end_chapter":   selected["end_chapter"],
		"goal":          "替代走向",
	}
	rehearsal := map[string]any{
		"subject_ref": "planning_patch.json",
		"scenarios": []any{
			map[string]any{
				"id": "route-a", "next_arc": selected,
				"opportunity": "强化当前读者期待", "risk": "结构可能重复",
			},
			map[string]any{
				"id": "route-b", "next_arc": alternative,
				"opportunity": "提供不同冲突结构", "risk": "需要重新铺垫",
			},
		},
		"selected_scenario_id": "route-a",
		"selection_reason":     "更贴合当前推进方向",
	}
	raw, err := json.Marshal(rehearsal)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

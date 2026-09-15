package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInvalidPlanningPatchDoesNotRejectLegalChapter(t *testing.T) {
	project, workspace, ready := planningChapterReadyProject(t, 3, 1)
	artifacts := validChapterArtifacts(1)
	artifacts["planning_patch.json"] = []byte(`{"next_arc":{"id":"arc-2","start_chapter":9,"end_chapter":10,"goal":"错误起点"}}`)
	got := submitAndSettleChapter(t, project, workspace, ready, artifacts)
	if got.Result != "ACCEPTED" || got.PlanningStatus != "rejected" {
		t.Fatalf("settlement=%+v", got)
	}
	if got.NewCanonRoot == "" {
		t.Fatal("legal prose was not committed")
	}
	assertNextTaskRequiresPlanningRepair(t, workspace)
}

func TestArcBoundaryWithoutPlanRequiresRepairOnNextChapter(t *testing.T) {
	project, workspace, ready := planningChapterReadyProject(t, 1, 1)
	got := submitAndSettleChapter(t, project, workspace, ready, validChapterArtifacts(1))
	if got.Result != "ACCEPTED" || got.PlanningStatus != "not_present" {
		t.Fatalf("settlement=%+v", got)
	}
	assertNextTaskRequiresPlanningRepair(t, workspace)
}
func TestValidPlanningPatchAdvancesArcWithoutRepair(t *testing.T) {
	project, workspace, ready := planningChapterReadyProject(t, 1, 1)
	artifacts := validChapterArtifacts(1)
	artifacts["planning_patch.json"] = []byte(`{"next_arc":{"id":"arc-2","start_chapter":2,"end_chapter":4,"goal":"进入人物互疑阶段"}}`)
	got := submitAndSettleChapter(t, project, workspace, ready, artifacts)
	if got.Result != "ACCEPTED" || got.PlanningStatus != "accepted" {
		t.Fatalf("settlement=%+v", got)
	}
	next := readReady(t, workspace)
	base := filepath.Join(workspace, "exchange", "outbox", next.TaskID, next.AttemptID)
	taskRaw, err := os.ReadFile(filepath.Join(base, "task.json"))
	if err != nil {
		t.Fatal(err)
	}
	constraintsRaw, err := os.ReadFile(filepath.Join(base, "constraints.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(taskRaw), "planning_patch.json") || strings.Contains(string(constraintsRaw), "planning_repair_required") {
		t.Fatalf("valid patch still required repair task=%s constraints=%s", taskRaw, constraintsRaw)
	}
	contextRaw, err := os.ReadFile(filepath.Join(base, "context.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(contextRaw), "进入人物互疑阶段") {
		t.Fatalf("next context missing advanced arc: %s", contextRaw)
	}
	var receipt map[string]any
	readJSONFile(t, got.ReceiptPath, &receipt)
	if receipt["planning_status"] != "accepted" {
		t.Fatalf("receipt=%+v", receipt)
	}
}

func TestPlanningLeadAppliesToFirstChapterTask(t *testing.T) {
	_, workspace, ready := planningChapterReadyProject(t, 2, 1)
	base := filepath.Join(workspace, "exchange", "outbox", ready.TaskID, ready.AttemptID)
	taskRaw, err := os.ReadFile(filepath.Join(base, "task.json"))
	if err != nil {
		t.Fatal(err)
	}
	constraintsRaw, err := os.ReadFile(filepath.Join(base, "constraints.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(taskRaw), "planning_patch.json") {
		t.Fatalf("planning lead made first patch mandatory: %s", taskRaw)
	}
	if !strings.Contains(string(constraintsRaw), "rolling_planning_due") {
		t.Fatalf("planning lead obligation missing from first chapter constraints=%s", constraintsRaw)
	}
}

func TestPlanningLeadAddsNonBlockingObligationBeforeBoundary(t *testing.T) {
	project, workspace, ready := planningChapterReadyProject(t, 3, 1)
	first := submitAndSettleChapter(t, project, workspace, ready, validChapterArtifacts(1))
	if first.Result != "ACCEPTED" || first.PlanningStatus != "not_present" {
		t.Fatalf("first settlement=%+v", first)
	}

	next := readReady(t, workspace)
	base := filepath.Join(workspace, "exchange", "outbox", next.TaskID, next.AttemptID)
	taskRaw, err := os.ReadFile(filepath.Join(base, "task.json"))
	if err != nil {
		t.Fatal(err)
	}
	constraintsRaw, err := os.ReadFile(filepath.Join(base, "constraints.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(taskRaw), "planning_patch.json") {
		t.Fatalf("planning lead made patch mandatory before boundary: %s", taskRaw)
	}
	if !strings.Contains(string(constraintsRaw), "rolling_planning_due") {
		t.Fatalf("planning lead obligation missing constraints=%s", constraintsRaw)
	}
}

func TestPlanningPatchBeforeBoundaryStaysQueued(t *testing.T) {
	project, workspace, ready := planningChapterReadyProject(t, 3, 1)
	artifacts := validChapterArtifacts(1)
	artifacts["planning_patch.json"] = []byte(`{"next_arc":{"id":"arc-2","start_chapter":4,"end_chapter":6,"goal":"第二弧目标"}}`)
	got := submitAndSettleChapter(t, project, workspace, ready, artifacts)
	if got.Result != "ACCEPTED" || got.PlanningStatus != "accepted" {
		t.Fatalf("settlement=%+v", got)
	}
	state := readCanonStateWithPlanning(t, project)
	if state.Planning.CurrentArc.ID != "arc-1" || state.Planning.NextArc == nil || state.Planning.NextArc.ID != "arc-2" {
		t.Fatalf("planning=%+v", state.Planning)
	}
}
func planningChapterReadyProject(t *testing.T, arcEnd, lead int) (*Project, string, readyView) {
	t.Helper()
	project, _, workspace := newCapabilityPassedProject(t)
	if err := project.Reconcile(); err != nil {
		t.Fatal(err)
	}
	ready := readReady(t, workspace)
	artifacts := validFoundationArtifacts()
	bookPlan, _ := json.Marshal(map[string]any{
		"direction":   "完成主线",
		"current_arc": map[string]any{"id": "arc-1", "start_chapter": 1, "end_chapter": arcEnd, "goal": "第一弧目标", "planning_lead_chapters": lead},
	})
	artifacts["book_plan.json"] = bookPlan
	result, err := project.SettleFoundation(FoundationSubmission{Manifest: manifestForReady(ready, artifacts), Artifacts: artifacts})
	if err != nil || result.Result != "ACCEPTED" {
		t.Fatalf("foundation=%+v err=%v", result, err)
	}
	return project, workspace, readReady(t, workspace)
}

func assertNextTaskRequiresPlanningRepair(t *testing.T, workspace string) {
	t.Helper()
	ready := readReady(t, workspace)
	base := filepath.Join(workspace, "exchange", "outbox", ready.TaskID, ready.AttemptID)
	taskRaw, err := os.ReadFile(filepath.Join(base, "task.json"))
	if err != nil {
		t.Fatal(err)
	}
	constraintsRaw, err := os.ReadFile(filepath.Join(base, "constraints.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(taskRaw), "planning_patch.json") || !strings.Contains(string(constraintsRaw), "planning_repair_required") {
		t.Fatalf("repair requirement missing task=%s constraints=%s", taskRaw, constraintsRaw)
	}
}

type planningArcView struct {
	ID           string `json:"id"`
	StartChapter int    `json:"start_chapter"`
	EndChapter   int    `json:"end_chapter"`
	Goal         string `json:"goal"`
}
type canonPlanningView struct {
	Planning struct {
		CurrentArc planningArcView  `json:"current_arc"`
		NextArc    *planningArcView `json:"next_arc,omitempty"`
	} `json:"planning"`
}

func readCanonStateWithPlanning(t *testing.T, project *Project) canonPlanningView {
	t.Helper()
	raw, err := project.store.ReadCoreCanonStateBytes()
	if err != nil {
		t.Fatal(err)
	}
	var out canonPlanningView
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestPlanningRepairAcceptsNextArcOnFirstChapterAfterBoundary(t *testing.T) {
	project, workspace, ready := planningChapterReadyProject(t, 1, 1)
	first := submitAndSettleChapter(t, project, workspace, ready, validChapterArtifacts(1))
	if first.Result != "ACCEPTED" || first.PlanningStatus != "not_present" {
		t.Fatalf("first settlement=%+v", first)
	}
	assertNextTaskRequiresPlanningRepair(t, workspace)

	nextReady := readReady(t, workspace)
	artifacts := longformChapterArtifacts(2, nil)
	artifacts["planning_patch.json"] = []byte(`{"next_arc":{"id":"arc-2","start_chapter":2,"end_chapter":4,"goal":"边界后修复规划"}}`)
	second := submitAndSettleChapter(t, project, workspace, nextReady, artifacts)
	if second.Result != "ACCEPTED" || second.PlanningStatus != "accepted" {
		t.Fatalf("second settlement=%+v", second)
	}
	state := readCanonStateWithPlanning(t, project)
	if state.Planning.CurrentArc.ID != "arc-2" || state.Planning.NextArc != nil {
		t.Fatalf("planning=%+v", state.Planning)
	}
	following := readReady(t, workspace)
	base := filepath.Join(workspace, "exchange", "outbox", following.TaskID, following.AttemptID)
	taskRaw, err := os.ReadFile(filepath.Join(base, "task.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(taskRaw), "planning_patch.json") {
		t.Fatalf("planning repair leaked into following attempt: %s", taskRaw)
	}
}

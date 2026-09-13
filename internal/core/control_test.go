package core

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBlockResolutionCreatesRewriteAttemptAndIsIdempotent(t *testing.T) {
	project, _, workspace, blocked, root := blockedChapterProject(t)
	msgID := "ctrl-block-1"
	writeControlMessage(t, workspace, msgID, map[string]any{
		"kind": "block_resolution", "base_canon_root": root,
		"block_id": blocked.BlockID, "choice": "保留硬约束 A，调整冲突目标",
	})
	project.submissionQuietPeriod = 0
	mustReadyControl(t, project, msgID)
	first, err := project.ProcessControlMessage(msgID)
	if err != nil {
		t.Fatal(err)
	}
	if first.Result != "ACCEPTED" {
		t.Fatalf("result=%+v", first)
	}
	status, _ := project.Status()
	if status.BlockID != "" || status.CanonRoot != root {
		t.Fatalf("status=%+v", status)
	}
	ready := readReady(t, workspace)
	if ready.TaskID != blocked.TaskID || ready.AttemptID == blocked.AttemptID || ready.AttemptReason != "rewrite" {
		t.Fatalf("READY=%+v blocked=%+v", ready, blocked)
	}
	second, err := project.ProcessControlMessage(msgID)
	if err != nil {
		t.Fatal(err)
	}
	if second.Result != first.Result {
		t.Fatalf("duplicate result=%+v first=%+v", second, first)
	}
	again := readReady(t, workspace)
	if again.AttemptID != ready.AttemptID {
		t.Fatalf("duplicate created another attempt: first=%+v second=%+v", ready, again)
	}
	constraints, err := os.ReadFile(filepath.Join(workspace, "exchange", "outbox", ready.TaskID, ready.AttemptID, "constraints.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(constraints), msgID) || !strings.Contains(string(constraints), "保留硬约束 A") {
		t.Fatalf("resolution missing from constraints: %s", constraints)
	}
}

func TestBlockResolutionRejectsWrongBlockID(t *testing.T) {
	project, _, workspace, blocked, root := blockedChapterProject(t)
	msgID := "ctrl-block-stale"
	writeControlMessage(t, workspace, msgID, map[string]any{
		"kind": "block_resolution", "base_canon_root": root,
		"block_id": "block-stale", "choice": "任意选择",
	})
	project.submissionQuietPeriod = 0
	mustReadyControl(t, project, msgID)
	result, err := project.ProcessControlMessage(msgID)
	if err != nil {
		t.Fatal(err)
	}
	if result.Result != "INVALID" || !strings.Contains(result.Problem, "block") {
		t.Fatalf("result=%+v", result)
	}
	status, _ := project.Status()
	if status.BlockID != blocked.BlockID || status.CanonRoot != root {
		t.Fatalf("status=%+v", status)
	}
}

func TestFuturePlanDirectiveAppliesOnlyToNextTask(t *testing.T) {
	project, _, workspace, current := acceptedFoundationProject(t)
	root := current.BaseCanonRoot
	msgID := "ctrl-future-1"
	writeControlMessage(t, workspace, msgID, map[string]any{
		"kind": "author_directive", "base_canon_root": root,
		"directive_scope": "future_plan", "instruction": "第二章开始减少追逐，转向人物互疑",
	})
	project.submissionQuietPeriod = 0
	mustReadyControl(t, project, msgID)
	result, err := project.ProcessControlMessage(msgID)
	if err != nil {
		t.Fatal(err)
	}
	if result.Result != "ACCEPTED" {
		t.Fatalf("result=%+v", result)
	}
	after := readReady(t, workspace)
	if after.TaskID != current.TaskID || after.AttemptID != current.AttemptID {
		t.Fatalf("current attempt changed: before=%+v after=%+v", current, after)
	}
	status, _ := project.Status()
	if status.CanonRoot != root {
		t.Fatalf("canon changed: %+v", status)
	}

	writeSubmission(t, workspace, current, validChapterArtifacts(1), false)
	_, _ = project.ScanActiveSubmission()
	_, _ = project.ScanActiveSubmission()
	settlement, err := project.SettleActiveSnapshot()
	if err != nil || settlement.Result != "ACCEPTED" {
		t.Fatalf("settle=%+v err=%v", settlement, err)
	}
	next := readReady(t, workspace)
	constraints, err := os.ReadFile(filepath.Join(workspace, "exchange", "outbox", next.TaskID, next.AttemptID, "constraints.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(constraints), msgID) || !strings.Contains(string(constraints), "人物互疑") {
		t.Fatalf("future directive missing from next task: %s", constraints)
	}
}

func TestHistoricalRevisionDirectiveCreatesRevisionTaskWithoutCanonChange(t *testing.T) {
	project, _, workspace, first := acceptedFoundationProject(t)
	writeSubmission(t, workspace, first, validChapterArtifacts(1), false)
	project.submissionQuietPeriod = 0
	_, _ = project.ScanActiveSubmission()
	_, _ = project.ScanActiveSubmission()
	settlement, err := project.SettleActiveSnapshot()
	if err != nil || settlement.Result != "ACCEPTED" {
		t.Fatalf("settle=%+v err=%v", settlement, err)
	}
	root := settlement.NewCanonRoot
	before := readReady(t, workspace)

	msgID := "ctrl-revision-1"
	writeControlMessage(t, workspace, msgID, map[string]any{
		"kind": "author_directive", "base_canon_root": root,
		"directive_scope": "historical_revision", "chapter": 1,
		"instruction": "第一章中主角不能主动透露真实身份",
	})
	mustReadyControl(t, project, msgID)
	result, err := project.ProcessControlMessage(msgID)
	if err != nil {
		t.Fatal(err)
	}
	if result.Result != "ACCEPTED" {
		t.Fatalf("result=%+v", result)
	}
	after := readReady(t, workspace)
	if after.TaskKind != "revision" || after.Target != "chapter:1" || after.TaskID == before.TaskID {
		t.Fatalf("revision READY=%+v before=%+v", after, before)
	}
	status, _ := project.Status()
	if status.CanonRoot != root {
		t.Fatalf("revision directive changed canon: %+v", status)
	}
}

func TestControlMessageRejectsStaleCanonRoot(t *testing.T) {
	project, _, workspace, current := acceptedFoundationProject(t)
	msgID := "ctrl-stale-root"
	writeControlMessage(t, workspace, msgID, map[string]any{
		"kind": "author_directive", "base_canon_root": "stale-root",
		"directive_scope": "future_plan", "instruction": "无效旧前态",
	})
	project.submissionQuietPeriod = 0
	mustReadyControl(t, project, msgID)
	result, err := project.ProcessControlMessage(msgID)
	if err != nil {
		t.Fatal(err)
	}
	if result.Result != "INVALID" || !strings.Contains(result.Problem, "canon") {
		t.Fatalf("result=%+v", result)
	}
	after := readReady(t, workspace)
	if after.TaskID != current.TaskID || after.AttemptID != current.AttemptID {
		t.Fatalf("stale control changed task: %+v", after)
	}
}

func mustReadyControl(t *testing.T, project *Project, messageID string) {
	t.Helper()
	first, err := project.ScanControlMessage(messageID)
	if err != nil {
		t.Fatal(err)
	}
	if first.State != "PENDING" {
		t.Fatalf("first scan=%+v", first)
	}
	second, err := project.ScanControlMessage(messageID)
	if err != nil {
		t.Fatal(err)
	}
	if second.State != "READY_TO_VALIDATE" {
		t.Fatalf("second scan=%+v", second)
	}
}

func writeControlMessage(t *testing.T, workspace, messageID string, fields map[string]any) {
	t.Helper()
	base := filepath.Join(workspace, "exchange", "control", "inbox", messageID)
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatal(err)
	}
	fields["schema_version"] = 1
	fields["message_id"] = messageID
	projectRaw, err := os.ReadFile(filepath.Join(workspace, "project.json"))
	if err != nil {
		t.Fatal(err)
	}
	var project map[string]any
	if err := json.Unmarshal(projectRaw, &project); err != nil {
		t.Fatal(err)
	}
	fields["project_id"] = project["project_id"]
	controlRaw, err := json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, "control.json"), controlRaw, 0o644); err != nil {
		t.Fatal(err)
	}
	manifest := map[string]any{
		"schema_version": 1, "project_id": project["project_id"], "message_id": messageID,
		"base_canon_root": fields["base_canon_root"], "protocol_version": project["protocol_version"],
		"files": []string{"control.json"},
	}
	manifestRaw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, "manifest.json"), manifestRaw, 0o644); err != nil {
		t.Fatal(err)
	}
}

func blockedChapterProject(t *testing.T) (*Project, string, string, readyView, string) {
	t.Helper()
	project, local, workspace, ready := acceptedFoundationProject(t)
	artifacts := validChapterArtifacts(1)
	artifacts["chapter_contract.json"] = []byte(`{"chapter":1,"declared_pov":"character-000001","hard_constraints":[{"id":"hc-a"}]}`)
	artifacts["self_review.json"] = []byte(`{"author_decision_required":{"constraint_refs":["hc-a"],"conflict":"两个目标不能同时满足","options":["保留 A","保留 B"]}}`)
	writeSubmission(t, workspace, ready, artifacts, false)
	project.submissionQuietPeriod = 0
	_, _ = project.ScanActiveSubmission()
	_, _ = project.ScanActiveSubmission()
	settlement, err := project.SettleActiveSnapshot()
	if err != nil || settlement.Result != "BLOCKED" {
		t.Fatalf("block settle=%+v err=%v", settlement, err)
	}
	blocked := readReady(t, workspace)
	return project, local, workspace, blocked, ready.BaseCanonRoot
}

func TestControlApplyingRecoveryIsIdempotent(t *testing.T) {
	for _, stage := range []string{"production", "ready", "settled"} {
		t.Run(stage, func(t *testing.T) {
			project, local, workspace, blocked, root := blockedChapterProject(t)
			msgID := "ctrl-recover-" + stage
			writeControlMessage(t, workspace, msgID, map[string]any{
				"kind": "block_resolution", "base_canon_root": root,
				"block_id": blocked.BlockID, "choice": "采用方案 A",
			})
			project.submissionQuietPeriod = 0
			mustReadyControl(t, project, msgID)
			project.controlFault = func(got string) error {
				if got == stage {
					return errors.New("injected control crash")
				}
				return nil
			}
			if _, err := project.ProcessControlMessage(msgID); err == nil {
				t.Fatal("expected injected failure")
			}
			reopened, err := OpenProject(local)
			if err != nil {
				t.Fatal(err)
			}
			result, err := reopened.ProcessControlMessage(msgID)
			if err != nil {
				t.Fatal(err)
			}
			if result.Result != "ACCEPTED" || result.AttemptID == "" {
				t.Fatalf("recovered result=%+v", result)
			}
			ready := readReady(t, workspace)
			if ready.AttemptID != result.AttemptID || ready.TaskID != blocked.TaskID {
				t.Fatalf("READY=%+v result=%+v", ready, result)
			}
			again, err := reopened.ProcessControlMessage(msgID)
			if err != nil {
				t.Fatal(err)
			}
			if again.AttemptID != result.AttemptID {
				t.Fatalf("duplicate recovery created attempt: first=%+v second=%+v", result, again)
			}
		})
	}
}

func TestHistoricalRevisionDoesNotConsumeFuturePlan(t *testing.T) {
	project, _, workspace, first := acceptedFoundationProject(t)
	writeSubmission(t, workspace, first, validChapterArtifacts(1), false)
	project.submissionQuietPeriod = 0
	_, _ = project.ScanActiveSubmission()
	_, _ = project.ScanActiveSubmission()
	settled, err := project.SettleActiveSnapshot()
	if err != nil || settled.Result != "ACCEPTED" {
		t.Fatalf("settle=%+v err=%v", settled, err)
	}
	root := settled.NewCanonRoot
	futureID := "ctrl-future-before-revision"
	writeControlMessage(t, workspace, futureID, map[string]any{
		"kind": "author_directive", "base_canon_root": root,
		"directive_scope": "future_plan", "instruction": "第三章以后减少追逐",
	})
	mustReadyControl(t, project, futureID)
	if _, err := project.ProcessControlMessage(futureID); err != nil {
		t.Fatal(err)
	}
	revisionID := "ctrl-revision-after-future"
	writeControlMessage(t, workspace, revisionID, map[string]any{
		"kind": "author_directive", "base_canon_root": root,
		"directive_scope": "historical_revision", "chapter": 1,
		"instruction": "第一章隐藏真实身份",
	})
	mustReadyControl(t, project, revisionID)
	if _, err := project.ProcessControlMessage(revisionID); err != nil {
		t.Fatal(err)
	}
	ready := readReady(t, workspace)
	raw, err := os.ReadFile(filepath.Join(workspace, "exchange", "outbox", ready.TaskID, ready.AttemptID, "constraints.json"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if !strings.Contains(text, revisionID) || strings.Contains(text, futureID) {
		t.Fatalf("revision constraints leaked future plan: %s", text)
	}
	state, err := project.store.LoadCoreProductionState()
	if err != nil {
		t.Fatal(err)
	}
	if len(state.PendingControls) != 1 || state.PendingControls[0].MessageID != futureID {
		t.Fatalf("future plan was consumed by revision: %+v", state.PendingControls)
	}
}

func TestControlSnapshotRejectsDriveMutationAfterLock(t *testing.T) {
	project, _, workspace, current := acceptedFoundationProject(t)
	msgID := "ctrl-snapshot-drift"
	writeControlMessage(t, workspace, msgID, map[string]any{
		"kind": "author_directive", "base_canon_root": current.BaseCanonRoot,
		"directive_scope": "future_plan", "instruction": "保持原计划",
	})
	project.submissionQuietPeriod = 0
	mustReadyControl(t, project, msgID)
	path := filepath.Join(workspace, "exchange", "control", "inbox", msgID, "control.json")
	if err := os.WriteFile(path, []byte(`{"tampered":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	status, err := project.ScanControlMessage(msgID)
	if err != nil {
		t.Fatal(err)
	}
	if status.State != "INVALID" || !status.Conflict {
		t.Fatalf("mutated control status=%+v", status)
	}
}

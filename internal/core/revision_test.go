package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRevisionSupersedesDownstreamAndRebasesInOrder(t *testing.T) {
	project, local, workspace, ready := acceptedFoundationProject(t)
	for chapter := 1; chapter <= 3; chapter++ {
		got := submitAndSettleChapter(t, project, workspace, ready, revisionChapterArtifacts(chapter, fmt.Sprintf("第%d章旧正文", chapter)))
		if got.Result != "ACCEPTED" {
			t.Fatalf("chapter %d settlement=%+v", chapter, got)
		}
		ready = readReady(t, workspace)
	}
	oldHead, err := project.Status()
	if err != nil {
		t.Fatal(err)
	}
	revision := startHistoricalRevision(t, project, workspace, oldHead.CanonRoot, 1, "重写第一章")
	if revision.TaskKind != "revision" || revision.Target != "chapter:1" || revision.AttemptReason != "initial" {
		t.Fatalf("revision READY=%+v", revision)
	}
	if revision.BaseCanonRoot == oldHead.CanonRoot {
		t.Fatalf("revision did not branch from target parent: %+v", revision)
	}

	first := submitAndSettleChapter(t, project, workspace, revision, revisionChapterArtifacts(1, "第一章新版正文"))
	if first.Result != "ACCEPTED" || first.NewCanonRoot == oldHead.CanonRoot {
		t.Fatalf("revision settlement=%+v", first)
	}
	next := readReady(t, workspace)
	if next.TaskKind != "revision" || next.Target != "chapter:2" || next.AttemptReason != "rebase" || next.BaseCanonRoot != first.NewCanonRoot {
		t.Fatalf("chapter 2 replay READY=%+v first=%+v", next, first)
	}
	assertReplayState(t, local, 1, 3, []int{1, 2, 3})

	second := submitAndSettleChapter(t, project, workspace, next, revisionChapterArtifacts(2, "第二章重放正文"))
	if second.Result != "ACCEPTED" {
		t.Fatalf("chapter 2 replay=%+v", second)
	}
	next = readReady(t, workspace)
	if next.TaskKind != "revision" || next.Target != "chapter:3" || next.AttemptReason != "rebase" || next.BaseCanonRoot != second.NewCanonRoot {
		t.Fatalf("chapter 3 replay READY=%+v second=%+v", next, second)
	}

	third := submitAndSettleChapter(t, project, workspace, next, revisionChapterArtifacts(3, "第三章重放正文"))
	if third.Result != "ACCEPTED" {
		t.Fatalf("chapter 3 replay=%+v", third)
	}
	following := readReady(t, workspace)
	if following.TaskKind != "chapter" || following.Target != "chapter:4" || following.AttemptReason != "initial" || following.BaseCanonRoot != third.NewCanonRoot {
		t.Fatalf("normal production did not resume: %+v", following)
	}
	assertNoReplayState(t, local)
}

func TestRevisionTaskPackCarriesOldProseCandidate(t *testing.T) {
	project, _, workspace, ready := acceptedFoundationProject(t)
	for chapter := 1; chapter <= 2; chapter++ {
		got := submitAndSettleChapter(t, project, workspace, ready, revisionChapterArtifacts(chapter, fmt.Sprintf("第%d章唯一旧正文候选", chapter)))
		if got.Result != "ACCEPTED" {
			t.Fatalf("chapter %d settlement=%+v", chapter, got)
		}
		ready = readReady(t, workspace)
	}
	status, _ := project.Status()
	revision := startHistoricalRevision(t, project, workspace, status.CanonRoot, 1, "修改第一章")
	contextRaw, err := os.ReadFile(filepath.Join(workspace, "exchange", "outbox", revision.TaskID, revision.AttemptID, "context.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(contextRaw), "第1章唯一旧正文候选") {
		t.Fatalf("revision context missing old prose candidate: %s", contextRaw)
	}
}

func TestRevisionReceiptBranchesFromTargetParentRoot(t *testing.T) {
	project, _, workspace, ready := acceptedFoundationProject(t)
	original := submitAndSettleChapter(t, project, workspace, ready, revisionChapterArtifacts(1, "第一章旧版本"))
	if original.Result != "ACCEPTED" {
		t.Fatalf("original=%+v", original)
	}
	revision := startHistoricalRevision(t, project, workspace, original.NewCanonRoot, 1, "改第一章")
	settled := submitAndSettleChapter(t, project, workspace, revision, revisionChapterArtifacts(1, "第一章新版本"))
	if settled.Result != "ACCEPTED" {
		t.Fatalf("revision=%+v", settled)
	}
	var receipt struct {
		PreviousRoot string `json:"previous_root"`
		NewRoot      string `json:"new_root"`
	}
	readJSONFile(t, settled.ReceiptPath, &receipt)
	if receipt.PreviousRoot != revision.BaseCanonRoot || receipt.PreviousRoot == original.NewCanonRoot || receipt.NewRoot != settled.NewCanonRoot {
		t.Fatalf("revision lineage receipt=%+v ready=%+v original=%+v settled=%+v", receipt, revision, original, settled)
	}
}

func TestHistoricalRevisionRejectsSecondActiveRevision(t *testing.T) {
	project, _, workspace, ready := acceptedFoundationProject(t)
	first := submitAndSettleChapter(t, project, workspace, ready, revisionChapterArtifacts(1, "第一章旧版本"))
	if first.Result != "ACCEPTED" {
		t.Fatalf("first=%+v", first)
	}
	_ = startHistoricalRevision(t, project, workspace, first.NewCanonRoot, 1, "第一次历史修订")
	msgID := "ctrl-revision-concurrent"
	writeControlMessage(t, workspace, msgID, map[string]any{
		"kind": "author_directive", "base_canon_root": first.NewCanonRoot,
		"directive_scope": "historical_revision", "chapter": 1,
		"instruction": "第二条并发历史修订",
	})
	mustReadyControl(t, project, msgID)
	result, err := project.ProcessControlMessage(msgID)
	if err != nil {
		t.Fatal(err)
	}
	if result.Result != "INVALID" || !strings.Contains(strings.ToLower(result.Problem), "revision") {
		t.Fatalf("second revision result=%+v", result)
	}
}

func revisionChapterArtifacts(chapter int, marker string) map[string][]byte {
	body := marker + "。主角看见门口的灯。"
	contract, _ := json.Marshal(map[string]any{"chapter": chapter, "declared_pov": "character-000001"})
	events, _ := json.Marshal(map[string]any{"events": []map[string]any{{"local_id": "e1", "kind": "onscreen", "evidence_anchor": "主角看见门口的灯"}}})
	plan, _ := json.Marshal(map[string]any{
		"chapter": chapter, "base_canon_root": "__READY_BASE_CANON_ROOT__",
		"objective": "修订并推进本章", "reader_payoff": "保持修订后的剧情推进",
		"beats":              []map[string]any{{"id": "beat-1", "intent": "承接前态"}, {"id": "beat-2", "intent": "完成修订目标"}},
		"ending_hook_intent": "保持后续驱动力",
	})
	review := validChapterReviewMap("pass")
	review["plan_ref"] = "chapter_plan.json"
	reviewRaw, _ := json.Marshal(review)
	return map[string][]byte{
		"chapter.md": []byte(body), "chapter_contract.json": contract,
		"chapter_plan.json": plan, "chapter_review.json": reviewRaw,
		"events.json": events, "self_review.json": []byte(`{"ok":true}`), "state_delta.json": []byte(`{"changes":[]}`),
	}
}

func startHistoricalRevision(t *testing.T, project *Project, workspace, root string, chapter int, instruction string) readyView {
	t.Helper()
	msgID := fmt.Sprintf("ctrl-revision-ch%d-%d", chapter, len(instruction))
	writeControlMessage(t, workspace, msgID, map[string]any{
		"kind": "author_directive", "base_canon_root": root,
		"directive_scope": "historical_revision", "chapter": chapter, "instruction": instruction,
	})
	project.submissionQuietPeriod = 0
	mustReadyControl(t, project, msgID)
	result, err := project.ProcessControlMessage(msgID)
	if err != nil {
		t.Fatal(err)
	}
	if result.Result != "ACCEPTED" {
		t.Fatalf("historical revision control=%+v", result)
	}
	return readReady(t, workspace)
}

func assertReplayState(t *testing.T, local string, start, head int, superseded []int) {
	t.Helper()
	var raw map[string]any
	readJSONFile(t, filepath.Join(local, "meta", "core", "production.json"), &raw)
	replay, ok := raw["revision_replay"].(map[string]any)
	if !ok {
		t.Fatalf("revision_replay missing: %+v", raw)
	}
	if int(replay["start_chapter"].(float64)) != start || int(replay["original_head_chapter"].(float64)) != head {
		t.Fatalf("revision_replay=%+v", replay)
	}
	items, ok := replay["superseded"].([]any)
	if !ok || len(items) != len(superseded) {
		t.Fatalf("superseded=%+v want=%v", replay["superseded"], superseded)
	}
	for i, item := range items {
		entry := item.(map[string]any)
		if int(entry["chapter"].(float64)) != superseded[i] || entry["status"] != "superseded" {
			t.Fatalf("superseded[%d]=%+v", i, entry)
		}
	}
}

func assertNoReplayState(t *testing.T, local string) {
	t.Helper()
	var raw map[string]any
	readJSONFile(t, filepath.Join(local, "meta", "core", "production.json"), &raw)
	if replay, ok := raw["revision_replay"]; ok && replay != nil {
		t.Fatalf("revision replay still active: %+v", replay)
	}
}

func TestRevisionFromMiddleBranchesFromPreviousAcceptedChapter(t *testing.T) {
	project, _, workspace, ready := acceptedFoundationProject(t)
	roots := make([]string, 0, 3)
	for chapter := 1; chapter <= 3; chapter++ {
		got := submitAndSettleChapter(t, project, workspace, ready, revisionChapterArtifacts(chapter, fmt.Sprintf("第%d章旧正文", chapter)))
		if got.Result != "ACCEPTED" {
			t.Fatalf("chapter %d settlement=%+v", chapter, got)
		}
		roots = append(roots, got.NewCanonRoot)
		ready = readReady(t, workspace)
	}
	revision := startHistoricalRevision(t, project, workspace, roots[2], 2, "从第二章分叉")
	if revision.BaseCanonRoot != roots[0] || revision.Target != "chapter:2" {
		t.Fatalf("revision READY=%+v roots=%v", revision, roots)
	}
	contextRaw, err := os.ReadFile(filepath.Join(workspace, "exchange", "outbox", revision.TaskID, revision.AttemptID, "context.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(contextRaw), "第2章旧正文") {
		t.Fatalf("revision context missing chapter 2 candidate: %s", contextRaw)
	}
	settled := submitAndSettleChapter(t, project, workspace, revision, revisionChapterArtifacts(2, "第二章新版正文"))
	if settled.Result != "ACCEPTED" {
		t.Fatalf("revision settlement=%+v", settled)
	}
	next := readReady(t, workspace)
	if next.TaskKind != "revision" || next.Target != "chapter:3" || next.AttemptReason != "rebase" || next.BaseCanonRoot != settled.NewCanonRoot {
		t.Fatalf("rebase READY=%+v", next)
	}
}

func TestSupersededHistoryPersistsAfterReplayCompletes(t *testing.T) {
	project, local, workspace, ready := acceptedFoundationProject(t)
	for chapter := 1; chapter <= 2; chapter++ {
		got := submitAndSettleChapter(t, project, workspace, ready, revisionChapterArtifacts(chapter, fmt.Sprintf("第%d章旧正文", chapter)))
		if got.Result != "ACCEPTED" {
			t.Fatalf("chapter %d settlement=%+v", chapter, got)
		}
		ready = readReady(t, workspace)
	}
	status, _ := project.Status()
	revision := startHistoricalRevision(t, project, workspace, status.CanonRoot, 1, "持久化旧分支标记")
	first := submitAndSettleChapter(t, project, workspace, revision, revisionChapterArtifacts(1, "第一章新版正文"))
	if first.Result != "ACCEPTED" {
		t.Fatalf("first replay=%+v", first)
	}
	secondReady := readReady(t, workspace)
	second := submitAndSettleChapter(t, project, workspace, secondReady, revisionChapterArtifacts(2, "第二章新版正文"))
	if second.Result != "ACCEPTED" {
		t.Fatalf("second replay=%+v", second)
	}
	var raw map[string]any
	readJSONFile(t, filepath.Join(local, "meta", "core", "production.json"), &raw)
	if replay, ok := raw["revision_replay"]; ok && replay != nil {
		t.Fatalf("active replay remained after catch-up: %+v", replay)
	}
	items, ok := raw["superseded_chapters"].([]any)
	if !ok || len(items) != 2 {
		t.Fatalf("persistent superseded history=%+v", raw["superseded_chapters"])
	}
	for i, item := range items {
		entry, ok := item.(map[string]any)
		if !ok || int(entry["chapter"].(float64)) != i+1 || entry["status"] != "superseded" || entry["attempt_id"] == "" || entry["canon_root"] == "" {
			t.Fatalf("superseded[%d]=%+v", i, item)
		}
	}
}

func TestRevisionCommitCrashRecoveryPreservesReplayLineage(t *testing.T) {
	for _, stage := range []string{"canon", "receipt", "production", "ready"} {
		t.Run(stage, func(t *testing.T) {
			project, local, workspace, ready := acceptedFoundationProject(t)
			for chapter := 1; chapter <= 2; chapter++ {
				got := submitAndSettleChapter(t, project, workspace, ready, revisionChapterArtifacts(chapter, fmt.Sprintf("第%d章旧正文", chapter)))
				if got.Result != "ACCEPTED" {
					t.Fatalf("chapter %d settlement=%+v", chapter, got)
				}
				ready = readReady(t, workspace)
			}
			status, _ := project.Status()
			revision := startHistoricalRevision(t, project, workspace, status.CanonRoot, 1, "崩溃恢复修订")
			writeSubmission(t, workspace, revision, revisionChapterArtifacts(1, "第一章崩溃恢复版"), false)
			project.submissionQuietPeriod = 0
			_, _ = project.ScanActiveSubmission()
			_, _ = project.ScanActiveSubmission()
			project.commitFault = func(got string) error {
				if got == stage {
					return fmt.Errorf("simulated revision crash after %s", stage)
				}
				return nil
			}
			if _, err := project.SettleActiveSnapshot(); err == nil {
				t.Fatalf("expected simulated crash at %s", stage)
			}

			reopened, err := OpenProject(local)
			if err != nil {
				t.Fatal(err)
			}
			if err := reopened.Reconcile(); err != nil {
				t.Fatalf("recover %s: %v", stage, err)
			}
			next := readReady(t, workspace)
			if next.TaskKind != "revision" || next.Target != "chapter:2" || next.AttemptReason != "rebase" {
				t.Fatalf("recovered READY=%+v", next)
			}
			root, err := reopened.RecomputeCanonRoot()
			if err != nil {
				t.Fatal(err)
			}
			statusAfter, err := reopened.Status()
			if err != nil {
				t.Fatal(err)
			}
			if root != statusAfter.CanonRoot || next.BaseCanonRoot != statusAfter.CanonRoot {
				t.Fatalf("recovery lineage root=%s status=%+v READY=%+v", root, statusAfter, next)
			}
		})
	}
}

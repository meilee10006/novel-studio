package core

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestStatusPublishesDeterministicExportReadiness(t *testing.T) {
	project, _, workspace, ready := acceptedFoundationProject(t)
	first := longformChapterArtifacts(1, []map[string]any{
		{"kind": "foreshadow", "foreshadow_id": "fs-open", "description": "红色纸伞必须回收", "state": "seeded", "event_ref": "e1"},
		{"kind": "ending_resolution", "event_ref": "e1"},
	})
	if got := submitAndSettleChapter(t, project, workspace, ready, first); got.Result != "ACCEPTED" {
		t.Fatalf("chapter 1=%+v", got)
	}
	assertExportReadinessJSON(t, filepath.Join(workspace, "exchange", "STATUS.json"), false, "foreshadow fs-open is seeded")
	status, err := project.Status()
	if err != nil {
		t.Fatal(err)
	}
	statusRaw, err := json.Marshal(status)
	if err != nil {
		t.Fatal(err)
	}
	assertExportReadinessBytes(t, statusRaw, false, "foreshadow fs-open is seeded")

	ready = readReady(t, workspace)
	second := longformChapterArtifacts(2, []map[string]any{
		{"kind": "foreshadow", "foreshadow_id": "fs-open", "state": "payoff_ready", "event_ref": "e1"},
		{"kind": "foreshadow", "foreshadow_id": "fs-open", "state": "paid_off", "event_ref": "e1"},
		{"kind": "foreshadow", "foreshadow_id": "fs-open", "state": "closed", "event_ref": "e1"},
		{"kind": "ending_resolution", "event_ref": "e1"},
	})
	if got := submitAndSettleChapter(t, project, workspace, ready, second); got.Result != "ACCEPTED" {
		t.Fatalf("chapter 2=%+v", got)
	}
	assertExportReadinessJSON(t, filepath.Join(workspace, "exchange", "STATUS.json"), true, "")
	status, err = project.Status()
	if err != nil {
		t.Fatal(err)
	}
	statusRaw, err = json.Marshal(status)
	if err != nil {
		t.Fatal(err)
	}
	assertExportReadinessBytes(t, statusRaw, true, "")
}

func assertExportReadinessJSON(t *testing.T, path string, wantReady bool, wantProblem string) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	assertExportReadinessBytes(t, raw, wantReady, wantProblem)
}

func assertExportReadinessBytes(t *testing.T, raw []byte, wantReady bool, wantProblem string) {
	t.Helper()
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	ready, ok := doc["export_ready"].(bool)
	if !ok || ready != wantReady {
		t.Fatalf("export_ready=%v want=%v doc=%s", doc["export_ready"], wantReady, raw)
	}
	problems, _ := doc["export_problems"].([]any)
	if wantProblem == "" {
		if len(problems) != 0 {
			t.Fatalf("unexpected export problems: %v", problems)
		}
		return
	}
	for _, rawProblem := range problems {
		if problem, _ := rawProblem.(string); strings.Contains(problem, wantProblem) {
			return
		}
	}
	t.Fatalf("missing export problem %q in %v", wantProblem, problems)
}

func TestFinalExportRequiresEndingResolution(t *testing.T) {
	project, _, workspace, ready := acceptedFoundationProject(t)
	settled := submitAndSettleChapter(t, project, workspace, ready, longformChapterArtifacts(1, nil))
	if settled.Result != "ACCEPTED" {
		t.Fatalf("chapter settlement=%+v", settled)
	}
	out := filepath.Join(t.TempDir(), "book.md")
	_, err := project.ExportBook(out)
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "ending") {
		t.Fatalf("ExportBook err=%v, want ending constraint failure", err)
	}
	if _, statErr := os.Stat(out); !os.IsNotExist(statErr) {
		t.Fatalf("blocked export created output: %v", statErr)
	}
}

func TestFinalExportWritesActiveCanonInOrder(t *testing.T) {
	project, _, workspace, ready := acceptedFoundationProject(t)
	first := revisionChapterArtifacts(1, "第一章导出正文")
	if got := submitAndSettleChapter(t, project, workspace, ready, first); got.Result != "ACCEPTED" {
		t.Fatalf("chapter 1=%+v", got)
	}
	ready = readReady(t, workspace)
	second := revisionChapterArtifacts(2, "第二章导出正文")
	delta, err := json.Marshal(map[string]any{"changes": []map[string]any{{"kind": "ending_resolution", "event_ref": "e1"}}})
	if err != nil {
		t.Fatal(err)
	}
	second["state_delta.json"] = delta
	settled := submitAndSettleChapter(t, project, workspace, ready, second)
	if settled.Result != "ACCEPTED" {
		t.Fatalf("chapter 2=%+v", settled)
	}
	out := filepath.Join(t.TempDir(), "book.md")
	result, err := project.ExportBook(out)
	if err != nil {
		t.Fatalf("ExportBook: %v", err)
	}
	status, err := project.Status()
	if err != nil {
		t.Fatal(err)
	}
	if result.CanonRoot != status.CanonRoot || result.Chapters != 2 || result.Path == "" {
		t.Fatalf("result=%+v status=%+v", result, status)
	}
	body, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	firstAt := strings.Index(text, "第一章导出正文")
	secondAt := strings.Index(text, "第二章导出正文")
	if firstAt < 0 || secondAt < 0 || firstAt >= secondAt {
		t.Fatalf("export order/content=%q", text)
	}
}

func TestFinalExportBlockedDuringRevisionReplay(t *testing.T) {
	project, _, workspace, ready := acceptedFoundationProject(t)
	artifacts := revisionChapterArtifacts(1, "第一章已收束正文")
	delta, err := json.Marshal(map[string]any{"changes": []map[string]any{{"kind": "ending_resolution", "event_ref": "e1"}}})
	if err != nil {
		t.Fatal(err)
	}
	artifacts["state_delta.json"] = delta
	settled := submitAndSettleChapter(t, project, workspace, ready, artifacts)
	if settled.Result != "ACCEPTED" {
		t.Fatalf("chapter settlement=%+v", settled)
	}
	_ = startHistoricalRevision(t, project, workspace, settled.NewCanonRoot, 1, "结局后修订第一章")
	out := filepath.Join(t.TempDir(), "book.md")
	_, err = project.ExportBook(out)
	if err == nil || (!strings.Contains(strings.ToLower(err.Error()), "revision") && !strings.Contains(strings.ToLower(err.Error()), "replay")) {
		t.Fatalf("ExportBook err=%v, want revision replay block", err)
	}
	if _, statErr := os.Stat(out); !os.IsNotExist(statErr) {
		t.Fatalf("blocked replay export created output: %v", statErr)
	}
}

func TestFinalExportBlocksNonTerminalLongformObligations(t *testing.T) {
	project, _, workspace, ready := acceptedFoundationProject(t)
	artifacts := revisionChapterArtifacts(1, "第一章结局正文")
	delta, err := json.Marshal(map[string]any{"changes": []map[string]any{
		{"kind": "foreshadow", "foreshadow_id": "f-open", "state": "seeded", "event_ref": "e1"},
		{"kind": "ending_resolution", "event_ref": "e1"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	artifacts["state_delta.json"] = delta
	settled := submitAndSettleChapter(t, project, workspace, ready, artifacts)
	if settled.Result != "ACCEPTED" {
		t.Fatalf("chapter settlement=%+v", settled)
	}
	out := filepath.Join(t.TempDir(), "book.md")
	_, err = project.ExportBook(out)
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "foreshadow") {
		t.Fatalf("ExportBook err=%v, want non-terminal foreshadow block", err)
	}
}

func TestFinalExportRequiresEndingResolutionOnLatestAcceptedChapter(t *testing.T) {
	project, _, workspace, ready := acceptedFoundationProject(t)
	first := revisionChapterArtifacts(1, "第一章曾经宣告结局")
	delta, err := json.Marshal(map[string]any{"changes": []map[string]any{{"kind": "ending_resolution", "event_ref": "e1"}}})
	if err != nil {
		t.Fatal(err)
	}
	first["state_delta.json"] = delta
	if got := submitAndSettleChapter(t, project, workspace, ready, first); got.Result != "ACCEPTED" {
		t.Fatalf("chapter 1=%+v", got)
	}
	ready = readReady(t, workspace)
	if got := submitAndSettleChapter(t, project, workspace, ready, revisionChapterArtifacts(2, "第二章继续写了")); got.Result != "ACCEPTED" {
		t.Fatalf("chapter 2=%+v", got)
	}
	out := filepath.Join(t.TempDir(), "book.md")
	_, err = project.ExportBook(out)
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "latest") {
		t.Fatalf("ExportBook err=%v, want latest-chapter ending block", err)
	}
}

func TestFinalExportRefusesConcurrentProjectWriter(t *testing.T) {
	project, local, workspace, ready := acceptedFoundationProject(t)
	artifacts := revisionChapterArtifacts(1, "第一章并发导出正文")
	delta, err := json.Marshal(map[string]any{"changes": []map[string]any{{"kind": "ending_resolution", "event_ref": "e1"}}})
	if err != nil {
		t.Fatal(err)
	}
	artifacts["state_delta.json"] = delta
	if got := submitAndSettleChapter(t, project, workspace, ready, artifacts); got.Result != "ACCEPTED" {
		t.Fatalf("chapter settlement=%+v", got)
	}
	lockReady := filepath.Join(t.TempDir(), "lock-ready")
	release := filepath.Join(t.TempDir(), "lock-release")
	cmd := exec.Command(os.Args[0], "-test.run=^TestProjectLockHelper$")
	cmd.Env = append(os.Environ(),
		"NOVEL_CORE_LOCK_HELPER=1",
		"NOVEL_CORE_LOCK_ROOT="+local,
		"NOVEL_CORE_LOCK_READY="+lockReady,
		"NOVEL_CORE_LOCK_RELEASE="+release,
	)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = os.WriteFile(release, []byte("release"), 0o600)
		_ = cmd.Wait()
	}()
	waitForFile(t, lockReady)

	_, err = project.ExportBook(filepath.Join(t.TempDir(), "book.md"))
	if !errors.Is(err, ErrProjectLocked) {
		t.Fatalf("ExportBook err=%v, want ErrProjectLocked", err)
	}
}

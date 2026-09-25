package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSubmissionStillRejectsMutationAfterSharedSnapshotRefactor(t *testing.T) {
	project, _, workspace := newCapabilityPassedProject(t)
	project.submissionQuietPeriod = 0
	if err := project.Reconcile(); err != nil {
		t.Fatal(err)
	}
	ready := readReady(t, workspace)
	files := validFoundationArtifacts()
	writeSubmission(t, workspace, ready, files, false)

	if _, err := project.ScanActiveSubmission(); err != nil {
		t.Fatal(err)
	}
	first, err := project.ScanActiveSubmission()
	if err != nil || first.State != "READY_TO_VALIDATE" {
		t.Fatalf("first scan=%+v err=%v", first, err)
	}

	path := filepath.Join(workspace, "exchange", "inbox", ready.TaskID, ready.AttemptID, "foundation.json")
	if err := os.WriteFile(path, []byte(`{"title":"changed"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	second, err := project.ScanActiveSubmission()
	if err != nil {
		t.Fatal(err)
	}
	if !second.Conflict || second.State != "INVALID" {
		t.Fatalf("mutated locked submission=%+v", second)
	}
}

func TestSubmissionSnapshotPendingUntilManifestAndFilesStable(t *testing.T) {
	project, _, workspace := newChapterReadyProject(t)
	project.submissionQuietPeriod = 0
	ready := readReady(t, workspace)

	status, err := project.ScanActiveSubmission()
	if err != nil {
		t.Fatal(err)
	}
	if status.State != "PENDING" {
		t.Fatalf("state=%q want PENDING", status.State)
	}

	artifacts := validChapterArtifacts(1)
	writeSubmission(t, workspace, ready, artifacts, false)
	status, err = project.ScanActiveSubmission()
	if err != nil {
		t.Fatal(err)
	}
	if status.State != "PENDING" {
		t.Fatalf("first complete scan state=%q want PENDING", status.State)
	}
	status, err = project.ScanActiveSubmission()
	if err != nil {
		t.Fatal(err)
	}
	if status.State != "READY_TO_VALIDATE" || status.SnapshotDigest == "" {
		t.Fatalf("second stable scan=%+v", status)
	}
	if _, err := os.Stat(filepath.Join(project.root, "meta", "core", "snapshots", ready.AttemptID, "chapter.md")); err != nil {
		t.Fatalf("local snapshot missing: %v", err)
	}
}

func TestSubmissionSnapshotManifestBeforeFilesRemainsPending(t *testing.T) {
	project, _, workspace := newChapterReadyProject(t)
	project.submissionQuietPeriod = 0
	ready := readReady(t, workspace)
	artifacts := validChapterArtifacts(1)
	writeSubmission(t, workspace, ready, artifacts, true)

	status, err := project.ScanActiveSubmission()
	if err != nil {
		t.Fatal(err)
	}
	if status.State != "PENDING" {
		t.Fatalf("state=%q want PENDING", status.State)
	}
}
func TestSubmissionSnapshotChangeBetweenScansResetsStability(t *testing.T) {
	project, _, workspace := newChapterReadyProject(t)
	project.submissionQuietPeriod = 0
	ready := readReady(t, workspace)
	artifacts := validChapterArtifacts(1)
	writeSubmission(t, workspace, ready, artifacts, false)
	first, err := project.ScanActiveSubmission()
	if err != nil {
		t.Fatal(err)
	}
	if first.State != "PENDING" {
		t.Fatalf("first scan=%+v", first)
	}

	path := filepath.Join(workspace, "exchange", "inbox", ready.TaskID, ready.AttemptID, "chapter.md")
	if err := os.WriteFile(path, []byte("第二版正文，发生了变化。"), 0o644); err != nil {
		t.Fatal(err)
	}
	second, err := project.ScanActiveSubmission()
	if err != nil {
		t.Fatal(err)
	}
	if second.State != "PENDING" || second.ObservedDigest == first.ObservedDigest {
		t.Fatalf("second scan should reset stability: first=%+v second=%+v", first, second)
	}
}
func TestSubmissionSnapshotLocksLocalBytesAndDetectsDriveConflict(t *testing.T) {
	project, _, workspace := newChapterReadyProject(t)
	project.submissionQuietPeriod = 0
	ready := readReady(t, workspace)
	artifacts := validChapterArtifacts(1)
	writeSubmission(t, workspace, ready, artifacts, false)
	if _, err := project.ScanActiveSubmission(); err != nil {
		t.Fatal(err)
	}
	locked, err := project.ScanActiveSubmission()
	if err != nil {
		t.Fatal(err)
	}
	if locked.State != "READY_TO_VALIDATE" {
		t.Fatalf("locked=%+v", locked)
	}

	driveChapter := filepath.Join(workspace, "exchange", "inbox", ready.TaskID, ready.AttemptID, "chapter.md")
	if err := os.WriteFile(driveChapter, []byte("篡改后的正文"), 0o644); err != nil {
		t.Fatal(err)
	}
	conflict, err := project.ScanActiveSubmission()
	if err != nil {
		t.Fatal(err)
	}
	if conflict.State != "INVALID" || !conflict.Conflict {
		t.Fatalf("conflict=%+v", conflict)
	}
	localChapter, err := os.ReadFile(filepath.Join(project.root, "meta", "core", "snapshots", ready.AttemptID, "chapter.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(localChapter) != string(artifacts["chapter.md"]) {
		t.Fatalf("local snapshot changed with Drive: %q", localChapter)
	}
}

func TestSubmissionSnapshotRejectsUnknownFiles(t *testing.T) {
	project, _, workspace := newChapterReadyProject(t)
	project.submissionQuietPeriod = 0
	ready := readReady(t, workspace)
	artifacts := validChapterArtifacts(1)
	writeSubmission(t, workspace, ready, artifacts, false)
	inbox := filepath.Join(workspace, "exchange", "inbox", ready.TaskID, ready.AttemptID)
	if err := os.WriteFile(filepath.Join(inbox, "extra.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	status, err := project.ScanActiveSubmission()
	if err != nil {
		t.Fatal(err)
	}
	if status.State != "INVALID" {
		t.Fatalf("state=%q want INVALID", status.State)
	}
}
func newChapterReadyProject(t *testing.T) (*Project, string, string) {
	t.Helper()
	project, local, workspace := newCapabilityPassedProject(t)
	if err := project.Reconcile(); err != nil {
		t.Fatal(err)
	}
	ready := readReady(t, workspace)
	artifacts := validFoundationArtifacts()
	result, err := project.SettleFoundation(FoundationSubmission{
		Manifest: manifestForReady(ready, artifacts), Artifacts: artifacts,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Result != "ACCEPTED" {
		t.Fatalf("foundation result=%+v", result)
	}
	return project, local, workspace
}

func validChapterArtifacts(chapter int) map[string][]byte {
	plan, _ := json.Marshal(map[string]any{
		"chapter": chapter, "base_canon_root": "__READY_BASE_CANON_ROOT__",
		"objective": "推进本章主线", "reader_payoff": "给读者明确推进结果",
		"beats":              []map[string]any{{"id": "beat-1", "intent": "建立压力"}, {"id": "beat-2", "intent": "行动并形成结果"}},
		"ending_hook_intent": "推出下一章问题",
	})
	review := validChapterReviewMap("pass")
	review["plan_ref"] = "chapter_plan.json"
	reviewRaw, _ := json.Marshal(review)
	return map[string][]byte{
		"chapter.md":            []byte("第一章正文。主角来到起点。"),
		"chapter_contract.json": []byte(`{"chapter":1,"declared_pov":"character-000001"}`),
		"chapter_plan.json":     plan,
		"chapter_review.json":   reviewRaw,
		"events.json":           []byte(`{"events":[{"local_id":"e1","kind":"onscreen","evidence_anchor":"主角来到起点"}]}`),
		"self_review.json":      []byte(`{"ok":true}`),
		"state_delta.json":      []byte(`{"changes":[]}`),
	}
}
func cloneArtifactBytes(input map[string][]byte) map[string][]byte {
	out := make(map[string][]byte, len(input))
	for name, data := range input {
		out[name] = append([]byte(nil), data...)
	}
	return out
}

func writeSubmission(t *testing.T, workspace string, ready readyView, artifacts map[string][]byte, manifestOnly bool) {
	t.Helper()
	inbox := filepath.Join(workspace, "exchange", "inbox", ready.TaskID, ready.AttemptID)
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatal(err)
	}
	if raw, ok := artifacts["chapter_plan.json"]; ok {
		patched := []byte(strings.ReplaceAll(string(raw), "__READY_BASE_CANON_ROOT__", ready.BaseCanonRoot))
		artifacts = cloneArtifactBytes(artifacts)
		artifacts["chapter_plan.json"] = patched
	}
	manifest := manifestForReady(ready, artifacts)
	manifestRaw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if !manifestOnly {
		for name, data := range artifacts {
			if err := os.WriteFile(filepath.Join(inbox, name), data, 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := os.WriteFile(filepath.Join(inbox, "manifest.json"), manifestRaw, 0o644); err != nil {
		t.Fatal(err)
	}
}
func TestSubmissionSnapshotRecoversExistingIdenticalLocalSnapshot(t *testing.T) {
	project, _, workspace := newChapterReadyProject(t)
	project.submissionQuietPeriod = 0
	ready := readReady(t, workspace)
	artifacts := validChapterArtifacts(1)
	writeSubmission(t, workspace, ready, artifacts, false)
	if _, err := project.ScanActiveSubmission(); err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{}
	inbox := filepath.Join(workspace, "exchange", "inbox", ready.TaskID, ready.AttemptID)
	for name := range artifacts {
		files[name], _ = os.ReadFile(filepath.Join(inbox, name))
	}
	files["manifest.json"], _ = os.ReadFile(filepath.Join(inbox, "manifest.json"))
	if err := project.store.SaveCoreSnapshot(ready.AttemptID, files); err != nil {
		t.Fatal(err)
	}
	status, err := project.ScanActiveSubmission()
	if err != nil {
		t.Fatal(err)
	}
	if status.State != "READY_TO_VALIDATE" {
		t.Fatalf("status=%+v", status)
	}
}

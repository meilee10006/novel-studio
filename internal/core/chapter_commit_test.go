package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestChapterSnapshotAcceptedAdvancesCanonAndReady(t *testing.T) {
	project, local, workspace, first := acceptedFoundationProject(t)
	artifacts := validChapterArtifacts(1)
	artifacts["state_delta.json"] = []byte(`{"changes":[{"kind":"resource_add","local_id":"cash","name":"现金","event_ref":"e1"},{"kind":"resource","resource_id":"cash","delta":1,"event_ref":"e1"}]}`)
	writeSubmission(t, workspace, first, artifacts, false)
	project.submissionQuietPeriod = 0
	if _, err := project.ScanActiveSubmission(); err != nil {
		t.Fatal(err)
	}
	status, err := project.ScanActiveSubmission()
	if err != nil {
		t.Fatal(err)
	}
	if status.State != "READY_TO_VALIDATE" {
		t.Fatalf("snapshot state=%+v", status)
	}

	settlement, err := project.SettleActiveSnapshot()
	if err != nil {
		t.Fatalf("SettleActiveSnapshot: %v", err)
	}
	if settlement.Result != "ACCEPTED" || settlement.NewCanonRoot == "" {
		t.Fatalf("settlement=%+v", settlement)
	}
	if settlement.NewCanonRoot == first.BaseCanonRoot {
		t.Fatal("canon root did not advance")
	}
	eventID := mappingID(settlement.IDMappings, "story_event", "e1")
	if eventID == "" {
		t.Fatalf("story event mapping missing: %+v", settlement.IDMappings)
	}
	deltaPath := filepath.Join(local, "meta", "core", "canon", "artifacts", "chapters", "000001", "state_delta.json")
	deltaRaw, err := os.ReadFile(deltaPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(deltaRaw), "event_ref") || !strings.Contains(string(deltaRaw), eventID) {
		t.Fatalf("state delta did not rewrite event ref: %s", deltaRaw)
	}

	next := readReady(t, workspace)
	if next.TaskKind != "chapter" || next.Target != "chapter:2" || next.BaseCanonRoot != settlement.NewCanonRoot {
		t.Fatalf("next READY=%+v", next)
	}
	chapterPath := filepath.Join(local, "meta", "core", "canon", "artifacts", "chapters", "000001", "chapter.md")
	if raw, err := os.ReadFile(chapterPath); err != nil || string(raw) != string(artifacts["chapter.md"]) {
		t.Fatalf("canonical chapter mismatch: err=%v body=%q", err, raw)
	}
	recomputed, err := project.RecomputeCanonRoot()
	if err != nil {
		t.Fatal(err)
	}
	if recomputed != settlement.NewCanonRoot {
		t.Fatalf("recomputed=%q want %q", recomputed, settlement.NewCanonRoot)
	}

	published := filepath.Join(workspace, "published", "chapters", "000001.md")
	if raw, err := os.ReadFile(published); err != nil || string(raw) != string(artifacts["chapter.md"]) {
		t.Fatalf("published chapter mismatch: err=%v body=%q", err, raw)
	}
	if _, err := os.Stat(filepath.Join(local, "meta", "checkpoints.jsonl")); !os.IsNotExist(err) {
		t.Fatalf("provider-free commit wrote legacy checkpoint journal: %v", err)
	}
}

func TestChapterEvidenceAnchorFailureCreatesRewriteWithoutCanonAdvance(t *testing.T) {
	project, _, workspace, first := acceptedFoundationProject(t)
	artifacts := validChapterArtifacts(1)
	artifacts["events.json"] = []byte(`{"events":[{"local_id":"evt-1","kind":"visible","evidence_anchor":"正文中不存在"}]}`)
	writeSubmission(t, workspace, first, artifacts, false)
	project.submissionQuietPeriod = 0
	_, _ = project.ScanActiveSubmission()
	_, _ = project.ScanActiveSubmission()
	settlement, err := project.SettleActiveSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	if settlement.Result != "REWRITE" || settlement.NewCanonRoot != "" {
		t.Fatalf("settlement=%+v", settlement)
	}
	if got := readReady(t, workspace); got.TaskID != first.TaskID || got.AttemptID == first.AttemptID || got.AttemptReason != "rewrite" {
		t.Fatalf("rewrite READY=%+v", got)
	}
	status, err := project.Status()
	if err != nil {
		t.Fatal(err)
	}
	if status.CanonRoot != first.BaseCanonRoot {
		t.Fatalf("canon advanced on rewrite: %q", status.CanonRoot)
	}
}

func TestSettledChapterCannotBeReclaimedByOldDriveAttempt(t *testing.T) {
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

	oldDir := filepath.Join(workspace, "exchange", "inbox", first.TaskID, first.AttemptID)
	if err := os.WriteFile(filepath.Join(oldDir, "chapter.md"), []byte("篡改旧章节"), 0o644); err != nil {
		t.Fatal(err)
	}
	status, err := project.ScanActiveSubmission()
	if err != nil {
		t.Fatal(err)
	}
	if status.AttemptID == first.AttemptID {
		t.Fatalf("old attempt reclaimed active scan: %+v", status)
	}
	current, err := project.Status()
	if err != nil {
		t.Fatal(err)
	}
	if current.CanonRoot != root || current.ActiveTarget != "chapter:2" {
		t.Fatalf("old Drive changed authority: %+v", current)
	}
}

func TestChapterRejectsUnknownDeclaredPOV(t *testing.T) {
	project, _, workspace, ready := acceptedFoundationProject(t)
	before, err := project.Status()
	if err != nil {
		t.Fatal(err)
	}
	artifacts := validChapterArtifacts(1)
	artifacts["chapter_contract.json"] = []byte(`{"chapter":1,"declared_pov":"character-999999"}`)
	writeSubmission(t, workspace, ready, artifacts, false)
	project.submissionQuietPeriod = 0
	_, _ = project.ScanActiveSubmission()
	_, _ = project.ScanActiveSubmission()
	result, err := project.SettleActiveSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	if result.Result != "REWRITE" {
		t.Fatalf("unknown POV result=%+v", result)
	}
	after, err := project.Status()
	if err != nil {
		t.Fatal(err)
	}
	if after.CanonRoot != before.CanonRoot || after.ActiveTarget != "chapter:1" || after.ActiveAttemptID == ready.AttemptID {
		t.Fatalf("unknown POV changed authority incorrectly: before=%+v after=%+v", before, after)
	}
}

func acceptedFoundationProject(t *testing.T) (*Project, string, string, readyView) {
	t.Helper()
	project, local, workspace := newChapterReadyProject(t)
	return project, local, workspace, readReady(t, workspace)
}

func TestChapterCommitCrashRecoveryCompletesAtomically(t *testing.T) {
	stages := []string{"canon", "receipt", "published", "production", "result", "ready"}
	for _, stage := range stages {
		t.Run(stage, func(t *testing.T) {
			project, local, workspace, first := acceptedFoundationProject(t)
			writeSubmission(t, workspace, first, validChapterArtifacts(1), false)
			project.submissionQuietPeriod = 0
			_, _ = project.ScanActiveSubmission()
			_, _ = project.ScanActiveSubmission()
			project.commitFault = func(got string) error {
				if got == stage {
					return fmt.Errorf("simulated crash after %s", stage)
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
				t.Fatalf("recover: %v", err)
			}
			status, err := reopened.Status()
			if err != nil {
				t.Fatal(err)
			}
			if status.ActiveTarget != "chapter:2" || status.CanonRoot == first.BaseCanonRoot {
				t.Fatalf("recovery left partial state: %+v", status)
			}
			root, err := reopened.RecomputeCanonRoot()
			if err != nil || root != status.CanonRoot {
				t.Fatalf("recomputed root=%q status=%q err=%v", root, status.CanonRoot, err)
			}
		})
	}
}

func TestChapterCommitRetryReusesPendingJournal(t *testing.T) {
	project, _, workspace, first := acceptedFoundationProject(t)
	writeSubmission(t, workspace, first, validChapterArtifacts(1), false)
	project.submissionQuietPeriod = 0
	_, _ = project.ScanActiveSubmission()
	_, _ = project.ScanActiveSubmission()
	project.commitFault = func(stage string) error {
		if stage == "receipt" {
			return fmt.Errorf("simulated crash after receipt")
		}
		return nil
	}
	if _, err := project.SettleActiveSnapshot(); err == nil {
		t.Fatal("expected simulated crash")
	}
	project.commitFault = nil
	settlement, err := project.SettleActiveSnapshot()
	if err != nil {
		t.Fatalf("retry settle: %v", err)
	}
	if settlement.Result != "ACCEPTED" {
		t.Fatalf("settlement=%+v", settlement)
	}
	if got := readReady(t, workspace); got.Target != "chapter:2" || got.BaseCanonRoot != settlement.NewCanonRoot {
		t.Fatalf("next READY=%+v", got)
	}
	if _, err := os.Stat(filepath.Join(project.root, "meta", "checkpoints.jsonl")); !os.IsNotExist(err) {
		t.Fatalf("recovered provider-free commit wrote legacy checkpoint journal: %v", err)
	}
}

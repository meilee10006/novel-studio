package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestProtocolInvalidSubmissionCreatesRetryNotBlocked(t *testing.T) {
	project, _, workspace, ready := acceptedFoundationProject(t)
	artifacts := validChapterArtifacts(1)
	writeSubmission(t, workspace, ready, artifacts, false)

	manifestPath := filepath.Join(workspace, "exchange", "inbox", ready.TaskID, ready.AttemptID, "manifest.json")
	var manifest map[string]any
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	manifest["completion_nonce"] = "wrong-nonce"
	raw, _ = json.Marshal(manifest)
	if err := os.WriteFile(manifestPath, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	status, err := project.ScanActiveSubmission()
	if err != nil {
		t.Fatal(err)
	}
	if status.State != "INVALID" {
		t.Fatalf("status=%+v", status)
	}
	before, err := project.Status()
	if err != nil {
		t.Fatal(err)
	}

	result, err := project.RetryInvalidSubmission()
	if err != nil {
		t.Fatal(err)
	}
	if result.Result != "RETRY" {
		t.Fatalf("result=%+v", result)
	}
	next := readReady(t, workspace)
	if next.TaskID != ready.TaskID || next.Target != ready.Target {
		t.Fatalf("retry changed task: before=%+v after=%+v", ready, next)
	}
	if next.AttemptID == ready.AttemptID || next.AttemptReason != "retry" {
		t.Fatalf("retry attempt=%+v", next)
	}
	after, err := project.Status()
	if err != nil {
		t.Fatal(err)
	}
	if after.CanonRoot != before.CanonRoot {
		t.Fatalf("retry advanced canon: %q -> %q", before.CanonRoot, after.CanonRoot)
	}
	if after.BlockID != "" {
		t.Fatalf("protocol retry became blocked: %+v", after)
	}
}

func TestAuthorDecisionRequiredBlocksWithoutCanonAdvance(t *testing.T) {
	project, _, workspace, ready := acceptedFoundationProject(t)
	artifacts := blockedChapterArtifacts()
	writeSubmission(t, workspace, ready, artifacts, false)
	project.submissionQuietPeriod = 0
	_, _ = project.ScanActiveSubmission()
	_, _ = project.ScanActiveSubmission()
	before, _ := project.Status()
	settlement, err := project.SettleActiveSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	if settlement.Result != "BLOCKED" || settlement.BlockID == "" {
		t.Fatalf("settlement=%+v", settlement)
	}
	after, err := project.Status()
	if err != nil {
		t.Fatal(err)
	}
	if after.CanonRoot != before.CanonRoot {
		t.Fatalf("blocked advanced canon: %q -> %q", before.CanonRoot, after.CanonRoot)
	}
	if after.BlockID != settlement.BlockID {
		t.Fatalf("status block=%q settlement=%q", after.BlockID, settlement.BlockID)
	}
	if got := readReady(t, workspace); got.Status != "blocked" || got.BlockID != settlement.BlockID {
		t.Fatalf("blocked READY=%+v", got)
	}
	again, err := project.SettleActiveSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	if again.Result != "BLOCKED" || again.BlockID != settlement.BlockID {
		t.Fatalf("block id not stable: first=%+v again=%+v", settlement, again)
	}
}

func TestUnknownHardConstraintReferenceIsRewriteNotBlocked(t *testing.T) {
	project, _, workspace, ready := acceptedFoundationProject(t)
	artifacts := blockedChapterArtifacts()
	artifacts["self_review.json"] = []byte(`{"ok":false,"author_decision_required":{"constraint_refs":["missing"],"conflict":"二选一","options":["A","B"]}}`)
	writeSubmission(t, workspace, ready, artifacts, false)
	project.submissionQuietPeriod = 0
	_, _ = project.ScanActiveSubmission()
	_, _ = project.ScanActiveSubmission()
	settlement, err := project.SettleActiveSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	if settlement.Result != "REWRITE" || settlement.BlockID != "" {
		t.Fatalf("settlement=%+v", settlement)
	}
}

func blockedChapterArtifacts() map[string][]byte {
	artifacts := validChapterArtifacts(1)
	artifacts["chapter_contract.json"] = []byte(`{
		"chapter":1,
		"declared_pov":"character-000001",
		"hard_constraints":[{"id":"hc-choice"}]
	}`)
	artifacts["self_review.json"] = []byte(`{
		"ok":false,
		"author_decision_required":{
			"constraint_refs":["hc-choice"],
			"conflict":"两个硬约束无法同时满足",
			"options":["A","B"]
		}
	}`)
	return artifacts
}

func TestUnknownSubmissionSchemaCreatesRetryNotBlocked(t *testing.T) {
	project, _, workspace, ready := acceptedFoundationProject(t)
	artifacts := validChapterArtifacts(1)
	writeSubmission(t, workspace, ready, artifacts, false)
	manifestPath := filepath.Join(workspace, "exchange", "inbox", ready.TaskID, ready.AttemptID, "manifest.json")
	var manifest map[string]any
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	manifest["schema_version"] = 999
	raw, _ = json.Marshal(manifest)
	if err := os.WriteFile(manifestPath, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	status, err := project.ScanActiveSubmission()
	if err != nil {
		t.Fatal(err)
	}
	if status.State != "INVALID" {
		t.Fatalf("status=%+v", status)
	}
	result, err := project.RetryInvalidSubmission()
	if err != nil {
		t.Fatal(err)
	}
	if result.Result != "RETRY" {
		t.Fatalf("result=%+v", result)
	}
	if got := readReady(t, workspace); got.AttemptReason != "retry" || got.BlockID != "" {
		t.Fatalf("retry READY=%+v", got)
	}
}

package core

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/chenhongyang/novel-studio/internal/domain"
)

func TestRewriteFeedbackIsStructuredAndKeepsLegacyViolations(t *testing.T) {
	project, _, workspace, ready := acceptedFoundationProject(t)
	artifacts := longformChapterArtifacts(1, nil)
	artifacts["chapter_contract.json"] = []byte(`{"chapter":1,"declared_pov":"character-000001","hard_constraints":"hc-a"}`)

	got := submitAndSettleChapter(t, project, workspace, ready, artifacts)
	if got.Result != "REWRITE" || len(got.Violations) == 0 {
		t.Fatalf("settlement=%+v", got)
	}
	assertStructuredRewriteFeedback(t, got.Violations, got.RewriteFeedback)
	if item := got.RewriteFeedback[0]; item.Code != "chapter_contract.hard_constraints.invalid" || item.EvidenceRefs[0] != "chapter_contract.json" || item.AllowedScope[0] != "chapter_contract.json" {
		t.Fatalf("chapter feedback classification=%+v", item)
	}

	var result struct {
		Violations      []string          `json:"violations"`
		RewriteFeedback []RewriteFeedback `json:"rewrite_feedback"`
	}
	readJSONFile(t, filepath.Join(workspace, "exchange", "result", ready.AttemptID+".json"), &result)
	if len(result.Violations) != len(got.Violations) {
		t.Fatalf("workspace violations=%v settlement=%v", result.Violations, got.Violations)
	}
	assertStructuredRewriteFeedback(t, result.Violations, result.RewriteFeedback)

	var receipt struct {
		ValidationDigest string `json:"validation_digest"`
	}
	readJSONFile(t, got.ReceiptPath, &receipt)
	wantDigest, err := digestJSON(map[string]any{
		"result": "REWRITE", "violations": got.Violations, "rewrite_feedback": got.RewriteFeedback, "planning_status": "",
	})
	if err != nil {
		t.Fatal(err)
	}
	if receipt.ValidationDigest != wantDigest {
		t.Fatalf("validation digest=%q want %q", receipt.ValidationDigest, wantDigest)
	}
}

func TestFoundationRewriteFeedbackUsesSameStructuredShape(t *testing.T) {
	project, _, workspace := newCapabilityPassedProject(t)
	if err := project.Reconcile(); err != nil {
		t.Fatal(err)
	}
	ready := readReady(t, workspace)
	artifacts := validFoundationArtifacts()
	artifacts["characters.json"] = []byte(`{"characters":"hero"}`)

	got, err := project.SettleFoundation(FoundationSubmission{
		Manifest:  manifestForReady(ready, artifacts),
		Artifacts: artifacts,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Result != "REWRITE" || len(got.Violations) == 0 {
		t.Fatalf("settlement=%+v", got)
	}
	assertStructuredRewriteFeedback(t, got.Violations, got.RewriteFeedback)
	if item := got.RewriteFeedback[0]; item.Code != "foundation.characters.invalid" || item.EvidenceRefs[0] != "characters.json" || item.AllowedScope[0] != "characters.json" {
		t.Fatalf("foundation feedback classification=%+v", item)
	}

	var result struct {
		Violations      []string          `json:"violations"`
		RewriteFeedback []RewriteFeedback `json:"rewrite_feedback"`
	}
	readJSONFile(t, filepath.Join(workspace, "exchange", "result", ready.AttemptID+".json"), &result)
	assertStructuredRewriteFeedback(t, result.Violations, result.RewriteFeedback)

	var receipt struct {
		ValidationDigest string `json:"validation_digest"`
	}
	readJSONFile(t, got.ReceiptPath, &receipt)
	wantDigest, err := digestJSON(map[string]any{
		"result": "REWRITE", "violations": got.Violations, "rewrite_feedback": got.RewriteFeedback,
	})
	if err != nil {
		t.Fatal(err)
	}
	if receipt.ValidationDigest != wantDigest {
		t.Fatalf("validation digest=%q want %q", receipt.ValidationDigest, wantDigest)
	}
}

func TestFoundationRewriteFeedbackKeepsFoundationArtifactScope(t *testing.T) {
	project, _, workspace := newCapabilityPassedProject(t)
	if err := project.Reconcile(); err != nil {
		t.Fatal(err)
	}
	ready := readReady(t, workspace)
	artifacts := validFoundationArtifacts()
	artifacts["foundation.json"] = []byte(`{"title":"测试书","protagonist":{"entity_type":"character","local_ref":"same"},"opening_location":{"entity_type":"location","local_ref":""}}`)

	got, err := project.SettleFoundation(FoundationSubmission{
		Manifest:  manifestForReady(ready, artifacts),
		Artifacts: artifacts,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Result != "REWRITE" {
		t.Fatalf("settlement=%+v", got)
	}
	for _, item := range got.RewriteFeedback {
		if strings.Contains(item.Observed, "foundation.opening_location") {
			if item.Code != "foundation.invalid" {
				t.Fatalf("feedback code=%q want foundation.invalid: %+v", item.Code, item)
			}
			if len(item.EvidenceRefs) != 1 || item.EvidenceRefs[0] != "foundation.json" || len(item.AllowedScope) != 1 || item.AllowedScope[0] != "foundation.json" {
				t.Fatalf("feedback scope=%+v", item)
			}
			return
		}
	}
	t.Fatalf("missing foundation.opening_location feedback: %+v", got.RewriteFeedback)
}

func TestRewriteFeedbackClassifiesExistingDeterministicViolations(t *testing.T) {
	task := &domain.CoreTask{Kind: "chapter", Target: "chapter:1"}
	tests := []struct {
		name, violation, wantCode, wantArtifact string
	}{
		{"travel constraint", "modeled travel constraint requires at least 3 ticks: x -> y", "state_delta.invalid", "state_delta.json"},
		{"state delta object", "state_delta.json must be an object", "state_delta.invalid", "state_delta.json"},
		{"state change object", "state change must be an object", "state_delta.invalid", "state_delta.json"},
		{"character add", "character_add requires canon_id and name", "state_delta.invalid", "state_delta.json"},
		{"current canonical ref", "current attempt canonical character id must be referenced by local id: character-000001", "state_delta.invalid", "state_delta.json"},
		{"offscreen hard constraint ref", "offscreen story event references unknown hard constraint: hc-a", "events.invalid", "events.json"},
		{"author decision", "author decision requires at least two options", "self_review.invalid", "self_review.json"},
		{"author decision hard constraint ref", "author decision references unknown hard constraint: hc-a", "self_review.invalid", "self_review.json"},
		{"planning arc", "arc chapter range is invalid", "planning.invalid", "planning_patch.json"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			feedback := buildRewriteFeedback([]string{tc.violation}, task)
			if len(feedback) != 1 {
				t.Fatalf("feedback=%+v", feedback)
			}
			item := feedback[0]
			if item.Code != tc.wantCode || len(item.EvidenceRefs) != 1 || item.EvidenceRefs[0] != tc.wantArtifact || len(item.AllowedScope) != 1 || item.AllowedScope[0] != tc.wantArtifact {
				t.Fatalf("feedback=%+v want code=%q artifact=%q", item, tc.wantCode, tc.wantArtifact)
			}
		})
	}
}

func assertStructuredRewriteFeedback(t *testing.T, violations []string, feedback []RewriteFeedback) {
	t.Helper()
	if len(feedback) != len(violations) {
		t.Fatalf("feedback=%+v violations=%v", feedback, violations)
	}
	for i, item := range feedback {
		if item.Code == "" || item.Expected == "" || item.Observed == "" || len(item.EvidenceRefs) == 0 || len(item.AllowedScope) == 0 {
			t.Fatalf("feedback[%d]=%+v", i, item)
		}
		if item.Observed != violations[i] {
			t.Fatalf("feedback[%d].observed=%q violation=%q", i, item.Observed, violations[i])
		}
		if !uniqueNonEmptyStrings(item.EvidenceRefs) || !uniqueNonEmptyStrings(item.AllowedScope) {
			t.Fatalf("feedback[%d] has invalid refs/scope: %+v", i, item)
		}
	}
}

func uniqueNonEmptyStrings(items []string) bool {
	seen := map[string]bool{}
	for _, item := range items {
		if item == "" || seen[item] {
			return false
		}
		seen[item] = true
	}
	return true
}

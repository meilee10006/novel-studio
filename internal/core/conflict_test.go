package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConflictGetsCanonicalIDAndLifecycle(t *testing.T) {
	project, workspace, firstID, secondID, _ := projectWithTwoCharacters(t)
	ready := readReady(t, workspace)
	first := longformChapterArtifacts(1, []map[string]any{{
		"kind": "conflict", "local_id": "rivalry", "description": "甲乙争夺同一把钥匙",
		"participants": []string{firstID, secondID}, "state": "open",
		"escalation_condition": "双方公开争夺钥匙", "close_condition": "钥匙归属明确且双方停止争夺", "event_ref": "e1",
	}})
	settled := submitAndSettleChapter(t, project, workspace, ready, first)
	if settled.Result != "ACCEPTED" {
		t.Fatalf("conflict create settlement=%+v", settled)
	}
	conflictID := mappingID(settled.IDMappings, "conflict", "rivalry")
	if conflictID == "" {
		t.Fatalf("conflict mapping missing: %+v", settled.IDMappings)
	}
	state := readCanonState(t, project)
	if got := state.Longform.Conflicts[conflictID].State; got != "open" {
		t.Fatalf("conflict state=%q conflicts=%+v", got, state.Longform.Conflicts)
	}

	next := readReady(t, workspace)
	canonPath := filepath.Join(workspace, "exchange", "outbox", next.TaskID, next.AttemptID, "canon_excerpt.json")
	canonRaw, err := os.ReadFile(canonPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{conflictID, "甲乙争夺同一把钥匙", "open"} {
		if !strings.Contains(string(canonRaw), want) {
			t.Fatalf("next task lost conflict %q: %s", want, canonRaw)
		}
	}

	escalate := longformChapterArtifacts(2, []map[string]any{{
		"kind": "conflict", "conflict_id": conflictID, "state": "escalated", "event_ref": "e1",
	}})
	if got := submitAndSettleChapter(t, project, workspace, next, escalate); got.Result != "ACCEPTED" {
		t.Fatalf("conflict escalation=%+v", got)
	}
	next = readReady(t, workspace)
	resolve := longformChapterArtifacts(3, []map[string]any{{
		"kind": "conflict", "conflict_id": conflictID, "state": "resolved", "event_ref": "e1",
	}})
	if got := submitAndSettleChapter(t, project, workspace, next, resolve); got.Result != "ACCEPTED" {
		t.Fatalf("conflict resolution=%+v", got)
	}
	state = readCanonState(t, project)
	if got := state.Longform.Conflicts[conflictID].State; got != "resolved" {
		t.Fatalf("resolved conflict state=%q conflicts=%+v", got, state.Longform.Conflicts)
	}

	next = readReady(t, workspace)
	rogue := longformChapterArtifacts(4, []map[string]any{{
		"kind": "conflict", "conflict_id": "conflict-999999", "state": "escalated", "event_ref": "e1",
	}})
	got := submitAndSettleChapter(t, project, workspace, next, rogue)
	if got.Result != "REWRITE" || !containsViolation(got.Violations, "conflict") {
		t.Fatalf("unknown conflict transition=%+v", got)
	}
}

func TestConflictDefinitionFieldsCannotChangeAfterCreation(t *testing.T) {
	for _, tc := range []struct {
		name  string
		field string
		value string
	}{
		{"description", "description", "甲乙改为争夺另一把钥匙"},
		{"escalation", "escalation_condition", "见面即升级"},
		{"close condition", "close_condition", "任意一方离开即可关闭"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			project, workspace, firstID, secondID, _ := projectWithTwoCharacters(t)
			ready := readReady(t, workspace)
			first := longformChapterArtifacts(1, []map[string]any{{
				"kind": "conflict", "local_id": "rivalry", "description": "甲乙争夺同一把钥匙",
				"participants": []string{firstID, secondID}, "state": "open",
				"escalation_condition": "双方公开争夺钥匙", "close_condition": "钥匙归属明确", "event_ref": "e1",
			}})
			settled := submitAndSettleChapter(t, project, workspace, ready, first)
			if settled.Result != "ACCEPTED" {
				t.Fatalf("create=%+v", settled)
			}
			conflictID := mappingID(settled.IDMappings, "conflict", "rivalry")
			next := readReady(t, workspace)
			change := map[string]any{"kind": "conflict", "conflict_id": conflictID, "state": "escalated", "event_ref": "e1", tc.field: tc.value}
			got := submitAndSettleChapter(t, project, workspace, next, longformChapterArtifacts(2, []map[string]any{change}))
			if got.Result != "REWRITE" || !containsViolation(got.Violations, "conflict") {
				t.Fatalf("mutation=%+v", got)
			}
		})
	}
}

func TestConflictRejectsUnknownParticipantAndIllegalTransition(t *testing.T) {
	project, workspace, firstID, _, _ := projectWithTwoCharacters(t)
	ready := readReady(t, workspace)
	unknown := longformChapterArtifacts(1, []map[string]any{{
		"kind": "conflict", "local_id": "bad", "description": "坏冲突",
		"participants": []string{firstID, "character-999999"}, "state": "open",
		"escalation_condition": "升级", "close_condition": "关闭", "event_ref": "e1",
	}})
	if got := submitAndSettleChapter(t, project, workspace, ready, unknown); got.Result != "REWRITE" || !containsViolation(got.Violations, "participant") {
		t.Fatalf("unknown participant=%+v", got)
	}

	ready = readReady(t, workspace)
	created := longformChapterArtifacts(1, []map[string]any{{
		"kind": "conflict", "local_id": "direct", "description": "直接跳终态",
		"participants": []string{firstID}, "state": "resolved",
		"escalation_condition": "升级", "close_condition": "关闭", "event_ref": "e1",
	}})
	got := submitAndSettleChapter(t, project, workspace, ready, created)
	if got.Result != "REWRITE" || !containsViolation(got.Violations, "transition") {
		t.Fatalf("illegal initial conflict state=%+v", got)
	}
}

func TestFinalExportRequiresAllConflictsResolved(t *testing.T) {
	project, workspace, firstID, secondID, _ := projectWithTwoCharacters(t)
	ready := readReady(t, workspace)
	first := longformChapterArtifacts(1, []map[string]any{
		{
			"kind": "conflict", "local_id": "rivalry", "description": "甲乙争夺同一把钥匙",
			"participants": []string{firstID, secondID}, "state": "open",
			"escalation_condition": "双方公开争夺钥匙", "close_condition": "钥匙归属明确且双方停止争夺", "event_ref": "e1",
		},
		{"kind": "ending_resolution", "event_ref": "e1"},
	})
	settled := submitAndSettleChapter(t, project, workspace, ready, first)
	if settled.Result != "ACCEPTED" {
		t.Fatalf("chapter 1=%+v", settled)
	}
	conflictID := mappingID(settled.IDMappings, "conflict", "rivalry")
	out := filepath.Join(t.TempDir(), "book.md")
	if _, err := project.ExportBook(out); err == nil || !strings.Contains(strings.ToLower(err.Error()), "conflict") {
		t.Fatalf("open conflict export err=%v", err)
	}

	ready = readReady(t, workspace)
	second := longformChapterArtifacts(2, []map[string]any{
		{"kind": "conflict", "conflict_id": conflictID, "state": "escalated", "event_ref": "e1"},
		{"kind": "ending_resolution", "event_ref": "e1"},
	})
	if got := submitAndSettleChapter(t, project, workspace, ready, second); got.Result != "ACCEPTED" {
		t.Fatalf("chapter 2=%+v", got)
	}
	if _, err := project.ExportBook(out); err == nil || !strings.Contains(strings.ToLower(err.Error()), "conflict") {
		t.Fatalf("escalated conflict export err=%v", err)
	}

	ready = readReady(t, workspace)
	third := longformChapterArtifacts(3, []map[string]any{
		{"kind": "conflict", "conflict_id": conflictID, "state": "resolved", "event_ref": "e1"},
		{"kind": "ending_resolution", "event_ref": "e1"},
	})
	if got := submitAndSettleChapter(t, project, workspace, ready, third); got.Result != "ACCEPTED" {
		t.Fatalf("chapter 3=%+v", got)
	}
	if _, err := project.ExportBook(out); err != nil {
		t.Fatalf("resolved conflict export: %v", err)
	}
}
